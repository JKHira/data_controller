package app

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"
	"image/color"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/gui"
	"github.com/trade-engine/data-controller/internal/gui/controllers"
	"github.com/trade-engine/data-controller/internal/gui/panels"
	"github.com/trade-engine/data-controller/internal/gui/state"
	"github.com/trade-engine/data-controller/internal/restapi"
	"github.com/trade-engine/data-controller/internal/services"
	arrowsink "github.com/trade-engine/data-controller/internal/sink/arrow"
	"github.com/trade-engine/data-controller/internal/ws"
)

// Application represents the main GUI application
type Application struct {
	logger *zap.Logger
	cfg    *config.Config

	// Fyne app and window
	fyneApp fyne.App
	window  fyne.Window

	// Application state
	state *state.AppState

	// Configuration management
	configManager *config.ConfigManager

	// Controllers
	fileController *controllers.FileController

	// Panels
	filesPanel  *panels.FilesPanel
	viewerPanel *panels.ViewerPanel

	// Services
	arrowReader          *arrowsink.FileReader
	arrowHandler         *arrowsink.Handler
	connectionManager    *ws.ConnectionManager
	liveStreamData       *gui.LiveStreamData
	isRunning            bool
	activeExchange       string
	customSubscriptions  []gui.ChannelSubscription
	configRefreshManager *services.ConfigRefreshManager
	configRefreshCancel  context.CancelFunc
	configStatusTimer    *time.Timer

	// Context and lifecycle
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// NewApplication creates a new GUI application
func NewApplication(logger *zap.Logger, cfg *config.Config) *Application {
	ctx, cancel := context.WithCancel(context.Background())

	fyneApp := app.New()

	// Apply dark theme if configured
	if cfg.GUI.Theme == "dark" {
		fyneApp.Settings().SetTheme(theme.DarkTheme())
	}

	window := fyneApp.NewWindow(cfg.GUI.Title)
	window.Resize(fyne.NewSize(float32(fixedWindowWidth), float32(fixedWindowHeight)))
	window.SetFixedSize(true)
	cfg.GUI.Width = fixedWindowWidth
	cfg.GUI.Height = fixedWindowHeight

	// Initialize services
	arrowReader := arrowsink.NewFileReader(logger)
	arrowHandler := arrowsink.NewHandler(cfg, logger)

	// Initialize router and connection manager
	router := ws.NewRouter(logger)
	router.SetHandler(arrowHandler)
	connectionManager := ws.NewConnectionManager(cfg, logger, router)

	// Initialize application state
	appState := state.NewAppState()

	// Initialize controllers
	fileController := controllers.NewFileController(logger, cfg, appState, arrowReader)

	// Initialize panels
	filesPanel := panels.NewFilesPanel(logger, cfg, appState, fileController, window)
	viewerPanel := panels.NewViewerPanel(appState, fileController)

	// Initialize live stream data
	liveStreamData := gui.NewLiveStreamData(20)

	var refreshManager *services.ConfigRefreshManager
	if mgr, err := services.NewConfigRefreshManager(cfg, logger); err != nil {
		logger.Warn("Failed to initialise config refresh manager", zap.Error(err))
	} else {
		refreshManager = mgr
	}

	configManager, cmErr := initialiseConfigManager(logger, cfg)
	if cmErr != nil {
		logger.Error("Failed to initialise config manager", zap.Error(cmErr))
	}

	return &Application{
		logger:               logger,
		cfg:                  cfg,
		fyneApp:              fyneApp,
		window:               window,
		ctx:                  ctx,
		cancel:               cancel,
		state:                appState,
		fileController:       fileController,
		filesPanel:           filesPanel,
		viewerPanel:          viewerPanel,
		arrowReader:          arrowReader,
		arrowHandler:         arrowHandler,
		connectionManager:    connectionManager,
		liveStreamData:       liveStreamData,
		isRunning:            false,
		configRefreshManager: refreshManager,
		configManager:        configManager,
	}
}

const (
	defaultBitfinexRestBase = "https://api-pub.bitfinex.com/v2"
	fixedWindowWidth        = 2400
	fixedWindowHeight       = 1300
)

func initialiseConfigManager(logger *zap.Logger, cfg *config.Config) (*config.ConfigManager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}

	configDir := filepath.Dir(cfg.GlobalConfigPath)
	if configDir == "" {
		configDir = "."
	}
	if !filepath.IsAbs(configDir) {
		if abs, err := filepath.Abs(configDir); err == nil {
			configDir = abs
		}
	}

	basePath := filepath.Dir(configDir)
	if basePath == "" {
		basePath = "."
	}
	if !filepath.IsAbs(basePath) {
		if abs, err := filepath.Abs(basePath); err == nil {
			basePath = abs
		}
	}

	restFetcher := config.NewBitfinexRESTFetcher(defaultBitfinexRestBase)
	manager := config.NewConfigManager(logger, basePath, restFetcher)
	if err := manager.Initialize(cfg.ActiveExchange); err != nil {
		return nil, err
	}

	return manager, nil
}

// Initialize sets up the application UI and starts background services
func (a *Application) Initialize() error {
	// Initialize status bindings
	a.state.StatusBinding.Set("💤 Disconnected")
	a.state.StatsBinding.Set("Statistics:\nTickers: 0\nTrades: 0\nBook Levels: 0\nErrors: 0")
	a.state.ConfigStatusBinding.Set("Config: Ready")

	// Create main layout
	a.createLayout()

	// Setup window close handler
	a.window.SetCloseIntercept(a.handleWindowClose)

	// Start background services
	go a.statusUpdater()
	// 自動スキャン廃止: Scanボタン押下時のみ実行

	// 初期ファイルリスト更新も廃止

	return nil
}

// createLayout creates the main application layout
func (a *Application) createLayout() {
	// Top bar - status only (using modular component)
	topBar := gui.CreateTopBar(a.state.StatusBinding)

	var wsPane, restPane fyne.CanvasObject
	if a.configManager == nil {
		wsPane = widget.NewCard(
			"WebSocket",
			"Config manager initialisation failed",
			widget.NewLabel("WebSocket control is unavailable. Check configuration logs."),
		)
		restPane = widget.NewCard(
			"REST API",
			"Unavailable",
			widget.NewLabel("REST config controls are disabled."),
		)
	} else {
		wsPane, restPane = gui.BuildExchangePanesV2(
			a.cfg,
			a.configManager,
			a.handleWsConnectConfig,
			a.handleWsDisconnectConfig,
			a.configRefreshManager,
			a.publishConfigStatus,
			a.logger,
		)
	}

	filesCard := a.filesPanel.GetContent()
	fileViewerCard := a.viewerPanel.GetContent()
	controlPanel := widget.NewCard("Controls", "", container.NewVBox())

	wrapColumn := func(obj fyne.CanvasObject, width float32) fyne.CanvasObject {
		background := canvas.NewRectangle(color.Transparent)
		background.SetMinSize(fyne.NewSize(width, obj.MinSize().Height))
		return container.NewMax(background, obj)
	}

	columns := container.New(layout.NewHBoxLayout(),
		wrapColumn(wsPane, 380),
		wrapColumn(restPane, 380),
		wrapColumn(filesCard, 380),
		wrapColumn(fileViewerCard, 380),
		wrapColumn(controlPanel, 780),
	)

	mainContent := container.NewHScroll(columns)

	// Bottom bar - configuration activity
	bottomBar := gui.CreateBottomBar(a.state.ConfigStatusBinding)

	// Final layout
	content := container.NewBorder(
		topBar,    // top
		bottomBar, // bottom
		nil, nil,  // left, right
		mainContent, // center
	)

	a.window.SetContent(content)
}

// Run starts the application
func (a *Application) Run() {
	a.window.ShowAndRun()
}

// handleWsConnect handles WebSocket connection requests for a specific exchange
func (a *Application) handleWsConnect(exchange string, symbols []string) error {
	a.logger.Info("GUI: WebSocket connect requested",
		zap.String("exchange", exchange),
		zap.Int("symbol_count", len(symbols)))

	if a.isRunning {
		err := fmt.Errorf("websocket already connected")
		a.logger.Warn("Connect request ignored: already connected",
			zap.String("active_exchange", a.activeExchange))
		return err
	}

	if len(symbols) == 0 {
		err := fmt.Errorf("no symbols selected")
		a.logger.Warn("Connect request ignored: no symbols selected")
		return err
	}

	// Start Arrow handler before connection
	if err := a.arrowHandler.Start(); err != nil {
		a.logger.Error("Failed to start Arrow handler", zap.Error(err))
		a.state.StatusBinding.Set("❌ Arrow handler failed")
		return err
	}

	// Convert all channel subscriptions to SubscribeRequests
	// Each channel panel (Ticker, Trades, Books, RawBooks, Candles) provides
	// its own symbol-specific subscriptions via GetSubscriptions()
	var customSubs []ws.SubscribeRequest
	for _, sub := range a.customSubscriptions {
		req := ws.SubscribeRequest{
			Event:   "subscribe",
			Channel: sub.Channel,
			Symbol:  sub.Symbol,
		}

		// Handle channel-specific parameters
		if sub.Channel == "candles" && sub.Key != "" {
			req.Key = sub.Key
		}
		if sub.Channel == "book" {
			// Books channel parameters
			if sub.Prec != "" {
				req.Prec = &sub.Prec
			}
			if sub.Freq != "" {
				req.Freq = &sub.Freq
			}
			if sub.Len != "" {
				req.Len = &sub.Len
			}
			// Generate unique SubID for book subscriptions
			subID := int64(time.Now().UnixNano())
			req.SubID = &subID
		}

		customSubs = append(customSubs, req)
	}
	a.connectionManager.SetCustomSubscriptions(customSubs)

	if err := a.connectionManager.StartWithSymbols(symbols); err != nil {
		a.logger.Error("Failed to establish WebSocket connection", zap.Error(err))
		a.state.StatusBinding.Set("❌ Connection failed")
		// Stop Arrow handler if connection fails
		a.arrowHandler.Stop()
		return err
	}

	a.isRunning = true
	a.activeExchange = exchange
	a.state.SetConnected(true)
	a.state.StatusBinding.Set(fmt.Sprintf("🟢 %s Connected", exchange))

	if a.configRefreshCancel != nil {
		a.configRefreshCancel()
	}

	a.ensureConfigFreshness(exchange, true)

	if a.configRefreshManager != nil {
		ctx, cancel := context.WithCancel(a.ctx)
		a.configRefreshCancel = cancel
		go a.configRefreshLoop(ctx, exchange)
	}

	return nil
}

func (a *Application) handleWsConnectConfig(wsConfig *gui.WSConnectionConfig) error {
	if wsConfig == nil {
		return fmt.Errorf("websocket configuration is nil")
	}

	exchange := wsConfig.Exchange
	if exchange == "" {
		exchange = a.cfg.ActiveExchange
	}
	if exchange == "" {
		exchange = "bitfinex"
	}

	// Validate that we have channel subscriptions
	if len(wsConfig.Channels) == 0 {
		return fmt.Errorf("no channels selected for connection")
	}

	// Extract symbols for logging/display purposes only
	// The actual subscriptions are handled via wsConfig.Channels (customSubscriptions)
	symbols := make([]string, 0)
	for _, sub := range wsConfig.Channels {
		if sub.Symbol != "" {
			symbols = append(symbols, sub.Symbol)
		}
	}
	symbols = uniqueStrings(symbols)
	a.cfg.Symbols = symbols
	a.cfg.WebSocket.ConfFlags = wsConfig.ConfFlags
	if a.arrowHandler != nil {
		a.arrowHandler.UpdateConfFlags(wsConfig.ConfFlags)
	}

	var (
		tickerEnabled bool
		tradesEnabled bool
		booksEnabled  bool
		bookPrec      string
		bookFreq      string
		bookLen       string
	)

	for _, sub := range wsConfig.Channels {
		switch sub.Channel {
		case "ticker":
			tickerEnabled = true
		case "trades":
			tradesEnabled = true
		case "book":
			booksEnabled = true
			if bookPrec == "" && sub.Prec != "" {
				bookPrec = sub.Prec
			}
			if bookFreq == "" && sub.Freq != "" {
				bookFreq = sub.Freq
			}
			if bookLen == "" && sub.Len != "" {
				bookLen = sub.Len
			}
		case "candles":
			// Candles channel - subscription will be handled via SubscribeRequests
			a.logger.Info("Candles channel enabled via GUI",
				zap.String("symbol", sub.Symbol),
				zap.String("key", sub.Key))
		}
	}

	if !tickerEnabled && !tradesEnabled && !booksEnabled {
		// Don't enable ticker by default if any channel is selected
		hasAnyChannel := false
		for _, sub := range wsConfig.Channels {
			if sub.Channel != "" {
				hasAnyChannel = true
				break
			}
		}
		if !hasAnyChannel {
			tickerEnabled = true
		}
	}

	a.cfg.Channels.Ticker.Enabled = tickerEnabled
	a.cfg.Channels.Trades.Enabled = tradesEnabled
	a.cfg.Channels.Books.Enabled = booksEnabled

	if bookPrec != "" {
		a.cfg.Channels.Books.Precision = bookPrec
	}
	if bookFreq != "" {
		a.cfg.Channels.Books.Frequency = bookFreq
	}
	if bookLen != "" {
		if length, err := strconv.Atoi(bookLen); err == nil {
			a.cfg.Channels.Books.Length = length
		}
	}

	// Store custom subscriptions (like candles) for connection manager
	a.customSubscriptions = wsConfig.Channels

	return a.handleWsConnect(exchange, symbols)
}

// handleWsDisconnect handles WebSocket disconnection requests for a specific exchange
func (a *Application) handleWsDisconnect(exchange string) error {
	a.logger.Info("GUI: WebSocket disconnect requested",
		zap.String("exchange", exchange))

	if !a.isRunning {
		err := fmt.Errorf("websocket not connected")
		a.logger.Warn("Disconnect request ignored: no active connection")
		return err
	}

	if a.activeExchange != "" && a.activeExchange != exchange {
		a.logger.Warn("Disconnect request for non-active exchange",
			zap.String("active_exchange", a.activeExchange),
			zap.String("requested_exchange", exchange))
	}

	a.connectionManager.Stop()

	// Stop Arrow handler to close all files properly
	if err := a.arrowHandler.Stop(); err != nil {
		a.logger.Error("Failed to stop Arrow handler", zap.Error(err))
	}

	a.isRunning = false
	a.activeExchange = ""
	a.state.SetConnected(false)
	a.state.StatusBinding.Set("💤 Disconnected")

	if a.configRefreshCancel != nil {
		a.configRefreshCancel()
		a.configRefreshCancel = nil
	}

	return nil
}

func (a *Application) handleWsDisconnectConfig() error {
	exchange := a.activeExchange
	if exchange == "" {
		exchange = a.cfg.ActiveExchange
	}
	if exchange == "" {
		exchange = "bitfinex"
	}

	return a.handleWsDisconnect(exchange)
}

// handleFilterFiles handles file filtering
func (a *Application) handleFilterFiles() {
	// This is a placeholder - implement actual filtering logic
	a.logger.Info("Filter files functionality not yet implemented")
}

// handleWindowClose handles application shutdown
func (a *Application) handleWindowClose() {
	a.logger.Info("GUI: Window close requested")

	// Save current application state before closing
	if a.configManager != nil {
		if err := a.configManager.SaveState(); err != nil {
			a.logger.Warn("Failed to save state on window close", zap.Error(err))
		} else {
			a.logger.Info("Application state saved successfully on close")
		}
	}

	// Stop connection if active
	if a.isRunning {
		a.connectionManager.Stop()

		// Stop Arrow handler to close all files properly
		if err := a.arrowHandler.Stop(); err != nil {
			a.logger.Error("Failed to stop Arrow handler on window close", zap.Error(err))
		}

		a.isRunning = false
	}

	// Stop config status timer
	if a.configStatusTimer != nil {
		a.configStatusTimer.Stop()
		a.configStatusTimer = nil
	}

	// Cancel context and wait for goroutines
	a.cancel()
	a.wg.Wait()

	// Quit the application
	a.fyneApp.Quit()

	if a.configManager != nil {
		if err := a.configManager.Shutdown(); err != nil {
			a.logger.Warn("Failed to shut down config manager", zap.Error(err))
		}
	}
}

// statusUpdater updates the status display periodically
func (a *Application) statusUpdater() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.updateStatus()
		}
	}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		result = append(result, v)
	}
	return result
}

// fileListUpdater: 自動スキャン廃止（Scanボタンのみで実行）
// func (a *Application) fileListUpdater() {
// 	廃止: 5秒ごとのスキャンは無駄なリソース消費
// }

// updateStatus updates the application status
func (a *Application) updateStatus() {
	if a.isRunning {
		status := "🟢 Connected"
		if a.activeExchange != "" {
			status = fmt.Sprintf("🟢 %s Connected", a.activeExchange)
		}
		a.state.StatusBinding.Set(status)
	} else {
		a.state.StatusBinding.Set("💤 Disconnected")
	}

	// Update statistics if available
	if a.arrowHandler != nil {
		stats := a.arrowHandler.GetStatistics()
		if stats != nil {
			statsText := "Statistics:\n"
			statsText += fmt.Sprintf("Tickers: %d\n", stats.TickersReceived)
			statsText += fmt.Sprintf("Trades: %d\n", stats.TradesReceived)
			statsText += fmt.Sprintf("Book Levels: %d\n", stats.BookLevelsReceived)
			statsText += fmt.Sprintf("Errors: %d", stats.Errors)
			a.state.StatsBinding.Set(statsText)
		}
	}
}

func (a *Application) ensureConfigFreshness(exchange string, includeOptional bool) {
	if a.configRefreshManager == nil {
		return
	}

	ctx, cancel := context.WithTimeout(a.ctx, 2*time.Minute)
	defer cancel()

	results, err := a.configRefreshManager.EnsureFreshness(ctx, exchange, includeOptional)
	if err != nil {
		a.logger.Warn("Config refresh check failed", zap.Error(err))
	}

	a.handleConfigResults(exchange, results)
}

func (a *Application) configRefreshLoop(ctx context.Context, exchange string) {
	ticker := time.NewTicker(45 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.ensureConfigFreshness(exchange, false)
		}
	}
}

func (a *Application) handleConfigResults(exchange string, results []restapi.FetchResult) {
	if len(results) == 0 {
		return
	}

	message := services.SummarizeResults(exchange, results)
	if message != "" {
		a.publishConfigStatus(message)
	}
}

func (a *Application) publishConfigStatus(message string) {
	if message == "" {
		if a.configStatusTimer != nil {
			a.configStatusTimer.Stop()
			a.configStatusTimer = nil
		}
		fyne.Do(func() {
			a.state.ConfigStatusBinding.Set("Config: Ready")
		})
		return
	}

	if a.configStatusTimer != nil {
		a.configStatusTimer.Stop()
	}

	fyne.Do(func() {
		a.state.ConfigStatusBinding.Set("Config: " + message)
	})

	a.configStatusTimer = time.AfterFunc(3*time.Minute, func() {
		fyne.Do(func() {
			a.state.ConfigStatusBinding.Set("Config: Ready")
		})
	})
}
