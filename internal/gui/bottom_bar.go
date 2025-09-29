package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

// CreateBottomBar creates the bottom status bar showing recent config activity.
func CreateBottomBar(status binding.String) fyne.CanvasObject {
	statusLabel := widget.NewLabelWithData(status)
	statusLabel.Importance = widget.MediumImportance
	statusLabel.Alignment = fyne.TextAlignLeading

	return container.NewHBox(statusLabel)
}
