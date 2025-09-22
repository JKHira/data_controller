package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// CreateDataFilesPanel creates the data files browser panel with all filter controls
func CreateDataFilesPanel(filesList *widget.List, startDateEntry, endDateEntry *widget.Entry,
	channelSelect, symbolSelect *widget.Select, filterBtn, showAllBtn, loadContentBtn *widget.Button,
	onDatePicker func(string, *widget.Entry)) fyne.CanvasObject {

	// Create date picker buttons for easier date selection
	startDatePickerBtn := widget.NewButton("📅", func() {
		onDatePicker("Start Date", startDateEntry)
	})
	startDatePickerBtn.Resize(fyne.NewSize(30, 30))

	endDatePickerBtn := widget.NewButton("📅", func() {
		onDatePicker("End Date", endDateEntry)
	})
	endDatePickerBtn.Resize(fyne.NewSize(30, 30))

	// Create GridWrapLayout containers to guarantee minimum width for date entries
	startEntryWrap := container.New(layout.NewGridWrapLayout(fyne.NewSize(140, 32)), startDateEntry)
	endEntryWrap := container.New(layout.NewGridWrapLayout(fyne.NewSize(140, 32)), endDateEntry)

	// Create filter controls layout with better spacing and picker buttons
	dateControls := container.NewHBox(
		widget.NewLabel("Start Date:"),
		startEntryWrap,
		startDatePickerBtn,
		widget.NewSeparator(),
		widget.NewLabel("End Date:"),
		endEntryWrap,
		endDatePickerBtn,
	)

	filterTypeControls := container.NewHBox(
		widget.NewLabel("Channel:"),
		channelSelect,
		widget.NewSeparator(),
		widget.NewLabel("Symbol:"),
		symbolSelect,
	)

	actionControls := container.NewHBox(
		filterBtn,
		showAllBtn,
		loadContentBtn,
	)

	filterControls := container.NewVBox(
		dateControls,
		filterTypeControls,
		actionControls,
		widget.NewSeparator(),
	)

	// Files list with filter controls - use Border layout for proper expansion
	filesListScroll := container.NewVScroll(filesList)
	filesContent := container.NewBorder(
		filterControls, // top
		nil,            // bottom
		nil,            // left
		nil,            // right
		filesListScroll,// center (takes remaining space)
	)

	return widget.NewCard("📁 Data Files", "", filesContent)
}