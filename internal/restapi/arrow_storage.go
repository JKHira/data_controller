package restapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/ipc"
	"github.com/apache/arrow/go/v17/arrow/memory"
	"go.uber.org/zap"
)

// ArrowStorage handles Arrow IPC storage for base data
type ArrowStorage struct {
	logger *zap.Logger
	mem    memory.Allocator
}

// ManifestEntry represents a single entry in the JSONL manifest
type ManifestEntry struct {
	Timestamp time.Time `json:"ts"`
	Exchange  string    `json:"exchange"`
	DataType  string    `json:"data_type"`
	Endpoint  string    `json:"endpoint"`
	FilePath  string    `json:"file"`
	Count     int       `json:"count"`
	SizeBytes int64     `json:"size_bytes"`
	Format    string    `json:"format"`
}

// NewArrowStorage creates a new Arrow storage handler
func NewArrowStorage(logger *zap.Logger) *ArrowStorage {
	return &ArrowStorage{
		logger: logger,
		mem:    memory.NewGoAllocator(),
	}
}

// SaveBaseDataAsArrow saves base data in Arrow IPC format with manifest
func (a *ArrowStorage) SaveBaseDataAsArrow(data interface{}, endpoint, exchange string, timestamp time.Time) (string, error) {
	// Create base directory structure
	baseDir := fmt.Sprintf("data/%s/restapi/basedata", exchange)
	dateDir := timestamp.Format("2006-01-02")
	hourDir := fmt.Sprintf("hour=%02d", timestamp.Hour())

	fullDir := filepath.Join(baseDir, "date="+dateDir, hourDir)
	if err := createDirIfNotExists(fullDir); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Generate filename
	timestampStr := timestamp.Format("20060102T150405Z")
	filename := fmt.Sprintf("%s-%s.arrow", endpoint, timestampStr)
	filePath := filepath.Join(fullDir, filename)

	// Convert data to Arrow format
	record, err := a.convertToArrowRecord(data, endpoint)
	if err != nil {
		return "", fmt.Errorf("failed to convert to Arrow: %w", err)
	}
	defer record.Release()

	// Write Arrow IPC file
	if err := a.writeArrowFile(filePath, record); err != nil {
		return "", fmt.Errorf("failed to write Arrow file: %w", err)
	}

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %w", err)
	}

	// Update manifest
	manifestEntry := ManifestEntry{
		Timestamp: timestamp,
		Exchange:  exchange,
		DataType:  "basedata",
		Endpoint:  endpoint,
		FilePath:  filePath,
		Count:     int(record.NumRows()),
		SizeBytes: fileInfo.Size(),
		Format:    "arrow_ipc",
	}

	if err := a.updateManifest(baseDir, manifestEntry); err != nil {
		a.logger.Warn("Failed to update manifest", zap.Error(err))
		// Don't fail the entire operation for manifest issues
	}

	a.logger.Info("Saved base data as Arrow IPC",
		zap.String("file", filePath),
		zap.Int64("rows", record.NumRows()),
		zap.Int64("size_bytes", fileInfo.Size()))

	return filePath, nil
}

// convertToArrowRecord converts various data types to Arrow Record
func (a *ArrowStorage) convertToArrowRecord(data interface{}, endpoint string) (arrow.Record, error) {
	switch d := data.(type) {
	case []string:
		return a.convertStringArrayToRecord(d, endpoint)
	case []interface{}:
		return a.convertInterfaceArrayToRecord(d, endpoint)
	default:
		return nil, fmt.Errorf("unsupported data type for Arrow conversion: %T", data)
	}
}

// convertStringArrayToRecord converts string array to Arrow Record
func (a *ArrowStorage) convertStringArrayToRecord(data []string, endpoint string) (arrow.Record, error) {
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "endpoint", Type: arrow.BinaryTypes.String},
			{Name: "symbol", Type: arrow.BinaryTypes.String},
			{Name: "timestamp", Type: arrow.FixedWidthTypes.Timestamp_us},
		},
		nil,
	)

	builder := array.NewRecordBuilder(a.mem, schema)
	defer builder.Release()

	endpointBuilder := builder.Field(0).(*array.StringBuilder)
	symbolBuilder := builder.Field(1).(*array.StringBuilder)
	timestampBuilder := builder.Field(2).(*array.TimestampBuilder)

	now := arrow.Timestamp(time.Now().UnixMicro())

	for _, symbol := range data {
		endpointBuilder.Append(endpoint)
		symbolBuilder.Append(symbol)
		timestampBuilder.Append(now)
	}

	return builder.NewRecord(), nil
}

// convertInterfaceArrayToRecord converts interface array to Arrow Record
func (a *ArrowStorage) convertInterfaceArrayToRecord(data []interface{}, endpoint string) (arrow.Record, error) {
	if len(data) == 0 {
		// Return empty record with basic schema
		schema := arrow.NewSchema(
			[]arrow.Field{
				{Name: "endpoint", Type: arrow.BinaryTypes.String},
				{Name: "data", Type: arrow.BinaryTypes.String},
				{Name: "timestamp", Type: arrow.FixedWidthTypes.Timestamp_us},
			},
			nil,
		)
		builder := array.NewRecordBuilder(a.mem, schema)
		defer builder.Release()
		return builder.NewRecord(), nil
	}

	// For tickers data (array of arrays)
	if _, ok := data[0].([]interface{}); ok {
		return a.convertTickersToRecord(data)
	}

	// For simple data, convert to JSON strings
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "endpoint", Type: arrow.BinaryTypes.String},
			{Name: "data", Type: arrow.BinaryTypes.String},
			{Name: "timestamp", Type: arrow.FixedWidthTypes.Timestamp_us},
		},
		nil,
	)

	builder := array.NewRecordBuilder(a.mem, schema)
	defer builder.Release()

	endpointBuilder := builder.Field(0).(*array.StringBuilder)
	dataBuilder := builder.Field(1).(*array.StringBuilder)
	timestampBuilder := builder.Field(2).(*array.TimestampBuilder)

	now := arrow.Timestamp(time.Now().UnixMicro())

	for _, item := range data {
		jsonData, err := json.Marshal(item)
		if err != nil {
			jsonData = []byte(fmt.Sprintf("%v", item))
		}

		endpointBuilder.Append(endpoint)
		dataBuilder.Append(string(jsonData))
		timestampBuilder.Append(now)
	}

	return builder.NewRecord(), nil
}

// convertTickersToRecord converts ticker data to Arrow Record
func (a *ArrowStorage) convertTickersToRecord(data []interface{}) (arrow.Record, error) {
	schema := arrow.NewSchema(
		[]arrow.Field{
			{Name: "symbol", Type: arrow.BinaryTypes.String},
			{Name: "bid", Type: arrow.PrimitiveTypes.Float64},
			{Name: "bid_size", Type: arrow.PrimitiveTypes.Float64},
			{Name: "ask", Type: arrow.PrimitiveTypes.Float64},
			{Name: "ask_size", Type: arrow.PrimitiveTypes.Float64},
			{Name: "daily_change", Type: arrow.PrimitiveTypes.Float64},
			{Name: "daily_change_relative", Type: arrow.PrimitiveTypes.Float64},
			{Name: "last_price", Type: arrow.PrimitiveTypes.Float64},
			{Name: "volume", Type: arrow.PrimitiveTypes.Float64},
			{Name: "high", Type: arrow.PrimitiveTypes.Float64},
			{Name: "low", Type: arrow.PrimitiveTypes.Float64},
			{Name: "timestamp", Type: arrow.FixedWidthTypes.Timestamp_us},
		},
		nil,
	)

	builder := array.NewRecordBuilder(a.mem, schema)
	defer builder.Release()

	now := arrow.Timestamp(time.Now().UnixMicro())

	for _, item := range data {
		if tickerArray, ok := item.([]interface{}); ok && len(tickerArray) >= 11 {
			// Bitfinex ticker format: [SYMBOL, BID, BID_SIZE, ASK, ASK_SIZE, DAILY_CHANGE, DAILY_CHANGE_RELATIVE, LAST_PRICE, VOLUME, HIGH, LOW]

			builder.Field(0).(*array.StringBuilder).Append(fmt.Sprintf("%v", tickerArray[0])) // symbol

			for i := 1; i <= 10; i++ {
				if val, ok := tickerArray[i].(float64); ok {
					builder.Field(i).(*array.Float64Builder).Append(val)
				} else {
					builder.Field(i).(*array.Float64Builder).AppendNull()
				}
			}

			builder.Field(11).(*array.TimestampBuilder).Append(now) // timestamp
		}
	}

	return builder.NewRecord(), nil
}

// writeArrowFile writes Arrow Record to IPC file
func (a *ArrowStorage) writeArrowFile(filePath string, record arrow.Record) error {
	// Create temporary file for atomic write
	tempPath := filePath + ".tmp"

	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer file.Close()

	writer := ipc.NewWriter(file, ipc.WithSchema(record.Schema()))
	defer writer.Close()

	if err := writer.Write(record); err != nil {
		os.Remove(tempPath)
		return fmt.Errorf("failed to write record: %w", err)
	}

	// Atomic rename
	return os.Rename(tempPath, filePath)
}

// updateManifest appends entry to JSONL manifest file
func (a *ArrowStorage) updateManifest(baseDir string, entry ManifestEntry) error {
	manifestPath := filepath.Join(baseDir, "manifest.jsonl")

	// Ensure directory exists
	if err := createDirIfNotExists(baseDir); err != nil {
		return err
	}

	// Convert entry to JSON
	jsonData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest entry: %w", err)
	}

	// Append to manifest file
	file, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open manifest file: %w", err)
	}
	defer file.Close()

	if _, err := file.Write(append(jsonData, '\n')); err != nil {
		return fmt.Errorf("failed to write to manifest: %w", err)
	}

	return nil
}
