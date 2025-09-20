package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/sink/parquet"
	"github.com/trade-engine/data-controller/internal/ws"
)

type NoGUIApplication struct {
	cfg               *config.Config
	logger            *zap.Logger
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup

	// Components
	router            *ws.Router
	connectionManager *ws.ConnectionManager
	parquetHandler    *parquet.Handler

	// State
	isRunning         bool
	isRunningMutex    sync.RWMutex
}

func main() {
	configPath := flag.String("config", "config.yml", "Path to configuration file")
	noGUI := flag.Bool("nogui", false, "Run without GUI")
	flag.Parse()

	if *noGUI {
		app, err := NewNoGUIApplication(*configPath)
		if err != nil {
			panic(err)
		}

		if err := app.Run(); err != nil {
			app.logger.Fatal("Application failed", zap.Error(err))
		}
	} else {
		// GUI version - not implemented in this binary
		fmt.Println("GUI mode not available in this build. Use -nogui flag.")
		os.Exit(1)
	}
}

func NewNoGUIApplication(configPath string) (*NoGUIApplication, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	logger, err := createNoGUILogger(cfg.Application.LogLevel)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &NoGUIApplication{
		cfg:    cfg,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}

	if err := app.initializeComponents(); err != nil {
		return nil, err
	}

	return app, nil
}

func (a *NoGUIApplication) initializeComponents() error {
	a.logger.Info("Initializing components")

	// Initialize router
	a.router = ws.NewRouter(a.logger)

	// Initialize parquet handler
	a.parquetHandler = parquet.NewHandler(a.cfg, a.logger)

	// Set router handler
	a.router.SetHandler(a.parquetHandler)

	// Initialize connection manager
	a.connectionManager = ws.NewConnectionManager(a.cfg, a.logger, a.router)

	a.logger.Info("Components initialized successfully")
	return nil
}

func (a *NoGUIApplication) Run() error {
	a.logger.Info("Starting Bitfinex Data Controller (No GUI Mode)",
		zap.String("version", a.cfg.Application.Version),
		zap.Strings("symbols", a.cfg.Symbols))

	// Setup signal handling
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Start data collection automatically
	if err := a.startDataCollection(); err != nil {
		return fmt.Errorf("failed to start data collection: %w", err)
	}

	// Print status
	fmt.Printf("Data collection started successfully!\n")
	fmt.Printf("Collecting data for symbols: %v\n", a.cfg.Symbols)
	fmt.Printf("Storage path: %s\n", a.cfg.Storage.BasePath)
	fmt.Printf("Press Ctrl+C to stop...\n")

	// Start status reporting goroutine
	go a.statusReporter()

	// Handle signals
	a.handleSignals(signalChan)

	// Wait for all goroutines to finish
	a.wg.Wait()

	a.logger.Info("Application stopped")
	return nil
}

func (a *NoGUIApplication) statusReporter() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if a.parquetHandler != nil {
				stats := a.parquetHandler.GetStatistics()
				writerStats := a.parquetHandler.GetWriterStats()

				a.logger.Info("Status Report",
					zap.Int64("tickers", stats.TickersReceived),
					zap.Int64("trades", stats.TradesReceived),
					zap.Int64("book_levels", stats.BookLevelsReceived),
					zap.Int64("raw_book_events", stats.RawBookEventsReceived),
					zap.Int64("errors", stats.Errors),
					zap.Any("segments", writerStats["segments_count"]))
			}
		}
	}
}

func (a *NoGUIApplication) startDataCollection() error {
	a.isRunningMutex.Lock()
	defer a.isRunningMutex.Unlock()

	if a.isRunning {
		return nil
	}

	a.logger.Info("Starting data collection")

	// Start parquet handler
	if err := a.parquetHandler.Start(); err != nil {
		return err
	}

	// Start connection manager
	if err := a.connectionManager.Start(); err != nil {
		a.parquetHandler.Stop()
		return err
	}

	a.isRunning = true
	a.logger.Info("Data collection started successfully")

	return nil
}

func (a *NoGUIApplication) stopDataCollection() error {
	a.isRunningMutex.Lock()
	defer a.isRunningMutex.Unlock()

	if !a.isRunning {
		return nil
	}

	a.logger.Info("Stopping data collection")

	// Stop connection manager
	a.connectionManager.Stop()

	// Stop parquet handler
	if err := a.parquetHandler.Stop(); err != nil {
		a.logger.Error("Failed to stop parquet handler", zap.Error(err))
	}

	a.isRunning = false
	a.logger.Info("Data collection stopped successfully")

	return nil
}

func (a *NoGUIApplication) handleSignals(signalChan chan os.Signal) {
	for {
		select {
		case sig := <-signalChan:
			a.logger.Info("Received signal", zap.String("signal", sig.String()))

			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				a.logger.Info("Shutting down gracefully...")

				// Stop data collection
				if err := a.stopDataCollection(); err != nil {
					a.logger.Error("Failed to stop data collection", zap.Error(err))
				}

				// Cancel context
				a.cancel()
				return
			}
		case <-a.ctx.Done():
			return
		}
	}
}

func (a *NoGUIApplication) Shutdown() error {
	a.logger.Info("Shutting down application")

	// Stop data collection
	if err := a.stopDataCollection(); err != nil {
		a.logger.Error("Failed to stop data collection during shutdown", zap.Error(err))
	}

	// Cancel context
	a.cancel()

	// Wait for goroutines with timeout
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		a.logger.Info("All goroutines stopped")
	case <-time.After(10 * time.Second):
		a.logger.Warn("Timeout waiting for goroutines to stop")
	}

	return nil
}

func createNoGUILogger(level string) (*zap.Logger, error) {
	var config zap.Config

	switch level {
	case "debug":
		config = zap.NewDevelopmentConfig()
	case "info":
		config = zap.NewProductionConfig()
	case "warn":
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.WarnLevel)
	case "error":
		config = zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(zap.ErrorLevel)
	default:
		config = zap.NewProductionConfig()
	}

	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stderr"}

	return config.Build()
}