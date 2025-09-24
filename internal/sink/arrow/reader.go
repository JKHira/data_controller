package arrow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/apache/arrow/go/v17/arrow/ipc"
	"go.uber.org/zap"
)

type FileReader struct {
	logger *zap.Logger
}

type RecordData struct {
	Records []map[string]interface{}
	Schema  *arrow.Schema
	Total   int64
}

type PageData struct {
	Records    []map[string]interface{}
	PageNumber int
	PageSize   int
	TotalPages int
	HasNext    bool
	HasPrev    bool
	BytesRead  int64
	TotalBytes int64
}

type SourceType string

const (
	SourceWS   SourceType = "ws"
	SourceREST SourceType = "rest"
	SourceFile SourceType = "file"
)

type FileInfo struct {
	Path         string
	Size         int64
	ModTime      time.Time
	Exchange     string
	Channel      string
	Symbol       string
	SourceType   SourceType
	Category     string
	Date         string
	Hour         string
	IsAccessible bool
	ErrorMsg     string
}

const (
	MaxBytesPerPage = 10 * 1024 * 1024 // 10MB per page
)

func NewFileReader(logger *zap.Logger) *FileReader {
	return &FileReader{
		logger: logger,
	}
}

// GetFilesByDateRange returns files filtered by date range
func (r *FileReader) GetFilesByDateRange(basePath string, startDate, endDate time.Time, channel, symbol string) ([]FileInfo, error) {
	r.logger.Info("Getting files by date range",
		zap.String("basePath", basePath),
		zap.Time("startDate", startDate),
		zap.Time("endDate", endDate),
		zap.String("channel", channel),
		zap.String("symbol", symbol))

	var files []FileInfo

	// Walk through directory structure
	err := r.walkDataDirectory(basePath, func(path string, info os.FileInfo) error {
		if !strings.HasSuffix(path, ".arrow") {
			return nil
		}

		fileInfo := r.parseFilePath(path, info)

		// Filter by date range
		if !fileInfo.ModTime.IsZero() {
			if fileInfo.ModTime.Before(startDate) || fileInfo.ModTime.After(endDate) {
				return nil
			}
		}

		// Filter by channel and symbol if specified
		if channel != "" && fileInfo.Channel != channel {
			return nil
		}
		if symbol != "" && fileInfo.Symbol != symbol {
			return nil
		}

		files = append(files, fileInfo)
		return nil
	})

	if err != nil {
		r.logger.Error("Failed to walk data directory", zap.Error(err))
		return nil, err
	}

	// Sort files by modification time (oldest first)
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.Before(files[j].ModTime)
	})

	r.logger.Info("Found files in date range", zap.Int("count", len(files)))
	return files, nil
}

// ReadArrowFileWithPagination reads an Arrow file with 10MB pagination support
func (r *FileReader) ReadArrowFileWithPagination(filePath string, pageNumber, pageSize int) (*PageData, error) {
	r.logger.Info("Reading Arrow file with pagination",
		zap.String("file", filePath),
		zap.Int("pageNumber", pageNumber),
		zap.Int("pageSize", pageSize))

	// Get file info first
	stat, err := os.Stat(filePath)
	if err != nil {
		r.logger.Error("Failed to stat file", zap.String("file", filePath), zap.Error(err))
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	file, err := os.Open(filePath)
	if err != nil {
		r.logger.Error("Failed to open Arrow file", zap.String("file", filePath), zap.Error(err))
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Try both File and Stream readers
	reader, err := r.createArrowReader(file, filePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return r.readWithByteLimitPagination(reader, stat.Size(), pageNumber, pageSize)
}

// createArrowReader tries to create appropriate Arrow reader
func (r *FileReader) createArrowReader(file *os.File, filePath string) (ArrowReader, error) {
	// First, try FileReader (for Arrow File format)
	if fileReader, err := ipc.NewFileReader(file); err == nil {
		r.logger.Debug("Successfully created Arrow File reader", zap.String("file", filePath))
		return &ArrowFileReaderWrapper{fileReader}, nil
	} else {
		r.logger.Debug("Failed to create Arrow File reader", zap.String("file", filePath), zap.Error(err))
	}

	// Reset file position
	file.Seek(0, 0)

	// Try StreamReader (for Arrow Stream format)
	if streamReader, err := ipc.NewReader(file); err == nil {
		r.logger.Debug("Successfully created Arrow Stream reader", zap.String("file", filePath))
		return &ArrowStreamReaderWrapper{reader: streamReader}, nil
	} else {
		r.logger.Error("Failed to create Arrow Stream reader", zap.String("file", filePath), zap.Error(err))
		return nil, fmt.Errorf("failed to create arrow reader (tried both File and Stream formats): %w", err)
	}
}

// ArrowReader interface to abstract File and Stream readers
type ArrowReader interface {
	Schema() *arrow.Schema
	NumRecords() int
	Record(i int) (arrow.Record, error)
	NextRecord() (arrow.Record, error)
	Close() error
}

// ArrowFileReaderWrapper wraps ipc.FileReader
type ArrowFileReaderWrapper struct {
	*ipc.FileReader
}

func (w *ArrowFileReaderWrapper) NextRecord() (arrow.Record, error) {
	return nil, fmt.Errorf("NextRecord not supported for File reader")
}

// ArrowStreamReaderWrapper wraps ipc.Reader
type ArrowStreamReaderWrapper struct {
	reader *ipc.Reader
}

func (w *ArrowStreamReaderWrapper) Schema() *arrow.Schema {
	return w.reader.Schema()
}

func (w *ArrowStreamReaderWrapper) NumRecords() int {
	return -1 // Unknown for stream
}

func (w *ArrowStreamReaderWrapper) Record(i int) (arrow.Record, error) {
	return nil, fmt.Errorf("indexed Record access not supported for Stream reader")
}

func (w *ArrowStreamReaderWrapper) NextRecord() (arrow.Record, error) {
	if w.reader.Next() {
		return w.reader.Record(), nil
	}
	return nil, fmt.Errorf("no more records")
}

func (w *ArrowStreamReaderWrapper) Close() error {
	// ipc.Reader doesn't have a Close method in this version
	// Just return nil as there's nothing to explicitly close
	return nil
}

// readWithByteLimitPagination implements 10MB limit per page
func (r *FileReader) readWithByteLimitPagination(reader ArrowReader, totalFileSize int64, pageNumber, pageSize int) (*PageData, error) {
	schema := reader.Schema()
	var allRecords []map[string]interface{}
	var bytesRead int64

	// For File reader, use indexed access
	if fileReader, ok := reader.(*ArrowFileReaderWrapper); ok {
		numBatches := fileReader.NumRecords()

		for i := 0; i < numBatches && bytesRead < MaxBytesPerPage; i++ {
			record, err := fileReader.Record(i)
			if err != nil {
				r.logger.Error("Failed to read record", zap.Int("batch", i), zap.Error(err))
				continue
			}

			batchRecords, batchBytes := r.processRecord(record, schema)
			allRecords = append(allRecords, batchRecords...)
			bytesRead += int64(batchBytes)

			record.Release()

			// Stop if we've reached the byte limit
			if bytesRead >= MaxBytesPerPage {
				r.logger.Debug("Reached byte limit for page",
					zap.Int64("bytesRead", bytesRead),
					zap.Int64("limit", MaxBytesPerPage))
				break
			}
		}
	} else {
		// For Stream reader, use sequential access
		for bytesRead < MaxBytesPerPage {
			record, err := reader.NextRecord()
			if err != nil {
				if strings.Contains(err.Error(), "no more records") {
					break
				}
				r.logger.Error("Failed to read next record", zap.Error(err))
				break
			}

			batchRecords, batchBytes := r.processRecord(record, schema)
			allRecords = append(allRecords, batchRecords...)
			bytesRead += int64(batchBytes)

			record.Release()

			// Stop if we've reached the byte limit
			if bytesRead >= MaxBytesPerPage {
				r.logger.Debug("Reached byte limit for page",
					zap.Int64("bytesRead", bytesRead),
					zap.Int64("limit", MaxBytesPerPage))
				break
			}
		}
	}

	// Calculate pagination based on byte limits
	totalPages := int(totalFileSize / MaxBytesPerPage)
	if totalFileSize%MaxBytesPerPage > 0 {
		totalPages++
	}

	return &PageData{
		Records:    allRecords,
		PageNumber: pageNumber,
		PageSize:   len(allRecords),
		TotalPages: totalPages,
		HasNext:    pageNumber < totalPages,
		HasPrev:    pageNumber > 1,
		BytesRead:  bytesRead,
		TotalBytes: totalFileSize,
	}, nil
}

// processRecord converts Arrow record to map slice and estimates byte size
func (r *FileReader) processRecord(record arrow.Record, schema *arrow.Schema) ([]map[string]interface{}, int) {
	var records []map[string]interface{}
	estimatedBytes := 0

	for row := int64(0); row < record.NumRows(); row++ {
		rowData := make(map[string]interface{})

		for col := 0; col < int(record.NumCols()); col++ {
			field := schema.Field(col)
			column := record.Column(col)

			value := r.getValueAtIndex(column, row)
			rowData[field.Name] = value

			// Estimate byte size (rough approximation)
			switch v := value.(type) {
			case string:
				estimatedBytes += len(v)
			case int64, float64:
				estimatedBytes += 8
			case bool:
				estimatedBytes += 1
			default:
				estimatedBytes += 8 // default estimation
			}
		}

		records = append(records, rowData)
		estimatedBytes += 50 // overhead per record
	}

	return records, estimatedBytes
}

// ReadArrowFileSummary reads basic information about the Arrow file
func (r *FileReader) ReadArrowFileSummary(filePath string) (map[string]interface{}, error) {
	r.logger.Info("Reading Arrow file summary", zap.String("file", filePath))

	file, err := os.Open(filePath)
	if err != nil {
		r.logger.Error("Failed to open file for summary", zap.String("file", filePath), zap.Error(err))
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	reader, err := r.createArrowReader(file, filePath)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	schema := reader.Schema()
	totalRecords := int64(0)
	numBatches := 0

	// Count records differently based on reader type
	if fileReader, ok := reader.(*ArrowFileReaderWrapper); ok {
		numBatches = fileReader.NumRecords()
		for i := 0; i < numBatches; i++ {
			record, err := fileReader.Record(i)
			if err != nil {
				continue
			}
			totalRecords += record.NumRows()
			record.Release()
		}
	} else {
		// For stream reader, we need to iterate through all records
		for {
			record, err := reader.NextRecord()
			if err != nil {
				break
			}
			totalRecords += record.NumRows()
			numBatches++
			record.Release()
		}
	}

	// Get file info
	stat, _ := file.Stat()

	summary := map[string]interface{}{
		"file_size":     stat.Size(),
		"total_records": totalRecords,
		"num_batches":   numBatches,
		"num_columns":   schema.NumFields(),
		"schema_fields": make([]map[string]string, 0),
	}

	// Add schema information
	fields := make([]map[string]string, 0)
	for i := 0; i < schema.NumFields(); i++ {
		field := schema.Field(i)
		fields = append(fields, map[string]string{
			"name": field.Name,
			"type": field.Type.String(),
		})
	}
	summary["schema_fields"] = fields

	r.logger.Info("File summary completed",
		zap.String("file", filePath),
		zap.Int64("totalRecords", totalRecords),
		zap.Int("numBatches", numBatches))

	return summary, nil
}

func (r *FileReader) getValueAtIndex(column arrow.Array, index int64) interface{} {
	if column.IsNull(int(index)) {
		return nil
	}

	switch arr := column.(type) {
	case *array.Int64:
		return arr.Value(int(index))
	case *array.Int32:
		return int64(arr.Value(int(index)))
	case *array.Float64:
		return arr.Value(int(index))
	case *array.Boolean:
		return arr.Value(int(index))
	case *array.String:
		return arr.Value(int(index))
	case *array.Timestamp:
		return arr.Value(int(index))
	default:
		return fmt.Sprintf("<%s>", arr.DataType().String())
	}
}

// Helper functions for file management

func (r *FileReader) walkDataDirectory(basePath string, walkFn func(path string, info os.FileInfo) error) error {
	return r.walkDirectoryRecursive(basePath, walkFn)
}

func (r *FileReader) walkDirectoryRecursive(dirPath string, walkFn func(path string, info os.FileInfo) error) error {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullPath := fmt.Sprintf("%s/%s", dirPath, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if entry.IsDir() {
			// Recursively walk subdirectories
			if err := r.walkDirectoryRecursive(fullPath, walkFn); err != nil {
				r.logger.Error("Error walking subdirectory", zap.String("path", fullPath), zap.Error(err))
			}
		} else {
			// Process file
			if err := walkFn(fullPath, info); err != nil {
				r.logger.Error("Error processing file", zap.String("path", fullPath), zap.Error(err))
			}
		}
	}

	return nil
}

func (r *FileReader) parseFilePath(path string, info os.FileInfo) FileInfo {
	fileInfo := FileInfo{
		Path:         path,
		Size:         info.Size(),
		ModTime:      info.ModTime(),
		IsAccessible: true,
	}

	// Parse metadata from path
	// Expected formats:
	// - WebSocket: data/{exchange}/websocket/{channel}/{symbol}/dt=date/...
	// - REST API: data/{exchange}/restapi/{category}/date=YYYY-MM-DD/hour=HH/...
	parts := strings.Split(path, string(os.PathSeparator))

	for i, part := range parts {
		if part == "data" && i+1 < len(parts) {
			// Extract exchange (e.g., bitfinex)
			fileInfo.Exchange = parts[i+1]

			// Determine source type and parse accordingly
			if i+2 < len(parts) {
				if parts[i+2] == "websocket" || parts[i+2] == "v2" {
					// WebSocket data: data/{exchange}/websocket/{channel}/{symbol}/...
					fileInfo.SourceType = SourceWS
					if i+4 < len(parts) {
						fileInfo.Channel = parts[i+3]
						fileInfo.Symbol = parts[i+4]
					}
				} else if strings.Contains(parts[i+2], "rest") {
					// REST API data: data/{exchange}/restapi/{category}/...
					fileInfo.SourceType = SourceREST
					if i+3 < len(parts) {
						fileInfo.Category = parts[i+3]
						if fileInfo.Category == "basedata" {
							fileInfo.Channel = "basedata"
						}
					}
				}
			}
			break
		}

		// Parse date and hour from path components
		if strings.HasPrefix(part, "dt=") {
			fileInfo.Date = strings.TrimPrefix(part, "dt=")
		}
		if strings.HasPrefix(part, "date=") {
			fileInfo.Date = strings.TrimPrefix(part, "date=")
		}
		if strings.HasPrefix(part, "hour=") {
			fileInfo.Hour = strings.TrimPrefix(part, "hour=")
		}
	}

	// Auto-detect category based on filename or path if not set
	if fileInfo.Category == "" {
		lower := strings.ToLower(path)
		if strings.Contains(lower, "base") || strings.Contains(lower, "basedata") {
			fileInfo.Category = "basedata"
		} else if strings.Contains(lower, "index") {
			fileInfo.Category = "index"
		}
	}

	// Set default source type if not determined
	if fileInfo.SourceType == "" {
		fileInfo.SourceType = SourceFile
	}

	return fileInfo
}

// ScanDataFiles walks basePath and returns FileInfo entries with metadata parsed from path
func (r *FileReader) ScanDataFiles(basePath string) ([]FileInfo, error) {
	r.logger.Info("Scanning data files with metadata", zap.String("basePath", basePath))

	var files []FileInfo

	err := filepath.WalkDir(basePath, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // continue on errors
		}
		if d.IsDir() {
			return nil
		}

		// Include various file types for metadata scanning
		ext := strings.ToLower(filepath.Ext(p))
		if ext != ".arrow" && ext != ".parquet" && ext != ".jsonl" && ext != ".zst" && ext != ".json" {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		// Parse file metadata using existing parseFilePath function
		fileInfo := r.parseFilePath(p, info)

		files = append(files, fileInfo)
		return nil
	})

	if err != nil {
		r.logger.Error("Failed to scan data files", zap.Error(err))
		return nil, err
	}

	// Sort by modification time (newest first for better UX)
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime.After(files[j].ModTime)
	})

	r.logger.Info("Scanned data files",
		zap.Int("totalFiles", len(files)),
		zap.String("basePath", basePath))

	return files, nil
}