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
	h.logger.Info("Stopping Arrow handler")

	close(h.stopCh)

	if h.flushTicker != nil {
		h.flushTicker.Stop()
	}

	h.wg.Wait()

	if err := h.writer.Close(); err != nil {
		h.logger.Error("Failed to close writer", zap.Error(err))
		return err
	}

	h.logger.Info("Arrow handler stopped")
	return nil
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

	// Broadcast to GUI first (non-blocking)
	h.broadcastData("trade", trade.Symbol, trade)

	if err := h.writer.WriteTrade(trade); err != nil {
		h.logger.Error("Failed to write trade",
			zap.String("symbol", trade.Symbol),
			zap.Int64("trade_id", trade.TradeID),
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