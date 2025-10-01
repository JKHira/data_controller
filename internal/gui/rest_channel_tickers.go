package gui

import (
	"fmt"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// RestChannelTickers represents the Tickers History data type configuration panel
type RestChannelTickers struct {
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

// NewRestChannelTickers creates a new Tickers History configuration panel
func NewRestChannelTickers(symbols []string, onChange func()) *RestChannelTickers {
	t := &RestChannelTickers{
		enabled:   false,
		onChanged: onChange,
	}
	t.ExtendBaseWidget(t)

	// Initialize components
	t.initComponents(symbols)

	return t
}

// initComponents initializes all UI components
func (t *RestChannelTickers) initComponents(symbols []string) {
	// Enable checkbox
	t.enableCheck = widget.NewCheck("Enable Tickers History Data Collection", func(checked bool) {
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

	// Limit slider (10-250, step 10)
	t.limitSlider = widget.NewSlider(10, 250)
	t.limitSlider.Step = 10
	t.limitSlider.Value = 100
	t.limitSlider.OnChanged = func(value float64) {
		rounded := math.Round(value/10.0) * 10.0
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
func (t *RestChannelTickers) CreateRenderer() fyne.WidgetRenderer {
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
func (t *RestChannelTickers) IsEnabled() bool {
	return t.enabled
}

// SetEnabled sets the enabled state
func (t *RestChannelTickers) SetEnabled(enabled bool) {
	t.enabled = enabled
	t.enableCheck.SetChecked(enabled)
}

// GetSelectedSymbols returns selected symbols
func (t *RestChannelTickers) GetSelectedSymbols() []string {
	return t.symbolSelector.GetSelected()
}

// SetSelectedSymbols sets selected symbols
func (t *RestChannelTickers) SetSelectedSymbols(symbols []string) {
	t.symbolSelector.SetSelected(symbols)
}

// GetTimeRange returns the selected time range
func (t *RestChannelTickers) GetTimeRange() (start, end time.Time) {
	return t.timeRangePicker.GetTimeRange()
}

// SetTimeRange sets the time range
func (t *RestChannelTickers) SetTimeRange(start, end time.Time) {
	t.timeRangePicker.SetTimeRange(start, end)
}

// GetLimit returns the request limit
func (t *RestChannelTickers) GetLimit() int {
	return int(t.limitSlider.Value)
}

// SetLimit sets the request limit
func (t *RestChannelTickers) SetLimit(limit int) {
	rounded := math.Round(float64(limit)/10.0) * 10.0
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
func (t *RestChannelTickers) GetSort() int {
	if t.sortRadio.Selected == "Old to New (1)" {
		return 1
	}
	return -1
}

// SetSort sets the sort direction
func (t *RestChannelTickers) SetSort(sort int) {
	if sort == 1 {
		t.sortRadio.SetSelected("Old to New (1)")
	} else {
		t.sortRadio.SetSelected("New to Old (-1)")
	}
}

// UpdateSymbols updates the available symbols list
func (t *RestChannelTickers) UpdateSymbols(symbols []string) {
	t.symbolSelector.SetSymbols(symbols)
}
