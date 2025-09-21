package main

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/sink/arrow"
	"github.com/trade-engine/data-controller/internal/ws"
)

type TerminalGUIApplication struct {
	cfg    *config.Config
	logger *zap.Logger
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Components
	router            *ws.Router
	connectionManager *ws.ConnectionManager
	arrowHandler      *arrow.Handler

	// State
	isRunning      bool
	isRunningMutex sync.RWMutex

	// GUI State
	scanner *bufio.Scanner
}

func NewTerminalGUIApplication(configPath string) (*TerminalGUIApplication, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	logger, err := createGUILogger(cfg.Application.LogLevel)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	app := &TerminalGUIApplication{
		cfg:     cfg,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
		scanner: bufio.NewScanner(os.Stdin),
	}

	if err := app.initializeComponents(); err != nil {
		return nil, err
	}

	return app, nil
}

func (a *TerminalGUIApplication) initializeComponents() error {
	a.logger.Info("Initializing components")

	// Initialize router
	a.router = ws.NewRouter(a.logger)

	// Initialize arrow handler
	a.arrowHandler = arrow.NewHandler(a.cfg, a.logger)

	// Set router handler
	a.router.SetHandler(a.arrowHandler)

	// Initialize connection manager
	a.connectionManager = ws.NewConnectionManager(a.cfg, a.logger, a.router)

	a.logger.Info("Components initialized successfully")
	return nil
}

func (a *TerminalGUIApplication) Run() {
	fmt.Printf("\n")
	fmt.Printf("=========================================\n")
	fmt.Printf("  Data Controller GUI\n")
	fmt.Printf("=========================================\n")
	fmt.Printf("Version: %s\n", a.cfg.Application.Version)
	fmt.Printf("Symbols: %v\n", a.cfg.Symbols)
	fmt.Printf("Storage: %s\n", a.cfg.Storage.BasePath)
	fmt.Printf("=========================================\n")

	// Start status updater
	go a.statusUpdater()

	// Main GUI loop
	for {
		a.displayMenu()

		if !a.scanner.Scan() {
			break
		}

		input := strings.TrimSpace(a.scanner.Text())
		if !a.handleInput(input) {
			break
		}
	}

	// Cleanup
	a.shutdown()
}

func (a *TerminalGUIApplication) displayMenu() {
	fmt.Printf("\n")

	// Connection status
	status := "Disconnected"
	if a.isRunning {
		status = "Connected & Collecting Data"
	}
	fmt.Printf("Status: %s\n", status)

	// Statistics
	if a.arrowHandler != nil {
		stats := a.arrowHandler.GetStatistics()
		fmt.Printf("Stats: Tickers:%d Trades:%d Books:%d Errors:%d\n",
			stats.TickersReceived, stats.TradesReceived,
			stats.BookLevelsReceived, stats.Errors)
	}

	fmt.Printf("\n")
	fmt.Printf("Available Commands:\n")
	fmt.Printf("1. Start WebSocket Data Collection\n")
	fmt.Printf("2. Stop WebSocket Data Collection\n")
	fmt.Printf("3. View Data Files\n")
	fmt.Printf("4. Show Statistics\n")
	fmt.Printf("5. Exit\n")
	fmt.Printf("\nEnter command (1-5): ")
}

func (a *TerminalGUIApplication) handleInput(input string) bool {
	switch input {
	case "1":
		a.handleStartCollection()
	case "2":
		a.handleStopCollection()
	case "3":
		a.handleViewFiles()
	case "4":
		a.handleShowStats()
	case "5":
		fmt.Printf("Exiting...\n")
		return false
	default:
		fmt.Printf("Invalid command. Please enter 1-5.\n")
	}
	return true
}

func (a *TerminalGUIApplication) handleStartCollection() {
	if a.isRunning {
		fmt.Printf("Data collection is already running!\n")
		return
	}

	fmt.Printf("Starting WebSocket data collection...\n")
	if err := a.startDataCollection(); err != nil {
		fmt.Printf("Error starting data collection: %v\n", err)
		return
	}

	fmt.Printf("✅ Data collection started successfully!\n")
	fmt.Printf("Collecting from: %v\n", a.cfg.Symbols)
}

func (a *TerminalGUIApplication) handleStopCollection() {
	if !a.isRunning {
		fmt.Printf("Data collection is not running!\n")
		return
	}

	fmt.Printf("Stopping WebSocket data collection...\n")
	if err := a.stopDataCollection(); err != nil {
		fmt.Printf("Error stopping data collection: %v\n", err)
		return
	}

	fmt.Printf("✅ Data collection stopped successfully!\n")
}

func (a *TerminalGUIApplication) handleViewFiles() {
	fmt.Printf("\n📁 Data Files:\n")
	files := a.getDataFiles()

	if len(files) == 0 {
		fmt.Printf("No data files found. Start data collection first.\n")
		return
	}

	for i, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		fmt.Printf("%d. %s (%d bytes, %s)\n",
			i+1,
			filepath.Base(file),
			info.Size(),
			info.ModTime().Format("15:04:05"))

		if i >= 9 { // Show max 10 files
			fmt.Printf("... and %d more files\n", len(files)-10)
			break
		}
	}

	fmt.Printf("\nEnter file number to view details (or press Enter to continue): ")
	if a.scanner.Scan() {
		input := strings.TrimSpace(a.scanner.Text())
		if input != "" {
			if num, err := strconv.Atoi(input); err == nil && num > 0 && num <= len(files) {
				a.showFileDetails(files[num-1])
			}
		}
	}
}

func (a *TerminalGUIApplication) showFileDetails(filePath string) {
	info, err := os.Stat(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	fmt.Printf("\n📄 File Details:\n")
	fmt.Printf("Path: %s\n", filePath)
	fmt.Printf("Size: %d bytes\n", info.Size())
	fmt.Printf("Modified: %s\n", info.ModTime().Format(time.RFC3339))
	fmt.Printf("Type: %s\n", filepath.Ext(filePath))

	if strings.HasSuffix(filePath, ".arrow") {
		fmt.Printf("\nThis is an Arrow file containing trading data:\n")
		fmt.Printf("- Ticker data (bid/ask prices)\n")
		fmt.Printf("- Trade events (price, volume, timestamp)\n")
		fmt.Printf("- Order book levels (aggregated depth)\n")
	}
}

func (a *TerminalGUIApplication) handleShowStats() {
	fmt.Printf("\n📊 Detailed Statistics:\n")

	if a.arrowHandler == nil {
		fmt.Printf("Handler not initialized\n")
		return
	}

	stats := a.arrowHandler.GetStatistics()
	writerStats := a.arrowHandler.GetWriterStats()

	fmt.Printf("Tickers Received: %d\n", stats.TickersReceived)
	fmt.Printf("Trades Received: %d\n", stats.TradesReceived)
	fmt.Printf("Book Levels Received: %d\n", stats.BookLevelsReceived)
	fmt.Printf("Raw Book Events: %d\n", stats.RawBookEventsReceived)
	fmt.Printf("Control Messages: %d\n", stats.ControlsReceived)
	fmt.Printf("Errors: %d\n", stats.Errors)

	if segmentCount, ok := writerStats["segments_count"]; ok {
		fmt.Printf("Segments Created: %v\n", segmentCount)
	}

	if !stats.LastFlushTime.IsZero() {
		fmt.Printf("Last Flush: %s\n", stats.LastFlushTime.Format("15:04:05"))
	}
}

func (a *TerminalGUIApplication) getDataFiles() []string {
	var files []string

	dataPath := a.cfg.Storage.BasePath
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		return files
	}

	err := filepath.WalkDir(dataPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if !d.IsDir() && strings.HasSuffix(path, ".arrow") {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		a.logger.Error("Failed to walk data directory", zap.Error(err))
	}

	return files
}

func (a *TerminalGUIApplication) statusUpdater() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			// Background status updates (can add logging here if needed)
		}
	}
}

func (a *TerminalGUIApplication) startDataCollection() error {
	a.isRunningMutex.Lock()
	defer a.isRunningMutex.Unlock()

	if a.isRunning {
		return nil
	}

	a.logger.Info("Starting data collection")

	// Start arrow handler
	if err := a.arrowHandler.Start(); err != nil {
		return err
	}

	// Start connection manager
	if err := a.connectionManager.Start(); err != nil {
		a.arrowHandler.Stop()
		return err
	}

	a.isRunning = true
	a.logger.Info("Data collection started successfully")

	return nil
}

func (a *TerminalGUIApplication) stopDataCollection() error {
	a.isRunningMutex.Lock()
	defer a.isRunningMutex.Unlock()

	if !a.isRunning {
		return nil
	}

	a.logger.Info("Stopping data collection")

	// Stop connection manager
	a.connectionManager.Stop()

	// Stop arrow handler
	if err := a.arrowHandler.Stop(); err != nil {
		a.logger.Error("Failed to stop arrow handler", zap.Error(err))
	}

	a.isRunning = false
	a.logger.Info("Data collection stopped successfully")

	return nil
}

func (a *TerminalGUIApplication) shutdown() {
	a.logger.Info("Shutting down GUI application")

	// Stop data collection if running
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
}

func createGUILogger(level string) (*zap.Logger, error) {
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
