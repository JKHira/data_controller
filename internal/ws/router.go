package ws

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/trade-engine/data-controller/pkg/schema"
	"go.uber.org/zap"
)

type Router struct {
	logger       *zap.Logger
	tickerChan   chan *schema.Ticker
	tradesChan   chan *schema.Trade
	booksChan    chan *schema.BookLevel
	rawBooksChan chan *schema.RawBookEvent
	candlesChan  chan *schema.Candle
	controlsChan chan *schema.Control
}

type MessageHandler interface {
	HandleTicker(ticker *schema.Ticker)
	HandleTrade(trade *schema.Trade)
	HandleBookLevel(level *schema.BookLevel)
	HandleRawBookEvent(event *schema.RawBookEvent)
	HandleCandle(candle *schema.Candle)
	HandleControl(control *schema.Control)
}

func NewRouter(logger *zap.Logger) *Router {
	return &Router{
		logger:       logger,
		tickerChan:   make(chan *schema.Ticker, 10000),
		tradesChan:   make(chan *schema.Trade, 10000),
		booksChan:    make(chan *schema.BookLevel, 10000),
		rawBooksChan: make(chan *schema.RawBookEvent, 10000),
		candlesChan:  make(chan *schema.Candle, 10000),
		controlsChan: make(chan *schema.Control, 1000),
	}
}

func (r *Router) SetHandler(handler MessageHandler) {
	go func() {
		for ticker := range r.tickerChan {
			handler.HandleTicker(ticker)
		}
	}()

	go func() {
		for trade := range r.tradesChan {
			handler.HandleTrade(trade)
		}
	}()

	go func() {
		for level := range r.booksChan {
			handler.HandleBookLevel(level)
		}
	}()

	go func() {
		for event := range r.rawBooksChan {
			handler.HandleRawBookEvent(event)
		}
	}()

	go func() {
		for candle := range r.candlesChan {
			handler.HandleCandle(candle)
		}
	}()

	go func() {
		for control := range r.controlsChan {
			handler.HandleControl(control)
		}
	}()
}

func (r *Router) RouteMessage(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string) error {
	return r.RouteMessageWithSeq(chanID, channelInfo, data, connID, nil)
}

func (r *Router) RouteMessageWithSeq(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, seq *int64) error {
	recvTS := time.Now().UnixNano()

	switch channelInfo.Channel {
	case "ticker":
		return r.routeTicker(chanID, channelInfo, data, connID, recvTS, seq)
	case "trades":
		return r.routeTrades(chanID, channelInfo, data, connID, recvTS, seq)
	case "book":
		// Check if this is raw books (R0 precision)
		if channelInfo.SubReq.Prec != nil && *channelInfo.SubReq.Prec == "R0" {
			return r.routeRawBooks(chanID, channelInfo, data, connID, recvTS, seq)
		}
		return r.routeBooks(chanID, channelInfo, data, connID, recvTS, seq)
	case "candles":
		return r.routeCandles(chanID, channelInfo, data, connID, recvTS, seq)
	default:
		r.logger.Warn("Unknown channel type", zap.String("channel", channelInfo.Channel))
	}

	return nil
}

func (r *Router) routeTicker(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64, seq *int64) error {
	// Bitfinex ticker message format: [CHANNEL_ID, [ticker_array], TIMESTAMP]
	// So data[0] contains the ticker array with 10 values
	if len(data) < 1 {
		r.logger.Warn("Ticker data too short", zap.Int("length", len(data)))
		return nil
	}

	// Parse the nested ticker array
	var tickerArray []float64
	if err := json.Unmarshal(data[0], &tickerArray); err != nil {
		r.logger.Error("Failed to unmarshal ticker array", zap.Error(err))
		return err
	}

	if len(tickerArray) < 10 {
		r.logger.Warn("Ticker array too short", zap.Int("length", len(tickerArray)))
		return nil
	}

	var values [10]float64
	for i := 0; i < 10; i++ {
		values[i] = tickerArray[i]
	}

	ticker := &schema.Ticker{
		CommonFields: schema.CommonFields{
			Symbol:         channelInfo.Symbol,
			PairOrCurrency: channelInfo.Pair,
			Seq:            seq,
			RecvTS:         recvTS,
			ChanID:         chanID,
			Channel:        schema.ChannelTicker,
		},
		Bid:            values[0],
		BidSize:        values[1],
		Ask:            values[2],
		AskSize:        values[3],
		DailyChange:    values[4],
		DailyChangeRel: values[5],
		Last:           values[6],
		Vol:            values[7],
		High:           values[8],
		Low:            values[9],
	}

	select {
	case r.tickerChan <- ticker:
	default:
		r.logger.Warn("Ticker channel full, dropping message")
	}

	return nil
}

func (r *Router) routeTrades(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64, seq *int64) error {
	var msgType string
	var tradeData []json.RawMessage
	isSnapshot := false

	r.logger.Debug("routeTrades called", zap.Int("data_len", len(data)))

	if len(data) >= 1 {
		// Check if it's a snapshot (array of trade arrays)
		var testArray []json.RawMessage
		if err := json.Unmarshal(data[0], &testArray); err == nil {
			isSnapshot = true
			r.logger.Debug("Snapshot detected", zap.Int("trades_count", len(testArray)))
			for _, item := range testArray {
				var singleTrade [4]json.RawMessage
				if err := json.Unmarshal(item, &singleTrade); err != nil {
					r.logger.Warn("Failed to unmarshal trade", zap.Error(err))
					continue
				}
				r.processSingleTrade(chanID, channelInfo, singleTrade[:], connID, recvTS, isSnapshot, "snapshot", seq)
			}
			return nil
		}

		// Check for message type (te/tu)
		if len(data) >= 2 {
			if err := json.Unmarshal(data[0], &msgType); err == nil {
				if msgType == "te" || msgType == "tu" {
					tradeData = data[1:]
				}
			} else {
				msgType = "unknown"
				tradeData = data
			}
		}
	}

	if len(tradeData) >= 4 {
		return r.processSingleTrade(chanID, channelInfo, tradeData, connID, recvTS, isSnapshot, msgType, seq)
	}

	return nil
}

func (r *Router) processSingleTrade(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64, isSnapshot bool, msgType string, seq *int64) error {
	r.logger.Debug("Processing single trade",
		zap.Int("data_len", len(data)),
		zap.Bool("is_snapshot", isSnapshot),
		zap.String("msg_type", msgType))

	if len(data) < 4 {
		r.logger.Warn("Trade data too short", zap.Int("length", len(data)))
		return nil
	}

	var tradeID int64
	var mts int64
	var amount float64
	var price float64

	if err := json.Unmarshal(data[0], &tradeID); err != nil {
		return err
	}
	if err := json.Unmarshal(data[1], &mts); err != nil {
		return err
	}
	if err := json.Unmarshal(data[2], &amount); err != nil {
		return err
	}
	if err := json.Unmarshal(data[3], &price); err != nil {
		return err
	}

	trade := &schema.Trade{
		CommonFields: schema.CommonFields{
			Symbol:         channelInfo.Symbol,
			PairOrCurrency: channelInfo.Pair,
			Seq:            seq,
			RecvTS:         recvTS,
			ChanID:         chanID,
			Channel:        schema.ChannelTrades,
		},
		TradeID:    tradeID,
		MTS:        mts,
		Amount:     amount,
		Price:      price,
		MsgType:    schema.MessageType(msgType),
		IsSnapshot: isSnapshot,
	}

	r.logger.Debug("Sending trade to channel",
		zap.String("symbol", trade.Symbol),
		zap.Int64("trade_id", trade.TradeID))

	select {
	case r.tradesChan <- trade:
		r.logger.Debug("Trade sent successfully", zap.Int64("trade_id", trade.TradeID))
	default:
		r.logger.Warn("Trades channel full, dropping message")
	}

	return nil
}

func (r *Router) routeBooks(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64, seq *int64) error {
	var isSnapshot bool
	var bookData []json.RawMessage

	if len(data) >= 1 {
		var testArray []json.RawMessage
		if err := json.Unmarshal(data[0], &testArray); err == nil {
			isSnapshot = true
			for _, item := range testArray {
				var singleLevel [3]json.RawMessage
				if err := json.Unmarshal(item, &singleLevel); err != nil {
					continue
				}
				r.processSingleBookLevel(chanID, channelInfo, singleLevel[:], connID, recvTS, isSnapshot, seq)
			}
			return nil
		}
		bookData = data
	}

	if len(bookData) >= 3 {
		return r.processSingleBookLevel(chanID, channelInfo, bookData, connID, recvTS, isSnapshot, seq)
	}

	return nil
}

func (r *Router) processSingleBookLevel(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64, isSnapshot bool, seq *int64) error {
	if len(data) < 3 {
		return nil
	}

	var price float64
	var count int32
	var amount float64

	if err := json.Unmarshal(data[0], &price); err != nil {
		return err
	}
	if err := json.Unmarshal(data[1], &count); err != nil {
		return err
	}
	if err := json.Unmarshal(data[2], &amount); err != nil {
		return err
	}

	side := schema.SideBid
	if amount < 0 {
		side = schema.SideAsk
	}

	subReq := getSubRequestBySubID(*channelInfo.SubID)
	prec := "P0"
	freq := "F0"
	length := int32(25)

	if subReq != nil {
		if subReq.Prec != nil {
			prec = *subReq.Prec
		}
		if subReq.Freq != nil {
			freq = *subReq.Freq
		}
		if subReq.Len != nil {
			if len := parseIntFromString(*subReq.Len); len > 0 {
				length = int32(len)
			}
		}
	}

	level := &schema.BookLevel{
		CommonFields: schema.CommonFields{
			Symbol:         channelInfo.Symbol,
			PairOrCurrency: channelInfo.Pair,
			Seq:            seq,
			RecvTS:         recvTS,
			ChanID:         chanID,
			Channel:        schema.ChannelBooks,
			BookPrec:       prec,
			BookFreq:       freq,
			BookLen:        fmt.Sprintf("%d", length),
		},
		Price:      price,
		Count:      count,
		Amount:     amount,
		Side:       side,
		Prec:       prec,
		Freq:       freq,
		Len:        length,
		IsSnapshot: isSnapshot,
	}

	select {
	case r.booksChan <- level:
	default:
		r.logger.Warn("Books channel full, dropping message")
	}

	return nil
}

func (r *Router) routeRawBooks(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64, seq *int64) error {
	var isSnapshot bool
	var bookData []json.RawMessage

	if len(data) >= 1 {
		var testArray []json.RawMessage
		if err := json.Unmarshal(data[0], &testArray); err == nil {
			isSnapshot = true
			for _, item := range testArray {
				var singleOrder [3]json.RawMessage
				if err := json.Unmarshal(item, &singleOrder); err != nil {
					continue
				}
				r.processSingleRawBookEvent(chanID, channelInfo, singleOrder[:], connID, recvTS, isSnapshot, seq)
			}
			return nil
		}
		bookData = data
	}

	if len(bookData) >= 3 {
		return r.processSingleRawBookEvent(chanID, channelInfo, bookData, connID, recvTS, isSnapshot, seq)
	}

	return nil
}

func (r *Router) processSingleRawBookEvent(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64, isSnapshot bool, seq *int64) error {
	if len(data) < 3 {
		return nil
	}

	var orderID int64
	var price float64
	var amount float64

	if err := json.Unmarshal(data[0], &orderID); err != nil {
		return err
	}
	if err := json.Unmarshal(data[1], &price); err != nil {
		return err
	}
	if err := json.Unmarshal(data[2], &amount); err != nil {
		return err
	}

	op := schema.OperationUpsert
	if price == 0 {
		op = schema.OperationDelete
	}

	side := schema.SideBid
	if amount < 0 {
		side = schema.SideAsk
	}

	event := &schema.RawBookEvent{
		CommonFields: schema.CommonFields{
			Symbol:         channelInfo.Symbol,
			PairOrCurrency: channelInfo.Pair,
			Seq:            seq,
			RecvTS:         recvTS,
			ChanID:         chanID,
			Channel:        schema.ChannelRawBooks,
			BookPrec:       derefString(channelInfo.SubReq.Prec),
			BookFreq:       derefString(channelInfo.SubReq.Freq),
			BookLen:        derefString(channelInfo.SubReq.Len),
		},
		OrderID:    orderID,
		Price:      price,
		Amount:     amount,
		Op:         op,
		Side:       side,
		IsSnapshot: isSnapshot,
	}

	select {
	case r.rawBooksChan <- event:
	default:
		r.logger.Warn("Raw books channel full, dropping message")
	}

	return nil
}

var globalSubRequests []SubscribeRequest

func setGlobalSubRequests(requests []SubscribeRequest) {
	globalSubRequests = requests
}

func getSubRequestBySubID(subID int64) *SubscribeRequest {
	for _, req := range globalSubRequests {
		if req.SubID != nil && *req.SubID == subID {
			return &req
		}
	}
	return nil
}

func parseIntFromString(s string) int {
	switch s {
	case "1":
		return 1
	case "25":
		return 25
	case "100":
		return 100
	case "250":
		return 250
	default:
		return 0
	}
}

func (r *Router) routeCandles(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64, seq *int64) error {
	// Bitfinex candles message format: [CHANNEL_ID, [MTS, OPEN, CLOSE, HIGH, LOW, VOLUME]]
	// data[0] contains the candle array with 6 values
	if len(data) < 1 {
		r.logger.Warn("Candles data too short", zap.Int("length", len(data)))
		return nil
	}

	// Check if this is a snapshot (array of candles) or single update
	var testArray []json.RawMessage
	if err := json.Unmarshal(data[0], &testArray); err == nil && len(testArray) > 0 {
		// Snapshot: array of candle arrays
		for _, item := range testArray {
			if err := r.processSingleCandle(chanID, channelInfo, item, connID, recvTS, true, seq); err != nil {
				r.logger.Error("Failed to process candle in snapshot", zap.Error(err))
			}
		}
		return nil
	}

	// Single candle update
	return r.processSingleCandle(chanID, channelInfo, data[0], connID, recvTS, false, seq)
}

func (r *Router) processSingleCandle(chanID int32, channelInfo *ChannelInfo, candleData json.RawMessage, connID string, recvTS int64, isSnapshot bool, seq *int64) error {
	// Parse candle array [MTS, OPEN, CLOSE, HIGH, LOW, VOLUME]
	var mts int64
	var open, close, high, low, volume float64

	var candleArray [6]json.RawMessage
	if err := json.Unmarshal(candleData, &candleArray); err != nil {
		r.logger.Error("Failed to parse candle array", zap.Error(err))
		return err
	}

	if err := json.Unmarshal(candleArray[0], &mts); err != nil {
		return err
	}
	if err := json.Unmarshal(candleArray[1], &open); err != nil {
		return err
	}
	if err := json.Unmarshal(candleArray[2], &close); err != nil {
		return err
	}
	if err := json.Unmarshal(candleArray[3], &high); err != nil {
		return err
	}
	if err := json.Unmarshal(candleArray[4], &low); err != nil {
		return err
	}
	if err := json.Unmarshal(candleArray[5], &volume); err != nil {
		return err
	}

	// Extract timeframe from channelInfo.SubReq.Key (format: "trade:1m:tBTCUSD")
	timeframe := ""
	if channelInfo.SubReq.Key != "" {
		parts := strings.Split(channelInfo.SubReq.Key, ":")
		if len(parts) >= 2 {
			timeframe = parts[1]
		}
	}

	key := deriveChannelKey(schema.ChannelCandles, channelInfo)
	candle := &schema.Candle{
		CommonFields: schema.CommonFields{
			Symbol:         channelInfo.Symbol,
			PairOrCurrency: channelInfo.Pair,
			Seq:            seq,
			RecvTS:         recvTS / 1000,
			ChanID:         chanID,
			Channel:        schema.ChannelCandles,
			ChannelKey:     key,
			Timeframe:      timeframe,
		},
		MTS:        mts,
		Open:       open,
		Close:      close,
		High:       high,
		Low:        low,
		Volume:     volume,
		IsSnapshot: isSnapshot,
	}

	select {
	case r.candlesChan <- candle:
	default:
		r.logger.Warn("Candles channel full, dropping message")
	}

	return nil
}

func deriveChannelKey(channel schema.Channel, info *ChannelInfo) string {
	if info == nil {
		return ""
	}
	if channel == schema.ChannelCandles {
		return info.SubReq.Key
	}
	return ""
}

func derefString(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

func (r *Router) Close() {
	close(r.tickerChan)
	close(r.tradesChan)
	close(r.booksChan)
	close(r.rawBooksChan)
	close(r.candlesChan)
	close(r.controlsChan)
}
