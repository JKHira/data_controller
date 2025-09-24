package controllers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/domain"
	"github.com/trade-engine/data-controller/internal/gui/state"
	"github.com/trade-engine/data-controller/internal/sink/arrow"
)

// FileController manages file operations and state
type FileController struct {
	logger      *zap.Logger
	cfg         *config.Config
	state       *state.AppState
	arrowReader *arrow.FileReader

	// UI components
	fileViewer *widget.Entry
	pageLabel  *widget.Label
	prevBtn    *widget.Button
	nextBtn    *widget.Button
	closeBtn   *widget.Button
}

// NewFileController creates a new file controller
func NewFileController(
	logger *zap.Logger,
	cfg *config.Config,
	appState *state.AppState,
	arrowReader *arrow.FileReader,
) *FileController {
	return &FileController{
		logger:      logger,
		cfg:         cfg,
		state:       appState,
		arrowReader: arrowReader,
	}
}

// SetUIComponents sets the UI components that this controller manages
func (fc *FileController) SetUIComponents(
	fileViewer *widget.Entry,
	pageLabel *widget.Label,
	prevBtn, nextBtn, closeBtn *widget.Button,
) {
	fc.fileViewer = fileViewer
	fc.pageLabel = pageLabel
	fc.prevBtn = prevBtn
	fc.nextBtn = nextBtn
	fc.closeBtn = closeBtn
}

// UpdateFileList refreshes the file list from disk
func (fc *FileController) UpdateFileList() {
	dataPath := fc.cfg.Storage.BasePath
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		fc.state.FilesData = make([]domain.FileItem, 0)
		fc.state.FilteredFiles = make([]domain.FileItem, 0)
		return
	}

	arrowFiles, err := fc.arrowReader.ScanDataFiles(dataPath)
	if err != nil {
		fc.logger.Error("Failed to scan data files", zap.Error(err))
		fc.state.FilesData = make([]domain.FileItem, 0)
		fc.state.FilteredFiles = make([]domain.FileItem, 0)
		return
	}

	// Convert arrow.FileInfo to domain.FileItem
	files := make([]domain.FileItem, len(arrowFiles))
	for i, arrowFile := range arrowFiles {
		files[i] = domain.FileItem{
			Path:     arrowFile.Path,
			Size:     arrowFile.Size,
			ModTime:  arrowFile.ModTime,
			Exchange: arrowFile.Exchange,
			Source:   string(arrowFile.SourceType),
			Category: arrowFile.Category,
			Symbol:   "", // Will be extracted from path if needed
			Date:     "", // Will be extracted from path if needed
			Hour:     "", // Will be extracted from path if needed
			Ext:      strings.TrimPrefix(filepath.Ext(arrowFile.Path), "."),
		}
	}

	fc.state.FilesData = files
	fc.state.FilteredFiles = files
}

// HandleFileSelection handles single-click file selection
func (fc *FileController) HandleFileSelection(fileInfo arrow.FileInfo) {
	fc.logger.Info("GUI: File selected (single click)", zap.String("file", fileInfo.Path))

	info, err := os.Stat(fileInfo.Path)
	if err != nil {
		fc.fileViewer.SetText(fmt.Sprintf("❌ Error reading file: %v", err))
		return
	}

	content := fc.buildFileInfoContent(fileInfo, info)
	fc.fileViewer.SetText(content)
}

// HandleFileSelectionDomain handles single-click file selection for domain.FileItem
func (fc *FileController) HandleFileSelectionDomain(fileItem domain.FileItem) {
	fc.logger.Info("GUI: File selected (single click)", zap.String("file", fileItem.Path))

	info, err := os.Stat(fileItem.Path)
	if err != nil {
		fc.fileViewer.SetText(fmt.Sprintf("❌ Error reading file: %v", err))
		return
	}

	content := fc.buildFileInfoContentDomain(fileItem, info)
	fc.fileViewer.SetText(content)
}

// HandleFileDoubleClick handles double-click file opening
func (fc *FileController) HandleFileDoubleClick(fileInfo arrow.FileInfo) {
	fc.logger.Info("GUI: File double-clicked", zap.String("file", fileInfo.Path))

	fc.state.SetCurrentFile(fileInfo.Path)

	ext := strings.ToLower(filepath.Ext(fileInfo.Path))
	if ext == ".arrow" || ext == ".parquet" {
		fc.loadArrowFileData()
		return
	}

	fc.displayFileInfo()
}

// HandleFileDoubleClickDomain handles double-click file opening for domain.FileItem
func (fc *FileController) HandleFileDoubleClickDomain(fileItem domain.FileItem) {
	fc.logger.Info("GUI: File double-clicked", zap.String("file", fileItem.Path))

	fc.state.SetCurrentFile(fileItem.Path)

	ext := strings.ToLower(filepath.Ext(fileItem.Path))
	if ext == ".arrow" || ext == ".parquet" {
		fc.loadArrowFileData()
		return
	}

	fc.displayFileInfo()
}

// HandlePreviousPage handles previous page navigation
func (fc *FileController) HandlePreviousPage() {
	if !fc.state.CanNavigatePrevious() {
		return
	}

	fc.state.CurrentPage--
	fc.loadArrowFileData()
	fc.updatePageControls()
}

// HandleNextPage handles next page navigation
func (fc *FileController) HandleNextPage() {
	if !fc.state.CanNavigateNext() {
		return
	}

	fc.state.CurrentPage++
	fc.loadArrowFileData()
	fc.updatePageControls()
}

// HandleCloseFile handles file viewer close
func (fc *FileController) HandleCloseFile() {
	fc.fileViewer.SetText("")
	fc.state.SetCurrentFile("")
	fc.state.SetPageInfo(0, 0)

	fc.prevBtn.Disable()
	fc.nextBtn.Disable()
	fc.closeBtn.Disable()
	fc.updatePageControls()
}

// FilterFiles filters the file list based on provided filter function
func (fc *FileController) FilterFiles(filterFunc func(arrow.FileInfo) bool) {
	filteredFiles := make([]domain.FileItem, 0)
	for _, fileItem := range fc.state.FilesData {
		// Convert domain.FileItem back to arrow.FileInfo for compatibility
		arrowFileInfo := arrow.FileInfo{
			Path:       fileItem.Path,
			Size:       fileItem.Size,
			ModTime:    fileItem.ModTime,
			Exchange:   fileItem.Exchange,
			SourceType: arrow.SourceType(fileItem.Source),
			Category:   fileItem.Category,
		}
		if filterFunc(arrowFileInfo) {
			filteredFiles = append(filteredFiles, fileItem)
		}
	}
	fc.state.FilteredFiles = filteredFiles
}

// buildFileInfoContent creates formatted file information display
func (fc *FileController) buildFileInfoContent(fileInfo arrow.FileInfo, info os.FileInfo) string {
	content := fmt.Sprintf("📄 File: %s\n", filepath.Base(fileInfo.Path))
	content += fmt.Sprintf("📁 Path: %s\n", fileInfo.Path)
	content += fmt.Sprintf("📏 Size: %d bytes (%.2f MB)\n", info.Size(), float64(info.Size())/(1024*1024))
	content += fmt.Sprintf("🕒 Modified: %s\n", info.ModTime().Format(time.RFC3339))
	content += fmt.Sprintf("🏷️ Type: %s\n\n", filepath.Ext(fileInfo.Path))

	// Add metadata info
	if fileInfo.Exchange != "" {
		content += fmt.Sprintf("🏦 Exchange: %s\n", fileInfo.Exchange)
	}
	if fileInfo.Channel != "" {
		content += fmt.Sprintf("📡 Channel: %s\n", fileInfo.Channel)
	}
	if fileInfo.SourceType != "" {
		content += fmt.Sprintf("🔄 Source: %s\n", string(fileInfo.SourceType))
	}
	if fileInfo.Category != "" {
		content += fmt.Sprintf("📂 Category: %s\n", fileInfo.Category)
	}
	content += "\n"

	if strings.HasSuffix(fileInfo.Path, ".arrow") {
		content += "📊 This is an Arrow file containing trading data.\n"
		content += "💡 Double-click to view contents (10MB chunks)\n"
		content += "🔍 File contains structured data with timestamps, prices, and volumes.\n"
	}

	return content
}

// buildFileInfoContentDomain creates formatted file information display for domain.FileItem
func (fc *FileController) buildFileInfoContentDomain(fileItem domain.FileItem, info os.FileInfo) string {
	content := fmt.Sprintf("📄 File: %s\n", filepath.Base(fileItem.Path))
	content += fmt.Sprintf("📁 Path: %s\n", fileItem.Path)
	content += fmt.Sprintf("📏 Size: %d bytes (%.2f MB)\n", info.Size(), float64(info.Size())/(1024*1024))
	content += fmt.Sprintf("🕒 Modified: %s\n", info.ModTime().Format(time.RFC3339))
	content += fmt.Sprintf("🏷️ Type: %s\n\n", filepath.Ext(fileItem.Path))

	// Add metadata info from domain.FileItem
	if fileItem.Exchange != "" {
		content += fmt.Sprintf("🏦 Exchange: %s\n", fileItem.Exchange)
	}
	if fileItem.Source != "" {
		content += fmt.Sprintf("📡 Source: %s\n", fileItem.Source)
	}
	if fileItem.Category != "" {
		content += fmt.Sprintf("📂 Category: %s\n", fileItem.Category)
	}
	if fileItem.Symbol != "" {
		content += fmt.Sprintf("💱 Symbol: %s\n", fileItem.Symbol)
	}
	if fileItem.Date != "" {
		content += fmt.Sprintf("📅 Date: %s\n", fileItem.Date)
	}
	if fileItem.Hour != "" {
		content += fmt.Sprintf("🕐 Hour: %s\n", fileItem.Hour)
	}
	content += "\n"

	if strings.HasSuffix(fileItem.Path, ".arrow") {
		content += "📊 This is an Arrow file containing trading data.\n"
		content += "💡 Double-click to view contents (10MB chunks)\n"
		content += "🔍 File contains structured data with timestamps, prices, and volumes.\n"
	}

	return content
}

// loadArrowFileData loads Arrow file data with pagination
func (fc *FileController) loadArrowFileData() {
	pageData, err := fc.arrowReader.ReadArrowFileWithPagination(
		fc.state.CurrentFilePath,
		fc.state.CurrentPage,
		fc.state.PageSize,
	)
	if err != nil {
		fc.logger.Error("Failed to read Arrow file", zap.Error(err))
		fc.fileViewer.SetText(fmt.Sprintf("❌ Error reading file: %v", err))
		return
	}

	fc.state.SetPageInfo(fc.state.CurrentPage, pageData.TotalPages)
	fc.displayArrowData(pageData)
	fc.updatePageControls()
}

// displayFileInfo displays basic file information
func (fc *FileController) displayFileInfo() {
	content := fmt.Sprintf("📄 File: %s\n", filepath.Base(fc.state.CurrentFilePath))
	content += fmt.Sprintf("📁 Path: %s\n", fc.state.CurrentFilePath)
	content += fmt.Sprintf("🏷️ Type: %s\n\n", filepath.Ext(fc.state.CurrentFilePath))
	content += "🚫 This file type is not supported for data viewing.\n"

	fc.fileViewer.SetText(content)
}

// displayArrowData displays Arrow file data with formatting
func (fc *FileController) displayArrowData(pageData *arrow.PageData) {
	content := fmt.Sprintf("📊 Arrow File: %s\n", filepath.Base(fc.state.CurrentFilePath))
	content += fmt.Sprintf("📊 Data Read: %.2f MB / %.2f MB\n\n", float64(pageData.BytesRead)/(1024*1024), float64(pageData.TotalBytes)/(1024*1024))

	// Page data with 10MB chunk info
	content += fmt.Sprintf("📄 Chunk %d/%d (~10MB per chunk):\n", pageData.PageNumber, pageData.TotalPages)
	content += fmt.Sprintf("📊 Records in this chunk: %d\n", len(pageData.Records))
	content += fmt.Sprintf("💾 Bytes loaded: %.2f MB\n", float64(pageData.BytesRead)/(1024*1024))
	content += strings.Repeat("─", 80) + "\n\n"

	// Display records (show up to the requested page size)
	maxRecords := min(fc.state.PageSize, len(pageData.Records))
	for i := 0; i < maxRecords; i++ {
		record := pageData.Records[i]
		content += fmt.Sprintf("🔢 Record #%d:\n", i+1)

		// Show important fields first
		importantFields := []string{"exchange", "symbol", "ts_micros", "price", "amount", "bid", "ask", "last"}
		for _, fieldName := range importantFields {
			if value, exists := record[fieldName]; exists {
				if value == nil {
					content += fmt.Sprintf("  %s: <null>\n", fieldName)
				} else {
					content += fmt.Sprintf("  %s: %v\n", fieldName, value)
				}
			}
		}

		// Show other fields (limit to save space)
		otherFieldCount := 0
		for key, value := range record {
			// Skip already shown fields
			isImportant := false
			for _, imp := range importantFields {
				if key == imp {
					isImportant = true
					break
				}
			}
			if isImportant {
				continue
			}

			if otherFieldCount < 3 { // Show max 3 additional fields
				if value == nil {
					content += fmt.Sprintf("  %s: <null>\n", key)
				} else {
					content += fmt.Sprintf("  %s: %v\n", key, value)
				}
				otherFieldCount++
			}
		}
		content += "\n"
	}

	if len(pageData.Records) > maxRecords {
		content += fmt.Sprintf("... and %d more records in this chunk\n", len(pageData.Records) - maxRecords)
		content += "💡 Use Previous/Next buttons to navigate through data\n"
	}

	fc.fileViewer.SetText(content)
}

// updatePageControls updates the pagination control states
func (fc *FileController) updatePageControls() {
	fc.pageLabel.SetText(fc.state.GetCurrentPageLabel())

	if fc.state.CanNavigatePrevious() {
		fc.prevBtn.Enable()
	} else {
		fc.prevBtn.Disable()
	}

	if fc.state.CanNavigateNext() {
		fc.nextBtn.Enable()
	} else {
		fc.nextBtn.Disable()
	}

	if fc.state.CurrentFilePath != "" {
		fc.closeBtn.Enable()
	} else {
		fc.closeBtn.Disable()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}