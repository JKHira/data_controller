//go:build gui
// +build gui

package main

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/sink/arrow"
	"github.com/trade-engine/data-controller/internal/ws"
	"github.com/trade-engine/data-controller/pkg/schema"
)

type FyneGUIApplication struct {
	cfg               *config.Config
	logger            *zap.Logger
	ctx               context.Context
	cancel            context.CancelFunc
	wg                sync.WaitGroup

	// Components
	router            *ws.Router
	connectionManager *ws.ConnectionManager
	arrowHandler      *arrow.Handler
	arrowReader       *arrow.FileReader

	// State
	isRunning         bool
	isRunningMutex    sync.RWMutex

	// Fyne Components
	app               fyne.App
	window            fyne.Window
	connectBtn        *widget.Button
	disconnectBtn     *widget.Button
	statusLabel       *widget.Label
	statsText         *widget.Entry
	filesList         *widget.List
	fileViewer        *widget.Entry

	// Data Bindings
	statusBinding     binding.String
	statsBinding      binding.String
	filesData         []string

	// Data stream display
	dataStreamList    *widget.List
	streamData        []string
	streamMutex       sync.Mutex
	maxStreamEntries  int

	// File viewer state
	currentFilePath   string
	currentPage       int
	pageSize          int
	totalPages        int
	prevBtn           *widget.Button
	nextBtn           *widget.Button
	closeBtn          *widget.Button
	pageLabel         *widget.Label
	selectedFileIndex int


	// Date/time filter state
	startDateEntry    *widget.Entry
	endDateEntry      *widget.Entry
	channelSelect     *widget.Select
	symbolSelect      *widget.Select
	filterBtn         *widget.Button
	showAllBtn        *widget.Button
	loadContentBtn    *widget.Button
	filteredFiles     []arrow.FileInfo
	allFiles          []string
}

func NewFyneGUIApplication(configPath string) (*FyneGUIApplication, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, err
	}

	logger, err := createFyneLogger(cfg.Application.LogLevel)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Create Fyne app
	myApp := app.NewWithID("bitfinex.data.controller")

	guiApp := &FyneGUIApplication{
		cfg:    cfg,
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
		app:    myApp,
	}

	if err := guiApp.initializeComponents(); err != nil {
		return nil, err
	}

	guiApp.createGUI()

	return guiApp, nil
}

func (a *FyneGUIApplication) initializeComponents() error {
	a.logger.Info("Initializing components")

	// Initialize router
	a.router = ws.NewRouter(a.logger)

	// Initialize arrow handler
	a.arrowHandler = arrow.NewHandler(a.cfg, a.logger)

	// Initialize arrow reader
	a.arrowReader = arrow.NewFileReader(a.logger)

	// Register data stream callback for live display
	a.arrowHandler.RegisterDataCallback(a.addStreamData)

	// Set router handler
	a.router.SetHandler(a.arrowHandler)

	// Initialize connection manager
	a.connectionManager = ws.NewConnectionManager(a.cfg, a.logger, a.router)

	// Initialize data bindings
	a.statusBinding = binding.NewString()
	a.statusBinding.Set("Disconnected")

	a.statsBinding = binding.NewString()
	a.statsBinding.Set("Statistics:\nTickers: 0\nTrades: 0\nBook Levels: 0\nErrors: 0")

	// Initialize stream data
	a.maxStreamEntries = 20
	a.streamData = make([]string, 0, a.maxStreamEntries)

	// Initialize file viewer state
	a.pageSize = 100 // Default page size
	a.currentPage = 1
	a.selectedFileIndex = -1 // No selection initially

	// Initialize filter state
	a.allFiles = make([]string, 0)
	a.filteredFiles = make([]arrow.FileInfo, 0)

	a.logger.Info("Components initialized successfully")
	return nil
}

func (a *FyneGUIApplication) createGUI() {
	// Create window
	a.window = a.app.NewWindow(a.cfg.GUI.Title)
	a.window.Resize(fyne.NewSize(float32(a.cfg.GUI.Width), float32(a.cfg.GUI.Height)))

	// Create connection control buttons
	a.connectBtn = widget.NewButton("🔌 Connect WebSocket", a.handleConnect)
	a.connectBtn.Importance = widget.HighImportance

	a.disconnectBtn = widget.NewButton("⏹️ Disconnect WebSocket", a.handleDisconnect)
	a.disconnectBtn.Importance = widget.MediumImportance
	a.disconnectBtn.Disable()

	// Create status displays with data binding
	a.statusLabel = widget.NewLabelWithData(a.statusBinding)
	a.statusLabel.Importance = widget.MediumImportance

	a.statsText = widget.NewMultiLineEntry()
	a.statsText.Wrapping = fyne.TextWrapWord
	a.statsText.Disable() // Read-only

	// Create date/time filter controls
	today := time.Now()
	yesterday := today.AddDate(0, 0, -1)

	a.startDateEntry = widget.NewEntry()
	a.startDateEntry.SetText(yesterday.Format("2006-01-02"))
	a.startDateEntry.SetPlaceHolder("YYYY-MM-DD")
	a.startDateEntry.Resize(fyne.NewSize(120, 30)) // Set wider size

	a.endDateEntry = widget.NewEntry()
	a.endDateEntry.SetText(today.Format("2006-01-02"))
	a.endDateEntry.SetPlaceHolder("YYYY-MM-DD")
	a.endDateEntry.Resize(fyne.NewSize(120, 30)) // Set wider size

	a.channelSelect = widget.NewSelect([]string{"", "ticker", "trades", "books", "raw_books"}, nil)
	a.channelSelect.SetSelected("")

	a.symbolSelect = widget.NewSelect([]string{"", "tBTCUSD", "tETHUSD", "tLTCUSD"}, nil)
	a.symbolSelect.SetSelected("")

	a.filterBtn = widget.NewButton("🔍 Filter", a.handleFilterFiles)
	a.showAllBtn = widget.NewButton("📁 Show All", a.handleShowAllFiles)

	a.loadContentBtn = widget.NewButton("📖 Load Content", a.handleLoadContent)

	// Create file browser
	a.filesList = widget.NewList(
		func() int { return len(a.filesData) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(a.filesData) {
				label := obj.(*widget.Label)
				label.SetText(filepath.Base(a.filesData[id]))
			}
		},
	)
	a.filesList.OnSelected = a.handleFileSelection

	// Create file viewer with pagination controls
	a.fileViewer = widget.NewMultiLineEntry()
	a.fileViewer.SetPlaceHolder("Select a file to view its content...")
	a.fileViewer.Wrapping = fyne.TextWrapWord
	a.fileViewer.Disable() // Read-only
	// Remove fixed size to allow dynamic resizing like Statistics card

	// Create pagination controls
	a.prevBtn = widget.NewButton("◀ Previous", a.handlePreviousPage)
	a.nextBtn = widget.NewButton("Next ▶", a.handleNextPage)
	a.closeBtn = widget.NewButton("✕ Close", a.handleCloseFile)
	a.pageLabel = widget.NewLabel("Page 0/0")

	a.prevBtn.Disable()
	a.nextBtn.Disable()
	a.closeBtn.Disable()

	// Create data stream display
	a.dataStreamList = widget.NewList(
		func() int { return len(a.streamData) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(a.streamData) {
				label := obj.(*widget.Label)
				label.SetText(a.streamData[id])
			}
		},
	)

	// Create layout
	a.createLayout()

	// Setup window close handler
	a.window.SetCloseIntercept(a.handleWindowClose)

	// Start background updaters
	go a.statusUpdater()
	go a.fileListUpdater()

	// Initial file list update
	a.updateFileList()
}

func (a *FyneGUIApplication) createLayout() {
	// Top toolbar with connection controls
	connectionControls := container.NewHBox(
		a.connectBtn,
		a.disconnectBtn,
		widget.NewSeparator(),
		widget.NewLabel("Status:"),
		a.statusLabel,
	)

	// Left panel: Statistics
	statsCard := widget.NewCard("📊 Statistics", "", a.statsText)

	// Right panel: File browser and viewer
	// Create date picker buttons for easier date selection
	startDatePickerBtn := widget.NewButton("📅", func() {
		a.showDatePicker("Start Date", a.startDateEntry)
	})
	startDatePickerBtn.Resize(fyne.NewSize(30, 30))

	endDatePickerBtn := widget.NewButton("📅", func() {
		a.showDatePicker("End Date", a.endDateEntry)
	})
	endDatePickerBtn.Resize(fyne.NewSize(30, 30))

	// Create GridWrapLayout containers to guarantee minimum width for date entries
	startEntryWrap := container.New(layout.NewGridWrapLayout(fyne.NewSize(140, 32)), a.startDateEntry)
	endEntryWrap := container.New(layout.NewGridWrapLayout(fyne.NewSize(140, 32)), a.endDateEntry)

	// Create filter controls layout with better spacing and picker buttons
	dateControls := container.NewHBox(
		widget.NewLabel("Start Date:"),
		startEntryWrap,
		startDatePickerBtn,
		widget.NewSeparator(),
		widget.NewLabel("End Date:"),
		endEntryWrap,
		endDatePickerBtn,
	)

	filterTypeControls := container.NewHBox(
		widget.NewLabel("Channel:"),
		a.channelSelect,
		widget.NewSeparator(),
		widget.NewLabel("Symbol:"),
		a.symbolSelect,
	)

	actionControls := container.NewHBox(
		a.filterBtn,
		a.showAllBtn,
		a.loadContentBtn,
	)

	filterControls := container.NewVBox(
		dateControls,
		filterTypeControls,
		actionControls,
		widget.NewSeparator(),
	)

	// Files list with filter controls - use Border layout for proper expansion
	filesListScroll := container.NewVScroll(a.filesList)
	filesContent := container.NewBorder(
		filterControls, // top
		nil,            // bottom
		nil,            // left
		nil,            // right
		filesListScroll,// center (takes remaining space)
	)
	filesCard := widget.NewCard("📁 Data Files", "", filesContent)

	// File viewer with controls - use Border layout for proper expansion
	viewerControls := container.NewHBox(
		a.prevBtn,
		a.nextBtn,
		widget.NewSeparator(),
		a.pageLabel,
		widget.NewSeparator(),
		a.closeBtn,
	)
	viewerScroll := container.NewVScroll(a.fileViewer)
	viewerContent := container.NewBorder(
		viewerControls, // top
		nil,            // bottom
		nil,            // left
		nil,            // right
		viewerScroll,   // center (takes remaining space)
	)
	viewerCard := widget.NewCard("👁️ File Viewer", "", viewerContent)

	rightPanel := container.NewVSplit(filesCard, viewerCard)
	rightPanel.SetOffset(0.3) // 30% files, 70% viewer (more space for viewer)

	// Main content area
	mainContent := container.NewHSplit(statsCard, rightPanel)
	mainContent.SetOffset(0.3) // 30% stats, 70% files

	// Data stream area (bottom panel)
	streamCard := widget.NewCard("📡 Live Data Stream (Latest 20)", "", a.dataStreamList)
	streamCard.Resize(fyne.NewSize(800, 150)) // Fixed height for stream

	// Status bar
	statusBar := container.NewHBox(
		widget.NewLabel("🎯 Symbols:"),
		widget.NewLabel(strings.Join(a.cfg.Symbols, ", ")),
		widget.NewSeparator(),
		widget.NewLabel("💾 Storage:"),
		widget.NewLabel(filepath.Base(a.cfg.Storage.BasePath)),
	)

	// Bottom section with stream and status
	bottomSection := container.NewVBox(streamCard, statusBar)

	// Complete layout using Border container
	content := container.NewBorder(
		connectionControls, // top
		bottomSection,      // bottom
		nil,                // left
		nil,                // right
		mainContent,        // center
	)

	a.window.SetContent(content)
}

func (a *FyneGUIApplication) handleConnect() {
	a.logger.Info("GUI: Connect button clicked")

	if err := a.startDataCollection(); err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	a.connectBtn.Disable()
	a.disconnectBtn.Enable()
	a.statusBinding.Set("🟢 Connected & Collecting Data")

	dialog.ShowInformation("Success", "✅ WebSocket connection started!\nCollecting data from: "+strings.Join(a.cfg.Symbols, ", "), a.window)
}

func (a *FyneGUIApplication) handleDisconnect() {
	a.logger.Info("GUI: Disconnect button clicked")

	if err := a.stopDataCollection(); err != nil {
		dialog.ShowError(err, a.window)
		return
	}

	a.connectBtn.Enable()
	a.disconnectBtn.Disable()
	a.statusBinding.Set("🔴 Disconnected")

	// Update file list after disconnection
	a.updateFileList()

	dialog.ShowInformation("Success", "⏹️ WebSocket connection stopped!\nData collection has been saved.", a.window)
}

func (a *FyneGUIApplication) handleFileSelection(id widget.ListItemID) {
	if id >= len(a.filesData) {
		return
	}

	// Track selected file index
	a.selectedFileIndex = int(id)

	filePath := a.filesData[id]
	a.logger.Info("GUI: File selected (single click)", zap.String("file", filePath))

	// For single click, just show file info
	info, err := os.Stat(filePath)
	if err != nil {
		a.fileViewer.SetText(fmt.Sprintf("❌ Error reading file: %v", err))
		return
	}

	content := fmt.Sprintf("📄 File: %s\n", filepath.Base(filePath))
	content += fmt.Sprintf("📁 Path: %s\n", filePath)
	content += fmt.Sprintf("📏 Size: %d bytes (%.2f MB)\n", info.Size(), float64(info.Size())/(1024*1024))
	content += fmt.Sprintf("🕒 Modified: %s\n", info.ModTime().Format(time.RFC3339))
	content += fmt.Sprintf("🏷️ Type: %s\n\n", filepath.Ext(filePath))

	if strings.HasSuffix(filePath, ".arrow") {
		content += "📊 This is an Arrow file containing trading data.\n"
		content += "💡 Double-click to view contents (10MB chunks)\n"
		content += "🔍 File contains structured data with timestamps, prices, and volumes.\n"
	}

	a.fileViewer.SetText(content)
	a.disablePageControls()
	a.closeBtn.Enable()
}

// New handler for double-click functionality
func (a *FyneGUIApplication) handleFileDoubleClick(filePath string) {
	a.logger.Info("GUI: File double-clicked", zap.String("file", filePath))

	// Set current file and reset page
	a.currentFilePath = filePath
	a.currentPage = 1

	if strings.HasSuffix(filePath, ".arrow") {
		a.loadArrowFileData()
	} else {
		a.displayFileInfo()
	}
}

func (a *FyneGUIApplication) handleFilterFiles() {
	a.logger.Info("GUI: Filter files clicked")

	startDate, err := time.Parse("2006-01-02", a.startDateEntry.Text)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Invalid start date format. Use YYYY-MM-DD"), a.window)
		return
	}

	endDate, err := time.Parse("2006-01-02", a.endDateEntry.Text)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Invalid end date format. Use YYYY-MM-DD"), a.window)
		return
	}

	// Add full day to end date
	endDate = endDate.Add(24 * time.Hour)

	channel := a.channelSelect.Selected
	symbol := a.symbolSelect.Selected

	filteredFiles, err := a.arrowReader.GetFilesByDateRange(a.cfg.Storage.BasePath, startDate, endDate, channel, symbol)
	if err != nil {
		dialog.ShowError(fmt.Errorf("Failed to filter files: %v", err), a.window)
		return
	}

	a.filteredFiles = filteredFiles

	// Update files list with filtered results
	a.filesData = make([]string, len(filteredFiles))
	for i, fileInfo := range filteredFiles {
		a.filesData[i] = fileInfo.Path
	}

	a.filesList.Refresh()

	dialog.ShowInformation("Filter Results",
		fmt.Sprintf("Found %d files matching criteria\nDate range: %s to %s\nChannel: %s\nSymbol: %s",
			len(filteredFiles),
			startDate.Format("2006-01-02"),
			endDate.Add(-24*time.Hour).Format("2006-01-02"),
			channel, symbol), a.window)
}

func (a *FyneGUIApplication) handleShowAllFiles() {
	a.logger.Info("GUI: Show all files clicked")

	// Reset to show all files
	a.updateFileList()
	dialog.ShowInformation("Files", fmt.Sprintf("Showing all %d files", len(a.filesData)), a.window)
}

func (a *FyneGUIApplication) handleLoadContent() {
	if a.selectedFileIndex < 0 || a.selectedFileIndex >= len(a.filesData) {
		dialog.ShowInformation("No Selection", "Please select a file first to load its content.", a.window)
		return
	}

	filePath := a.filesData[a.selectedFileIndex]
	a.handleFileDoubleClick(filePath)
}

func (a *FyneGUIApplication) displayFileInfo() {
	info, err := os.Stat(a.currentFilePath)
	if err != nil {
		a.fileViewer.SetText(fmt.Sprintf("❌ Error reading file: %v", err))
		a.disablePageControls()
		return
	}

	content := fmt.Sprintf("📄 File: %s\n", filepath.Base(a.currentFilePath))
	content += fmt.Sprintf("📁 Path: %s\n", a.currentFilePath)
	content += fmt.Sprintf("📏 Size: %d bytes\n", info.Size())
	content += fmt.Sprintf("🕒 Modified: %s\n", info.ModTime().Format(time.RFC3339))
	content += fmt.Sprintf("🏷️ Type: %s\n\n", filepath.Ext(a.currentFilePath))
	content += "🚫 This file type is not supported for data viewing.\n"

	a.fileViewer.SetText(content)
	a.disablePageControls()
	a.closeBtn.Enable()
}

func (a *FyneGUIApplication) loadArrowFileData() {
	a.logger.Info("Loading Arrow file data", zap.String("file", a.currentFilePath))

	// First get file summary
	summary, err := a.arrowReader.ReadArrowFileSummary(a.currentFilePath)
	if err != nil {
		a.fileViewer.SetText(fmt.Sprintf("❌ Error reading Arrow file: %v", err))
		a.disablePageControls()
		return
	}

	// Load page data
	pageData, err := a.arrowReader.ReadArrowFileWithPagination(a.currentFilePath, a.currentPage, a.pageSize)
	if err != nil {
		a.fileViewer.SetText(fmt.Sprintf("❌ Error reading Arrow file page: %v", err))
		a.disablePageControls()
		return
	}

	a.totalPages = pageData.TotalPages
	a.displayArrowData(summary, pageData)
	a.updatePageControls()
}

func (a *FyneGUIApplication) displayArrowData(summary map[string]interface{}, pageData *arrow.PageData) {
	content := fmt.Sprintf("📊 Arrow File: %s\n", filepath.Base(a.currentFilePath))
	content += fmt.Sprintf("📏 File Size: %v bytes (%.2f MB)\n", summary["file_size"], float64(summary["file_size"].(int64))/(1024*1024))
	content += fmt.Sprintf("📈 Total Records: %v\n", summary["total_records"])
	content += fmt.Sprintf("📦 Batches: %v\n", summary["num_batches"])
	content += fmt.Sprintf("📝 Columns: %v\n", summary["num_columns"])
	content += fmt.Sprintf("📊 Data Read: %.2f MB / %.2f MB\n\n", float64(pageData.BytesRead)/(1024*1024), float64(pageData.TotalBytes)/(1024*1024))

	// Schema information (show only first few fields to save space)
	if fields, ok := summary["schema_fields"].([]map[string]string); ok {
		content += "🗃️ Schema (showing first 8 fields):\n"
		maxFields := min(8, len(fields))
		for i := 0; i < maxFields; i++ {
			field := fields[i]
			content += fmt.Sprintf("  • %s: %s\n", field["name"], field["type"])
		}
		if len(fields) > 8 {
			content += fmt.Sprintf("  ... and %d more fields\n", len(fields)-8)
		}
		content += "\n"
	}

	// Page data with 10MB chunk info
	content += fmt.Sprintf("📄 Chunk %d/%d (~10MB per chunk):\n", pageData.PageNumber, pageData.TotalPages)
	content += fmt.Sprintf("📊 Records in this chunk: %d\n", len(pageData.Records))
	content += fmt.Sprintf("💾 Bytes loaded: %.2f MB\n", float64(pageData.BytesRead)/(1024*1024))
	content += strings.Repeat("─", 80) + "\n\n"

	// Display records (limit to first 50 for readability)
	maxRecords := min(50, len(pageData.Records))
	for i := 0; i < maxRecords; i++ {
		record := pageData.Records[i]
		content += fmt.Sprintf("🔢 Record #%d:\n", i+1)

		// Show important fields first
		importantFields := []string{"exchange", "symbol", "ts_micros", "price", "amount", "bid", "ask", "last"}
		for _, fieldName := range importantFields {
			if value, exists := record[fieldName]; exists {
				if value == nil {
					content += fmt.Sprintf("  %s: <null>\n", fieldName)
				} else {
					content += fmt.Sprintf("  %s: %v\n", fieldName, value)
				}
			}
		}

		// Show other fields (limit to save space)
		otherFieldCount := 0
		for key, value := range record {
			// Skip already shown fields
			isImportant := false
			for _, imp := range importantFields {
				if key == imp {
					isImportant = true
					break
				}
			}
			if isImportant {
				continue
			}

			if otherFieldCount < 3 { // Show max 3 additional fields
				if value == nil {
					content += fmt.Sprintf("  %s: <null>\n", key)
				} else {
					content += fmt.Sprintf("  %s: %v\n", key, value)
				}
				otherFieldCount++
			}
		}
		content += "\n"
	}

	if len(pageData.Records) > 50 {
		content += fmt.Sprintf("... and %d more records in this chunk\n", len(pageData.Records)-50)
		content += "💡 Use Previous/Next buttons to navigate through data\n"
	}

	a.fileViewer.SetText(content)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (a *FyneGUIApplication) updatePageControls() {
	a.pageLabel.SetText(fmt.Sprintf("Page %d/%d", a.currentPage, a.totalPages))

	if a.currentPage <= 1 {
		a.prevBtn.Disable()
	} else {
		a.prevBtn.Enable()
	}

	if a.currentPage >= a.totalPages {
		a.nextBtn.Disable()
	} else {
		a.nextBtn.Enable()
	}

	a.closeBtn.Enable()
}

func (a *FyneGUIApplication) disablePageControls() {
	a.prevBtn.Disable()
	a.nextBtn.Disable()
	a.pageLabel.SetText("Page 0/0")
}

func (a *FyneGUIApplication) handlePreviousPage() {
	if a.currentPage > 1 {
		a.currentPage--
		a.loadArrowFileData()
	}
}

func (a *FyneGUIApplication) handleNextPage() {
	if a.currentPage < a.totalPages {
		a.currentPage++
		a.loadArrowFileData()
	}
}

func (a *FyneGUIApplication) handleCloseFile() {
	a.currentFilePath = ""
	a.currentPage = 1
	a.totalPages = 0
	a.fileViewer.SetText("")
	a.fileViewer.SetPlaceHolder("Select a file to view its content...")
	a.disablePageControls()
	a.closeBtn.Disable()
}

func (a *FyneGUIApplication) handleWindowClose() {
	if a.isRunning {
		dialog.ShowConfirm("Close Application",
			"⚠️ Data collection is still running.\nStop collection and close?",
			func(confirm bool) {
				if confirm {
					a.stopDataCollection()
					a.shutdown()
					a.app.Quit()
				}
			}, a.window)
	} else {
		a.shutdown()
		a.app.Quit()
	}
}

func (a *FyneGUIApplication) statusUpdater() {
	ticker := time.NewTicker(time.Duration(a.cfg.GUI.RefreshInterval))
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

func (a *FyneGUIApplication) fileListUpdater() {
	ticker := time.NewTicker(5 * time.Second) // Update files every 5 seconds
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			if !a.isRunning {
				fyne.Do(func() {
					a.updateFileList()
				})
			}
		}
	}
}

func (a *FyneGUIApplication) updateStatus() {
	if a.arrowHandler == nil {
		return
	}

	stats := a.arrowHandler.GetStatistics()
	writerStats := a.arrowHandler.GetWriterStats()

	statsText := "📊 Real-time Statistics:\n\n"
	statsText += fmt.Sprintf("📈 Tickers: %d\n", stats.TickersReceived)
	statsText += fmt.Sprintf("💰 Trades: %d\n", stats.TradesReceived)
	statsText += fmt.Sprintf("📚 Book Levels: %d\n", stats.BookLevelsReceived)
	statsText += fmt.Sprintf("📝 Raw Book Events: %d\n", stats.RawBookEventsReceived)
	statsText += fmt.Sprintf("🎛️ Control Messages: %d\n", stats.ControlsReceived)
	statsText += fmt.Sprintf("❌ Errors: %d\n", stats.Errors)

	if segmentCount, ok := writerStats["segments_count"]; ok {
		statsText += fmt.Sprintf("🗂️ Segments: %v\n", segmentCount)
	}

	if !stats.LastFlushTime.IsZero() {
		statsText += fmt.Sprintf("💾 Last Flush: %s\n", stats.LastFlushTime.Format("15:04:05"))
	}

	// Thread-safe UI update
	fyne.Do(func() {
		a.statsText.SetText(statsText)
	})
}

func (a *FyneGUIApplication) updateFileList() {
	files := a.getDataFiles()
	a.filesData = files

	// Refresh the list
	a.filesList.Refresh()
}

func (a *FyneGUIApplication) getDataFiles() []string {
	var files []string

	dataPath := a.cfg.Storage.BasePath
	if _, err := os.Stat(dataPath); os.IsNotExist(err) {
		return files
	}

	err := filepath.WalkDir(dataPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Continue walking
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

func (a *FyneGUIApplication) addStreamData(dataType, symbol string, data interface{}) {
	a.streamMutex.Lock()
	defer a.streamMutex.Unlock()

	timestamp := time.Now().Format("15:04:05.000")
	var message string

	switch dataType {
	case "ticker":
		if ticker, ok := data.(*schema.Ticker); ok {
			message = fmt.Sprintf("[%s] 📈 %s: Bid=%.2f Ask=%.2f", timestamp, symbol, ticker.Bid, ticker.Ask)
		}
	case "trade":
		if trade, ok := data.(*schema.Trade); ok {
			side := "BUY"
			if trade.Amount < 0 {
				side = "SELL"
			}
			message = fmt.Sprintf("[%s] 💰 %s: %s %.6f @ %.2f", timestamp, symbol, side, abs(trade.Amount), trade.Price)
		}
	case "book":
		if book, ok := data.(*schema.BookLevel); ok {
			side := "BID"
			if book.Side == schema.SideAsk {
				side = "ASK"
			}
			message = fmt.Sprintf("[%s] 📚 %s: %s %.2f (%.4f)", timestamp, symbol, side, book.Price, book.Amount)
		}
	case "raw_book":
		if rawBook, ok := data.(*schema.RawBookEvent); ok {
			action := "UPDATE"
			if rawBook.Amount == 0 {
				action = "DELETE"
			}
			message = fmt.Sprintf("[%s] 📝 %s: %s %.2f", timestamp, symbol, action, rawBook.Price)
		}
	default:
		message = fmt.Sprintf("[%s] 📡 %s: %s", timestamp, symbol, dataType)
	}

	// Add to beginning of slice
	a.streamData = append([]string{message}, a.streamData...)

	// Keep only the latest entries
	if len(a.streamData) > a.maxStreamEntries {
		a.streamData = a.streamData[:a.maxStreamEntries]
	}

	// Update UI on main thread
	fyne.Do(func() {
		a.dataStreamList.Refresh()
	})
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func (a *FyneGUIApplication) Run() {
	a.window.ShowAndRun()
}

func (a *FyneGUIApplication) startDataCollection() error {
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

func (a *FyneGUIApplication) stopDataCollection() error {
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

func (a *FyneGUIApplication) shutdown() {
	a.logger.Info("Shutting down Fyne GUI application")

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
}

func (a *FyneGUIApplication) showDatePicker(title string, targetEntry *widget.Entry) {
	// Create a simple date picker dialog
	today := time.Now()

	// Create year selector
	years := make([]string, 11) // Show 5 years back and 5 years forward
	currentYear := today.Year()
	for i := 0; i < 11; i++ {
		years[i] = fmt.Sprintf("%d", currentYear-5+i)
	}

	// Create month selector
	months := []string{
		"01", "02", "03", "04", "05", "06",
		"07", "08", "09", "10", "11", "12",
	}

	// Create day selector
	days := make([]string, 31)
	for i := 1; i <= 31; i++ {
		days[i-1] = fmt.Sprintf("%02d", i)
	}

	yearSelect := widget.NewSelect(years, nil)
	monthSelect := widget.NewSelect(months, nil)
	daySelect := widget.NewSelect(days, nil)

	// Set default values to current date
	yearSelect.SetSelected(fmt.Sprintf("%d", currentYear))
	monthSelect.SetSelected(fmt.Sprintf("%02d", today.Month()))
	daySelect.SetSelected(fmt.Sprintf("%02d", today.Day()))

	// Add OK button functionality
	okButton := widget.NewButton("OK", func() {
		year := yearSelect.Selected
		month := monthSelect.Selected
		day := daySelect.Selected

		if year != "" && month != "" && day != "" {
			selectedDate := fmt.Sprintf("%s-%s-%s", year, month, day)
			targetEntry.SetText(selectedDate)
		}
	})

	// Create dialog content with all components including buttons
	dialogContent := container.NewVBox(
		widget.NewLabel(fmt.Sprintf("Select %s", title)),
		widget.NewSeparator(),
		widget.NewLabel("Year:"),
		yearSelect,
		widget.NewLabel("Month:"),
		monthSelect,
		widget.NewLabel("Day:"),
		daySelect,
		widget.NewSeparator(),
		container.NewHBox(
			widget.NewButton("Today", func() {
				today := time.Now()
				selectedDate := today.Format("2006-01-02")
				targetEntry.SetText(selectedDate)
			}),
			okButton,
		),
	)

	// Create dialog
	pickerDialog := dialog.NewCustomConfirm(title, "OK", "Cancel", dialogContent,
		func(confirmed bool) {
			if confirmed {
				year := yearSelect.Selected
				month := monthSelect.Selected
				day := daySelect.Selected

				if year != "" && month != "" && day != "" {
					selectedDate := fmt.Sprintf("%s-%s-%s", year, month, day)
					targetEntry.SetText(selectedDate)
				}
			}
		}, a.window)
	pickerDialog.Resize(fyne.NewSize(300, 400))
	pickerDialog.Show()
}

func createFyneLogger(level string) (*zap.Logger, error) {
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