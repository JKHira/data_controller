//go:build gui
// +build gui

package components

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
)

// ShowDatePicker displays a date picker dialog
func ShowDatePicker(title string, targetEntry *widget.Entry, window fyne.Window) {
	today := time.Now()

	// Create year selector
	years := make([]string, 11) // Show 5 years back and 5 years forward
	currentYear := today.Year()
	for i := 0; i < 11; i++ {
		years[i] = fmt.Sprintf("%d", currentYear-5+i)
	}

	// Create month selector
	months := []string{
		"01", "02", "03", "04", "05", "06",
		"07", "08", "09", "10", "11", "12",
	}

	// Create day selector
	days := make([]string, 31)
	for i := 1; i <= 31; i++ {
		days[i-1] = fmt.Sprintf("%02d", i)
	}

	yearSelect := widget.NewSelect(years, nil)
	monthSelect := widget.NewSelect(months, nil)
	daySelect := widget.NewSelect(days, nil)

	// Set default values to current date
	yearSelect.SetSelected(fmt.Sprintf("%d", currentYear))
	monthSelect.SetSelected(fmt.Sprintf("%02d", today.Month()))
	daySelect.SetSelected(fmt.Sprintf("%02d", today.Day()))

	// Add OK button functionality
	okButton := widget.NewButton("OK", func() {
		year := yearSelect.Selected
		month := monthSelect.Selected
		day := daySelect.Selected

		if year != "" && month != "" && day != "" {
			selectedDate := fmt.Sprintf("%s-%s-%s", year, month, day)
			targetEntry.SetText(selectedDate)
		}
	})

	// Create dialog content with all components including buttons
	dialogContent := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Select %s", title)),
		widget.NewSeparator(),
		widget.NewLabel("Year:"),
		yearSelect,
		widget.NewLabel("Month:"),
		monthSelect,
		widget.NewLabel("Day:"),
		daySelect,
		widget.NewSeparator(),
		container.NewHBox(
			widget.NewButton("Today", func() {
				today := time.Now()
				selectedDate := today.Format("2006-01-02")
				targetEntry.SetText(selectedDate)
			}),
			okButton,
		),
	)

	// Create dialog
	pickerDialog := dialog.NewCustomConfirm(title, "OK", "Cancel", dialogContent,
		func(confirmed bool) {
			if confirmed {
				year := yearSelect.Selected
				month := monthSelect.Selected
				day := daySelect.Selected

				if year != "" && month != "" && day != "" {
					selectedDate := fmt.Sprintf("%s-%s-%s", year, month, day)
					targetEntry.SetText(selectedDate)
				}
			}
		}, window)
	pickerDialog.Resize(fyne.NewSize(300, 400))
	pickerDialog.Show()
}