package gui

import (
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
)

// TickerChannelPanel manages ticker channel configuration
type TickerChannelPanel struct {
	logger        *zap.Logger
	configManager *config.ConfigManager
	exchange      string

	// UI Components
	enableCheck *widget.Check
	symbolList  *widget.CheckGroup
	searchEntry *widget.Entry
	container   *fyne.Container

	// State
	enabled          bool
	selectedSymbols  map[string]bool // actual Bitfinex symbols (e.g., tBTCUSD)
	availableSymbols []string        // actual symbols
	displaySymbols   []string        // canonical display strings (e.g., BTC-USD)
	displayToSymbol  map[string]string
	symbolToDisplay  map[string]string

	// Observers / helpers
	onStateChange func()
	limitChecker  func(delta int) bool
	updating      bool
}

// NewTickerChannelPanel creates a new ticker channel panel
func NewTickerChannelPanel(logger *zap.Logger, configManager *config.ConfigManager, exchange string) *TickerChannelPanel {
	panel := &TickerChannelPanel{
		logger:          logger,
		configManager:   configManager,
		exchange:        exchange,
		selectedSymbols: make(map[string]bool),
		displayToSymbol: make(map[string]string),
		symbolToDisplay: make(map[string]string),
	}

	panel.loadAvailableSymbols()
	return panel
}

// SetOnStateChange registers a callback fired when the panel state mutates
func (p *TickerChannelPanel) SetOnStateChange(fn func()) {
	p.onStateChange = fn
}

// SetLimitChecker installs a callback to validate subscription increments
func (p *TickerChannelPanel) SetLimitChecker(fn func(delta int) bool) {
	p.limitChecker = fn
}

// Build constructs the UI
func (p *TickerChannelPanel) Build() fyne.CanvasObject {
	p.enableCheck = widget.NewCheck("Enable Ticker Channel", func(checked bool) {
		p.enabled = checked
		if checked {
			p.symbolList.Enable()
		} else {
			p.symbolList.Disable()
		}

		if p.updating {
			return
		}

		if checked {
			delta := len(p.selectedSymbols)
			if delta > 0 && p.limitChecker != nil && !p.limitChecker(delta) {
				p.updating = true
				p.enableCheck.SetChecked(false)
				p.symbolList.Disable()
				p.updating = false
				return
			}
		}

		p.persistState()
		p.notifyStateChange()
	})

	p.searchEntry = widget.NewEntry()
	p.searchEntry.SetPlaceHolder("Search symbols (e.g., BTC, ETH)...")
	p.searchEntry.OnChanged = func(text string) {
		p.filterSymbols(text)
	}

	options := p.displaySymbols[:min(len(p.displaySymbols), 100)]
	p.symbolList = widget.NewCheckGroup(options, func(selected []string) {
		if p.updating {
			return
		}

		prevCount := len(p.selectedSymbols)
		newCount := len(selected)
		delta := newCount - prevCount
		if delta > 0 && p.limitChecker != nil && !p.limitChecker(delta) {
			p.updating = true
			p.symbolList.SetSelected(p.currentDisplaySelection())
			p.updating = false
			return
		}

		p.selectedSymbols = make(map[string]bool)
		for _, display := range selected {
			if symbol, ok := p.displayToSymbol[display]; ok {
				p.selectedSymbols[symbol] = true
			} else {
				// Fallback: assume the value is already an exchange symbol
				p.selectedSymbols[display] = true
			}
		}

		p.persistState()
		p.notifyStateChange()
	})
	p.symbolList.Disable()

	infoLabel := widget.NewLabel("Ticker provides real-time price updates for selected symbols.")

	symbolScroll := container.NewVScroll(p.symbolList)
	symbolScroll.SetMinSize(fyne.NewSize(400, 400))

	p.container = container.NewVBox(
		infoLabel,
		widget.NewSeparator(),
		p.enableCheck,
		p.searchEntry,
		symbolScroll,
	)

	return p.container
}

// loadAvailableSymbols loads available trading pairs from config and normalizes their display
func (p *TickerChannelPanel) loadAvailableSymbols() {
	p.availableSymbols = []string{}
	p.displaySymbols = []string{}
	p.displayToSymbol = make(map[string]string)
	p.symbolToDisplay = make(map[string]string)

	normalizer := p.configManager.GetNormalizer()
	pairs, err := p.configManager.GetAvailablePairs(p.exchange, "exchange")
	if err != nil {
		p.logger.Warn("Failed to load pairs", zap.Error(err))
		fallback := []string{"tBTCUSD", "tETHUSD", "tLTCUSD"}
		for _, symbol := range fallback {
			display := symbol
			if normalizer != nil {
				if normalized, nerr := normalizer.NormalizePair(symbol); nerr == nil {
					display = normalized.Internal
				}
			}
			p.availableSymbols = append(p.availableSymbols, symbol)
			p.displaySymbols = append(p.displaySymbols, display)
			p.displayToSymbol[display] = symbol
			p.symbolToDisplay[symbol] = display
		}
		return
	}

	for _, pair := range pairs {
		symbol := pair
		if !strings.HasPrefix(symbol, "t") && !strings.HasPrefix(symbol, "f") {
			symbol = "t" + symbol
		}

		display := symbol
		if normalizer != nil {
			if normalized, nerr := normalizer.NormalizePair(symbol); nerr == nil {
				display = normalized.Internal
				if normalized.IsFunding {
					display = display + " (Funding)"
				}
			}
		}

		p.availableSymbols = append(p.availableSymbols, symbol)
		p.displaySymbols = append(p.displaySymbols, display)
		p.displayToSymbol[display] = symbol
		p.symbolToDisplay[symbol] = display
	}

	if len(p.availableSymbols) > 500 {
		p.availableSymbols = p.availableSymbols[:500]
		p.displaySymbols = p.displaySymbols[:500]
	}
}

// filterSymbols filters the symbol list based on search text
func (p *TickerChannelPanel) filterSymbols(searchText string) {
	if p.symbolList == nil {
		return
	}

	if searchText == "" {
		p.symbolList.Options = p.displaySymbols[:min(len(p.displaySymbols), 100)]
		p.symbolList.Refresh()
		return
	}

	filtered := []string{}
	searchUpper := strings.ToUpper(searchText)
	for _, display := range p.displaySymbols {
		if strings.Contains(strings.ToUpper(display), searchUpper) {
			filtered = append(filtered, display)
			if len(filtered) >= 100 {
				break
			}
		}
	}

	p.symbolList.Options = filtered
	p.symbolList.Refresh()
}

// GetSubscriptions returns the channel subscriptions
func (p *TickerChannelPanel) GetSubscriptions() []ChannelSubscription {
	if !p.enabled {
		return []ChannelSubscription{}
	}

	subs := []ChannelSubscription{}
	for symbol := range p.selectedSymbols {
		subs = append(subs, ChannelSubscription{
			Channel: "ticker",
			Symbol:  symbol,
		})
	}

	return subs
}

// GetSubscriptionCount returns the number of subscriptions
func (p *TickerChannelPanel) GetSubscriptionCount() int {
	if !p.enabled {
		return 0
	}
	return len(p.selectedSymbols)
}

// LoadState loads state from UIState
func (p *TickerChannelPanel) LoadState(uiState *config.UIState) {
	if uiState == nil || uiState.ChannelStates == nil {
		return
	}

	if channelState, ok := uiState.ChannelStates["ticker"].(map[string]interface{}); ok {
		if enabled, ok := channelState["enabled"].(bool); ok {
			p.enabled = enabled
			if p.enableCheck != nil {
				p.updating = true
				p.enableCheck.SetChecked(enabled)
				p.updating = false
				if enabled {
					p.symbolList.Enable()
				} else {
					p.symbolList.Disable()
				}
			}
		}

		if symbols, ok := channelState["selected_symbols"].([]interface{}); ok {
			p.selectedSymbols = make(map[string]bool)
			displaySelection := []string{}
			for _, sym := range symbols {
				if symStr, ok := sym.(string); ok {
					p.selectedSymbols[symStr] = true
					if display, exists := p.symbolToDisplay[symStr]; exists {
						displaySelection = append(displaySelection, display)
					} else {
						displaySelection = append(displaySelection, symStr)
					}
				}
			}
			if p.symbolList != nil {
				p.updating = true
				p.symbolList.SetSelected(displaySelection)
				p.updating = false
			}
		}
	}
}

// SaveState saves state to UIState
func (p *TickerChannelPanel) SaveState(uiState *config.UIState) {
	if uiState.ChannelStates == nil {
		uiState.ChannelStates = make(map[string]interface{})
	}

	selectedList := make([]string, 0, len(p.selectedSymbols))
	for sym := range p.selectedSymbols {
		selectedList = append(selectedList, sym)
	}

	uiState.ChannelStates["ticker"] = map[string]interface{}{
		"enabled":          p.enabled,
		"selected_symbols": selectedList,
	}
}

// Reset resets the panel to initial state
func (p *TickerChannelPanel) Reset() {
	p.enabled = false
	p.selectedSymbols = make(map[string]bool)

	if p.enableCheck != nil {
		p.updating = true
		p.enableCheck.SetChecked(false)
		p.updating = false
		p.symbolList.Disable()
	}
	if p.symbolList != nil {
		p.updating = true
		p.symbolList.SetSelected([]string{})
		p.updating = false
	}
	if p.searchEntry != nil {
		p.searchEntry.SetText("")
	}

	p.persistState()
	p.notifyStateChange()
}

// ReloadSymbols refreshes the available symbol list from disk
func (p *TickerChannelPanel) ReloadSymbols() {
	currentSymbols := p.currentActualSymbols()
	searchText := ""
	if p.searchEntry != nil {
		searchText = p.searchEntry.Text
	}

	p.loadAvailableSymbols()

	if p.symbolList == nil {
		return
	}

	options := p.displaySymbols[:min(len(p.displaySymbols), 100)]
	p.symbolList.Options = options
	p.symbolList.Refresh()

	if searchText != "" {
		p.filterSymbols(searchText)
	}

	availableSet := make(map[string]struct{}, len(p.availableSymbols))
	for _, sym := range p.availableSymbols {
		availableSet[sym] = struct{}{}
	}

	p.selectedSymbols = make(map[string]bool)
	displaySelection := []string{}
	for _, sym := range currentSymbols {
		if _, ok := availableSet[sym]; ok {
			p.selectedSymbols[sym] = true
			if display, exists := p.symbolToDisplay[sym]; exists {
				displaySelection = append(displaySelection, display)
			} else {
				displaySelection = append(displaySelection, sym)
			}
		}
	}

	p.updating = true
	p.symbolList.SetSelected(displaySelection)
	p.updating = false

	p.persistState()
	p.notifyStateChange()
}

func (p *TickerChannelPanel) notifyStateChange() {
	if p.onStateChange != nil {
		p.onStateChange()
	}
}

func (p *TickerChannelPanel) persistState() {
	if p.configManager == nil {
		return
	}
	state := p.configManager.GetApplicationState()
	if state == nil {
		return
	}

	uiState := state.GetUIState(p.exchange)
	p.SaveState(uiState)
	state.UpdateUIState(p.exchange, uiState)
	if err := p.configManager.SaveState(); err != nil {
		p.logger.Warn("failed to persist ticker channel state", zap.Error(err))
	}
}

func (p *TickerChannelPanel) currentActualSymbols() []string {
	out := make([]string, 0, len(p.selectedSymbols))
	for sym := range p.selectedSymbols {
		out = append(out, sym)
	}
	sort.Strings(out)
	return out
}

func (p *TickerChannelPanel) currentDisplaySelection() []string {
	actual := p.currentActualSymbols()
	display := make([]string, 0, len(actual))
	for _, sym := range actual {
		if label, ok := p.symbolToDisplay[sym]; ok {
			display = append(display, label)
		} else {
			display = append(display, sym)
		}
	}
	return display
}
