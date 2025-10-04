package arrow

import (
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/pkg/schema"
)

type DataCallback func(dataType, symbol string, data interface{})

type Handler struct {
	cfg         *config.Config
	logger      *zap.Logger
	writer      *Writer
	flushTicker *time.Ticker
	stopCh      chan struct{}
	wg          sync.WaitGroup
	stats       *Statistics
	mu          sync.Mutex
	stopped     bool
	stopOnce    sync.Once

	// GUI data streaming
	callbacks   []DataCallback
	callbacksMu sync.RWMutex
}

type Statistics struct {
	mu                   sync.RWMutex
	TickersReceived      int64
	TradesReceived       int64
	BookLevelsReceived   int64
	RawBookEventsReceived int64
	CandlesReceived      int64
	ControlsReceived     int64
	TotalBytesWritten    int64
	LastFlushTime        time.Time
	Errors               int64
}

func NewHandler(cfg *config.Config, logger *zap.Logger) *Handler {
	return &Handler{
		cfg:       cfg,
		logger:    logger,
		writer:    NewWriter(cfg, logger),
		stats:     &Statistics{},
		stopCh:    make(chan struct{}),
		callbacks: make([]DataCallback, 0),
	}
}

func (h *Handler) UpdateConfFlags(flags int64) {
	h.writer.UpdateConfFlags(flags)
}

func (h *Handler) RegisterDataCallback(callback DataCallback) {
	h.callbacksMu.Lock()
	defer h.callbacksMu.Unlock()
	h.callbacks = append(h.callbacks, callback)
}

func (h *Handler) broadcastData(dataType, symbol string, data interface{}) {
	h.callbacksMu.RLock()
	defer h.callbacksMu.RUnlock()

	for _, callback := range h.callbacks {
		// Non-blocking call to prevent GUI from blocking data processing
		go callback(dataType, symbol, data)
	}
}

func (h *Handler) Start() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.stopped && h.flushTicker != nil {
		h.logger.Warn("Arrow handler already started")
		return nil
	}

	h.stopped = false
	h.logger.Info("Starting Arrow handler")

	// Make the flush ticker safe
	d := h.cfg.Storage.Parquet.FlushInterval
	if d <= 0 {
		d = 2 * time.Second // sensible default
		h.logger.Warn("Invalid flush interval, using default", zap.Duration("default", d))
	}
	h.flushTicker = time.NewTicker(d)

	h.wg.Add(1)
	go h.flushRoutine()

	return nil
}

func (h *Handler) Stop() error {
	var err error
	h.stopOnce.Do(func() {
		h.logger.Info("Stopping Arrow handler")

		// Stop accepting new data
		h.mu.Lock()
		h.stopped = true
		close(h.stopCh)
		h.mu.Unlock()

		if h.flushTicker != nil {
			h.flushTicker.Stop()
		}

		// Wait for flush routine to finish
		h.wg.Wait()

		// Final flush before closing to ensure all buffered data is written
		h.logger.Info("Performing final flush before close")
		if flushErr := h.writer.FlushAll(); flushErr != nil {
			h.logger.Error("Failed to flush all data before close", zap.Error(flushErr))
			// Continue to close even if flush fails
		}

		// Close all writers and rename .tmp files to final files
		if closeErr := h.writer.Close(); closeErr != nil {
			h.logger.Error("Failed to close writer", zap.Error(closeErr))
			err = closeErr
		}

		h.logger.Info("Arrow handler stopped")
	})

	if !h.stopped {
		h.logger.Warn("Arrow handler already stopped")
	}

	return err
}

func (h *Handler) HandleTicker(ticker *schema.Ticker) {
	h.stats.mu.Lock()
	h.stats.TickersReceived++
	h.stats.mu.Unlock()

	h.logger.Debug("Received ticker data",
		zap.String("symbol", ticker.Symbol),
		zap.Float64("bid", ticker.Bid),
		zap.Float64("ask", ticker.Ask))

	// Broadcast to GUI first (non-blocking)
	h.broadcastData("ticker", ticker.Symbol, ticker)
	h.ensureMetadata(ticker.CommonFields)

	if err := h.writer.WriteTicker(ticker); err != nil {
		h.logger.Error("Failed to write ticker",
			zap.String("symbol", ticker.Symbol),
			zap.Error(err))
		h.incrementError()
	} else {
		h.logger.Debug("Successfully wrote ticker data", zap.String("symbol", ticker.Symbol))
	}
}

func (h *Handler) HandleTrade(trade *schema.Trade) {
	h.stats.mu.Lock()
	h.stats.TradesReceived++
	h.stats.mu.Unlock()

	h.logger.Debug("Received trade data",
		zap.String("symbol", trade.Symbol),
		zap.Int64("trade_id", trade.TradeID),
		zap.Float64("price", trade.Price),
		zap.Float64("amount", trade.Amount))

	// Broadcast to GUI first (non-blocking)
	h.broadcastData("trade", trade.Symbol, trade)
	h.ensureMetadata(trade.CommonFields)

	if err := h.writer.WriteTrade(trade); err != nil {
		h.logger.Error("Failed to write trade",
			zap.String("symbol", trade.Symbol),
			zap.Int64("trade_id", trade.TradeID),
			zap.Error(err))
		h.incrementError()
	}
}

func (h *Handler) HandleCandle(candle *schema.Candle) {
	h.stats.mu.Lock()
	h.stats.CandlesReceived++
	h.stats.mu.Unlock()

	h.logger.Debug("Received candle data",
		zap.String("symbol", candle.Symbol),
		zap.String("timeframe", candle.Timeframe),
		zap.Float64("open", candle.Open),
		zap.Float64("close", candle.Close))

	// Broadcast to GUI first (non-blocking)
	h.broadcastData("candle", candle.Symbol, candle)
	h.ensureMetadata(candle.CommonFields)

	if err := h.writer.WriteCandle(candle); err != nil {
		h.logger.Error("Failed to write candle",
			zap.String("symbol", candle.Symbol),
			zap.String("timeframe", candle.Timeframe),
			zap.Error(err))
		h.incrementError()
	}
}

func (h *Handler) HandleBookLevel(level *schema.BookLevel) {
	h.stats.mu.Lock()
	h.stats.BookLevelsReceived++
	h.stats.mu.Unlock()

	// Broadcast to GUI first (non-blocking)
	h.broadcastData("book", level.Symbol, level)
	h.ensureMetadata(level.CommonFields)

	if err := h.writer.WriteBookLevel(level); err != nil {
		h.logger.Error("Failed to write book level",
			zap.String("symbol", level.Symbol),
			zap.Float64("price", level.Price),
			zap.Error(err))
		h.incrementError()
	}
}

func (h *Handler) HandleRawBookEvent(event *schema.RawBookEvent) {
	h.stats.mu.Lock()
	h.stats.RawBookEventsReceived++
	h.stats.mu.Unlock()

	// Broadcast to GUI first (non-blocking)
	h.broadcastData("raw_book", event.Symbol, event)
	h.ensureMetadata(event.CommonFields)

	if err := h.writer.WriteRawBookEvent(event); err != nil {
		h.logger.Error("Failed to write raw book event",
			zap.String("symbol", event.Symbol),
			zap.Int64("order_id", event.OrderID),
			zap.Error(err))
		h.incrementError()
	}
}

func (h *Handler) HandleControl(control *schema.Control) {
	h.stats.mu.Lock()
	h.stats.ControlsReceived++
	h.stats.mu.Unlock()

	h.logger.Debug("Received control message",
		zap.String("type", control.Type),
		zap.String("reason", control.Reason))
}

func (h *Handler) ensureMetadata(common schema.CommonFields) {
	if common.Channel == "" {
		return
	}
	meta := schema.ChannelMetadata{
		Channel: common.Channel,
		Symbol:  common.Symbol,
		Pair:    common.PairOrCurrency,
		Key:     common.ChannelKey,
		ChanID:  common.ChanID,
		Timeframe: common.Timeframe,
		BookPrec:  common.BookPrec,
		BookFreq:  common.BookFreq,
		BookLen:   common.BookLen,
	}
	h.writer.UpdateChannelMetadata(meta)
}

func (h *Handler) flushRoutine() {
	defer h.wg.Done()

	for {
		select {
		case <-h.stopCh:
			h.logger.Info("Flush routine stopping")
			return
		case <-h.flushTicker.C:
			h.flush()
		}
	}
}

func (h *Handler) flush() {
	start := time.Now()

	if err := h.writer.FlushAll(); err != nil {
		h.logger.Error("Failed to flush data", zap.Error(err))
		h.incrementError()
		return
	}

	// Optional: time-based rotation for segments older than 15 minutes
	h.writer.RotateOldSegments(15 * time.Minute)

	duration := time.Since(start)

	h.stats.mu.Lock()
	h.stats.LastFlushTime = time.Now()
	h.stats.mu.Unlock()

	h.logger.Debug("Flushed data successfully",
		zap.Duration("duration", duration))
}

func (h *Handler) incrementError() {
	h.stats.mu.Lock()
	h.stats.Errors++
	h.stats.mu.Unlock()
}

func (h *Handler) GetStatistics() *Statistics {
	h.stats.mu.RLock()
	defer h.stats.mu.RUnlock()

	// Create a copy to avoid race conditions
	return &Statistics{
		TickersReceived:       h.stats.TickersReceived,
		TradesReceived:        h.stats.TradesReceived,
		BookLevelsReceived:    h.stats.BookLevelsReceived,
		RawBookEventsReceived: h.stats.RawBookEventsReceived,
		CandlesReceived:       h.stats.CandlesReceived,
		ControlsReceived:      h.stats.ControlsReceived,
		TotalBytesWritten:     h.stats.TotalBytesWritten,
		LastFlushTime:         h.stats.LastFlushTime,
		Errors:                h.stats.Errors,
	}
}

func (h *Handler) GetWriterStats() map[string]interface{} {
	return h.writer.GetStats()
}

func (h *Handler) ForceFlush() error {
	h.logger.Info("Force flushing all data")
	h.flush()
	return nil
}
