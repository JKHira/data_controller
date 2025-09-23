package gui

import (
	"context"
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/restapi"
)

// RestAPIPanel manages the REST API functionality
type RestAPIPanel struct {
	logger         *zap.Logger
	bitfinexClient *restapi.BitfinexClient

	// UI state
	isRunning      bool
	runningMutex   sync.RWMutex

	// Base Data options
	baseDataOptions *restapi.BaseDataOptions

	// UI components
	getBaseDataBtn  *widget.Button
	progressLog     *widget.List
	progressData    []string
	progressMutex   sync.Mutex

	// Checkboxes for base data options
	checkboxes      map[string]*widget.Check
}

// NewRestAPIPanel creates a new REST API panel
func NewRestAPIPanel(logger *zap.Logger) *RestAPIPanel {
	panel := &RestAPIPanel{
		logger:         logger,
		bitfinexClient: restapi.NewBitfinexClient(logger),
		baseDataOptions: &restapi.BaseDataOptions{},
		progressData:   make([]string, 0),
		checkboxes:     make(map[string]*widget.Check),
	}

	panel.createUI()
	return panel
}

// createUI creates the user interface components
func (p *RestAPIPanel) createUI() {
	// Get Base Data button
	p.getBaseDataBtn = widget.NewButton("📊 Get Base Data", p.handleGetBaseData)
	p.getBaseDataBtn.Importance = widget.MediumImportance

	// Progress log list
	p.progressLog = widget.NewList(
		func() int { return len(p.progressData) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(p.progressData) {
				label := obj.(*widget.Label)
				label.SetText(p.progressData[id])
			}
		},
	)
}

// CreateBitfinexBaseDataPanel creates the Bitfinex Base Data tab content
func (p *RestAPIPanel) CreateBitfinexBaseDataPanel() fyne.CanvasObject {
	// Create checkboxes for different data types
	p.createCheckboxes()

	// Listings section
	listingsCard := widget.NewCard("📋 Listings", "", container.NewVBox(
		p.checkboxes["spot_pairs"],
		p.checkboxes["margin_pairs"],
		p.checkboxes["futures_pairs"],
		p.checkboxes["currencies"],
		p.checkboxes["margin_currencies"],
	))

	// Mappings section
	mappingsCard := widget.NewCard("🗺️ Mappings", "", container.NewVBox(
		p.checkboxes["currency_labels"],
		p.checkboxes["currency_symbols"],
		p.checkboxes["currency_units"],
		p.checkboxes["currency_underlying"],
	))

	// Other data section
	otherCard := widget.NewCard("📊 Other Data", "", container.NewVBox(
		p.checkboxes["active_tickers"],
	))

	// Options layout - vertical arrangement for narrower width
	optionsLayout := container.NewVBox(
		listingsCard,
		mappingsCard,
		otherCard,
	)

	// Control buttons
	selectAllBtn := widget.NewButton("✓ Select All", p.handleSelectAll)
	selectNoneBtn := widget.NewButton("✗ Select None", p.handleSelectNone)

	buttonRow := container.NewHBox(
		p.getBaseDataBtn,
		widget.NewSeparator(),
		selectAllBtn,
		selectNoneBtn,
	)

	// Progress log
	progressCard := widget.NewCard("📄 Progress Log", "",
		container.NewVScroll(p.progressLog),
	)
	progressCard.Resize(fyne.NewSize(800, 200))

	// Main layout
	content := container.NewBorder(
		container.NewVBox(optionsLayout, buttonRow), // top
		progressCard,                                 // bottom
		nil, nil,                                    // left, right
		nil,                                         // center
	)

	return content
}

// createCheckboxes creates all the checkboxes for base data options
func (p *RestAPIPanel) createCheckboxes() {
	// Listings
	p.checkboxes["spot_pairs"] = widget.NewCheck("Spot Pairs (pub:list:pair:exchange)", p.updateOptions)
	p.checkboxes["margin_pairs"] = widget.NewCheck("Margin Pairs (pub:list:pair:margin)", p.updateOptions)
	p.checkboxes["futures_pairs"] = widget.NewCheck("Futures Pairs (pub:list:pair:futures)", p.updateOptions)
	p.checkboxes["currencies"] = widget.NewCheck("Currencies (pub:list:currency)", p.updateOptions)
	p.checkboxes["margin_currencies"] = widget.NewCheck("Margin Currencies (pub:list:currency:margin)", p.updateOptions)

	// Mappings
	p.checkboxes["currency_labels"] = widget.NewCheck("Currency Labels (pub:map:currency:label)", p.updateOptions)
	p.checkboxes["currency_symbols"] = widget.NewCheck("Currency Symbols (pub:map:currency:sym)", p.updateOptions)
	p.checkboxes["currency_units"] = widget.NewCheck("Currency Units (pub:map:currency:unit)", p.updateOptions)
	p.checkboxes["currency_underlying"] = widget.NewCheck("Currency Underlying (pub:map:currency:undl)", p.updateOptions)

	// Other data
	p.checkboxes["active_tickers"] = widget.NewCheck("Active Tickers (tickers?symbols=ALL)", p.updateOptions)

	// Set default selections (commonly used ones)
	p.checkboxes["spot_pairs"].SetChecked(true)
	p.checkboxes["currencies"].SetChecked(true)
	p.checkboxes["currency_labels"].SetChecked(true)
	p.checkboxes["active_tickers"].SetChecked(true)
}

// updateOptions updates the base data options based on checkbox states
func (p *RestAPIPanel) updateOptions(checked bool) {
	p.baseDataOptions.SpotPairs = p.checkboxes["spot_pairs"].Checked
	p.baseDataOptions.MarginPairs = p.checkboxes["margin_pairs"].Checked
	p.baseDataOptions.FuturesPairs = p.checkboxes["futures_pairs"].Checked
	p.baseDataOptions.Currencies = p.checkboxes["currencies"].Checked
	p.baseDataOptions.MarginCurrencies = p.checkboxes["margin_currencies"].Checked
	p.baseDataOptions.CurrencyLabels = p.checkboxes["currency_labels"].Checked
	p.baseDataOptions.CurrencySymbols = p.checkboxes["currency_symbols"].Checked
	p.baseDataOptions.CurrencyUnits = p.checkboxes["currency_units"].Checked
	p.baseDataOptions.CurrencyUnderlying = p.checkboxes["currency_underlying"].Checked
	p.baseDataOptions.ActiveTickers = p.checkboxes["active_tickers"].Checked
}

// handleSelectAll selects all checkboxes
func (p *RestAPIPanel) handleSelectAll() {
	for _, checkbox := range p.checkboxes {
		checkbox.SetChecked(true)
	}
}

// handleSelectNone deselects all checkboxes
func (p *RestAPIPanel) handleSelectNone() {
	for _, checkbox := range p.checkboxes {
		checkbox.SetChecked(false)
	}
}

// handleGetBaseData handles the Get Base Data button click
func (p *RestAPIPanel) handleGetBaseData() {
	p.runningMutex.Lock()
	defer p.runningMutex.Unlock()

	if p.isRunning {
		p.logger.Warn("Base data fetch already in progress")
		return
	}

	// Check if at least one option is selected
	hasSelection := p.baseDataOptions.SpotPairs || p.baseDataOptions.MarginPairs ||
		p.baseDataOptions.FuturesPairs || p.baseDataOptions.Currencies ||
		p.baseDataOptions.MarginCurrencies || p.baseDataOptions.CurrencyLabels ||
		p.baseDataOptions.CurrencySymbols || p.baseDataOptions.CurrencyUnits ||
		p.baseDataOptions.CurrencyUnderlying || p.baseDataOptions.ActiveTickers

	if !hasSelection {
		p.addProgressLog("❌ No data types selected. Please select at least one option.")
		return
	}

	p.isRunning = true
	p.getBaseDataBtn.SetText("📊 Fetching...")
	p.getBaseDataBtn.Importance = widget.HighImportance
	p.getBaseDataBtn.Disable()

	// Clear previous progress
	p.clearProgressLog()
	p.addProgressLog("🚀 Starting Bitfinex base data fetch...")

	// Start fetching in a separate goroutine
	go func() {
		defer func() {
			p.runningMutex.Lock()
			p.isRunning = false
			fyne.Do(func() {
				p.getBaseDataBtn.SetText("📊 Get Base Data")
				p.getBaseDataBtn.Importance = widget.MediumImportance
				p.getBaseDataBtn.Enable()
			})
			p.runningMutex.Unlock()
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		err := p.bitfinexClient.FetchBaseData(ctx, *p.baseDataOptions, p.onProgress)
		if err != nil {
			p.logger.Error("Base data fetch failed", zap.Error(err))
			p.addProgressLog(fmt.Sprintf("❌ Fetch failed: %v", err))
		} else {
			p.addProgressLog("✅ Base data fetch completed successfully!")
		}
	}()
}

// onProgress handles progress updates from the fetch operation
func (p *RestAPIPanel) onProgress(result restapi.FetchResult) {
	timestamp := result.Timestamp.Format("15:04:05")

	if result.Success {
		message := fmt.Sprintf("[%s] ✅ %s: %d items saved to %s",
			timestamp, result.Endpoint, result.Count, result.FilePath)
		p.addProgressLog(message)
	} else {
		message := fmt.Sprintf("[%s] ❌ %s: %s",
			timestamp, result.Endpoint, result.Error)
		p.addProgressLog(message)
	}
}

// addProgressLog adds a new message to the progress log
func (p *RestAPIPanel) addProgressLog(message string) {
	p.progressMutex.Lock()
	defer p.progressMutex.Unlock()

	// Add to beginning of slice for newest first
	p.progressData = append([]string{message}, p.progressData...)

	// Keep only latest 50 entries
	if len(p.progressData) > 50 {
		p.progressData = p.progressData[:50]
	}

	// Update UI on main thread
	fyne.Do(func() {
		p.progressLog.Refresh()
	})
}

// clearProgressLog clears the progress log
func (p *RestAPIPanel) clearProgressLog() {
	p.progressMutex.Lock()
	defer p.progressMutex.Unlock()

	p.progressData = make([]string, 0)

	fyne.Do(func() {
		p.progressLog.Refresh()
	})
}