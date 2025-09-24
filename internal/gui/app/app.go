package app

import (
	"context"
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/gui"
	"github.com/trade-engine/data-controller/internal/gui/controllers"
	"github.com/trade-engine/data-controller/internal/gui/panels"
	"github.com/trade-engine/data-controller/internal/gui/state"
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

	// Controllers
	fileController *controllers.FileController

	// Panels
	filesPanel  *panels.FilesPanel
	viewerPanel *panels.ViewerPanel

	// Services
	arrowReader       *arrowsink.FileReader
	arrowHandler      *arrowsink.Handler
	connectionManager *ws.ConnectionManager
	liveStreamData    *gui.LiveStreamData
	isRunning         bool

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
	window.Resize(fyne.NewSize(float32(cfg.GUI.Width), float32(cfg.GUI.Height)))

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

	return &Application{
		logger:            logger,
		cfg:               cfg,
		fyneApp:           fyneApp,
		window:            window,
		ctx:               ctx,
		cancel:            cancel,
		state:             appState,
		fileController:    fileController,
		filesPanel:        filesPanel,
		viewerPanel:       viewerPanel,
		arrowReader:       arrowReader,
		arrowHandler:      arrowHandler,
		connectionManager: connectionManager,
		liveStreamData:    liveStreamData,
		isRunning:         false,
	}
}

// Initialize sets up the application UI and starts background services
func (a *Application) Initialize() error {
	// Initialize status bindings
	a.state.StatusBinding.Set("💤 Disconnected")
	a.state.StatsBinding.Set("Statistics:\nTickers: 0\nTrades: 0\nBook Levels: 0\nErrors: 0")

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

	// New Exchange Panes (WebSocket + REST API) - replaces statsCard
	exchangePanes := gui.BuildExchangePanesWithHandlers(
		func(connected bool) {
			if connected {
				a.handleConnect()
			} else {
				a.handleDisconnect()
			}
		},
		func(connected bool) {
			// REST API handler - TODO: implement REST API functionality
			if connected {
				a.logger.Info("REST API Connect requested")
			} else {
				a.logger.Info("REST API Disconnect requested")
			}
		},
		a.logger, // Pass logger for REST API functionality
	)

	// Right panel: File browser and viewer (using modular components)
	// Data Files panel
	filesCard := a.filesPanel.GetContent()

	// File viewer panel
	fileViewerCard := a.viewerPanel.GetContent()

	// Right side - file browser and viewer
	rightPanel := container.NewVSplit(
		filesCard,
		fileViewerCard,
	)
	rightPanel.SetOffset(0.5) // 50/50 split

	// Main content - left and right panels
	mainContent := container.NewHSplit(
		exchangePanes,
		rightPanel,
	)
	mainContent.SetOffset(0.6) // 60% left, 40% right

	// Bottom bar - live stream (using modular component)
	bottomBar := gui.CreateBottomBar(a.cfg.Symbols, a.cfg.Storage.BasePath)

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

// handleConnect handles WebSocket connection
func (a *Application) handleConnect() {
	a.logger.Info("GUI: Connect button clicked")

	if a.isRunning {
		a.logger.Warn("Already connected")
		return
	}

	go func() {
		if err := a.connectionManager.Start(); err != nil {
			a.logger.Error("Failed to connect", zap.Error(err))
			a.state.StatusBinding.Set("❌ Connection failed")
		} else {
			a.isRunning = true
			a.state.SetConnected(true)
			a.state.StatusBinding.Set("🟢 Connected")
		}
	}()
}

// handleDisconnect handles WebSocket disconnection
func (a *Application) handleDisconnect() {
	a.logger.Info("GUI: Disconnect button clicked")

	if !a.isRunning {
		a.logger.Warn("Not connected")
		return
	}

	a.connectionManager.Stop()
	a.isRunning = false
	a.state.SetConnected(false)
	a.state.StatusBinding.Set("💤 Disconnected")
}

// handleFilterFiles handles file filtering
func (a *Application) handleFilterFiles() {
	// This is a placeholder - implement actual filtering logic
	a.logger.Info("Filter files functionality not yet implemented")
}

// handleWindowClose handles application shutdown
func (a *Application) handleWindowClose() {
	a.logger.Info("GUI: Window close requested")

	// Stop connection if active
	if a.isRunning {
		a.connectionManager.Stop()
		a.isRunning = false
	}

	// Cancel context and wait for goroutines
	a.cancel()
	a.wg.Wait()

	// Quit the application
	a.fyneApp.Quit()
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

// fileListUpdater: 自動スキャン廃止（Scanボタンのみで実行）
// func (a *Application) fileListUpdater() {
// 	廃止: 5秒ごとのスキャンは無駄なリソース消費
// }

// updateStatus updates the application status
func (a *Application) updateStatus() {
	if a.isRunning {
		a.state.StatusBinding.Set("🟢 Connected")
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