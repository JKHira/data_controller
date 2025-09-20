package ws

import (
	"encoding/json"
	"time"

	"go.uber.org/zap"
	"github.com/trade-engine/data-controller/pkg/schema"
)

type Router struct {
	logger        *zap.Logger
	tickerChan    chan *schema.Ticker
	tradesChan    chan *schema.Trade
	booksChan     chan *schema.BookLevel
	rawBooksChan  chan *schema.RawBookEvent
	controlsChan  chan *schema.Control
}

type MessageHandler interface {
	HandleTicker(ticker *schema.Ticker)
	HandleTrade(trade *schema.Trade)
	HandleBookLevel(level *schema.BookLevel)
	HandleRawBookEvent(event *schema.RawBookEvent)
	HandleControl(control *schema.Control)
}

func NewRouter(logger *zap.Logger) *Router {
	return &Router{
		logger:       logger,
		tickerChan:   make(chan *schema.Ticker, 10000),
		tradesChan:   make(chan *schema.Trade, 10000),
		booksChan:    make(chan *schema.BookLevel, 10000),
		rawBooksChan: make(chan *schema.RawBookEvent, 10000),
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
		for control := range r.controlsChan {
			handler.HandleControl(control)
		}
	}()
}

func (r *Router) RouteMessage(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string) error {
	recvTS := time.Now().UnixNano()

	switch channelInfo.Channel {
	case "ticker":
		return r.routeTicker(chanID, channelInfo, data, connID, recvTS)
	case "trades":
		return r.routeTrades(chanID, channelInfo, data, connID, recvTS)
	case "book":
		if channelInfo.SubID != nil {
			subReq := getSubRequestBySubID(*channelInfo.SubID)
			if subReq != nil && subReq.Prec != nil && *subReq.Prec == "R0" {
				return r.routeRawBooks(chanID, channelInfo, data, connID, recvTS)
			}
		}
		return r.routeBooks(chanID, channelInfo, data, connID, recvTS)
	default:
		r.logger.Warn("Unknown channel type", zap.String("channel", channelInfo.Channel))
	}

	return nil
}

func (r *Router) routeTicker(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64) error {
	if len(data) < 10 {
		r.logger.Warn("Ticker data too short", zap.Int("length", len(data)))
		return nil
	}

	var values [10]float64
	for i := 0; i < 10; i++ {
		if err := json.Unmarshal(data[i], &values[i]); err != nil {
			r.logger.Error("Failed to unmarshal ticker value", zap.Int("index", i), zap.Error(err))
			return err
		}
	}

	ticker := &schema.Ticker{
		CommonFields: schema.CommonFields{
			Exchange:       schema.ExchangeBitfinex,
			Channel:        schema.ChannelTicker,
			Symbol:         channelInfo.Symbol,
			PairOrCurrency: channelInfo.Pair,
			ConnID:         connID,
			ChanID:         chanID,
			RecvTS:         recvTS,
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

func (r *Router) routeTrades(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64) error {
	var msgType string
	var tradeData []json.RawMessage
	isSnapshot := false

	if len(data) >= 2 {
		var testArray []json.RawMessage
		if err := json.Unmarshal(data[0], &testArray); err == nil {
			isSnapshot = true
			for _, item := range testArray {
				var singleTrade [4]json.RawMessage
				if err := json.Unmarshal(item, &singleTrade); err != nil {
					continue
				}
				r.processSingleTrade(chanID, channelInfo, singleTrade[:], connID, recvTS, isSnapshot, "snapshot")
			}
			return nil
		}

		if err := json.Unmarshal(data[0], &msgType); err == nil {
			if msgType == "te" || msgType == "tu" {
				tradeData = data[1:]
			}
		} else {
			msgType = "unknown"
			tradeData = data
		}
	}

	if len(tradeData) >= 4 {
		return r.processSingleTrade(chanID, channelInfo, tradeData, connID, recvTS, isSnapshot, msgType)
	}

	return nil
}

func (r *Router) processSingleTrade(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64, isSnapshot bool, msgType string) error {
	if len(data) < 4 {
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
			Exchange:       schema.ExchangeBitfinex,
			Channel:        schema.ChannelTrades,
			Symbol:         channelInfo.Symbol,
			PairOrCurrency: channelInfo.Pair,
			ConnID:         connID,
			ChanID:         chanID,
			RecvTS:         recvTS,
			SrvMTS:         &mts,
		},
		TradeID:    tradeID,
		MTS:        mts,
		Amount:     amount,
		Price:      price,
		MsgType:    schema.MessageType(msgType),
		IsSnapshot: isSnapshot,
	}

	select {
	case r.tradesChan <- trade:
	default:
		r.logger.Warn("Trades channel full, dropping message")
	}

	return nil
}

func (r *Router) routeBooks(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64) error {
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
				r.processSingleBookLevel(chanID, channelInfo, singleLevel[:], connID, recvTS, isSnapshot)
			}
			return nil
		}
		bookData = data
	}

	if len(bookData) >= 3 {
		return r.processSingleBookLevel(chanID, channelInfo, bookData, connID, recvTS, isSnapshot)
	}

	return nil
}

func (r *Router) processSingleBookLevel(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64, isSnapshot bool) error {
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
			Exchange:       schema.ExchangeBitfinex,
			Channel:        schema.ChannelBooks,
			Symbol:         channelInfo.Symbol,
			PairOrCurrency: channelInfo.Pair,
			ConnID:         connID,
			ChanID:         chanID,
			SubID:          channelInfo.SubID,
			RecvTS:         recvTS,
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

func (r *Router) routeRawBooks(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64) error {
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
				r.processSingleRawBookEvent(chanID, channelInfo, singleOrder[:], connID, recvTS, isSnapshot)
			}
			return nil
		}
		bookData = data
	}

	if len(bookData) >= 3 {
		return r.processSingleRawBookEvent(chanID, channelInfo, bookData, connID, recvTS, isSnapshot)
	}

	return nil
}

func (r *Router) processSingleRawBookEvent(chanID int32, channelInfo *ChannelInfo, data []json.RawMessage, connID string, recvTS int64, isSnapshot bool) error {
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
			Exchange:       schema.ExchangeBitfinex,
			Channel:        schema.ChannelRawBooks,
			Symbol:         channelInfo.Symbol,
			PairOrCurrency: channelInfo.Pair,
			ConnID:         connID,
			ChanID:         chanID,
			SubID:          channelInfo.SubID,
			RecvTS:         recvTS,
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

func (r *Router) Close() {
	close(r.tickerChan)
	close(r.tradesChan)
	close(r.booksChan)
	close(r.rawBooksChan)
	close(r.controlsChan)
}