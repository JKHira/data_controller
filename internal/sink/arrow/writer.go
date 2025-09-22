package arrow

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/ipc"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/pkg/schema"
)

type Writer struct {
	cfg          *config.Config
	logger       *zap.Logger
	basePath     string
	ingestID     string

	// Segment management
	segments     map[string]*Segment
	segmentsMutex sync.RWMutex
	segmentSizeMB int64
}

type Segment struct {
	ID            string
	Channel       schema.Channel
	Symbol        string
	StartTime     time.Time
	EndTime       time.Time
	DirPath       string
	Writers       map[string]*ChannelWriter
	WritersMutex  sync.RWMutex
	CurrentSizeMB int64
	IsOpen        bool
	Mutex         sync.Mutex
}

type ChannelWriter struct {
	FilePath     string
	TempFilePath string
	File         *os.File
	Writer       *ipc.FileWriter
	Schema       *arrow.Schema
	Builder      *RecordBuilder
	RowCount     int64
	StartTime    time.Time
	Channel      schema.Channel
	Symbol       string
	Mutex        sync.Mutex
	IsOpen       bool
	Pool         memory.Allocator
}

type RecordBuilder struct {
	schema *arrow.Schema
	builders []array.Builder
	pool   memory.Allocator
}

func NewWriter(cfg *config.Config, logger *zap.Logger) *Writer {
	return &Writer{
		cfg:           cfg,
		logger:        logger,
		basePath:      cfg.Storage.BasePath,
		ingestID:      uuid.New().String(),
		segments:      make(map[string]*Segment),
		segmentSizeMB: int64(cfg.Storage.SegmentSizeMB),
	}
}

func (w *Writer) WriteRawBookEvent(event *schema.RawBookEvent) error {
	event.IngestID = w.ingestID
	event.SourceFile = "websocket"
	event.RecvTS = time.Now().UnixMicro()

	segment, err := w.getOrCreateSegment(schema.ChannelRawBooks, event.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get segment: %w", err)
	}

	writer, err := segment.getOrCreateWriter(schema.ChannelRawBooks, event.Symbol, w.cfg)
	if err != nil {
		return fmt.Errorf("failed to get writer: %w", err)
	}

	return writer.writeRawBookEvent(event)
}

func (w *Writer) WriteBookLevel(level *schema.BookLevel) error {
	level.IngestID = w.ingestID
	level.SourceFile = "websocket"
	level.RecvTS = time.Now().UnixMicro()

	segment, err := w.getOrCreateSegment(schema.ChannelBooks, level.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get segment: %w", err)
	}

	writer, err := segment.getOrCreateWriter(schema.ChannelBooks, level.Symbol, w.cfg)
	if err != nil {
		return fmt.Errorf("failed to get writer: %w", err)
	}

	return writer.writeBookLevel(level)
}

func (w *Writer) WriteTrade(trade *schema.Trade) error {
	trade.IngestID = w.ingestID
	trade.SourceFile = "websocket"
	trade.RecvTS = time.Now().UnixMicro()

	segment, err := w.getOrCreateSegment(schema.ChannelTrades, trade.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get segment: %w", err)
	}

	writer, err := segment.getOrCreateWriter(schema.ChannelTrades, trade.Symbol, w.cfg)
	if err != nil {
		return fmt.Errorf("failed to get writer: %w", err)
	}

	return writer.writeTrade(trade)
}

func (w *Writer) WriteTicker(ticker *schema.Ticker) error {
	ticker.IngestID = w.ingestID
	ticker.SourceFile = "websocket"
	ticker.RecvTS = time.Now().UnixMicro()

	segment, err := w.getOrCreateSegment(schema.ChannelTicker, ticker.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get segment: %w", err)
	}

	writer, err := segment.getOrCreateWriter(schema.ChannelTicker, ticker.Symbol, w.cfg)
	if err != nil {
		return fmt.Errorf("failed to get writer: %w", err)
	}

	return writer.writeTicker(ticker)
}

func (w *Writer) getOrCreateSegment(channel schema.Channel, symbol string) (*Segment, error) {
	segmentKey := fmt.Sprintf("%s_%s", channel, symbol)

	w.segmentsMutex.RLock()
	segment, exists := w.segments[segmentKey]
	w.segmentsMutex.RUnlock()

	if exists && segment.IsOpen {
		segment.Mutex.Lock()
		if segment.CurrentSizeMB < w.segmentSizeMB {
			segment.Mutex.Unlock()
			return segment, nil
		}
		segment.Mutex.Unlock()

		if err := w.closeSegment(segment); err != nil {
			w.logger.Error("Failed to close segment", zap.Error(err))
		}
	}

	return w.createNewSegment(channel, symbol, segmentKey)
}

func (w *Writer) createNewSegment(channel schema.Channel, symbol string, segmentKey string) (*Segment, error) {
	now := time.Now().UTC()

	dirName := fmt.Sprintf("seg=%s--%s--size~%dMB",
		now.Format("2006-01-02T15:04:05Z"),
		now.Add(time.Hour).Format("2006-01-02T15:04:05Z"),
		w.segmentSizeMB)

	dirPath := filepath.Join(w.basePath, "bitfinex", "v2", string(channel), symbol,
		fmt.Sprintf("dt=%s", now.Format("2006-01-02")),
		fmt.Sprintf("hour=%02d", now.Hour()),
		dirName)

	w.logger.Info("Creating new segment directory",
		zap.String("path", dirPath),
		zap.String("channel", string(channel)),
		zap.String("symbol", symbol))

	if err := os.MkdirAll(dirPath, 0755); err != nil {
		w.logger.Error("Failed to create directory",
			zap.String("path", dirPath),
			zap.Error(err))
		return nil, fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	w.logger.Info("Successfully created directory", zap.String("path", dirPath))

	segment := &Segment{
		ID:        uuid.New().String(),
		Channel:   channel,
		Symbol:    symbol,
		StartTime: now,
		DirPath:   dirPath,
		Writers:   make(map[string]*ChannelWriter),
		IsOpen:    true,
	}

	w.segmentsMutex.Lock()
	w.segments[segmentKey] = segment
	w.segmentsMutex.Unlock()

	w.logger.Info("Created new segment",
		zap.String("segment_id", segment.ID),
		zap.String("channel", string(channel)),
		zap.String("symbol", symbol),
		zap.String("path", dirPath))

	return segment, nil
}

func (s *Segment) getOrCreateWriter(channel schema.Channel, symbol string, cfg *config.Config) (*ChannelWriter, error) {
	writerKey := fmt.Sprintf("%s_%s", channel, symbol)

	s.WritersMutex.RLock()
	writer, exists := s.Writers[writerKey]
	s.WritersMutex.RUnlock()

	if exists {
		return writer, nil
	}

	return s.createNewWriter(channel, symbol, cfg, writerKey)
}

func (s *Segment) createNewWriter(channel schema.Channel, symbol string, cfg *config.Config, writerKey string) (*ChannelWriter, error) {
	now := time.Now().UTC()
	filename := fmt.Sprintf("part-%s-%s-%s-seq.arrow",
		channel, symbol, now.Format("20060102T150405Z"))

	filePath := filepath.Join(s.DirPath, filename)
	tempFilePath := filePath + ".tmp"

	file, err := os.Create(tempFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file %s: %w", tempFilePath, err)
	}

	// Log successful file creation
	fmt.Printf("Successfully created temp file: %s\n", tempFilePath)

	pool := memory.NewGoAllocator()

	var arrowSchema *arrow.Schema
	switch channel {
	case schema.ChannelRawBooks:
		arrowSchema = GetRawBookEventSchema()
	case schema.ChannelBooks:
		arrowSchema = GetBookLevelSchema()
	case schema.ChannelTrades:
		arrowSchema = GetTradeSchema()
	case schema.ChannelTicker:
		arrowSchema = GetTickerSchema()
	default:
		file.Close()
		return nil, fmt.Errorf("unsupported channel type: %s", channel)
	}

	writer, err := ipc.NewFileWriter(file, ipc.WithSchema(arrowSchema))
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("failed to create arrow file writer: %w", err)
	}

	builder := &RecordBuilder{
		schema: arrowSchema,
		pool:   pool,
	}
	builder.initBuilders()

	channelWriter := &ChannelWriter{
		FilePath:     filePath,
		TempFilePath: tempFilePath,
		File:         file,
		Writer:       writer,
		Schema:       arrowSchema,
		Builder:      builder,
		StartTime:    now,
		Channel:      channel,
		Symbol:       symbol,
		IsOpen:       true,
		Pool:         pool,
	}

	s.WritersMutex.Lock()
	s.Writers[writerKey] = channelWriter
	s.WritersMutex.Unlock()

	return channelWriter, nil
}

func (rb *RecordBuilder) initBuilders() {
	rb.builders = make([]array.Builder, len(rb.schema.Fields()))
	for i, field := range rb.schema.Fields() {
		rb.builders[i] = array.NewBuilder(rb.pool, field.Type)
	}
}

func (w *Writer) FlushAll() error {
	w.segmentsMutex.RLock()
	segments := make([]*Segment, 0, len(w.segments))
	for _, segment := range w.segments {
		segments = append(segments, segment)
	}
	w.segmentsMutex.RUnlock()

	for _, segment := range segments {
		segment.WritersMutex.RLock()
		writers := make([]*ChannelWriter, 0, len(segment.Writers))
		for _, writer := range segment.Writers {
			writers = append(writers, writer)
		}
		segment.WritersMutex.RUnlock()

		for _, writer := range writers {
			if err := writer.flush(); err != nil {
				w.logger.Error("Failed to flush writer", zap.Error(err))
			}

			// Update segment size after each flush and auto-rotate
			if fi, err := os.Stat(writer.TempFilePath); err == nil {
				mb := fi.Size() / (1024 * 1024)
				segment.Mutex.Lock()
				segment.CurrentSizeMB = mb
				shouldClose := segment.CurrentSizeMB >= w.segmentSizeMB
				segment.Mutex.Unlock()

				w.logger.Debug("Updated segment size",
					zap.String("segment_id", segment.ID),
					zap.Int64("current_mb", mb),
					zap.Int64("target_mb", w.segmentSizeMB),
					zap.Bool("should_close", shouldClose))

				if shouldClose {
					w.logger.Info("Segment size threshold reached, closing segment",
						zap.String("segment_id", segment.ID),
						zap.Int64("size_mb", mb))
					if err := w.closeSegment(segment); err != nil {
						w.logger.Error("Failed to close segment", zap.Error(err))
					}
				}
			}
		}
	}

	return nil
}

func (w *Writer) RotateOldSegments(maxAge time.Duration) {
	w.segmentsMutex.RLock()
	segmentsToClose := make([]*Segment, 0)
	now := time.Now()

	for _, segment := range w.segments {
		segment.Mutex.Lock()
		age := now.Sub(segment.StartTime)
		shouldRotate := age > maxAge && segment.IsOpen
		segment.Mutex.Unlock()

		if shouldRotate {
			w.logger.Info("Time-based rotation triggered",
				zap.String("segment_id", segment.ID),
				zap.Duration("age", age),
				zap.Duration("max_age", maxAge))
			segmentsToClose = append(segmentsToClose, segment)
		}
	}
	w.segmentsMutex.RUnlock()

	for _, segment := range segmentsToClose {
		if err := w.closeSegment(segment); err != nil {
			w.logger.Error("Failed to close old segment", zap.Error(err))
		}
	}
}

func (w *Writer) closeSegment(segment *Segment) error {
	segment.Mutex.Lock()
	defer segment.Mutex.Unlock()

	segment.EndTime = time.Now().UTC()
	segment.IsOpen = false

	segment.WritersMutex.Lock()
	for _, writer := range segment.Writers {
		if err := writer.close(); err != nil {
			w.logger.Error("Failed to close writer", zap.Error(err))
		}
	}
	segment.WritersMutex.Unlock()

	w.logger.Info("Closed segment",
		zap.String("segment_id", segment.ID),
		zap.String("channel", string(segment.Channel)),
		zap.String("symbol", segment.Symbol))

	return nil
}

func (w *Writer) Close() error {
	w.segmentsMutex.RLock()
	segments := make([]*Segment, 0, len(w.segments))
	for _, segment := range w.segments {
		segments = append(segments, segment)
	}
	w.segmentsMutex.RUnlock()

	for _, segment := range segments {
		if err := w.closeSegment(segment); err != nil {
			w.logger.Error("Failed to close segment", zap.Error(err))
		}
	}

	return nil
}

func (w *Writer) GetStats() map[string]interface{} {
	w.segmentsMutex.RLock()
	defer w.segmentsMutex.RUnlock()

	stats := map[string]interface{}{
		"segments_count": len(w.segments),
		"ingest_id":      w.ingestID,
		"segments":       make([]map[string]interface{}, 0),
	}

	for _, segment := range w.segments {
		segment.WritersMutex.RLock()
		segmentStats := map[string]interface{}{
			"id":           segment.ID,
			"channel":      string(segment.Channel),
			"symbol":       segment.Symbol,
			"start_time":   segment.StartTime,
			"end_time":     segment.EndTime,
			"is_open":      segment.IsOpen,
			"writers_count": len(segment.Writers),
			"current_size_mb": segment.CurrentSizeMB,
		}
		segment.WritersMutex.RUnlock()

		stats["segments"] = append(stats["segments"].([]map[string]interface{}), segmentStats)
	}

	return stats
}