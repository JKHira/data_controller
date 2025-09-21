package services

import (
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/sink/arrow"
)

// FileReaderService wraps the Arrow file reader with additional functionality
type FileReaderService struct {
	logger      *zap.Logger
	arrowReader *arrow.FileReader
}

// PageData represents a page of file content
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

func NewFileReaderService(logger *zap.Logger) *FileReaderService {
	return &FileReaderService{
		logger:      logger,
		arrowReader: arrow.NewFileReader(logger),
	}
}

// ReadFileWithPagination reads an Arrow file with pagination
func (frs *FileReaderService) ReadFileWithPagination(filePath string, pageNumber, pageSize int) (*PageData, error) {
	arrowPageData, err := frs.arrowReader.ReadArrowFileWithPagination(filePath, pageNumber, pageSize)
	if err != nil {
		frs.logger.Error("Failed to read Arrow file with pagination",
			zap.String("file", filePath),
			zap.Int("page", pageNumber),
			zap.Error(err))
		return nil, err
	}

	// Convert Arrow PageData to service PageData
	return &PageData{
		Records:    arrowPageData.Records,
		PageNumber: arrowPageData.PageNumber,
		PageSize:   arrowPageData.PageSize,
		TotalPages: arrowPageData.TotalPages,
		HasNext:    arrowPageData.HasNext,
		HasPrev:    arrowPageData.HasPrev,
		BytesRead:  arrowPageData.BytesRead,
		TotalBytes: arrowPageData.TotalBytes,
	}, nil
}

// GetFileSummary returns basic information about an Arrow file
func (frs *FileReaderService) GetFileSummary(filePath string) (map[string]interface{}, error) {
	summary, err := frs.arrowReader.ReadArrowFileSummary(filePath)
	if err != nil {
		frs.logger.Error("Failed to read Arrow file summary",
			zap.String("file", filePath),
			zap.Error(err))
		return nil, err
	}

	return summary, nil
}