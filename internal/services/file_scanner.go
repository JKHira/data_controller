package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/domain"
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

// legacySourceMap maps old directory names to new ones
var legacySourceMap = map[string]string{
	"v2":        "ws",
	"websocket": "ws",
	"restv2":    "restapi",
}

// normalizeSource converts legacy source names to new ones
func normalizeSource(src string) string {
	if n, ok := legacySourceMap[src]; ok {
		return n
	}
	return src
}

// sourceCandidates returns all possible directory names for a source
func sourceCandidates(src string) []string {
	s := normalizeSource(src)
	switch s {
	case "ws":
		return []string{"ws", "websocket", "v2"}
	case "restapi":
		return []string{"restapi", "restv2"}
	default:
		return []string{s}
	}
}

// FindFiles scans for files based on the given parameters
func (fs *FileScanner) FindFiles(ctx context.Context, params domain.ScanParams) ([]domain.FileItem, error) {
	var allFiles []domain.FileItem

	// "no data"選択時は即時空結果を返す
	if strings.EqualFold(params.Symbol, "no data") {
		return []domain.FileItem{}, nil
	}

	// Generate date range
	dates := fs.generateDateRange(params.DateFrom, params.DateTo)
	hours := fs.generateHours(params.Hour)
	sourceDirs := sourceCandidates(params.Source)

	// Handle "All" categories
	if strings.HasPrefix(params.Category, "All ") {
		return fs.findAllCategoryFiles(ctx, params, dates, hours, sourceDirs)
	}

	// Handle multiple exchanges (ALL support)
	var exchanges []string
	if params.Exchange == "" || strings.EqualFold(params.Exchange, "ALL") {
		// Get all exchanges from data/ directory
		dataRoot := filepath.Join(params.BasePath, "data")
		exchangeDirs, err := os.ReadDir(dataRoot)
		if err == nil {
			for _, ex := range exchangeDirs {
				if ex.IsDir() {
					exchanges = append(exchanges, ex.Name())
				}
			}
		}
	} else {
		exchanges = []string{params.Exchange}
	}

	// Scan all exchanges, sources, dates, hours
	for _, exchange := range exchanges {
		for _, sourceDir := range sourceDirs {
			for _, date := range dates {
				for _, hour := range hours {
					// Check if context is cancelled
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					default:
					}

					// Build path pattern with current exchange
					basePath := filepath.Join(
						params.BasePath,
						"data",
						exchange,
						sourceDir,
						params.Category,
					)

					// Add symbol if specified and not "ALL"
					if params.Symbol != "" && params.Symbol != "ALL" {
						basePath = filepath.Join(basePath, params.Symbol)
					}

					// Add date and hour
					basePath = filepath.Join(basePath, fmt.Sprintf("dt=%s", date))
					if hour != "" {
						basePath = filepath.Join(basePath, fmt.Sprintf("hour=%s", hour))
					}

					// Scan this path
					files, err := fs.scanPath(basePath, params)
					if err != nil {
						// Log error but continue scanning other paths
						fs.logger.Debug("Failed to scan path",
							zap.String("path", basePath),
							zap.Error(err))
						continue
					}

					// Set metadata for found files
					for i := range files {
						files[i].Exchange = exchange
						files[i].Source = normalizeSource(sourceDir)
						files[i].Category = params.Category
						files[i].Date = date
						files[i].Hour = hour
						if params.Symbol != "" && params.Symbol != "ALL" {
							files[i].Symbol = params.Symbol
						}
					}

					allFiles = append(allFiles, files...)
				}
			}
		}
	}

	// Filename filter removed as requested - search by path structure only
	return allFiles, nil
}

// generateDateRange generates a slice of date strings in YYYY-MM-DD format
func (fs *FileScanner) generateDateRange(from, to time.Time) []string {
	var dates []string

	// Normalize to date only (remove time component)
	from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())
	to = time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, to.Location())

	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		dates = append(dates, d.Format("2006-01-02"))
	}

	return dates
}

// generateHours generates hour strings based on the hour parameter
func (fs *FileScanner) generateHours(hour string) []string {
	if hour == "All" || hour == "" {
		hours := make([]string, 24)
		for i := 0; i < 24; i++ {
			hours[i] = fmt.Sprintf("%02d", i)
		}
		return hours
	}
	return []string{hour}
}

// scanPath scans a specific directory path for files
func (fs *FileScanner) scanPath(basePath string, params domain.ScanParams) ([]domain.FileItem, error) {
	var files []domain.FileItem

	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking even if there's an error
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check file extension
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".arrow" || ext == ".jsonl" {
			// Apply extension filter
			if params.Ext != "any" {
				if params.Ext == "arrow" && ext != ".arrow" {
					return nil
				}
				if params.Ext == "jsonl" && ext != ".jsonl" {
					return nil
				}
			}

			// Extract symbol from path if not specified in params
			symbol := params.Symbol
			if symbol == "" {
				symbol = fs.extractSymbolFromPath(path, params.Exchange, params.Source, params.Category)
			}

			files = append(files, domain.FileItem{
				Path:    path,
				Size:    info.Size(),
				ModTime: info.ModTime(),
				Symbol:  symbol,
				Ext:     strings.TrimPrefix(ext, "."),
			})
		}

		return nil
	})

	return files, err
}

// extractSymbolFromPath extracts symbol from file path
func (fs *FileScanner) extractSymbolFromPath(path, exchange, source, category string) string {
	// Try to extract symbol from path structure
	// Expected: .../exchange/source/category/symbol/dt=.../...
	parts := strings.Split(filepath.Clean(path), string(filepath.Separator))

	// Find the category index
	for i, part := range parts {
		if part == category && i+1 < len(parts) {
			// Next part should be symbol
			symbol := parts[i+1]
			// Skip if it starts with dt= (date partition)
			if !strings.HasPrefix(symbol, "dt=") {
				return symbol
			}
		}
	}

	return "unknown"
}

// findAllCategoryFiles handles "All books", "All trades" etc.
func (fs *FileScanner) findAllCategoryFiles(ctx context.Context, params domain.ScanParams, dates, hours []string, sourceDirs []string) ([]domain.FileItem, error) {
	var allFiles []domain.FileItem

	// Extract category from "All books" -> "books"
	category := strings.TrimPrefix(params.Category, "All ")

	for _, sourceDir := range sourceDirs {
		// Get all symbols for this category
		categoryPath := filepath.Join(params.BasePath, "data", params.Exchange, sourceDir, category)
		symbols, err := fs.getSymbolsInCategory(categoryPath)
		if err != nil {
			fs.logger.Debug("Failed to get symbols", zap.String("path", categoryPath), zap.Error(err))
			continue
		}

		// Scan each symbol
		for _, symbol := range symbols {
			for _, date := range dates {
				for _, hour := range hours {
					// Check if context is cancelled
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					default:
					}

					// Build path
					basePath := filepath.Join(categoryPath, symbol, fmt.Sprintf("dt=%s", date))
					if hour != "" {
						basePath = filepath.Join(basePath, fmt.Sprintf("hour=%s", hour))
					}

					// Scan this path
					files, err := fs.scanPath(basePath, params)
					if err != nil {
						continue
					}

					// Set metadata
					for i := range files {
						files[i].Exchange = params.Exchange
						files[i].Source = normalizeSource(sourceDir)
						files[i].Category = category
						files[i].Symbol = symbol
						files[i].Date = date
						files[i].Hour = hour
					}

					allFiles = append(allFiles, files...)
				}
			}
		}
	}

	return allFiles, nil
}

// getSymbolsInCategory returns all symbol directories in a category path
func (fs *FileScanner) getSymbolsInCategory(categoryPath string) ([]string, error) {
	entries, err := os.ReadDir(categoryPath)
	if err != nil {
		return nil, err
	}

	var symbols []string
	for _, entry := range entries {
		if entry.IsDir() {
			symbols = append(symbols, entry.Name())
		}
	}

	return symbols, nil
}

// applyFilter applies filename substring filter
func (fs *FileScanner) applyFilter(files []domain.FileItem, filter string) []domain.FileItem {
	if filter == "" {
		return files
	}

	var filtered []domain.FileItem
	filterLower := strings.ToLower(filter)

	for _, file := range files {
		filename := strings.ToLower(filepath.Base(file.Path))
		if strings.Contains(filename, filterLower) ||
			strings.Contains(strings.ToLower(file.Symbol), filterLower) ||
			strings.Contains(strings.ToLower(file.Date), filterLower) {
			filtered = append(filtered, file)
		}
	}

	return filtered
}