package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/gui"
	"github.com/trade-engine/data-controller/internal/sink/parquet"
	"github.com/trade-engine/data-controller/internal/ws"
)

type Application struct {
	cfg               *config.Config
	logger            *zap.Logger
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup

	// Components
	router            *ws.Router
	connectionManager *ws.ConnectionManager
	parquetHandler    *parquet.Handler
	guiApp            *gui.App

	// State
	isRunning         bool
	isRunningMutex    sync.RWMutex
}

func main() {
	configPath := flag.String("config", "config.yml", "Path to configuration file")
	flag.Parse()

	app, err := NewApplication(*configPath)
	if err != nil {
		panic(err)
	}

	if err := app.Run(); err != nil {
		app.logger.Fatal("Application failed", zap.Error(err))
	}
}

func NewApplication(configPath string) (*Application, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	logger, err := createLogger(cfg.Application.LogLevel)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &Application{
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

func (a *Application) initializeComponents() error {
	a.logger.Info("Initializing components")

	// Initialize router
	a.router = ws.NewRouter(a.logger)

	// Initialize parquet handler
	a.parquetHandler = parquet.NewHandler(a.cfg, a.logger)

	// Set router handler
	a.router.SetHandler(a.parquetHandler)

	// Initialize connection manager
	a.connectionManager = ws.NewConnectionManager(a.cfg, a.logger, a.router)

	// Initialize GUI
	a.guiApp = gui.NewApp(a.cfg, a.logger)
	a.guiApp.SetParquetHandler(a.parquetHandler)
	a.guiApp.SetCallbacks(a.startDataCollection, a.stopDataCollection)

	a.logger.Info("Components initialized successfully")
	return nil
}

func (a *Application) Run() error {
	a.logger.Info("Starting Bitfinex Data Controller",
		zap.String("version", a.cfg.Application.Version),
		zap.Strings("symbols", a.cfg.Symbols))

	// Setup signal handling
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// Start signal handler
	go a.handleSignals(signalChan)

	// Start GUI
	a.guiApp.Run()

	// Wait for all goroutines to finish
	a.wg.Wait()

	a.logger.Info("Application stopped")
	return nil
}

func (a *Application) startDataCollection() error {
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

func (a *Application) stopDataCollection() error {
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

func (a *Application) handleSignals(signalChan chan os.Signal) {
	for {
		select {
		case sig := <-signalChan:
			a.logger.Info("Received signal", zap.String("signal", sig.String()))

			switch sig {
			case syscall.SIGINT, syscall.SIGTERM:
				a.logger.Info("Shutting down gracefully...")

				// Stop data collection first
				if err := a.stopDataCollection(); err != nil {
					a.logger.Error("Failed to stop data collection", zap.Error(err))
				}

				// Stop GUI
				go func() {
					time.Sleep(1 * time.Second)
					a.guiApp.Stop()
				}()

				// Cancel context
				a.cancel()
				return
			}
		case <-a.ctx.Done():
			return
		}
	}
}

func (a *Application) Shutdown() error {
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

func createLogger(level string) (*zap.Logger, error) {
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