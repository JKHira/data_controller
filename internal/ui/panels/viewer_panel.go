//go:build gui
// +build gui

package panels

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/state"
	"github.com/trade-engine/data-controller/internal/services"
)

// ViewerPanel manages the file content viewer UI
type ViewerPanel struct {
	logger   *zap.Logger
	appState *state.AppState

	// UI components
	fileViewer *widget.Entry
	prevBtn    *widget.Button
	nextBtn    *widget.Button
	closeBtn   *widget.Button
	pageLabel  *widget.Label

	// Current page data
	currentPageData *services.PageData
}

func NewViewerPanel(logger *zap.Logger, appState *state.AppState) *ViewerPanel {
	vp := &ViewerPanel{
		logger:   logger,
		appState: appState,
	}

	vp.createComponents()
	return vp
}

func (vp *ViewerPanel) createComponents() {
	// File viewer
	vp.fileViewer = widget.NewMultiLineEntry()
	vp.fileViewer.SetPlaceHolder("Select a file to view its content...")
	vp.fileViewer.Wrapping = fyne.TextWrapWord
	vp.fileViewer.Disable() // Read-only

	// Pagination controls
	vp.prevBtn = widget.NewButton("◀ Previous", vp.handlePreviousPage)
	vp.nextBtn = widget.NewButton("Next ▶", vp.handleNextPage)
	vp.closeBtn = widget.NewButton("✕ Close", vp.handleClose)
	vp.pageLabel = widget.NewLabel("Page 0/0")

	// Initially disable pagination buttons
	vp.prevBtn.Disable()
	vp.nextBtn.Disable()
	vp.closeBtn.Disable()
}

func (vp *ViewerPanel) LoadFile(filePath string) {
	vp.logger.Info("Loading file content", zap.String("file", filePath))

	pageData, err := vp.appState.FileReader.ReadFileWithPagination(filePath, 1, vp.appState.PageSize)
	if err != nil {
		vp.logger.Error("Failed to read file", zap.String("file", filePath), zap.Error(err))
		vp.fileViewer.SetText(fmt.Sprintf("❌ Error reading file: %v", err))
		return
	}

	vp.currentPageData = pageData
	vp.appState.CurrentPage = pageData.PageNumber
	vp.appState.TotalPages = pageData.TotalPages

	// Format and display content
	content := vp.formatRecords(pageData.Records)
	vp.fileViewer.SetText(content)

	// Update pagination controls
	vp.updatePaginationControls()

	vp.logger.Info("File content loaded successfully",
		zap.String("file", filePath),
		zap.Int("records", len(pageData.Records)),
		zap.Int("page", pageData.PageNumber),
		zap.Int("totalPages", pageData.TotalPages))
}

func (vp *ViewerPanel) formatRecords(records []map[string]interface{}) string {
	if len(records) == 0 {
		return "No data available"
	}

	var builder strings.Builder
	builder.WriteString(fmt.Sprintf("📊 Records: %d\n", len(records)))
	builder.WriteString("═══════════════════════════════════════\n\n")

	for i, record := range records {
		if i >= 50 { // Limit display to first 50 records per page
			builder.WriteString(fmt.Sprintf("... and %d more records (use pagination to view more)\n", len(records)-i))
			break
		}

		builder.WriteString(fmt.Sprintf("Record #%d:\n", i+1))
		for key, value := range record {
			builder.WriteString(fmt.Sprintf("  %s: %v\n", key, value))
		}
		builder.WriteString("───────────────────────────────────────\n")
	}

	return builder.String()
}

func (vp *ViewerPanel) handlePreviousPage() {
	if vp.appState.CurrentFilePath == "" || vp.appState.CurrentPage <= 1 {
		return
	}

	vp.loadPage(vp.appState.CurrentPage - 1)
}

func (vp *ViewerPanel) handleNextPage() {
	if vp.appState.CurrentFilePath == "" || vp.appState.CurrentPage >= vp.appState.TotalPages {
		return
	}

	vp.loadPage(vp.appState.CurrentPage + 1)
}

func (vp *ViewerPanel) loadPage(pageNumber int) {
	pageData, err := vp.appState.FileReader.ReadFileWithPagination(
		vp.appState.CurrentFilePath, pageNumber, vp.appState.PageSize)
	if err != nil {
		vp.logger.Error("Failed to read page", zap.Int("page", pageNumber), zap.Error(err))
		return
	}

	vp.currentPageData = pageData
	vp.appState.CurrentPage = pageData.PageNumber

	// Format and display content
	content := vp.formatRecords(pageData.Records)
	vp.fileViewer.SetText(content)

	// Update pagination controls
	vp.updatePaginationControls()
}

func (vp *ViewerPanel) handleClose() {
	vp.fileViewer.SetText("Select a file to view its content...")
	vp.appState.CurrentFilePath = ""
	vp.appState.CurrentPage = 1
	vp.currentPageData = nil

	vp.prevBtn.Disable()
	vp.nextBtn.Disable()
	vp.closeBtn.Disable()
	vp.pageLabel.SetText("Page 0/0")
}

func (vp *ViewerPanel) updatePaginationControls() {
	if vp.currentPageData == nil {
		return
	}

	// Update page label
	vp.pageLabel.SetText(fmt.Sprintf("Page %d/%d (%.1fMB/%.1fMB)",
		vp.currentPageData.PageNumber,
		vp.currentPageData.TotalPages,
		float64(vp.currentPageData.BytesRead)/(1024*1024),
		float64(vp.currentPageData.TotalBytes)/(1024*1024)))

	// Update button states
	if vp.currentPageData.HasPrev {
		vp.prevBtn.Enable()
	} else {
		vp.prevBtn.Disable()
	}

	if vp.currentPageData.HasNext {
		vp.nextBtn.Enable()
	} else {
		vp.nextBtn.Disable()
	}

	vp.closeBtn.Enable()
}

func (vp *ViewerPanel) CreateWidget() *widget.Card {
	// Viewer controls
	viewerControls := container.NewHBox(
		vp.prevBtn,
		vp.nextBtn,
		widget.NewSeparator(),
		vp.pageLabel,
		widget.NewSeparator(),
		vp.closeBtn,
	)

	// Viewer with scroll
	viewerScroll := container.NewVScroll(vp.fileViewer)

	// Use Border layout for proper expansion
	viewerContent := container.NewBorder(
		viewerControls, // top
		nil,            // bottom
		nil,            // left
		nil,            // right
		viewerScroll,   // center (takes remaining space)
	)

	return widget.NewCard("👁️ File Viewer", "", viewerContent)
}