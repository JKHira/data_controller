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

// BooksChannelPanel manages books channel configuration
type BooksChannelPanel struct {
	logger           *zap.Logger
	configManager    *config.ConfigManager
	exchange         string
	enableCheck      *widget.Check
	symbolList       *widget.CheckGroup
	searchEntry      *widget.Entry
	precSelect       *widget.Select
	freqSelect       *widget.Select
	lenSelect        *widget.Select
	container        *fyne.Container
	enabled          bool
	selectedSymbols  map[string]bool
	availableSymbols []string
	displaySymbols   []string
	displayToSymbol  map[string]string
	symbolToDisplay  map[string]string
	precision        string
	frequency        string
	length           string

	onStateChange func()
	limitChecker  func(delta int) bool
	updating      bool
}

func NewBooksChannelPanel(logger *zap.Logger, configManager *config.ConfigManager, exchange string) *BooksChannelPanel {
	panel := &BooksChannelPanel{
		logger:          logger,
		configManager:   configManager,
		exchange:        exchange,
		selectedSymbols: make(map[string]bool),
		displayToSymbol: make(map[string]string),
		symbolToDisplay: make(map[string]string),
		precision:       "P0",
		frequency:       "F0",
		length:          "25",
	}
	panel.loadAvailableSymbols()
	return panel
}

func (p *BooksChannelPanel) SetOnStateChange(fn func()) {
	p.onStateChange = fn
}

func (p *BooksChannelPanel) SetLimitChecker(fn func(delta int) bool) {
	p.limitChecker = fn
}

func (p *BooksChannelPanel) Build() fyne.CanvasObject {
	p.enableCheck = widget.NewCheck("Enable Books Channel", func(checked bool) {
		p.enabled = checked
		if checked {
			p.symbolList.Enable()
			p.precSelect.Enable()
			p.freqSelect.Enable()
			p.lenSelect.Enable()
		} else {
			p.symbolList.Disable()
			p.precSelect.Disable()
			p.freqSelect.Disable()
			p.lenSelect.Disable()
		}

		if p.updating {
			return
		}

		if checked {
			delta := len(p.selectedSymbols)
			if p.limitChecker != nil && !p.limitChecker(delta) {
				p.updating = true
				p.enableCheck.SetChecked(false)
				p.symbolList.Disable()
				p.precSelect.Disable()
				p.freqSelect.Disable()
				p.lenSelect.Disable()
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

	p.precSelect = widget.NewSelect([]string{"P0", "P1", "P2", "P3", "P4", "R0"}, func(value string) {
		p.precision = value
		if p.updating {
			return
		}
		p.persistState()
	})
	p.precSelect.SetSelected("P0")
	p.precSelect.Disable()

	p.freqSelect = widget.NewSelect([]string{"F0", "F1"}, func(value string) {
		p.frequency = value
		if p.updating {
			return
		}
		p.persistState()
	})
	p.freqSelect.SetSelected("F0")
	p.freqSelect.Disable()

	p.lenSelect = widget.NewSelect([]string{"1", "25", "100", "250"}, func(value string) {
		p.length = value
		if p.updating {
			return
		}
		p.persistState()
	})
	p.lenSelect.SetSelected("25")
	p.lenSelect.Disable()

	infoLabel := widget.NewLabel("Books channel provides order book depth data.")

	configForm := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("Precision", p.precSelect),
			widget.NewFormItem("Frequency", p.freqSelect),
			widget.NewFormItem("Length", p.lenSelect),
		),
		widget.NewLabel("P0=5 sig figs, P1=4, P2=3, P3=2, P4=1, R0=Raw"),
		widget.NewLabel("F0=Realtime, F1=2 second intervals"),
	)

	symbolScroll := container.NewVScroll(p.symbolList)
	symbolScroll.SetMinSize(fyne.NewSize(400, 300))

	p.container = container.NewVBox(
		infoLabel,
		widget.NewSeparator(),
		p.enableCheck,
		configForm,
		widget.NewSeparator(),
		widget.NewLabel("Select Symbols:"),
		p.searchEntry,
		symbolScroll,
	)

	return p.container
}

func (p *BooksChannelPanel) loadAvailableSymbols() {
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

func (p *BooksChannelPanel) filterSymbols(searchText string) {
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

func (p *BooksChannelPanel) GetSubscriptions() []ChannelSubscription {
	if !p.enabled {
		return []ChannelSubscription{}
	}
	subs := []ChannelSubscription{}
	for symbol := range p.selectedSymbols {
		subs = append(subs, ChannelSubscription{
			Channel: "book",
			Symbol:  symbol,
			Prec:    p.precision,
			Freq:    p.frequency,
			Len:     p.length,
		})
	}
	return subs
}

func (p *BooksChannelPanel) GetSubscriptionCount() int {
	if !p.enabled {
		return 0
	}
	return len(p.selectedSymbols)
}

// IsEnabled returns whether the books channel is enabled
func (p *BooksChannelPanel) IsEnabled() bool {
	return p.enabled
}

func (p *BooksChannelPanel) LoadState(uiState *config.UIState) {
	if uiState == nil || uiState.ChannelStates == nil {
		return
	}
	if channelState, ok := uiState.ChannelStates["books"].(map[string]interface{}); ok {
		if enabled, ok := channelState["enabled"].(bool); ok {
			p.enabled = enabled
			if p.enableCheck != nil {
				p.updating = true
				p.enableCheck.SetChecked(enabled)
				p.updating = false
				if enabled {
					p.symbolList.Enable()
					p.precSelect.Enable()
					p.freqSelect.Enable()
					p.lenSelect.Enable()
				} else {
					p.symbolList.Disable()
					p.precSelect.Disable()
					p.freqSelect.Disable()
					p.lenSelect.Disable()
				}
			}
		}
		if prec, ok := channelState["precision"].(string); ok {
			p.precision = prec
			if p.precSelect != nil {
				p.updating = true
				p.precSelect.SetSelected(prec)
				p.updating = false
			}
		}
		if freq, ok := channelState["frequency"].(string); ok {
			p.frequency = freq
			if p.freqSelect != nil {
				p.updating = true
				p.freqSelect.SetSelected(freq)
				p.updating = false
			}
		}
		if length, ok := channelState["length"].(string); ok {
			p.length = length
			if p.lenSelect != nil {
				p.updating = true
				p.lenSelect.SetSelected(length)
				p.updating = false
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

func (p *BooksChannelPanel) SaveState(uiState *config.UIState) {
	if uiState.ChannelStates == nil {
		uiState.ChannelStates = make(map[string]interface{})
	}
	selectedList := make([]string, 0, len(p.selectedSymbols))
	for sym := range p.selectedSymbols {
		selectedList = append(selectedList, sym)
	}
	uiState.ChannelStates["books"] = map[string]interface{}{
		"enabled":          p.enabled,
		"precision":        p.precision,
		"frequency":        p.frequency,
		"length":           p.length,
		"selected_symbols": selectedList,
	}
}

func (p *BooksChannelPanel) Reset() {
	p.enabled = false
	p.selectedSymbols = make(map[string]bool)
	p.precision = "P0"
	p.frequency = "F0"
	p.length = "25"

	if p.enableCheck != nil {
		p.updating = true
		p.enableCheck.SetChecked(false)
		p.updating = false
		p.symbolList.Disable()
		p.precSelect.Disable()
		p.freqSelect.Disable()
		p.lenSelect.Disable()
	}
	if p.symbolList != nil {
		p.updating = true
		p.symbolList.SetSelected([]string{})
		p.updating = false
	}
	if p.searchEntry != nil {
		p.searchEntry.SetText("")
	}
	if p.precSelect != nil {
		p.updating = true
		p.precSelect.SetSelected("P0")
		p.updating = false
	}
	if p.freqSelect != nil {
		p.updating = true
		p.freqSelect.SetSelected("F0")
		p.updating = false
	}
	if p.lenSelect != nil {
		p.updating = true
		p.lenSelect.SetSelected("25")
		p.updating = false
	}

	p.persistState()
	p.notifyStateChange()
}

func (p *BooksChannelPanel) ReloadSymbols() {
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

func (p *BooksChannelPanel) notifyStateChange() {
	if p.onStateChange != nil {
		p.onStateChange()
	}
}

func (p *BooksChannelPanel) persistState() {
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
		p.logger.Warn("failed to persist books channel state", zap.Error(err))
	}
}

func (p *BooksChannelPanel) currentActualSymbols() []string {
	out := make([]string, 0, len(p.selectedSymbols))
	for sym := range p.selectedSymbols {
		out = append(out, sym)
	}
	sort.Strings(out)
	return out
}

func (p *BooksChannelPanel) currentDisplaySelection() []string {
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
