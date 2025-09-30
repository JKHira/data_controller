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

// TradesChannelPanel manages trades channel configuration
type TradesChannelPanel struct {
	logger           *zap.Logger
	configManager    *config.ConfigManager
	exchange         string
	enableCheck      *widget.Check
	symbolList       *widget.CheckGroup
	searchEntry      *widget.Entry
	container        *fyne.Container
	enabled          bool
	selectedSymbols  map[string]bool
	availableSymbols []string
	displaySymbols   []string
	displayToSymbol  map[string]string
	symbolToDisplay  map[string]string

	onStateChange func()
	limitChecker  func(delta int) bool
	updating      bool
}

func NewTradesChannelPanel(logger *zap.Logger, configManager *config.ConfigManager, exchange string) *TradesChannelPanel {
	panel := &TradesChannelPanel{
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

func (p *TradesChannelPanel) SetOnStateChange(fn func()) {
	p.onStateChange = fn
}

func (p *TradesChannelPanel) SetLimitChecker(fn func(delta int) bool) {
	p.limitChecker = fn
}

func (p *TradesChannelPanel) Build() fyne.CanvasObject {
	p.enableCheck = widget.NewCheck("Enable Trades Channel", func(checked bool) {
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
	p.searchEntry.SetPlaceHolder("Search symbols...")
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
				p.selectedSymbols[display] = true
			}
		}

		p.persistState()
		p.notifyStateChange()
	})
	p.symbolList.Disable()

	infoLabel := widget.NewLabel("Trades channel provides executed trade information.")
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

func (p *TradesChannelPanel) loadAvailableSymbols() {
	p.availableSymbols = []string{}
	p.displaySymbols = []string{}
	p.displayToSymbol = make(map[string]string)
	p.symbolToDisplay = make(map[string]string)

	normalizer := p.configManager.GetNormalizer()
	pairs, err := p.configManager.GetAvailablePairs(p.exchange, "exchange")
	if err != nil {
		fallback := []string{"tBTCUSD", "tETHUSD"}
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

func (p *TradesChannelPanel) filterSymbols(searchText string) {
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

func (p *TradesChannelPanel) GetSubscriptions() []ChannelSubscription {
	if !p.enabled {
		return []ChannelSubscription{}
	}
	subs := []ChannelSubscription{}
	for symbol := range p.selectedSymbols {
		subs = append(subs, ChannelSubscription{
			Channel: "trades",
			Symbol:  symbol,
		})
	}
	return subs
}

func (p *TradesChannelPanel) GetSubscriptionCount() int {
	if !p.enabled {
		return 0
	}
	return len(p.selectedSymbols)
}

func (p *TradesChannelPanel) LoadState(uiState *config.UIState) {
	if uiState == nil || uiState.ChannelStates == nil {
		return
	}

	if channelState, ok := uiState.ChannelStates["trades"].(map[string]interface{}); ok {
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

func (p *TradesChannelPanel) SaveState(uiState *config.UIState) {
	if uiState.ChannelStates == nil {
		uiState.ChannelStates = make(map[string]interface{})
	}
	selectedList := make([]string, 0, len(p.selectedSymbols))
	for sym := range p.selectedSymbols {
		selectedList = append(selectedList, sym)
	}
	uiState.ChannelStates["trades"] = map[string]interface{}{
		"enabled":          p.enabled,
		"selected_symbols": selectedList,
	}
}

func (p *TradesChannelPanel) Reset() {
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

func (p *TradesChannelPanel) ReloadSymbols() {
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

func (p *TradesChannelPanel) notifyStateChange() {
	if p.onStateChange != nil {
		p.onStateChange()
	}
}

func (p *TradesChannelPanel) persistState() {
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
		p.logger.Warn("failed to persist trades channel state", zap.Error(err))
	}
}

func (p *TradesChannelPanel) currentActualSymbols() []string {
	out := make([]string, 0, len(p.selectedSymbols))
	for sym := range p.selectedSymbols {
		out = append(out, sym)
	}
	sort.Strings(out)
	return out
}

func (p *TradesChannelPanel) currentDisplaySelection() []string {
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
