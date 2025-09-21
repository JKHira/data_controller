//go:build gui
// +build gui

package panels

import (
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/state"
	"github.com/trade-engine/data-controller/internal/ui/components"
)

// FilesPanel manages the file browser and filtering UI
type FilesPanel struct {
	logger    *zap.Logger
	appState  *state.AppState
	window    fyne.Window

	// UI components
	startDateEntry *widget.Entry
	endDateEntry   *widget.Entry
	channelSelect  *widget.Select
	symbolSelect   *widget.Select
	filterBtn      *widget.Button
	showAllBtn     *widget.Button
	loadContentBtn *widget.Button
	filesList      *widget.List

	// Callbacks
	onFileSelected func(filePath string)
}

func NewFilesPanel(logger *zap.Logger, appState *state.AppState, window fyne.Window) *FilesPanel {
	fp := &FilesPanel{
		logger:   logger,
		appState: appState,
		window:   window,
	}

	fp.createComponents()
	return fp
}

func (fp *FilesPanel) createComponents() {
	// Date entries
	today := time.Now()
	fp.startDateEntry = widget.NewEntry()
	fp.startDateEntry.SetText(today.Format("2006-01-02"))
	fp.startDateEntry.SetPlaceHolder("YYYY-MM-DD")

	fp.endDateEntry = widget.NewEntry()
	fp.endDateEntry.SetText(today.Format("2006-01-02"))
	fp.endDateEntry.SetPlaceHolder("YYYY-MM-DD")

	// Filter selectors
	fp.channelSelect = widget.NewSelect([]string{"", "ticker", "trades", "books", "raw_books"}, nil)
	fp.channelSelect.SetSelected("")

	fp.symbolSelect = widget.NewSelect([]string{"", "tBTCUSD", "tETHUSD", "tLTCUSD"}, nil)
	fp.symbolSelect.SetSelected("")

	// Action buttons
	fp.filterBtn = widget.NewButton("🔍 Filter", fp.handleFilter)
	fp.showAllBtn = widget.NewButton("📁 Show All", fp.handleShowAll)
	fp.loadContentBtn = widget.NewButton("📖 Load Content", fp.handleLoadContent)

	// Files list
	fp.filesList = widget.NewList(
		func() int { return len(fp.appState.GetFilesData()) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			files := fp.appState.GetFilesData()
			if id < len(files) {
				label := obj.(*widget.Label)
				label.SetText(filepath.Base(files[id]))
			}
		},
	)
	fp.filesList.OnSelected = fp.handleFileSelection
}

func (fp *FilesPanel) handleFilter() {
	startDate, err := time.Parse("2006-01-02", fp.startDateEntry.Text)
	if err != nil {
		startDate = time.Time{}
	}

	endDate, err := time.Parse("2006-01-02", fp.endDateEntry.Text)
	if err != nil {
		endDate = time.Time{}
	}

	fp.appState.SetFilter(startDate, endDate, fp.channelSelect.Selected, fp.symbolSelect.Selected)

	if err := fp.appState.UpdateFilesList(); err != nil {
		fp.logger.Error("Failed to update files list", zap.Error(err))
	}

	fp.filesList.Refresh()
}

func (fp *FilesPanel) handleShowAll() {
	fp.startDateEntry.SetText("")
	fp.endDateEntry.SetText("")
	fp.channelSelect.SetSelected("")
	fp.symbolSelect.SetSelected("")

	fp.appState.ClearFilter()

	if err := fp.appState.UpdateFilesList(); err != nil {
		fp.logger.Error("Failed to update files list", zap.Error(err))
	}

	fp.filesList.Refresh()
}

func (fp *FilesPanel) handleLoadContent() {
	if fp.appState.CurrentFilePath == "" {
		return
	}

	if fp.onFileSelected != nil {
		fp.onFileSelected(fp.appState.CurrentFilePath)
	}
}

func (fp *FilesPanel) handleFileSelection(id widget.ListItemID) {
	files := fp.appState.GetFilesData()
	if id >= len(files) {
		return
	}

	filePath := files[id]
	fp.appState.SetSelectedFile(int(id), filePath)
}

func (fp *FilesPanel) SetOnFileSelected(callback func(filePath string)) {
	fp.onFileSelected = callback
}

func (fp *FilesPanel) CreateWidget() *widget.Card {
	// Create date picker buttons
	startDatePickerBtn := widget.NewButton("📅", func() {
		components.ShowDatePicker("Start Date", fp.startDateEntry, fp.window)
	})
	startDatePickerBtn.Resize(fyne.NewSize(30, 30))

	endDatePickerBtn := widget.NewButton("📅", func() {
		components.ShowDatePicker("End Date", fp.endDateEntry, fp.window)
	})
	endDatePickerBtn.Resize(fyne.NewSize(30, 30))

	// Create GridWrapLayout containers to guarantee minimum width for date entries
	startEntryWrap := container.New(layout.NewGridWrapLayout(fyne.NewSize(140, 32)), fp.startDateEntry)
	endEntryWrap := container.New(layout.NewGridWrapLayout(fyne.NewSize(140, 32)), fp.endDateEntry)

	// Date controls
	dateControls := container.NewHBox(
		widget.NewLabel("Start Date:"),
		startEntryWrap,
		startDatePickerBtn,
		widget.NewSeparator(),
		widget.NewLabel("End Date:"),
		endEntryWrap,
		endDatePickerBtn,
	)

	// Filter type controls
	filterTypeControls := container.NewHBox(
		widget.NewLabel("Channel:"),
		fp.channelSelect,
		widget.NewSeparator(),
		widget.NewLabel("Symbol:"),
		fp.symbolSelect,
	)

	// Action controls
	actionControls := container.NewHBox(
		fp.filterBtn,
		fp.showAllBtn,
		fp.loadContentBtn,
	)

	// Combine filter controls
	filterControls := container.NewVBox(
		dateControls,
		filterTypeControls,
		actionControls,
		widget.NewSeparator(),
	)

	// Files list with scroll
	filesListScroll := container.NewVScroll(fp.filesList)

	// Use Border layout for proper expansion
	filesContent := container.NewBorder(
		filterControls, // top
		nil,            // bottom
		nil,            // left
		nil,            // right
		filesListScroll,// center (takes remaining space)
	)

	return widget.NewCard("📁 Data Files", "", filesContent)
}

func (fp *FilesPanel) RefreshFilesList() {
	if err := fp.appState.UpdateFilesList(); err != nil {
		fp.logger.Error("Failed to update files list", zap.Error(err))
	}
	fp.filesList.Refresh()
}