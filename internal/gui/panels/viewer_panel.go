package panels

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/trade-engine/data-controller/internal/gui/controllers"
	"github.com/trade-engine/data-controller/internal/gui/state"
)

// ViewerPanel handles the file viewer interface
type ViewerPanel struct {
	state          *state.AppState
	fileController *controllers.FileController

	// UI components
	fileViewer     *widget.Entry
	metadataViewer *widget.Entry
	prevBtn        *widget.Button
	nextBtn        *widget.Button
	closeBtn       *widget.Button
	pageLabel      *widget.Label
}

// NewViewerPanel creates a new viewer panel
func NewViewerPanel(appState *state.AppState, fileController *controllers.FileController) *ViewerPanel {
	panel := &ViewerPanel{
		state:          appState,
		fileController: fileController,
	}

	panel.createUI()
	panel.setupController()
	return panel
}

// createUI creates the viewer panel UI components
func (vp *ViewerPanel) createUI() {
	// Create file viewer with pagination controls
	vp.fileViewer = widget.NewMultiLineEntry()
	vp.fileViewer.SetPlaceHolder("Select a file to view its content...")
	vp.fileViewer.Wrapping = fyne.TextWrapWord
	// Use read-only mode instead of Disable() to maintain normal text color
	if readOnlyEntry, ok := interface{}(vp.fileViewer).(interface{ SetReadOnly(bool) }); ok {
		readOnlyEntry.SetReadOnly(true)
	}

	// Metadata viewer (fixed square area)
	vp.metadataViewer = widget.NewMultiLineEntry()
	vp.metadataViewer.SetPlaceHolder("Metadata will appear after loading an Arrow file...")
	vp.metadataViewer.Wrapping = fyne.TextWrapWord
	if readOnlyEntry, ok := interface{}(vp.metadataViewer).(interface{ SetReadOnly(bool) }); ok {
		readOnlyEntry.SetReadOnly(true)
	}

	// Create pagination controls
	vp.prevBtn = widget.NewButton("◀ Previous", vp.handlePreviousPage)
	vp.nextBtn = widget.NewButton("Next ▶", vp.handleNextPage)
	vp.closeBtn = widget.NewButton("✕ Close", vp.handleCloseFile)
	vp.pageLabel = widget.NewLabel("Page 0/0")

	vp.prevBtn.Disable()
	vp.nextBtn.Disable()
	vp.closeBtn.Disable()
}

// setupController connects the UI components to the controller
func (vp *ViewerPanel) setupController() {
	vp.fileController.SetUIComponents(
		vp.fileViewer,
		vp.metadataViewer,
		vp.pageLabel,
		vp.prevBtn,
		vp.nextBtn,
		vp.closeBtn,
	)
}

// GetContent returns the viewer panel content
func (vp *ViewerPanel) GetContent() fyne.CanvasObject {
	// File viewer with controls - use Border layout for proper expansion
	viewerControls := container.NewHBox(
		vp.prevBtn,
		vp.nextBtn,
		widget.NewSeparator(),
		vp.pageLabel,
		widget.NewSeparator(),
		vp.closeBtn,
	)
	metadataScroll := container.NewVScroll(vp.metadataViewer)
	metadataScroll.SetMinSize(fyne.NewSize(220, 220))
	metadataCard := widget.NewCard("📑 Metadata", "", metadataScroll)
	viewerScroll := container.NewVScroll(vp.fileViewer)
	contentBody := container.NewBorder(metadataCard, nil, nil, nil, viewerScroll)
	viewerContent := container.NewBorder(
		viewerControls,
		nil,
		nil,
		nil,
		contentBody,
	)
	return widget.NewCard("👁️ File Viewer", "", viewerContent)
}

// GetFileViewer returns the file viewer widget for external reference
func (vp *ViewerPanel) GetFileViewer() *widget.Entry {
	return vp.fileViewer
}

// handlePreviousPage handles previous page button clicks
func (vp *ViewerPanel) handlePreviousPage() {
	vp.fileController.HandlePreviousPage()
}

// handleNextPage handles next page button clicks
func (vp *ViewerPanel) handleNextPage() {
	vp.fileController.HandleNextPage()
}

// handleCloseFile handles close button clicks
func (vp *ViewerPanel) handleCloseFile() {
	vp.fileController.HandleCloseFile()
}
