package parquet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/parquet-go/parquet-go"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/pkg/schema"
)

type Writer struct {
	cfg            *config.Config
	logger         *zap.Logger
	segments       map[string]*Segment
	segmentsMutex  sync.RWMutex
	basePath       string
	segmentSizeMB  int64
	ingestID       string
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
	Manifest      *schema.SegmentManifest
	IsOpen        bool
	Mutex         sync.Mutex
}

type ChannelWriter struct {
	FilePath     string
	Writer       interface{}
	RowCount     int64
	LastFlush    time.Time
	TempFilePath string
	Mutex        sync.Mutex
}

type FlushStats struct {
	Channel     schema.Channel
	Symbol      string
	RowCount    int64
	FileSizeMB  float64
	Duration    time.Duration
	Timestamp   time.Time
}

func NewWriter(cfg *config.Config, logger *zap.Logger) *Writer {
	return &Writer{
		cfg:           cfg,
		logger:        logger,
		segments:      make(map[string]*Segment),
		basePath:      cfg.Storage.BasePath,
		segmentSizeMB: int64(cfg.Storage.SegmentSizeMB),
		ingestID:      uuid.New().String(),
	}
}

func (w *Writer) WriteRawBookEvent(event *schema.RawBookEvent) error {
	event.IngestID = w.ingestID
	event.SourceFile = "websocket"

	segment, err := w.getOrCreateSegment(schema.ChannelRawBooks, event.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get segment: %w", err)
	}

	writer, err := segment.getOrCreateWriter(schema.ChannelRawBooks, event.Symbol, w.cfg)
	if err != nil {
		return fmt.Errorf("failed to get writer: %w", err)
	}

	return writer.writeRow(event)
}

func (w *Writer) WriteBookLevel(level *schema.BookLevel) error {
	level.IngestID = w.ingestID
	level.SourceFile = "websocket"

	segment, err := w.getOrCreateSegment(schema.ChannelBooks, level.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get segment: %w", err)
	}

	writer, err := segment.getOrCreateWriter(schema.ChannelBooks, level.Symbol, w.cfg)
	if err != nil {
		return fmt.Errorf("failed to get writer: %w", err)
	}

	return writer.writeRow(level)
}

func (w *Writer) WriteTrade(trade *schema.Trade) error {
	trade.IngestID = w.ingestID
	trade.SourceFile = "websocket"

	segment, err := w.getOrCreateSegment(schema.ChannelTrades, trade.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get segment: %w", err)
	}

	writer, err := segment.getOrCreateWriter(schema.ChannelTrades, trade.Symbol, w.cfg)
	if err != nil {
		return fmt.Errorf("failed to get writer: %w", err)
	}

	return writer.writeRow(trade)
}

func (w *Writer) WriteTicker(ticker *schema.Ticker) error {
	ticker.IngestID = w.ingestID
	ticker.SourceFile = "websocket"

	segment, err := w.getOrCreateSegment(schema.ChannelTicker, ticker.Symbol)
	if err != nil {
		return fmt.Errorf("failed to get segment: %w", err)
	}

	writer, err := segment.getOrCreateWriter(schema.ChannelTicker, ticker.Symbol, w.cfg)
	if err != nil {
		return fmt.Errorf("failed to get writer: %w", err)
	}

	return writer.writeRow(ticker)
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
		Manifest: &schema.SegmentManifest{
			SchemaVersion:  "bfx.v1",
			Exchange:       "bitfinex",
			Channel:        string(channel),
			Symbol:         symbol,
			PairOrCurrency: symbol,
			WSURL:          "wss://api-pub.bitfinex.com/ws/2",
			ConnID:         w.ingestID,
			ConfFlags:      w.cfg.WebSocket.ConfFlags,
			Segment: schema.SegmentInfo{
				BytesTarget: w.segmentSizeMB * 1024 * 1024,
				UTCStart:    now,
				Files:       make([]string, 0),
			},
			Quality: schema.QualityMetrics{},
		},
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
	filename := fmt.Sprintf("part-%s-%s-%s-seq.parquet",
		channel, symbol, now.Format("20060102T150405Z"))

	filePath := filepath.Join(s.DirPath, filename)
	tempFilePath := filePath + ".tmp"

	// Use basic compression options
	var compressionOpt parquet.WriterOption
	switch cfg.Storage.Compression {
	case "zstd":
		compressionOpt = parquet.Compression(&parquet.Zstd)
	case "gzip":
		compressionOpt = parquet.Compression(&parquet.Gzip)
	case "snappy":
		compressionOpt = parquet.Compression(&parquet.Snappy)
	default:
		compressionOpt = parquet.Compression(&parquet.Zstd)
	}

	file, err := os.Create(tempFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file %s: %w", tempFilePath, err)
	}

	var parquetWriter interface{}
	switch channel {
	case schema.ChannelRawBooks:
		parquetWriter = parquet.NewGenericWriter[schema.RawBookEvent](file, compressionOpt)
	case schema.ChannelBooks:
		parquetWriter = parquet.NewGenericWriter[schema.BookLevel](file, compressionOpt)
	case schema.ChannelTrades:
		parquetWriter = parquet.NewGenericWriter[schema.Trade](file, compressionOpt)
	case schema.ChannelTicker:
		parquetWriter = parquet.NewGenericWriter[schema.Ticker](file, compressionOpt)
	default:
		file.Close()
		return nil, fmt.Errorf("unsupported channel type: %s", channel)
	}

	writer := &ChannelWriter{
		FilePath:     filePath,
		TempFilePath: tempFilePath,
		Writer:       parquetWriter,
		LastFlush:    now,
	}

	s.WritersMutex.Lock()
	s.Writers[writerKey] = writer
	s.WritersMutex.Unlock()

	return writer, nil
}

func (cw *ChannelWriter) writeRow(data interface{}) error {
	cw.Mutex.Lock()
	defer cw.Mutex.Unlock()

	var err error
	switch v := data.(type) {
	case *schema.RawBookEvent:
		if w, ok := cw.Writer.(*parquet.GenericWriter[schema.RawBookEvent]); ok {
			_, err = w.Write([]schema.RawBookEvent{*v})
		}
	case *schema.BookLevel:
		if w, ok := cw.Writer.(*parquet.GenericWriter[schema.BookLevel]); ok {
			_, err = w.Write([]schema.BookLevel{*v})
		}
	case *schema.Trade:
		if w, ok := cw.Writer.(*parquet.GenericWriter[schema.Trade]); ok {
			_, err = w.Write([]schema.Trade{*v})
		}
	case *schema.Ticker:
		if w, ok := cw.Writer.(*parquet.GenericWriter[schema.Ticker]); ok {
			_, err = w.Write([]schema.Ticker{*v})
		}
	default:
		return fmt.Errorf("unsupported data type: %T", data)
	}

	if err != nil {
		return fmt.Errorf("failed to write row: %w", err)
	}

	cw.RowCount++
	return nil
}

func (cw *ChannelWriter) flush() error {
	cw.Mutex.Lock()
	defer cw.Mutex.Unlock()

	if cw.Writer != nil {
		// Type assertion for different writer types
		switch w := cw.Writer.(type) {
		case *parquet.GenericWriter[schema.RawBookEvent]:
			if err := w.Flush(); err != nil {
				return fmt.Errorf("failed to flush writer: %w", err)
			}
		case *parquet.GenericWriter[schema.BookLevel]:
			if err := w.Flush(); err != nil {
				return fmt.Errorf("failed to flush writer: %w", err)
			}
		case *parquet.GenericWriter[schema.Trade]:
			if err := w.Flush(); err != nil {
				return fmt.Errorf("failed to flush writer: %w", err)
			}
		case *parquet.GenericWriter[schema.Ticker]:
			if err := w.Flush(); err != nil {
				return fmt.Errorf("failed to flush writer: %w", err)
			}
		}
	}

	cw.LastFlush = time.Now()
	return nil
}

func (cw *ChannelWriter) close() error {
	cw.Mutex.Lock()
	defer cw.Mutex.Unlock()

	if cw.Writer != nil {
		// Type assertion for different writer types
		switch w := cw.Writer.(type) {
		case *parquet.GenericWriter[schema.RawBookEvent]:
			if err := w.Close(); err != nil {
				return fmt.Errorf("failed to close writer: %w", err)
			}
		case *parquet.GenericWriter[schema.BookLevel]:
			if err := w.Close(); err != nil {
				return fmt.Errorf("failed to close writer: %w", err)
			}
		case *parquet.GenericWriter[schema.Trade]:
			if err := w.Close(); err != nil {
				return fmt.Errorf("failed to close writer: %w", err)
			}
		case *parquet.GenericWriter[schema.Ticker]:
			if err := w.Close(); err != nil {
				return fmt.Errorf("failed to close writer: %w", err)
			}
		}
		cw.Writer = nil
	}

	if err := os.Rename(cw.TempFilePath, cw.FilePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
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

		filename := filepath.Base(writer.FilePath)
		segment.Manifest.Segment.Files = append(segment.Manifest.Segment.Files, filename)
	}
	segment.WritersMutex.Unlock()

	segment.Manifest.Segment.UTCEnd = segment.EndTime

	manifestPath := filepath.Join(segment.DirPath, "manifest.json")
	manifestData, err := json.MarshalIndent(segment.Manifest, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	w.logger.Info("Closed segment",
		zap.String("segment_id", segment.ID),
		zap.String("channel", string(segment.Channel)),
		zap.String("symbol", segment.Symbol),
		zap.Int("file_count", len(segment.Manifest.Segment.Files)),
		zap.String("manifest_path", manifestPath))

	return nil
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
		}
	}

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