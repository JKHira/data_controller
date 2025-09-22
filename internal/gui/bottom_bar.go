package gui

import (
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// BottomBar creates the bottom status bar with symbols and storage info
func CreateBottomBar(symbols []string, storagePath string) fyne.CanvasObject {
	return container.NewHBox(
		widget.NewLabel("🎯 Symbols:"),
		widget.NewLabel(strings.Join(symbols, ", ")),
		widget.NewSeparator(),
		widget.NewLabel("💾 Storage:"),
		widget.NewLabel(filepath.Base(storagePath)),
	)
}