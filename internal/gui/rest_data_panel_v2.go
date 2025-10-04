package gui

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/restapi"
)

// ConnectionState represents the REST connection state
type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnected
	StateActive
)

var defaultRestSymbols = []string{"tBTCUSD", "tETHUSD"}

// RestDataPanelV2 is the new REST API Data panel with tabbed interface
type RestDataPanelV2 struct {
	widget.BaseWidget

	// State
	state ConnectionState
	mu    sync.RWMutex

	// Data type tabs
	candlesPanel *RestChannelCandles
	tradesPanel  *RestChannelTrades
	tickersPanel *RestChannelTickers
	dataTabs     *container.AppTabs

	// Shared buttons at bottom
	connectBtn   *widget.Button
	startBtn     *widget.Button
	logBtn       *widget.Button
	directoryBtn *widget.Button

	// Popup windows
	logWindow fyne.Window
	logText   *widget.Entry

	// REST client and rate limiter
	client      *restapi.BitfinexDataClient
	rateLimiter *restapi.SafeRateLimiter

	// Context for cancellation
	ctx        context.Context
	cancelFunc context.CancelFunc

	// Symbols list and loaders
	symbols      []string
	symbolLoader func() ([]string, error)

	// Data directory management
	dataDir string

	// Parent window for dialogs
	parentWindow fyne.Window

	// Callback for state changes
	onStateChange func(connected bool)
}

// NewRestDataPanelV2 creates a new REST API Data panel
// parentWindow can be nil and set later with SetParentWindow
func NewRestDataPanelV2(parentWindow fyne.Window) *RestDataPanelV2 {
	p := &RestDataPanelV2{
		state:        StateDisconnected,
		parentWindow: parentWindow,
		symbols:      []string{}, // Will be loaded from config
		dataDir:      filepath.Join("data", "bitfinex", "restapi", "data"),
		rateLimiter:  restapi.NewSafeRateLimiter(),
	}
	p.ExtendBaseWidget(p)

	p.initComponents()
	p.updateButtonStates()

	return p
}

// SetParentWindow sets the parent window for dialogs
func (p *RestDataPanelV2) SetParentWindow(w fyne.Window) {
	p.parentWindow = w
}

// SetOnStateChange sets the callback for connection state changes
func (p *RestDataPanelV2) SetOnStateChange(callback func(connected bool)) {
	p.onStateChange = callback
}

// SetSymbolLoader configures a loader used to refresh the available symbol list
func (p *RestDataPanelV2) SetSymbolLoader(loader func() ([]string, error)) {
	p.symbolLoader = loader
}

// RefreshSymbols reloads symbols using the configured loader
func (p *RestDataPanelV2) RefreshSymbols() error {
	return p.loadSymbols()
}

// SetDataDirectory updates the base directory used for REST data output
func (p *RestDataPanelV2) SetDataDirectory(path string) {
	clean := strings.TrimSpace(path)
	if clean == "" {
		return
	}
	p.dataDir = filepath.Clean(clean)
}

func (p *RestDataPanelV2) activeContext() context.Context {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.ctx != nil {
		return p.ctx
	}
	return context.Background()
}

// initComponents initializes all UI components
func (p *RestDataPanelV2) initComponents() {
	// Initialize log text first (needed by callbacks)
	p.logText = widget.NewMultiLineEntry()
	p.logText.Wrapping = fyne.TextWrapWord
	p.logText.Disable()

	// Create data type panels
	onChange := func() {
		p.logMessage("Configuration changed")
	}

	p.candlesPanel = NewRestChannelCandles(p.symbols, onChange)
	p.tradesPanel = NewRestChannelTrades(p.symbols, onChange)
	p.tickersPanel = NewRestChannelTickers(p.symbols, onChange)

	// Create tabbed interface
	p.dataTabs = container.NewAppTabs(
		container.NewTabItem("Candles", p.candlesPanel),
		container.NewTabItem("Trades", p.tradesPanel),
		container.NewTabItem("Tickers History", p.tickersPanel),
	)

	// Create shared buttons (square flat design)
	p.connectBtn = widget.NewButton("Connect", func() {
		p.toggleConnection()
	})
	p.connectBtn.Importance = widget.WarningImportance // Orange

	p.startBtn = widget.NewButton("Start", func() {
		p.toggleStart()
	})
	p.startBtn.Importance = widget.WarningImportance // Orange
	p.startBtn.Disable()

	p.logBtn = widget.NewButton("Activity Log", func() {
		p.showLogWindow()
	})

	p.directoryBtn = widget.NewButton("Open Directory", func() {
		p.openDirectory()
	})
}

// CreateRenderer creates the widget renderer
func (p *RestDataPanelV2) CreateRenderer() fyne.WidgetRenderer {
	// Buttons at bottom (shared across all tabs)
	buttonContainer := container.NewHBox(
		p.connectBtn,
		p.startBtn,
		p.logBtn,
		p.directoryBtn,
	)

	// Main layout: tabs on top, buttons at bottom
	content := container.NewBorder(
		nil,
		buttonContainer,
		nil, nil,
		p.dataTabs,
	)

	return widget.NewSimpleRenderer(content)
}

// toggleConnection handles Connect/Disconnect toggle
func (p *RestDataPanelV2) toggleConnection() {
	p.mu.Lock()
	var err error
	var defMessage string
	var infoMessage string

	switch p.state {
	case StateDisconnected:
		err = p.connect()
		if err == nil {
			p.state = StateConnected
			defMessage = "Connected to Bitfinex REST API"
		}

	case StateConnected:
		p.disconnect()
		p.state = StateDisconnected
		defMessage = "Disconnected from Bitfinex REST API"

	case StateActive:
		infoMessage = "Please stop active data collection before disconnecting."
	}
	p.mu.Unlock()

	if infoMessage != "" {
		if win := p.resolveWindow(); win != nil {
			dialog.ShowInformation("Cannot Disconnect", infoMessage, win)
		} else {
			p.logMessage("Cannot disconnect while jobs are active; stop them first")
		}
		return
	}

	if err != nil {
		p.logMessage(fmt.Sprintf("Connection failed: %v", err))
		p.showError(err)
		return
	}

	if defMessage != "" {
		p.logMessage(defMessage)
	}

	p.updateButtonStates()

	// Notify state change
	if p.onStateChange != nil {
		p.mu.RLock()
		connected := p.state != StateDisconnected
		p.mu.RUnlock()
		p.onStateChange(connected)
	}
}

// toggleStart handles Start/Stop toggle
func (p *RestDataPanelV2) toggleStart() {
	var (
		err        error
		defMessage string
	)

	p.mu.Lock()
	switch p.state {
	case StateDisconnected:
		// Should not happen (button disabled)
		p.mu.Unlock()
		return

	case StateConnected:
		ctx := p.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		p.state = StateActive
		p.mu.Unlock()

		err = p.startDataCollection(ctx)
		if err != nil {
			p.stopDataCollection()
			p.mu.Lock()
			p.state = StateConnected
			p.mu.Unlock()
			break
		}

		defMessage = "Started data collection"

	case StateActive:
		p.stopDataCollection()
		p.state = StateConnected
		p.mu.Unlock()
		defMessage = "Stopped data collection"

		p.updateButtonStates()
		if defMessage != "" {
			p.logMessage(defMessage)
		}
		return

	default:
		p.mu.Unlock()
		return
	}

	if err != nil {
		p.logMessage(fmt.Sprintf("Failed to start: %v", err))
		p.showError(err)
		return
	}

	if defMessage != "" {
		p.logMessage(defMessage)
	}

	p.updateButtonStates()
}

// connect establishes connection to REST API
func (p *RestDataPanelV2) connect() error {
	// Create REST client with logger
	logger := zap.NewNop() // TODO: Get logger from application config
	p.client = restapi.NewBitfinexDataClient(logger)

	// Create cancellation context
	p.ctx, p.cancelFunc = context.WithCancel(context.Background())

	// Load symbols from config or fetch from API
	if err := p.loadSymbols(); err != nil {
		p.logMessage(fmt.Sprintf("Failed to refresh symbols: %v", err))
	}

	return nil
}

// disconnect closes REST API connection
func (p *RestDataPanelV2) disconnect() {
	if p.cancelFunc != nil {
		p.cancelFunc()
	}
	p.client = nil
}

// startDataCollection starts fetching data for enabled data types
func (p *RestDataPanelV2) startDataCollection(ctx context.Context) error {
	// Check at least one data type is enabled
	if !p.candlesPanel.IsEnabled() && !p.tradesPanel.IsEnabled() && !p.tickersPanel.IsEnabled() {
		return fmt.Errorf("no data types enabled")
	}

	if strings.TrimSpace(p.dataDir) == "" {
		return fmt.Errorf("data directory is not configured")
	}

	if err := os.MkdirAll(p.dataDir, 0o755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	if ctx == nil {
		ctx = context.Background()
	}
	runStamp := time.Now().UTC().Format("20060102_150405")

	var wg sync.WaitGroup
	jobs := 0

	// Start goroutines for each enabled data type
	if p.candlesPanel.IsEnabled() {
		jobs++
		wg.Add(1)
		go p.collectCandles(ctx, runStamp, &wg)
	}

	if p.tradesPanel.IsEnabled() {
		jobs++
		wg.Add(1)
		go p.collectTrades(ctx, runStamp, &wg)
	}

	if p.tickersPanel.IsEnabled() {
		jobs++
		wg.Add(1)
		go p.collectTickers(ctx, runStamp, &wg)
	}

	if jobs == 0 {
		return fmt.Errorf("no data jobs scheduled")
	}

	go func() {
		wg.Wait()
		p.mu.Lock()
		if p.state == StateActive {
			p.state = StateConnected
		}
		p.mu.Unlock()
		p.logMessage("All REST data jobs completed")
		fyne.Do(func() {
			p.updateButtonStates()
		})
	}()

	return nil
}

// stopDataCollection stops all active data collection
func (p *RestDataPanelV2) stopDataCollection() {
	// Cancel context to stop all goroutines
	if p.cancelFunc != nil {
		p.cancelFunc()
	}

	// Recreate context for next start
	p.ctx, p.cancelFunc = context.WithCancel(context.Background())
}

// collectCandles fetches candles data and writes them to CSV files
func (p *RestDataPanelV2) collectCandles(ctx context.Context, runStamp string, wg *sync.WaitGroup) {
	defer wg.Done()

	if p.client == nil {
		p.logMessage("Candles: REST client not initialised")
		return
	}

	symbols := p.candlesPanel.GetSelectedSymbols()
	if len(symbols) == 0 {
		p.logMessage("Candles: no symbols selected")
		return
	}

	timeframes := p.candlesPanel.GetTimeframes()
	if len(timeframes) == 0 {
		p.logMessage("Candles: no timeframes selected")
		return
	}

	start, end := p.candlesPanel.GetTimeRange()
	if !end.After(start) {
		p.logMessage("Candles: end time must be after start time")
		return
	}

	limit := p.candlesPanel.GetLimit()
	if limit <= 0 {
		limit = 200
	}
	if limit > 10000 {
		limit = 10000
	}
	sortOrder := normaliseSort(p.candlesPanel.GetSort())

	outputDir := filepath.Join(p.dataDir, "candles")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		p.logMessage(fmt.Sprintf("Candles: create output directory failed: %v", err))
		return
	}

	p.logMessage(fmt.Sprintf("Candles: %d symbols, %d timeframes, %s to %s",
		len(symbols), len(timeframes), start.Format("2006-01-02"), end.Format("2006-01-02")))

	for _, symbol := range symbols {
		for _, tf := range timeframes {
			select {
			case <-ctx.Done():
				p.logMessage("Candles: operation cancelled")
				return
			default:
			}

			fileName := fmt.Sprintf("candles_%s_%s_%s.csv", sanitizeSymbolForFile(symbol), tf, runStamp)
			filePath := filepath.Join(outputDir, fileName)

			if err := p.fetchCandlesToCSV(ctx, symbol, tf, limit, sortOrder, start, end, filePath); err != nil {
				p.logMessage(fmt.Sprintf("Candles: %s %s failed: %v", symbol, tf, err))
			} else {
				p.logMessage(fmt.Sprintf("Candles: %s %s saved → %s", symbol, tf, fileName))
			}
		}
	}

	p.logMessage("Candles collection completed")
}

// collectTrades fetches trades data and writes them to CSV files
func (p *RestDataPanelV2) collectTrades(ctx context.Context, runStamp string, wg *sync.WaitGroup) {
	defer wg.Done()

	if p.client == nil {
		p.logMessage("Trades: REST client not initialised")
		return
	}

	symbols := p.tradesPanel.GetSelectedSymbols()
	if len(symbols) == 0 {
		p.logMessage("Trades: no symbols selected")
		return
	}

	start, end := p.tradesPanel.GetTimeRange()
	if !end.After(start) {
		p.logMessage("Trades: end time must be after start time")
		return
	}

	limit := p.tradesPanel.GetLimit()
	if limit <= 0 {
		limit = 100
	}
	if limit > 10000 {
		limit = 10000
	}
	sortOrder := normaliseSort(p.tradesPanel.GetSort())

	outputDir := filepath.Join(p.dataDir, "trades")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		p.logMessage(fmt.Sprintf("Trades: create output directory failed: %v", err))
		return
	}

	p.logMessage(fmt.Sprintf("Trades: %d symbols, %s to %s",
		len(symbols), start.Format("2006-01-02"), end.Format("2006-01-02")))

	for _, symbol := range symbols {
		select {
		case <-ctx.Done():
			p.logMessage("Trades: operation cancelled")
			return
		default:
		}

		fileName := fmt.Sprintf("trades_%s_%s.csv", sanitizeSymbolForFile(symbol), runStamp)
		filePath := filepath.Join(outputDir, fileName)

		if err := p.fetchTradesToCSV(ctx, symbol, limit, sortOrder, start, end, filePath); err != nil {
			p.logMessage(fmt.Sprintf("Trades: %s failed: %v", symbol, err))
		} else {
			p.logMessage(fmt.Sprintf("Trades: %s saved → %s", symbol, fileName))
		}
	}

	p.logMessage("Trades collection completed")
}

// collectTickers fetches tickers history data and writes them to a CSV file
func (p *RestDataPanelV2) collectTickers(ctx context.Context, runStamp string, wg *sync.WaitGroup) {
	defer wg.Done()

	if p.client == nil {
		p.logMessage("Tickers: REST client not initialised")
		return
	}

	symbols := p.tickersPanel.GetSelectedSymbols()
	if len(symbols) == 0 {
		p.logMessage("Tickers: no symbols selected")
		return
	}

	start, end := p.tickersPanel.GetTimeRange()
	if !end.After(start) {
		p.logMessage("Tickers: end time must be after start time")
		return
	}

	limit := p.tickersPanel.GetLimit()
	if limit <= 0 {
		limit = 100
	}
	if limit > 250 {
		limit = 250
	}
	sortOrder := normaliseSort(p.tickersPanel.GetSort())

	outputDir := filepath.Join(p.dataDir, "tickers")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		p.logMessage(fmt.Sprintf("Tickers: create output directory failed: %v", err))
		return
	}

	p.logMessage(fmt.Sprintf("Tickers: %d symbols, %s to %s",
		len(symbols), start.Format("2006-01-02"), end.Format("2006-01-02")))

	fileName := fmt.Sprintf("tickers_%s.csv", runStamp)
	filePath := filepath.Join(outputDir, fileName)

	if err := p.fetchTickersToCSV(ctx, symbols, limit, sortOrder, start, end, filePath); err != nil {
		p.logMessage(fmt.Sprintf("Tickers: failed: %v", err))
	} else {
		p.logMessage(fmt.Sprintf("Tickers: saved → %s", fileName))
	}

	p.logMessage("Tickers collection completed")
}

// loadSymbols loads symbols from config or API
func (p *RestDataPanelV2) loadSymbols() error {
	if p.symbolLoader == nil {
		if len(p.symbols) == 0 {
			p.SetSymbols(defaultRestSymbols)
		}
		return nil
	}

	symbols, err := p.symbolLoader()
	if err != nil {
		if len(p.symbols) == 0 {
			p.SetSymbols(defaultRestSymbols)
		}
		return err
	}

	if len(symbols) == 0 {
		if len(p.symbols) == 0 {
			p.SetSymbols(defaultRestSymbols)
		}
		return nil
	}

	p.SetSymbols(symbols)
	return nil
}

// updateButtonStates updates button appearance based on state
func (p *RestDataPanelV2) updateButtonStates() {
	switch p.state {
	case StateDisconnected:
		p.connectBtn.SetText("Connect")
		p.connectBtn.Importance = widget.WarningImportance
		p.startBtn.Disable()

	case StateConnected:
		p.connectBtn.SetText("Disconnect")
		p.connectBtn.Importance = widget.SuccessImportance
		p.startBtn.Enable()
		p.startBtn.SetText("Start")
		p.startBtn.Importance = widget.WarningImportance

	case StateActive:
		p.connectBtn.SetText("Disconnect")
		p.connectBtn.Importance = widget.SuccessImportance
		p.startBtn.Enable()
		p.startBtn.SetText("Stop")
		p.startBtn.Importance = widget.SuccessImportance
	}

	p.connectBtn.Refresh()
	p.startBtn.Refresh()
}

// showLogWindow opens the activity log popup window
func (p *RestDataPanelV2) showLogWindow() {
	if p.logWindow != nil && p.logWindow.Canvas() != nil {
		p.logWindow.Show()
		return
	}

	app := fyne.CurrentApp()
	p.logWindow = app.NewWindow("Activity Log")
	p.logWindow.Resize(fyne.NewSize(600, 400))

	// Clear button
	clearBtn := widget.NewButton("Clear Log", func() {
		p.logText.SetText("")
	})

	// Close button
	closeBtn := widget.NewButton("Close", func() {
		p.logWindow.Hide()
	})

	buttons := container.NewHBox(clearBtn, closeBtn)

	// Scrollable log text
	logScroll := container.NewVScroll(p.logText)

	content := container.NewBorder(
		nil,
		buttons,
		nil, nil,
		logScroll,
	)

	p.logWindow.SetContent(content)
	p.logWindow.Show()
}

func (p *RestDataPanelV2) resolveWindow() fyne.Window {
	if p.parentWindow != nil {
		return p.parentWindow
	}

	app := fyne.CurrentApp()
	if app == nil {
		return nil
	}
	drv := app.Driver()
	if drv == nil {
		return nil
	}

	for _, win := range drv.AllWindows() {
		if win != nil {
			return win
		}
	}

	return nil
}

func (p *RestDataPanelV2) showError(err error) {
	if err == nil {
		return
	}
	win := p.resolveWindow()
	if win == nil {
		p.logMessage(fmt.Sprintf("Error: %v", err))
		return
	}
	dialog.ShowError(err, win)
}

// openDirectory opens a popup allowing users to inspect and change the data directory
func (p *RestDataPanelV2) openDirectory() {
	win := p.resolveWindow()
	if win == nil {
		p.logMessage("Unable to open directory dialog: window not available")
		return
	}

	currentDir := p.dataDir
	if currentDir == "" {
		currentDir = filepath.Join("data", "bitfinex", "restapi", "data")
	}

	dirEntry := widget.NewEntry()
	dirEntry.SetText(currentDir)
	dirEntry.Disable()

	chooseBtn := widget.NewButton("Change...", func() {
		folderDialog := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
			if err != nil {
				dialog.ShowError(err, win)
				return
			}
			if uri == nil {
				return
			}
			path := uri.Path()
			if path == "" {
				return
			}
			p.SetDataDirectory(path)
			dirEntry.SetText(p.dataDir)
			p.logMessage("Data directory updated: " + p.dataDir)
		}, win)

		if base := dirEntry.Text; base != "" {
			if listable, err := storage.ListerForURI(storage.NewFileURI(base)); err == nil {
				folderDialog.SetLocation(listable)
			}
		}

		folderDialog.Show()
	})

	content := container.NewVBox(
		widget.NewLabel("Current data directory"),
		dirEntry,
		chooseBtn,
	)

	dialog.NewCustom("Data Directory", "Close", content, win).Show()
}

// logMessage adds a message to the activity log
func (p *RestDataPanelV2) logMessage(msg string) {
	if p.logText == nil {
		return // Log text not initialized yet
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("[%s] %s\n", timestamp, msg)

	fyne.Do(func() {
		p.logText.SetText(p.logText.Text + logLine)
		if p.logWindow != nil && p.logWindow.Canvas() != nil {
			p.logText.CursorRow = len(p.logText.Text)
		}
	})
}

// GetState returns the current connection state
func (p *RestDataPanelV2) GetState() ConnectionState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// SetSymbols updates the symbols list
func (p *RestDataPanelV2) SetSymbols(symbols []string) {
	normalized := normalizeSymbolList(symbols)
	if len(normalized) == 0 {
		normalized = normalizeSymbolList(defaultRestSymbols)
	}

	var (
		candlesPrev []string
		tradesPrev  []string
		tickersPrev []string
	)

	if p.candlesPanel != nil {
		candlesPrev = p.candlesPanel.GetSelectedSymbols()
	}
	if p.tradesPanel != nil {
		tradesPrev = p.tradesPanel.GetSelectedSymbols()
	}
	if p.tickersPanel != nil {
		tickersPrev = p.tickersPanel.GetSelectedSymbols()
	}

	p.symbols = normalized

	if p.candlesPanel != nil {
		p.candlesPanel.UpdateSymbols(normalized)
		if len(candlesPrev) > 0 {
			p.candlesPanel.SetSelectedSymbols(retainSelection(candlesPrev, normalized))
		}
	}
	if p.tradesPanel != nil {
		p.tradesPanel.UpdateSymbols(normalized)
		if len(tradesPrev) > 0 {
			p.tradesPanel.SetSelectedSymbols(retainSelection(tradesPrev, normalized))
		}
	}
	if p.tickersPanel != nil {
		p.tickersPanel.UpdateSymbols(normalized)
		if len(tickersPrev) > 0 {
			p.tickersPanel.SetSelectedSymbols(retainSelection(tickersPrev, normalized))
		}
	}
}

func normalizeSymbolList(symbols []string) []string {
	if len(symbols) == 0 {
		return []string{}
	}

	unique := make(map[string]struct{}, len(symbols))
	result := make([]string, 0, len(symbols))
	for _, raw := range symbols {
		if sym, ok := normalizeSymbol(raw); ok {
			if _, exists := unique[sym]; exists {
				continue
			}
			unique[sym] = struct{}{}
			result = append(result, sym)
		}
	}

	sort.Strings(result)
	return result
}

func normalizeSymbol(raw string) (string, bool) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", false
	}

	upper := strings.ToUpper(strings.ReplaceAll(trimmed, " ", ""))
	if strings.HasPrefix(upper, "T") || strings.HasPrefix(upper, "F") {
		if len(upper) == 1 {
			return "", false
		}
		return strings.ToLower(upper[:1]) + upper[1:], true
	}

	return "t" + upper, true
}

func retainSelection(selected, available []string) []string {
	if len(selected) == 0 || len(available) == 0 {
		return []string{}
	}

	availableSet := make(map[string]struct{}, len(available))
	for _, sym := range available {
		availableSet[sym] = struct{}{}
	}

	retained := make([]string, 0, len(selected))
	for _, raw := range selected {
		if sym, ok := normalizeSymbol(raw); ok {
			if _, exists := availableSet[sym]; exists {
				retained = append(retained, sym)
			}
		}
	}

	return retained
}

func (p *RestDataPanelV2) waitForRateLimiter(ctx context.Context, endpoint restapi.EndpointType) error {
	if p.rateLimiter == nil {
		return nil
	}
	return p.rateLimiter.Wait(ctx, endpoint)
}

func normaliseSort(sort int) int {
	if sort == -1 {
		return -1
	}
	return 1
}

func sanitizeSymbolForFile(symbol string) string {
	trimmed := strings.TrimSpace(symbol)
	if trimmed == "" {
		return "symbol"
	}
	trimmed = strings.TrimPrefix(trimmed, "t")
	replacer := strings.NewReplacer(":", "_", "/", "_", "-", "_", " ", "")
	clean := replacer.Replace(trimmed)
	if clean == "" {
		return "symbol"
	}
	return clean
}

func timeframeDuration(tf string) time.Duration {
	switch tf {
	case "1m":
		return time.Minute
	case "3m":
		return 3 * time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "1h":
		return time.Hour
	case "3h":
		return 3 * time.Hour
	case "6h":
		return 6 * time.Hour
	case "12h":
		return 12 * time.Hour
	case "1D":
		return 24 * time.Hour
	case "7D":
		return 7 * 24 * time.Hour
	case "14D":
		return 14 * 24 * time.Hour
	case "1W":
		return 7 * 24 * time.Hour
	case "1M":
		return 30 * 24 * time.Hour
	default:
		return 0
	}
}

func (p *RestDataPanelV2) fetchCandlesToCSV(ctx context.Context, symbol, timeframe string, limit, sortOrder int, start, end time.Time, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	header := []string{"mts", "open", "close", "high", "low", "volume", "symbol", "timeframe"}
	if err := writer.Write(header); err != nil {
		return err
	}

	startMs := start.UTC().UnixMilli()
	endMs := end.UTC().UnixMilli()
	current := startMs
	lastTimestamp := int64(-1)
	dur := timeframeDuration(timeframe)

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		if err := p.waitForRateLimiter(ctx, restapi.EndpointCandles); err != nil {
			return err
		}

		batch, err := p.client.FetchCandles(ctx, restapi.CandlesRequest{
			Symbol:    symbol,
			Timeframe: timeframe,
			Section:   "hist",
			Start:     current,
			End:       endMs,
			Limit:     limit,
			Sort:      sortOrder,
		})
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}

		for _, entry := range batch {
			mts := int64(entry[0])
			if mts < startMs {
				continue
			}
			if mts > endMs {
				return nil
			}
			if mts == lastTimestamp {
				continue
			}

			record := []string{
				fmt.Sprintf("%d", mts),
				formatFloat(entry[1]),
				formatFloat(entry[2]),
				formatFloat(entry[3]),
				formatFloat(entry[4]),
				formatFloat(entry[5]),
				symbol,
				timeframe,
			}
			if err := writer.Write(record); err != nil {
				return err
			}
			lastTimestamp = mts

			if dur > 0 && sortOrder == 1 && lastTimestamp-startMs > 0 {
				expected := lastTimestamp - int64(dur/time.Millisecond)
				_ = expected // reserved for future gap logging
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}

		if sortOrder == 1 {
			current = lastTimestamp + 1
			if current >= endMs {
				break
			}
		} else {
			current = lastTimestamp - 1
			if current <= startMs {
				break
			}
		}
	}

	return writer.Error()
}

func (p *RestDataPanelV2) fetchTradesToCSV(ctx context.Context, symbol string, limit, sortOrder int, start, end time.Time, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	header := []string{"id", "mts", "amount", "price", "symbol"}
	if err := writer.Write(header); err != nil {
		return err
	}

	startMs := start.UTC().UnixMilli()
	endMs := end.UTC().UnixMilli()
	current := startMs
	lastID := float64(0)

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		if err := p.waitForRateLimiter(ctx, restapi.EndpointTrades); err != nil {
			return err
		}

		batch, err := p.client.FetchTrades(ctx, restapi.TradesRequest{
			Symbol: symbol,
			Start:  current,
			End:    endMs,
			Limit:  limit,
			Sort:   sortOrder,
		})
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}

		for _, row := range batch {
			if len(row) < 4 {
				continue
			}
			mts := int64(row[1])
			if mts < startMs {
				continue
			}
			if mts > endMs {
				return nil
			}
			if row[0] == lastID {
				continue
			}

			record := []string{
				formatFloat(row[0]),
				formatFloat(row[1]),
				formatFloat(row[2]),
				formatFloat(row[3]),
				symbol,
			}
			if err := writer.Write(record); err != nil {
				return err
			}
			lastID = row[0]
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}

		if sortOrder == 1 {
			current = int64(batch[len(batch)-1][1]) + 1
			if current >= endMs {
				break
			}
		} else {
			current = int64(batch[len(batch)-1][1]) - 1
			if current <= startMs {
				break
			}
		}
	}

	return writer.Error()
}

func (p *RestDataPanelV2) fetchTickersToCSV(ctx context.Context, symbols []string, limit, sortOrder int, start, end time.Time, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	header := []string{"symbol", "bid", "ask", "mts"}
	if err := writer.Write(header); err != nil {
		return err
	}

	startMs := start.UTC().UnixMilli()
	endMs := end.UTC().UnixMilli()
	current := startMs

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		if err := p.waitForRateLimiter(ctx, restapi.EndpointTickers); err != nil {
			return err
		}

		batch, err := p.client.FetchTickersHistory(ctx, restapi.TickersHistoryRequest{
			Symbols: symbols,
			Start:   current,
			End:     endMs,
			Limit:   limit,
			Sort:    sortOrder,
		})
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}

		for _, row := range batch {
			if len(row) < 4 {
				continue
			}
			symbolVal := fmt.Sprintf("%v", row[0])
			bid := formatFloat(parseFloat(row[1]))
			ask := formatFloat(parseFloat(row[3]))
			mts := int64(parseFloat(row[len(row)-1]))
			if mts <= 0 {
				continue
			}
			if mts < startMs {
				continue
			}
			if mts > endMs {
				return nil
			}

			record := []string{symbolVal, bid, ask, formatFloat(float64(mts))}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return err
		}

		if sortOrder == 1 {
			last := batch[len(batch)-1]
			current = int64(parseFloat(last[len(last)-1])) + 1
			if current >= endMs {
				break
			}
		} else {
			last := batch[len(batch)-1]
			current = int64(parseFloat(last[len(last)-1])) - 1
			if current <= startMs {
				break
			}
		}
	}

	return writer.Error()
}

// formatFloat formats a float64 value for CSV output
func formatFloat(val float64) string {
	return strconv.FormatFloat(val, 'f', -1, 64)
}

// parseFloat converts various numeric types to float64
func parseFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f
		}
	}
	return 0
}
