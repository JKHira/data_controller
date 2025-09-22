package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

// TopBar creates the top information bar with status
func CreateTopBar(statusBinding binding.String) fyne.CanvasObject {
	statusLabel := widget.NewLabelWithData(statusBinding)
	statusLabel.Importance = widget.MediumImportance

	return container.NewHBox(
		widget.NewLabel("Status:"),
		statusLabel,
	)
}