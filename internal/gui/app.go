package gui

import (
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/sink/arrow"
)

type App struct {
	cfg             *config.Config
	logger          *zap.Logger
	fyneApp         fyne.App
	window          fyne.Window
	arrowHandler    *arrow.Handler

	// Control state
	isRunning       bool
	isRunningMutex  sync.RWMutex
	startCallback   func() error
	stopCallback    func() error

	// UI elements
	startButton     *widget.Button
	stopButton      *widget.Button
	statusLabel     *widget.Label
	statisticsCard  *widget.Card
	connectionsCard *widget.Card
	storageCard     *widget.Card

	// Statistics display
	tickersLabel       *widget.Label
	tradesLabel        *widget.Label
	bookLevelsLabel    *widget.Label
	rawBookEventsLabel *widget.Label
	errorsLabel        *widget.Label
	lastFlushLabel     *widget.Label

	// Storage display
	segmentsLabel      *widget.Label
	ingestIdLabel      *widget.Label

	// Update ticker
	updateTicker *time.Ticker
	stopCh       chan struct{}
}

func NewApp(cfg *config.Config, logger *zap.Logger) *App {
	fyneApp := app.New()
	fyneApp.SetIcon(theme.DocumentIcon())

	if cfg.GUI.Theme == "dark" {
		fyneApp.Settings().SetTheme(theme.DarkTheme())
	} else {
		fyneApp.Settings().SetTheme(theme.LightTheme())
	}

	window := fyneApp.NewWindow(cfg.GUI.Title)
	window.Resize(fyne.NewSize(float32(cfg.GUI.Width), float32(cfg.GUI.Height)))
	window.CenterOnScreen()

	guiApp := &App{
		cfg:       cfg,
		logger:    logger,
		fyneApp:   fyneApp,
		window:    window,
		isRunning: false,
		stopCh:    make(chan struct{}),
	}

	guiApp.setupUI()
	guiApp.startUpdateRoutine()

	return guiApp
}

func (a *App) SetCallbacks(startCallback, stopCallback func() error) {
	a.startCallback = startCallback
	a.stopCallback = stopCallback
}

func (a *App) SetArrowHandler(handler *arrow.Handler) {
	a.arrowHandler = handler
}

func (a *App) setupUI() {
	a.createControlButtons()
	a.createStatusDisplay()
	a.createStatisticsCard()
	a.createConnectionsCard()
	a.createStorageCard()

	// Main layout
	controlContainer := container.NewHBox(
		a.startButton,
		a.stopButton,
		layout.NewSpacer(),
		a.statusLabel,
	)

	statisticsContainer := container.NewGridWithRows(2,
		container.NewGridWithColumns(2, a.statisticsCard, a.connectionsCard),
		a.storageCard,
	)

	content := container.NewVBox(
		controlContainer,
		widget.NewSeparator(),
		statisticsContainer,
	)

	a.window.SetContent(content)
}

func (a *App) createControlButtons() {
	a.startButton = widget.NewButton("Start Data Collection", func() {
		a.handleStart()
	})
	a.startButton.Importance = widget.HighImportance

	a.stopButton = widget.NewButton("Stop Data Collection", func() {
		a.handleStop()
	})
	a.stopButton.Importance = widget.DangerImportance
	a.stopButton.Disable()
}

func (a *App) createStatusDisplay() {
	a.statusLabel = widget.NewLabel("Status: Stopped")
	a.statusLabel.TextStyle = fyne.TextStyle{Bold: true}
}

func (a *App) createStatisticsCard() {
	a.tickersLabel = widget.NewLabel("Tickers: 0")
	a.tradesLabel = widget.NewLabel("Trades: 0")
	a.bookLevelsLabel = widget.NewLabel("Book Levels: 0")
	a.rawBookEventsLabel = widget.NewLabel("Raw Book Events: 0")
	a.errorsLabel = widget.NewLabel("Errors: 0")
	a.lastFlushLabel = widget.NewLabel("Last Flush: Never")

	statsContent := container.NewVBox(
		a.tickersLabel,
		a.tradesLabel,
		a.bookLevelsLabel,
		a.rawBookEventsLabel,
		widget.NewSeparator(),
		a.errorsLabel,
		a.lastFlushLabel,
	)

	a.statisticsCard = widget.NewCard("Statistics", "", statsContent)
}

func (a *App) createConnectionsCard() {
	symbolsLabel := widget.NewLabel(fmt.Sprintf("Symbols: %v", a.cfg.Symbols))
	channelsLabel := widget.NewLabel("Channels:")

	var enabledChannels []string
	if a.cfg.Channels.Ticker.Enabled {
		enabledChannels = append(enabledChannels, "Ticker")
	}
	if a.cfg.Channels.Trades.Enabled {
		enabledChannels = append(enabledChannels, "Trades")
	}
	if a.cfg.Channels.Books.Enabled {
		enabledChannels = append(enabledChannels, "Books")
	}
	if a.cfg.Channels.RawBooks.Enabled {
		enabledChannels = append(enabledChannels, "Raw Books")
	}

	channelsListLabel := widget.NewLabel(fmt.Sprintf("  %v", enabledChannels))

	wsUrlLabel := widget.NewLabel(fmt.Sprintf("WebSocket: %s", a.cfg.WebSocket.URL))

	connectionsContent := container.NewVBox(
		symbolsLabel,
		widget.NewSeparator(),
		channelsLabel,
		channelsListLabel,
		widget.NewSeparator(),
		wsUrlLabel,
	)

	a.connectionsCard = widget.NewCard("Configuration", "", connectionsContent)
}

func (a *App) createStorageCard() {
	a.segmentsLabel = widget.NewLabel("Active Segments: 0")
	a.ingestIdLabel = widget.NewLabel("Ingest ID: N/A")

	basePathLabel := widget.NewLabel(fmt.Sprintf("Base Path: %s", a.cfg.Storage.BasePath))
	segmentSizeLabel := widget.NewLabel(fmt.Sprintf("Segment Size: %d MB", a.cfg.Storage.SegmentSizeMB))
	compressionLabel := widget.NewLabel(fmt.Sprintf("Compression: %s (Level %d)",
		a.cfg.Storage.Compression, a.cfg.Storage.CompressionLevel))

	flushButton := widget.NewButton("Force Flush", func() {
		a.handleForceFlush()
	})

	storageContent := container.NewVBox(
		a.segmentsLabel,
		a.ingestIdLabel,
		widget.NewSeparator(),
		basePathLabel,
		segmentSizeLabel,
		compressionLabel,
		widget.NewSeparator(),
		flushButton,
	)

	a.storageCard = widget.NewCard("Storage", "", storageContent)
}

func (a *App) handleStart() {
	if a.startCallback == nil {
		a.logger.Error("Start callback not set")
		return
	}

	a.logger.Info("Starting data collection from GUI")

	go func() {
		if err := a.startCallback(); err != nil {
			a.logger.Error("Failed to start data collection", zap.Error(err))
			return
		}

		a.isRunningMutex.Lock()
		a.isRunning = true
		a.isRunningMutex.Unlock()

		a.startButton.Disable()
		a.stopButton.Enable()
		a.statusLabel.SetText("Status: Running")
	}()
}

func (a *App) handleStop() {
	if a.stopCallback == nil {
		a.logger.Error("Stop callback not set")
		return
	}

	a.logger.Info("Stopping data collection from GUI")

	go func() {
		if err := a.stopCallback(); err != nil {
			a.logger.Error("Failed to stop data collection", zap.Error(err))
			return
		}

		a.isRunningMutex.Lock()
		a.isRunning = false
		a.isRunningMutex.Unlock()

		a.startButton.Enable()
		a.stopButton.Disable()
		a.statusLabel.SetText("Status: Stopped")
	}()
}

func (a *App) handleForceFlush() {
	if a.arrowHandler == nil {
		a.logger.Error("Arrow handler not set")
		return
	}

	a.logger.Info("Force flushing data from GUI")

	go func() {
		if err := a.arrowHandler.ForceFlush(); err != nil {
			a.logger.Error("Failed to force flush", zap.Error(err))
		}
	}()
}

func (a *App) startUpdateRoutine() {
	a.updateTicker = time.NewTicker(a.cfg.GUI.RefreshInterval)

	go func() {
		for {
			select {
			case <-a.stopCh:
				return
			case <-a.updateTicker.C:
				a.updateStatistics()
			}
		}
	}()
}

func (a *App) updateStatistics() {
	if a.arrowHandler == nil {
		return
	}

	stats := a.arrowHandler.GetStatistics()
	writerStats := a.arrowHandler.GetWriterStats()

	a.tickersLabel.SetText(fmt.Sprintf("Tickers: %d", stats.TickersReceived))
	a.tradesLabel.SetText(fmt.Sprintf("Trades: %d", stats.TradesReceived))
	a.bookLevelsLabel.SetText(fmt.Sprintf("Book Levels: %d", stats.BookLevelsReceived))
	a.rawBookEventsLabel.SetText(fmt.Sprintf("Raw Book Events: %d", stats.RawBookEventsReceived))
	a.errorsLabel.SetText(fmt.Sprintf("Errors: %d", stats.Errors))

	if !stats.LastFlushTime.IsZero() {
		a.lastFlushLabel.SetText(fmt.Sprintf("Last Flush: %s",
			stats.LastFlushTime.Format("15:04:05")))
	}

	if segmentsCount, ok := writerStats["segments_count"].(int); ok {
		a.segmentsLabel.SetText(fmt.Sprintf("Active Segments: %d", segmentsCount))
	}

	if ingestID, ok := writerStats["ingest_id"].(string); ok {
		if len(ingestID) > 8 {
			a.ingestIdLabel.SetText(fmt.Sprintf("Ingest ID: %s...", ingestID[:8]))
		} else {
			a.ingestIdLabel.SetText(fmt.Sprintf("Ingest ID: %s", ingestID))
		}
	}
}

func (a *App) Run() {
	a.logger.Info("Starting GUI application")

	if a.cfg.GUI.AutoStart {
		go func() {
			time.Sleep(2 * time.Second)
			a.handleStart()
		}()
	}

	a.window.ShowAndRun()
}

func (a *App) Stop() {
	a.logger.Info("Stopping GUI application")

	close(a.stopCh)

	if a.updateTicker != nil {
		a.updateTicker.Stop()
	}

	a.fyneApp.Quit()
}

func (a *App) IsRunning() bool {
	a.isRunningMutex.RLock()
	defer a.isRunningMutex.RUnlock()
	return a.isRunning
}

func (a *App) SetRunning(running bool) {
	a.isRunningMutex.Lock()
	defer a.isRunningMutex.Unlock()

	a.isRunning = running

	if running {
		a.startButton.Disable()
		a.stopButton.Enable()
		a.statusLabel.SetText("Status: Running")
	} else {
		a.startButton.Enable()
		a.stopButton.Disable()
		a.statusLabel.SetText("Status: Stopped")
	}
}