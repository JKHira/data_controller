package gui

import (
	"fmt"
	"math"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// RestChannelCandles represents the Candles data type configuration panel
type RestChannelCandles struct {
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

	// Candles-specific settings
	timeframeCheckGroup *widget.CheckGroup

	// Callback
	onChanged func()
}

// NewRestChannelCandles creates a new Candles configuration panel
func NewRestChannelCandles(symbols []string, onChange func()) *RestChannelCandles {
	c := &RestChannelCandles{
		enabled:   false,
		onChanged: onChange,
	}
	c.ExtendBaseWidget(c)

	// Initialize components
	c.initComponents(symbols)

	return c
}

// initComponents initializes all UI components
func (c *RestChannelCandles) initComponents(symbols []string) {
	// Enable checkbox
	c.enableCheck = widget.NewCheck("Enable Candles Data Collection", func(checked bool) {
		c.enabled = checked
		if c.onChanged != nil {
			c.onChanged()
		}
	})

	// Symbol selector (300px height)
	c.symbolSelector = NewSymbolSearchSelector(symbols, func(selected []string) {
		if c.onChanged != nil {
			c.onChanged()
		}
	})

	// Time range picker with 120px width entries
	c.timeRangePicker = NewTimeRangePicker(func(start, end time.Time) {
		if c.onChanged != nil {
			c.onChanged()
		}
	})

	// Limit slider (100-10000, step 100) with default 200
	c.limitSlider = widget.NewSlider(100, 10000)
	c.limitSlider.Step = 100
	c.limitSlider.Value = 200
	c.limitSlider.OnChanged = func(value float64) {
		rounded := math.Round(value/100.0) * 100.0
		if rounded != value {
			c.limitSlider.SetValue(rounded)
			return
		}
		c.limitLabel.SetText(fmt.Sprintf("Limit: %.0f", rounded))
		if c.onChanged != nil {
			c.onChanged()
		}
	}
	c.limitLabel = widget.NewLabel("Limit: 200")

	// Sort radio
	c.sortRadio = widget.NewRadioGroup([]string{"Old to New (1)", "New to Old (-1)"}, func(selected string) {
		if c.onChanged != nil {
			c.onChanged()
		}
	})
	c.sortRadio.SetSelected("Old to New (1)")
	c.sortRadio.Horizontal = true

	// Timeframes (Candles-specific, 160px height)
	timeframes := []string{
		"1m", "5m", "15m", "30m",
		"1h", "3h", "6h", "12h",
		"1D", "7D", "14D", "1M",
	}
	c.timeframeCheckGroup = widget.NewCheckGroup(timeframes, func(selected []string) {
		if c.onChanged != nil {
			c.onChanged()
		}
	})
}

// CreateRenderer creates the widget renderer
func (c *RestChannelCandles) CreateRenderer() fyne.WidgetRenderer {
	// Enable checkbox at top
	enableContainer := container.NewVBox(c.enableCheck)

	// Timeframes section (Candles-specific, 160px height)
	timeframeScroll := container.NewVScroll(c.timeframeCheckGroup)
	timeframeScroll.SetMinSize(fyne.NewSize(0, 160))

	timeframeSelectAll := widget.NewButton("Select All", func() {
		c.timeframeCheckGroup.SetSelected(c.timeframeCheckGroup.Options)
	})
	timeframeDeselectAll := widget.NewButton("Deselect All", func() {
		c.timeframeCheckGroup.SetSelected([]string{})
	})
	timeframeBtns := container.NewHBox(timeframeSelectAll, timeframeDeselectAll)

	timeframeContainer := container.NewBorder(
		widget.NewLabel("Timeframes:"),
		timeframeBtns,
		nil, nil,
		timeframeScroll,
	)

	// Symbol selector section
	symbolLabel := widget.NewLabel("Symbols:")
	symbolContainer := container.NewBorder(
		symbolLabel,
		nil,
		nil, nil,
		c.symbolSelector.Build(),
	)

	// Time range section
	timeRangeLabel := widget.NewLabel("Time Range:")
	timeRangeContainer := container.NewBorder(
		timeRangeLabel,
		nil,
		nil, nil,
		c.timeRangePicker,
	)

	// Request options section
	limitContainer := container.NewVBox(
		c.limitLabel,
		c.limitSlider,
	)

	sortLabel := widget.NewLabel("Sort:")
	sortContainer := container.NewVBox(sortLabel, c.sortRadio)

	optionsContainer := container.NewVBox(
		widget.NewLabel("Request Options:"),
		limitContainer,
		sortContainer,
	)

	// Main layout
	content := container.NewVBox(
		enableContainer,
		widget.NewSeparator(),
		timeframeContainer,
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
func (c *RestChannelCandles) IsEnabled() bool {
	return c.enabled
}

// SetEnabled sets the enabled state
func (c *RestChannelCandles) SetEnabled(enabled bool) {
	c.enabled = enabled
	c.enableCheck.SetChecked(enabled)
}

// GetSelectedSymbols returns selected symbols
func (c *RestChannelCandles) GetSelectedSymbols() []string {
	return c.symbolSelector.GetSelected()
}

// SetSelectedSymbols sets selected symbols
func (c *RestChannelCandles) SetSelectedSymbols(symbols []string) {
	c.symbolSelector.SetSelected(symbols)
}

// GetTimeRange returns the selected time range
func (c *RestChannelCandles) GetTimeRange() (start, end time.Time) {
	return c.timeRangePicker.GetTimeRange()
}

// SetTimeRange sets the time range
func (c *RestChannelCandles) SetTimeRange(start, end time.Time) {
	c.timeRangePicker.SetTimeRange(start, end)
}

// GetLimit returns the request limit
func (c *RestChannelCandles) GetLimit() int {
	return int(c.limitSlider.Value)
}

// SetLimit sets the request limit
func (c *RestChannelCandles) SetLimit(limit int) {
	rounded := math.Round(float64(limit)/100.0) * 100.0
	if rounded < c.limitSlider.Min {
		rounded = c.limitSlider.Min
	}
	if rounded > c.limitSlider.Max {
		rounded = c.limitSlider.Max
	}
	c.limitSlider.SetValue(rounded)
	c.limitLabel.SetText(fmt.Sprintf("Limit: %.0f", rounded))
}

// GetSort returns the sort direction (1 or -1)
func (c *RestChannelCandles) GetSort() int {
	if c.sortRadio.Selected == "Old to New (1)" {
		return 1
	}
	return -1
}

// SetSort sets the sort direction
func (c *RestChannelCandles) SetSort(sort int) {
	if sort == 1 {
		c.sortRadio.SetSelected("Old to New (1)")
	} else {
		c.sortRadio.SetSelected("New to Old (-1)")
	}
}

// GetTimeframes returns selected timeframes (Candles-specific)
func (c *RestChannelCandles) GetTimeframes() []string {
	return c.timeframeCheckGroup.Selected
}

// SetTimeframes sets selected timeframes
func (c *RestChannelCandles) SetTimeframes(timeframes []string) {
	c.timeframeCheckGroup.SetSelected(timeframes)
}

// UpdateSymbols updates the available symbols list
func (c *RestChannelCandles) UpdateSymbols(symbols []string) {
	c.symbolSelector.SetSymbols(symbols)
}
