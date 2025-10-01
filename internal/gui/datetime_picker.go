package gui

import (
	"fmt"
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// DateTimePicker provides a date and time selection widget
type DateTimePicker struct {
	widget.BaseWidget
	selectedDate time.Time
	onChanged    func(time.Time)

	// UI components
	dateLabel   *widget.Label
	timeEntry   *widget.Entry
	calendarBtn *widget.Button
	calendarWin fyne.Window
}

// NewDateTimePicker creates a new date/time picker initialized to current UTC time
func NewDateTimePicker(onChange func(time.Time)) *DateTimePicker {
	now := time.Now().UTC()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	picker := &DateTimePicker{
		selectedDate: startOfDay,
		onChanged:    onChange,
	}
	picker.ExtendBaseWidget(picker)
	return picker
}

// CreateRenderer creates the widget renderer
func (d *DateTimePicker) CreateRenderer() fyne.WidgetRenderer {
	// Date label showing selected date
	d.dateLabel = widget.NewLabel(d.selectedDate.Format("2006-01-02"))
	d.dateLabel.Alignment = fyne.TextAlignCenter

	// Time entry for HH:MM:SS input (120px width)
	d.timeEntry = widget.NewEntry()
	d.timeEntry.SetPlaceHolder("HH:MM:SS")
	d.timeEntry.SetText(d.selectedDate.Format("15:04:05"))
	d.timeEntry.OnChanged = func(s string) {
		d.updateTimeFromEntry()
	}

	// Calendar button to open calendar popup
	d.calendarBtn = widget.NewButton("📅", func() {
		d.showCalendar()
	})

	// Fixed-width container for time entry (120px)
	background := canvas.NewRectangle(color.Transparent)
	background.SetMinSize(fyne.NewSize(120, d.timeEntry.MinSize().Height))
	timeEntryContainer := container.NewMax(background, d.timeEntry)

	// Layout: [Date Label] [Calendar Button] [Time Entry]
	content := container.NewBorder(
		nil, nil,
		nil,
		d.calendarBtn,
		container.NewHBox(
			d.dateLabel,
			timeEntryContainer,
		),
	)

	return widget.NewSimpleRenderer(content)
}

// SetDateTime sets the current date/time value
func (d *DateTimePicker) SetDateTime(t time.Time) {
	d.selectedDate = t.UTC()
	// Update UI components only if they exist (after CreateRenderer is called)
	if d.dateLabel != nil {
		d.dateLabel.SetText(d.selectedDate.Format("2006-01-02"))
	}
	if d.timeEntry != nil {
		d.timeEntry.SetText(d.selectedDate.Format("15:04:05"))
	}
	d.Refresh()
}

// GetDateTime returns the currently selected date/time in UTC
func (d *DateTimePicker) GetDateTime() time.Time {
	return d.selectedDate
}

// updateTimeFromEntry parses time entry and updates selected date
func (d *DateTimePicker) updateTimeFromEntry() {
	timeStr := d.timeEntry.Text

	// Try parsing HH:MM:SS
	t, err := time.Parse("15:04:05", timeStr)
	if err != nil {
		// Try HH:MM
		t, err = time.Parse("15:04", timeStr)
		if err != nil {
			return // Invalid format, ignore
		}
	}

	// Combine date from selectedDate with new time
	year, month, day := d.selectedDate.Date()
	d.selectedDate = time.Date(
		year, month, day,
		t.Hour(), t.Minute(), t.Second(),
		0, time.UTC,
	)

	if d.onChanged != nil {
		d.onChanged(d.selectedDate)
	}
}

// showCalendar opens calendar selection popup
func (d *DateTimePicker) showCalendar() {
	if d.calendarWin != nil && d.calendarWin.Canvas() != nil {
		d.calendarWin.Show()
		return
	}

	app := fyne.CurrentApp()
	d.calendarWin = app.NewWindow("Select Date")
	d.calendarWin.Resize(fyne.NewSize(320, 400))

	// Create calendar grid
	cal := d.createCalendarGrid()

	// Month navigation
	monthLabel := widget.NewLabel(d.selectedDate.Format("January 2006"))
	monthLabel.Alignment = fyne.TextAlignCenter

	prevBtn := widget.NewButton("◀", func() {
		d.selectedDate = d.selectedDate.AddDate(0, -1, 0)
		d.refreshCalendar(cal, monthLabel)
	})

	nextBtn := widget.NewButton("▶", func() {
		d.selectedDate = d.selectedDate.AddDate(0, 1, 0)
		d.refreshCalendar(cal, monthLabel)
	})

	monthNav := container.NewBorder(
		nil, nil,
		prevBtn, nextBtn,
		monthLabel,
	)

	// Today and Close buttons
	todayBtn := widget.NewButton("Today", func() {
		now := time.Now().UTC()
		d.selectDate(now.Year(), int(now.Month()), now.Day())
		d.refreshCalendar(cal, monthLabel)
	})

	closeBtn := widget.NewButton("Close", func() {
		d.calendarWin.Hide()
	})

	actions := container.NewHBox(
		layout.NewSpacer(),
		todayBtn,
		closeBtn,
	)

	content := container.NewBorder(
		monthNav,
		actions,
		nil, nil,
		cal,
	)

	d.calendarWin.SetContent(content)
	d.calendarWin.Show()
}

// createCalendarGrid creates the calendar day grid
func (d *DateTimePicker) createCalendarGrid() *fyne.Container {
	// Day headers
	headers := []string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}
	headerWidgets := make([]fyne.CanvasObject, len(headers))
	for i, h := range headers {
		label := widget.NewLabel(h)
		label.Alignment = fyne.TextAlignCenter
		headerWidgets[i] = label
	}

	// Day buttons (max 6 weeks)
	dayButtons := make([]fyne.CanvasObject, 42)
	year, month, _ := d.selectedDate.Date()
	firstDay := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	startWeekday := int(firstDay.Weekday())
	daysInMonth := time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()

	dayNum := 1 - startWeekday
	for i := 0; i < 42; i++ {
		day := dayNum
		var btn *widget.Button

		if day < 1 || day > daysInMonth {
			// Empty cell for days outside current month
			btn = widget.NewButton("", nil)
			btn.Disable()
		} else {
			btn = widget.NewButton(fmt.Sprintf("%d", day), func() {
				d.selectDate(year, int(month), day)
				d.calendarWin.Hide()
			})

			// Highlight current selected day
			if day == d.selectedDate.Day() {
				btn.Importance = widget.HighImportance
			}
		}

		dayButtons[i] = btn
		dayNum++
	}

	// Combine headers and days
	allWidgets := append(headerWidgets, dayButtons...)

	return container.New(
		layout.NewGridLayout(7),
		allWidgets...,
	)
}

// refreshCalendar updates calendar grid with new month
func (d *DateTimePicker) refreshCalendar(cal *fyne.Container, monthLabel *widget.Label) {
	monthLabel.SetText(d.selectedDate.Format("January 2006"))

	// Recreate calendar grid
	newCal := d.createCalendarGrid()
	cal.Objects = newCal.Objects
	cal.Refresh()
}

// selectDate updates selected date maintaining current time
func (d *DateTimePicker) selectDate(year, month, day int) {
	hour, min, sec := d.selectedDate.Clock()
	d.selectedDate = time.Date(year, time.Month(month), day, hour, min, sec, 0, time.UTC)
	d.dateLabel.SetText(d.selectedDate.Format("2006-01-02"))
	d.Refresh()

	if d.onChanged != nil {
		d.onChanged(d.selectedDate)
	}
}

// TimeRangePicker provides start and end date/time selection
type TimeRangePicker struct {
	widget.BaseWidget
	startPicker *DateTimePicker
	endPicker   *DateTimePicker
	onChanged   func(start, end time.Time)
}

// NewTimeRangePicker creates a new time range picker
func NewTimeRangePicker(onChange func(start, end time.Time)) *TimeRangePicker {
	// Set default range: last 7 days, times normalised to 00:00:00 UTC
	now := time.Now().UTC()
	endTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	startBase := endTime.AddDate(0, 0, -7)

	tr := &TimeRangePicker{
		onChanged: onChange,
	}

	tr.startPicker = NewDateTimePicker(func(t time.Time) {
		if tr.onChanged != nil {
			tr.onChanged(tr.startPicker.GetDateTime(), tr.endPicker.GetDateTime())
		}
	})
	// Set initial time for startPicker
	tr.startPicker.selectedDate = startBase

	tr.endPicker = NewDateTimePicker(func(t time.Time) {
		if tr.onChanged != nil {
			tr.onChanged(tr.startPicker.GetDateTime(), tr.endPicker.GetDateTime())
		}
	})
	// Set initial time for endPicker
	tr.endPicker.selectedDate = endTime

	tr.ExtendBaseWidget(tr)
	return tr
}

// CreateRenderer creates the widget renderer
func (tr *TimeRangePicker) CreateRenderer() fyne.WidgetRenderer {
	startLabel := widget.NewLabel("Start:")
	endLabel := widget.NewLabel("End:")

	content := container.NewVBox(
		container.NewBorder(nil, nil, startLabel, nil, tr.startPicker),
		container.NewBorder(nil, nil, endLabel, nil, tr.endPicker),
	)

	return widget.NewSimpleRenderer(content)
}

// GetTimeRange returns the selected start and end times
func (tr *TimeRangePicker) GetTimeRange() (start, end time.Time) {
	return tr.startPicker.GetDateTime(), tr.endPicker.GetDateTime()
}

// SetTimeRange sets the start and end times
func (tr *TimeRangePicker) SetTimeRange(start, end time.Time) {
	tr.startPicker.SetDateTime(start)
	tr.endPicker.SetDateTime(end)
}
