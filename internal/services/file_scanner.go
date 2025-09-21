package services

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
)

// FileScanner handles file discovery and filtering
type FileScanner struct {
	logger   *zap.Logger
	basePath string
}

// FileFilter contains filter criteria
type FileFilter struct {
	StartDate time.Time
	EndDate   time.Time
	Channel   string
	Symbol    string
}

func NewFileScanner(logger *zap.Logger, basePath string) *FileScanner {
	return &FileScanner{
		logger:   logger,
		basePath: basePath,
	}
}

// GetAllFiles returns all Arrow files in the base path
func (fs *FileScanner) GetAllFiles() ([]string, error) {
	var files []string

	if _, err := os.Stat(fs.basePath); os.IsNotExist(err) {
		return files, nil
	}

	err := filepath.Walk(fs.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking
		}

		if !info.IsDir() && strings.HasSuffix(path, ".arrow") {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		fs.logger.Error("Failed to walk data directory", zap.Error(err))
		return nil, err
	}

	return files, nil
}

// GetFilteredFiles returns files matching the filter criteria
func (fs *FileScanner) GetFilteredFiles(filter FileFilter) ([]string, error) {
	allFiles, err := fs.GetAllFiles()
	if err != nil {
		return nil, err
	}

	var filteredFiles []string

	for _, file := range allFiles {
		// Get file info for date filtering
		info, err := os.Stat(file)
		if err != nil {
			fs.logger.Warn("Failed to stat file", zap.String("file", file), zap.Error(err))
			continue
		}

		// Apply date filter
		if !filter.StartDate.IsZero() && info.ModTime().Before(filter.StartDate) {
			continue
		}
		if !filter.EndDate.IsZero() && info.ModTime().After(filter.EndDate) {
			continue
		}

		// Apply channel and symbol filters
		if filter.Channel != "" && !strings.Contains(file, "/"+filter.Channel+"/") {
			continue
		}
		if filter.Symbol != "" && !strings.Contains(file, "/"+filter.Symbol+"/") {
			continue
		}

		filteredFiles = append(filteredFiles, file)
	}

	fs.logger.Info("Filtered files",
		zap.Int("total", len(allFiles)),
		zap.Int("filtered", len(filteredFiles)))

	return filteredFiles, nil
}