package gui

import (
	"fmt"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// RestChannelTrades represents the Trades data type configuration panel
type RestChannelTrades struct {
	widget.BaseWidget

	// Enable control
	enabled     bool
	enableCheck *widget.Check

	// Common settings
	symbolSelector  *SymbolSearchSelector
	timeRangePicker *TimeRangePicker
	limitSlider     *widget.Slider
	limitLabel      *widget.Label
	sortRadio       *widget.RadioGroup

	// Callback
	onChanged func()
}

// NewRestChannelTrades creates a new Trades configuration panel
func NewRestChannelTrades(symbols []string, onChange func()) *RestChannelTrades {
	t := &RestChannelTrades{
		enabled:   false,
		onChanged: onChange,
	}
	t.ExtendBaseWidget(t)

	// Initialize components
	t.initComponents(symbols)

	return t
}

// initComponents initializes all UI components
func (t *RestChannelTrades) initComponents(symbols []string) {
	// Enable checkbox
	t.enableCheck = widget.NewCheck("Enable Trades Data Collection", func(checked bool) {
		t.enabled = checked
		if t.onChanged != nil {
			t.onChanged()
		}
	})

	// Symbol selector (300px height)
	t.symbolSelector = NewSymbolSearchSelector(symbols, func(selected []string) {
		if t.onChanged != nil {
			t.onChanged()
		}
	})

	// Time range picker with 120px width entries
	t.timeRangePicker = NewTimeRangePicker(func(start, end time.Time) {
		if t.onChanged != nil {
			t.onChanged()
		}
	})

	// Limit slider (100-10000, step 100) with default 100
	t.limitSlider = widget.NewSlider(100, 10000)
	t.limitSlider.Step = 100
	t.limitSlider.Value = 100
	t.limitSlider.OnChanged = func(value float64) {
		rounded := math.Round(value/100.0) * 100.0
		if rounded != value {
			t.limitSlider.SetValue(rounded)
			return
		}
		t.limitLabel.SetText(fmt.Sprintf("Limit: %.0f", rounded))
		if t.onChanged != nil {
			t.onChanged()
		}
	}
	t.limitLabel = widget.NewLabel("Limit: 100")

	// Sort radio
	t.sortRadio = widget.NewRadioGroup([]string{"Old to New (1)", "New to Old (-1)"}, func(selected string) {
		if t.onChanged != nil {
			t.onChanged()
		}
	})
	t.sortRadio.SetSelected("Old to New (1)")
	t.sortRadio.Horizontal = true
}

// CreateRenderer creates the widget renderer
func (t *RestChannelTrades) CreateRenderer() fyne.WidgetRenderer {
	// Enable checkbox at top
	enableContainer := container.NewVBox(t.enableCheck)

	// Symbol selector section
	symbolLabel := widget.NewLabel("Symbols:")
	symbolContainer := container.NewBorder(
		symbolLabel,
		nil,
		nil, nil,
		t.symbolSelector.Build(),
	)

	// Time range section
	timeRangeLabel := widget.NewLabel("Time Range:")
	timeRangeContainer := container.NewBorder(
		timeRangeLabel,
		nil,
		nil, nil,
		t.timeRangePicker,
	)

	// Request options section
	limitContainer := container.NewVBox(
		t.limitLabel,
		t.limitSlider,
	)

	sortLabel := widget.NewLabel("Sort:")
	sortContainer := container.NewVBox(sortLabel, t.sortRadio)

	optionsContainer := container.NewVBox(
		widget.NewLabel("Request Options:"),
		limitContainer,
		sortContainer,
	)

	// Main layout
	content := container.NewVBox(
		enableContainer,
		widget.NewSeparator(),
		symbolContainer,
		widget.NewSeparator(),
		timeRangeContainer,
		widget.NewSeparator(),
		optionsContainer,
	)

	scrollable := container.NewVScroll(content)

	return widget.NewSimpleRenderer(scrollable)
}

// IsEnabled returns whether this data type is enabled
func (t *RestChannelTrades) IsEnabled() bool {
	return t.enabled
}

// SetEnabled sets the enabled state
func (t *RestChannelTrades) SetEnabled(enabled bool) {
	t.enabled = enabled
	t.enableCheck.SetChecked(enabled)
}

// GetSelectedSymbols returns selected symbols
func (t *RestChannelTrades) GetSelectedSymbols() []string {
	return t.symbolSelector.GetSelected()
}

// SetSelectedSymbols sets selected symbols
func (t *RestChannelTrades) SetSelectedSymbols(symbols []string) {
	t.symbolSelector.SetSelected(symbols)
}

// GetTimeRange returns the selected time range
func (t *RestChannelTrades) GetTimeRange() (start, end time.Time) {
	return t.timeRangePicker.GetTimeRange()
}

// SetTimeRange sets the time range
func (t *RestChannelTrades) SetTimeRange(start, end time.Time) {
	t.timeRangePicker.SetTimeRange(start, end)
}

// GetLimit returns the request limit
func (t *RestChannelTrades) GetLimit() int {
	return int(t.limitSlider.Value)
}

// SetLimit sets the request limit
func (t *RestChannelTrades) SetLimit(limit int) {
	rounded := math.Round(float64(limit)/100.0) * 100.0
	if rounded < t.limitSlider.Min {
		rounded = t.limitSlider.Min
	}
	if rounded > t.limitSlider.Max {
		rounded = t.limitSlider.Max
	}
	t.limitSlider.SetValue(rounded)
	t.limitLabel.SetText(fmt.Sprintf("Limit: %.0f", rounded))
}

// GetSort returns the sort direction (1 or -1)
func (t *RestChannelTrades) GetSort() int {
	if t.sortRadio.Selected == "Old to New (1)" {
		return 1
	}
	return -1
}

// SetSort sets the sort direction
func (t *RestChannelTrades) SetSort(sort int) {
	if sort == 1 {
		t.sortRadio.SetSelected("Old to New (1)")
	} else {
		t.sortRadio.SetSelected("New to Old (-1)")
	}
}

// UpdateSymbols updates the available symbols list
func (t *RestChannelTrades) UpdateSymbols(symbols []string) {
	t.symbolSelector.SetSymbols(symbols)
}
