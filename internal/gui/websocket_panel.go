package gui

import (
	"fmt"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
)

// WebSocketPanel manages the WebSocket configuration UI
type WebSocketPanel struct {
	logger        *zap.Logger
	configManager *config.ConfigManager
	exchange      string

	// UI Components
	container        *fyne.Container
	channelTabs      *container.AppTabs
	connectBtn       *widget.Button
	subscriptionInfo *widget.Label
	statusBar        *widget.Label
	timestampCheck   *widget.Check
	sequenceCheck    *widget.Check
	checksumCheck    *widget.Check
	bulkCheck        *widget.Check
	noDataBanner     fyne.CanvasObject

	// Channel panels
	tickerPanel  *TickerChannelPanel
	tradesPanel  *TradesChannelPanel
	booksPanel   *BooksChannelPanel
	candlesPanel *CandlesChannelPanel
	statusPanel  *StatusChannelPanel

	// State
	subscriptionCount binding.Int
	maxSubscriptions  int
	restoring         bool

	// Callbacks
	onConnect    func(config *WSConnectionConfig) error
	onDisconnect func() error
}

// WSConnectionConfig holds WebSocket connection configuration
type WSConnectionConfig struct {
	Exchange  string
	Symbols   []string
	Channels  []ChannelSubscription
	ConfFlags int64
}

// ChannelSubscription represents a channel subscription request
type ChannelSubscription struct {
	Channel string
	Symbol  string
	Prec    string
	Freq    string
	Len     string
	Key     string
}

// NewWebSocketPanel creates a new WebSocket configuration panel
func NewWebSocketPanel(logger *zap.Logger, configManager *config.ConfigManager, exchange string) *WebSocketPanel {
	panel := &WebSocketPanel{
		logger:            logger,
		configManager:     configManager,
		exchange:          exchange,
		subscriptionCount: binding.NewInt(),
		maxSubscriptions:  30, // Bitfinex limit
	}

	panel.subscriptionCount.Set(0)
	panel.buildUI()
	panel.loadState()

	return panel
}

// buildUI constructs the UI components
func (p *WebSocketPanel) buildUI() {
	// Create channel configuration panels
	p.tickerPanel = NewTickerChannelPanel(p.logger, p.configManager, p.exchange)
	p.tradesPanel = NewTradesChannelPanel(p.logger, p.configManager, p.exchange)
	p.booksPanel = NewBooksChannelPanel(p.logger, p.configManager, p.exchange)
	p.candlesPanel = NewCandlesChannelPanel(p.logger, p.configManager, p.exchange)
	p.statusPanel = NewStatusChannelPanel(p.logger, p.configManager, p.exchange)

	// Wire callbacks for subscription counting and limit enforcement
	p.tickerPanel.SetOnStateChange(p.handleChannelStateChange)
	p.tradesPanel.SetOnStateChange(p.handleChannelStateChange)
	p.booksPanel.SetOnStateChange(p.handleChannelStateChange)
	p.candlesPanel.SetOnStateChange(p.handleChannelStateChange)
	p.statusPanel.SetOnStateChange(p.handleChannelStateChange)

	p.tickerPanel.SetLimitChecker(p.canAddSubscriptions)
	p.tradesPanel.SetLimitChecker(p.canAddSubscriptions)
	p.booksPanel.SetLimitChecker(p.canAddSubscriptions)
	p.candlesPanel.SetLimitChecker(p.canAddSubscriptions)
	p.statusPanel.SetLimitChecker(p.canAddSubscriptions)

	// Create tabs for each channel type
	p.channelTabs = container.NewAppTabs(
		container.NewTabItem("Ticker", p.tickerPanel.Build()),
		container.NewTabItem("Trades", p.tradesPanel.Build()),
		container.NewTabItem("Books", p.booksPanel.Build()),
		container.NewTabItem("Candles", p.candlesPanel.Build()),
		container.NewTabItem("Status", p.statusPanel.Build()),
	)

	// Tab change callback to persist state
	p.channelTabs.OnSelected = func(tab *container.TabItem) {
		p.saveActiveTab(tab.Text)
	}

	// Connection flag controls
	p.timestampCheck = widget.NewCheck("Timestamp (32768)", func(checked bool) {
		if p.restoring {
			return
		}
		p.updateConnectionFlags(func(flags *config.ConnectionFlags) {
			flags.Timestamp = checked
		})
	})
	p.sequenceCheck = widget.NewCheck("Sequence Numbers (65536)", func(checked bool) {
		if p.restoring {
			return
		}
		p.updateConnectionFlags(func(flags *config.ConnectionFlags) {
			flags.Sequence = checked
		})
	})

	p.checksumCheck = widget.NewCheck("Order Book Checksum (131072)", func(checked bool) {
		if p.restoring {
			return
		}
		p.updateConnectionFlags(func(flags *config.ConnectionFlags) {
			flags.Checksum = checked
		})
	})

	p.bulkCheck = widget.NewCheck("Bulk Book Updates (536870912)", func(checked bool) {
		if p.restoring {
			return
		}
		p.updateConnectionFlags(func(flags *config.ConnectionFlags) {
			flags.Bulk = checked
		})
	})

	flagsHeader := widget.NewLabelWithStyle("Connection Flags", fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	flagsDescription := widget.NewLabel("Apply Bitfinex configuration flags immediately after connecting.")
	flagsDescription.Wrapping = fyne.TextWrapWord

	flagsGroup := container.NewVBox(
		flagsHeader,
		flagsDescription,
		p.timestampCheck,
		p.sequenceCheck,
		p.checksumCheck,
		p.bulkCheck,
	)

	bodyItems := []fyne.CanvasObject{}

	if !p.hasCachedConfig() {
		p.noDataBanner = p.buildNoDataBanner()
		bodyItems = append(bodyItems, p.noDataBanner)
	}

	bodyItems = append(bodyItems,
		p.channelTabs,
		widget.NewSeparator(),
		flagsGroup,
	)

	body := container.NewVBox(bodyItems...)
	bodyScroll := container.NewVScroll(body)

	// Subscription counter
	p.subscriptionInfo = widget.NewLabel("")
	p.updateSubscriptionInfo()

	// Status bar (hidden until a message needs to be shown)
	p.statusBar = widget.NewLabel("")
	p.statusBar.Wrapping = fyne.TextWrapWord
	p.statusBar.Hide()

	// Connect button
	p.connectBtn = widget.NewButton("Connect", func() {
		p.handleConnect()
	})

	// Bottom section with connection controls
	bottomSection := container.NewVBox(
		widget.NewSeparator(),
		p.subscriptionInfo,
		container.NewHBox(p.connectBtn),
		p.statusBar,
	)

	// Main container with scrolling
	p.container = container.NewBorder(
		nil,
		bottomSection,
		nil,
		nil,
		container.NewMax(bodyScroll),
	)
}

// Build returns the panel's UI container
func (p *WebSocketPanel) Build() fyne.CanvasObject {
	return p.container
}

// SetConnectCallback sets the callback for connect action
func (p *WebSocketPanel) SetConnectCallback(fn func(*WSConnectionConfig) error) {
	p.onConnect = fn
}

// SetDisconnectCallback sets the callback for disconnect action
func (p *WebSocketPanel) SetDisconnectCallback(fn func() error) {
	p.onDisconnect = fn
}

// handleConnect handles the connect button action
func (p *WebSocketPanel) handleConnect() {
	if p.connectBtn.Text == "Connect" {
		// Validate configuration
		if err := p.validateConfig(); err != nil {
			p.showError(err.Error())
			return
		}

		// Build connection config
		wsConfig := p.buildConnectionConfig()

		// Call connect callback
		if p.onConnect != nil {
			if err := p.onConnect(wsConfig); err != nil {
				p.showError(fmt.Sprintf("Connection failed: %v", err))
				return
			}
		}

		// Update UI
		p.connectBtn.SetText("Disconnect")
		p.setStatusMessage("")
		p.saveState()

		if p.configManager != nil {
			p.configManager.StartPeriodicUpdates(p.exchange)
		}

	} else {
		// Disconnect
		if p.onDisconnect != nil {
			if err := p.onDisconnect(); err != nil {
				p.showError(fmt.Sprintf("Disconnect failed: %v", err))
				return
			}
		}

		p.connectBtn.SetText("Connect")
		p.setStatusMessage("")

		if p.configManager != nil {
			p.configManager.StopPeriodicUpdates()
		}
	}
}

// validateConfig validates the current configuration
func (p *WebSocketPanel) validateConfig() error {
	// Check subscription count
	count, _ := p.subscriptionCount.Get()
	if count > p.maxSubscriptions {
		return fmt.Errorf("subscription count (%d) exceeds limit (%d)", count, p.maxSubscriptions)
	}

	if count == 0 {
		return fmt.Errorf("no channels selected for subscription")
	}

	return nil
}

// buildConnectionConfig builds the WebSocket connection configuration
func (p *WebSocketPanel) buildConnectionConfig() *WSConnectionConfig {
	config := &WSConnectionConfig{
		Exchange:  p.exchange,
		Symbols:   []string{},
		Channels:  []ChannelSubscription{},
		ConfFlags: p.calculateConfFlags(),
	}

	// Collect subscriptions from all panels
	config.Channels = append(config.Channels, p.tickerPanel.GetSubscriptions()...)
	config.Channels = append(config.Channels, p.tradesPanel.GetSubscriptions()...)
	config.Channels = append(config.Channels, p.booksPanel.GetSubscriptions()...)
	config.Channels = append(config.Channels, p.candlesPanel.GetSubscriptions()...)
	config.Channels = append(config.Channels, p.statusPanel.GetSubscriptions()...)

	// Extract unique symbols
	symbolSet := make(map[string]bool)
	for _, sub := range config.Channels {
		if sub.Symbol != "" {
			symbolSet[sub.Symbol] = true
		}
	}

	for symbol := range symbolSet {
		config.Symbols = append(config.Symbols, symbol)
	}
	sort.Strings(config.Symbols)

	return config
}

// calculateConfFlags calculates the WebSocket configuration flags
func (p *WebSocketPanel) calculateConfFlags() int64 {
	uiState := p.configManager.GetApplicationState().GetUIState(p.exchange)
	flags := int64(0)

	if uiState.ConnectionFlags.Timestamp {
		flags += 32768 // TIMESTAMP
	}
	if uiState.ConnectionFlags.Sequence {
		flags += 65536 // SEQ_ALL
	}
	if uiState.ConnectionFlags.Checksum {
		flags += 131072 // OB_CHECKSUM
	}
	if uiState.ConnectionFlags.Bulk {
		flags += 536870912 // BULK_UPDATES
	}

	return flags
}

// updateSubscriptionInfo updates the subscription counter display
func (p *WebSocketPanel) updateSubscriptionInfo() {
	count, _ := p.subscriptionCount.Get()
	text := fmt.Sprintf("Subscriptions: %d / %d", count, p.maxSubscriptions)

	if count >= p.maxSubscriptions {
		text += " ⚠️ LIMIT REACHED"
	} else if count >= p.maxSubscriptions-5 {
		text += " ⚠️ NEAR LIMIT"
	}

	p.subscriptionInfo.SetText(text)
}

// handleChannelStateChange recomputes the aggregate subscription count
func (p *WebSocketPanel) handleChannelStateChange() {
	totalSubs := p.tickerPanel.GetSubscriptionCount() +
		p.tradesPanel.GetSubscriptionCount() +
		p.booksPanel.GetSubscriptionCount() +
		p.candlesPanel.GetSubscriptionCount() +
		p.statusPanel.GetSubscriptionCount()

	p.subscriptionCount.Set(totalSubs)
	p.updateSubscriptionInfo()
}

// canAddSubscriptions validates whether additional subscriptions can be added without exceeding the limit
func (p *WebSocketPanel) canAddSubscriptions(delta int) bool {
	if delta <= 0 {
		return true
	}

	count, _ := p.subscriptionCount.Get()
	if count+delta > p.maxSubscriptions {
		warning := fmt.Sprintf("⚠️ Subscription limit reached (%d/%d). Remove channels before adding new ones.", count, p.maxSubscriptions)
		p.setStatusMessage(warning)
		return false
	}

	return true
}

// updateConnectionFlags persists connection flag changes to application state
func (p *WebSocketPanel) updateConnectionFlags(mutator func(*config.ConnectionFlags)) {
	if p.configManager == nil {
		return
	}

	state := p.configManager.GetApplicationState()
	if state == nil {
		return
	}

	uiState := state.GetUIState(p.exchange)
	flags := uiState.ConnectionFlags
	mutator(&flags)
	uiState.ConnectionFlags = flags
	state.UpdateUIState(p.exchange, uiState)

	if err := p.configManager.SaveState(); err != nil {
		p.logger.Warn("failed to persist connection flags", zap.Error(err))
	}
}

func (p *WebSocketPanel) hasCachedConfig() bool {
	if p.configManager == nil {
		return false
	}

	pairs, err := p.configManager.GetAvailablePairs(p.exchange, "exchange")
	if err != nil {
		return false
	}

	return len(pairs) > 0
}

func (p *WebSocketPanel) buildNoDataBanner() fyne.CanvasObject {
	message := widget.NewLabel("No cached Bitfinex REST config found. Fetching now is recommended to populate symbol lists.")
	message.Wrapping = fyne.TextWrapWord

	var card *widget.Card
	yesBtn := widget.NewButton("Yes", func() {
		if card != nil {
			card.Hide()
		}
		p.fetchInitialConfig(card)
	})

	laterBtn := widget.NewButton("Later", func() {
		if card != nil {
			card.Hide()
		}
		p.setStatusMessage("Config fetch postponed. UI may use limited fallback symbols.")
	})

	buttonRow := container.NewHBox(yesBtn, laterBtn)
	content := container.NewVBox(message, buttonRow)

	card = widget.NewCard(
		"Config cache empty",
		"No data. Do you want to fetch config data?",
		content,
	)

	return card
}

func (p *WebSocketPanel) fetchInitialConfig(banner fyne.CanvasObject) {
	if p.configManager == nil {
		p.setStatusMessage("Config manager not initialized")
		return
	}

	p.setStatusMessage("Fetching Bitfinex config...")

	err := p.configManager.RefreshConfigOnConnect(p.exchange)
	if err != nil {
		p.setStatusMessage(fmt.Sprintf("Config fetch failed: %v", err))
		if banner != nil {
			banner.Show()
		}
		return
	}

	p.tickerPanel.ReloadSymbols()
	p.tradesPanel.ReloadSymbols()
	p.booksPanel.ReloadSymbols()
	p.candlesPanel.ReloadSymbols()

	p.handleChannelStateChange()

	if banner != nil {
		banner.Hide()
	}

	p.setStatusMessage("Config data refreshed from REST API")
}

// IncrementSubscriptionCount increments the subscription counter
func (p *WebSocketPanel) IncrementSubscriptionCount() error {
	count, _ := p.subscriptionCount.Get()
	if count >= p.maxSubscriptions {
		return fmt.Errorf("subscription limit reached (%d)", p.maxSubscriptions)
	}

	p.subscriptionCount.Set(count + 1)
	p.updateSubscriptionInfo()
	return nil
}

// DecrementSubscriptionCount decrements the subscription counter
func (p *WebSocketPanel) DecrementSubscriptionCount() {
	count, _ := p.subscriptionCount.Get()
	if count > 0 {
		p.subscriptionCount.Set(count - 1)
		p.updateSubscriptionInfo()
	}
}

// GetSubscriptionCount returns the current subscription count
func (p *WebSocketPanel) GetSubscriptionCount() int {
	count, _ := p.subscriptionCount.Get()
	return count
}

// loadState loads the UI state from persistence
func (p *WebSocketPanel) loadState() {
	state := p.configManager.GetApplicationState()
	uiState := state.GetUIState(p.exchange)

	// Restore active tab
	if uiState.ActiveTab != "" {
		for i, tab := range p.channelTabs.Items {
			if strings.ToLower(tab.Text) == strings.ToLower(uiState.ActiveTab) {
				p.channelTabs.SelectIndex(i)
				break
			}
		}
	}

	// Restore connection flags
	p.restoring = true
	if p.timestampCheck != nil {
		p.timestampCheck.SetChecked(uiState.ConnectionFlags.Timestamp)
	}
	if p.sequenceCheck != nil {
		p.sequenceCheck.SetChecked(uiState.ConnectionFlags.Sequence)
	}
	if p.checksumCheck != nil {
		p.checksumCheck.SetChecked(uiState.ConnectionFlags.Checksum)
	}
	if p.bulkCheck != nil {
		p.bulkCheck.SetChecked(uiState.ConnectionFlags.Bulk)
	}
	p.restoring = false

	// Load individual panel states
	p.tickerPanel.LoadState(uiState)
	p.tradesPanel.LoadState(uiState)
	p.booksPanel.LoadState(uiState)
	p.candlesPanel.LoadState(uiState)
	p.statusPanel.LoadState(uiState)

	// Update subscription count display
	p.handleChannelStateChange()
}

// saveState saves the current UI state
func (p *WebSocketPanel) saveState() {
	state := p.configManager.GetApplicationState()
	uiState := state.GetUIState(p.exchange)

	// Save panel states
	p.tickerPanel.SaveState(uiState)
	p.tradesPanel.SaveState(uiState)
	p.booksPanel.SaveState(uiState)
	p.candlesPanel.SaveState(uiState)
	p.statusPanel.SaveState(uiState)

	// Persist connection flags from UI controls
	if p.timestampCheck != nil && p.sequenceCheck != nil && p.checksumCheck != nil && p.bulkCheck != nil {
		uiState.ConnectionFlags = config.ConnectionFlags{
			Timestamp: p.timestampCheck.Checked,
			Sequence:  p.sequenceCheck.Checked,
			Checksum:  p.checksumCheck.Checked,
			Bulk:      p.bulkCheck.Checked,
		}
	}

	state.UpdateUIState(p.exchange, uiState)

	// Save to disk
	if err := p.configManager.SaveState(); err != nil {
		p.logger.Error("Failed to save state", zap.Error(err))
	}
}

// saveActiveTab saves the currently active tab
func (p *WebSocketPanel) saveActiveTab(tabName string) {
	state := p.configManager.GetApplicationState()
	uiState := state.GetUIState(p.exchange)
	uiState.ActiveTab = strings.ToLower(tabName)

	state.UpdateUIState(p.exchange, uiState)
	p.configManager.SaveState()
}

func (p *WebSocketPanel) setStatusMessage(message string) {
	if p.statusBar == nil {
		return
	}
	trimmed := strings.TrimSpace(message)
	fyne.Do(func() {
		if trimmed == "" {
			p.statusBar.SetText("")
			p.statusBar.Hide()
		} else {
			p.statusBar.SetText(trimmed)
			p.statusBar.Show()
		}
	})
}

// showError displays an error message
func (p *WebSocketPanel) showError(message string) {
	p.setStatusMessage(fmt.Sprintf("Error: %s", message))
	p.logger.Error("WebSocket panel error", zap.String("message", message))
}

// Reset resets the panel to initial state
func (p *WebSocketPanel) Reset() {
	p.tickerPanel.Reset()
	p.tradesPanel.Reset()
	p.booksPanel.Reset()
	p.candlesPanel.Reset()
	p.statusPanel.Reset()

	if p.timestampCheck != nil && p.sequenceCheck != nil && p.checksumCheck != nil && p.bulkCheck != nil {
		p.restoring = true
		p.timestampCheck.SetChecked(true)
		p.sequenceCheck.SetChecked(false)
		p.checksumCheck.SetChecked(true)
		p.bulkCheck.SetChecked(false)
		p.restoring = false
		p.updateConnectionFlags(func(flags *config.ConnectionFlags) {
			flags.Timestamp = true
			flags.Sequence = false
			flags.Checksum = true
			flags.Bulk = false
		})
	}

	p.subscriptionCount.Set(0)
	p.updateSubscriptionInfo()
	p.connectBtn.SetText("Connect")
	p.setStatusMessage("")
}
