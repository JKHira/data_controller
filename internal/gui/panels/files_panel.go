package panels

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/domain"
	"github.com/trade-engine/data-controller/internal/gui/controllers"
	"github.com/trade-engine/data-controller/internal/gui/state"
	"github.com/trade-engine/data-controller/internal/services"
)

// FilesPanel handles the file browser interface with full filtering
type FilesPanel struct {
	logger         *zap.Logger
	cfg            *config.Config
	state          *state.AppState
	fileController *controllers.FileController
	fileScanner    *services.FileScanner
	window         fyne.Window

	// Filter controls
	sourceSelect    *widget.Select
	categorySelect  *widget.Select
	symbolSelect    *widget.Select
	dateFromPicker  *widget.DateEntry
	dateToPicker    *widget.DateEntry
	hourSelect      *widget.Select
	typeSelect      *widget.Select
	// filterEntry removed - filename filter not needed

	// State for symbol selection
	symbolRemember string

	// Action buttons
	scanBtn        *widget.Button
	loadBtn        *widget.Button

	// Results
	filesList      *widget.List
	statusLabel    *widget.Label

	// Async control
	scanCtx        context.Context
	scanCancel     context.CancelFunc
	isScanning     bool
}

// Source aliases for backward compatibility
var sourceAliases = map[string][]string{
	"websocket": {"websocket", "ws", "v2"},
	"restapi":   {"restapi", "restv2"},
}

// NewFilesPanel creates a new files panel with full filtering capabilities
func NewFilesPanel(logger *zap.Logger, cfg *config.Config, appState *state.AppState, fileController *controllers.FileController, window fyne.Window) *FilesPanel {
	panel := &FilesPanel{
		logger:         logger,
		cfg:            cfg,
		state:          appState,
		fileController: fileController,
		fileScanner:    services.NewFileScanner(logger, cfg.Storage.BasePath),
		window:         window,
	}

	panel.createUI()

	// Initialize symbol dropdown after UI creation
	go func() {
		// Small delay to ensure UI is fully initialized
		time.Sleep(100 * time.Millisecond)
		panel.refreshSymbols()
	}()

	return panel
}

// createUI creates the files panel UI components with full filtering
func (fp *FilesPanel) createUI() {
	// Filter controls
	fp.sourceSelect = widget.NewSelect([]string{"websocket", "restapi"}, nil)
	fp.sourceSelect.SetSelected("websocket")
	fp.sourceSelect.OnChanged = fp.onSourceChanged

	fp.categorySelect = widget.NewSelect([]string{"trades", "ticker", "books", "raw_books", "All trades", "All books"}, nil)
	fp.categorySelect.SetSelected("trades")
	fp.categorySelect.OnChanged = fp.onCategoryChanged

	fp.symbolSelect = widget.NewSelect([]string{}, nil)
	fp.symbolSelect.PlaceHolder = "Select symbol..."
	fp.symbolSelect.OnChanged = func(selected string) {
		fp.symbolRemember = selected
	}

	// Date pickers with year selection (Fyne v2.6 DateEntry)
	fp.dateFromPicker = widget.NewDateEntry()
	from := time.Now().AddDate(0, 0, -7)
	fp.dateFromPicker.SetDate(&from)

	fp.dateToPicker = widget.NewDateEntry()
	to := time.Now()
	fp.dateToPicker.SetDate(&to)

	hours := []string{"All"}
	for i := 0; i < 24; i++ {
		hours = append(hours, fmt.Sprintf("%02d", i))
	}
	fp.hourSelect = widget.NewSelect(hours, nil)
	fp.hourSelect.SetSelected("All")

	fp.typeSelect = widget.NewSelect([]string{"any", "arrow", "jsonl"}, nil)
	fp.typeSelect.SetSelected("any")

	// Filter entry removed - filename filter not needed as requested

	// Action buttons
	fp.scanBtn = widget.NewButton("🔍 Scan", fp.handleScan)
	fp.loadBtn = widget.NewButton("📖 Load", fp.handleLoad)
	fp.loadBtn.Disable()

	// Status label
	fp.statusLabel = widget.NewLabel("Ready to scan")

	// Files list
	fp.filesList = widget.NewList(
		func() int { return len(fp.state.FilteredFiles) },
		func() fyne.CanvasObject {
			main := widget.NewLabel("filename")
			sub := widget.NewLabel("metadata")
			sub.TextStyle.Italic = true
			return container.NewVBox(main, sub)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(fp.state.FilteredFiles) {
				item := fp.state.FilteredFiles[id]
				box := obj.(*fyne.Container)
				main := box.Objects[0].(*widget.Label)
				sub := box.Objects[1].(*widget.Label)

				main.SetText(filepath.Base(item.Path))

				// Build metadata
				metadata := fmt.Sprintf("%s · %s · %s · %s · %s · %.1fMB",
					item.Exchange, item.Source, item.Category,
					item.Symbol, item.Date, float64(item.Size)/(1024*1024))
				sub.SetText(metadata)
			}
		},
	)
	fp.filesList.OnSelected = fp.handleFileSelection
}

// GetContent returns the complete files panel with all controls
func (fp *FilesPanel) GetContent() fyne.CanvasObject {
	// Filter controls grid
	filterForm := container.NewGridWithColumns(2,
		widget.NewLabel("Source:"), fp.sourceSelect,
		widget.NewLabel("Category:"), fp.categorySelect,
		widget.NewLabel("Symbol:"), fp.symbolSelect,
		widget.NewLabel("Date From:"), fp.dateFromPicker,
		widget.NewLabel("Date To:"), fp.dateToPicker,
		widget.NewLabel("Hour:"), fp.hourSelect,
		widget.NewLabel("Type:"), fp.typeSelect,
	)

	// Action buttons
	buttonRow := container.NewHBox(fp.scanBtn, fp.loadBtn)

	// Main layout
	return container.NewVBox(
		widget.NewCard("File Loader", "", filterForm),
		buttonRow,
		fp.statusLabel,
		fp.filesList,
	)
}

// handleScan handles the scan button click (async)
func (fp *FilesPanel) handleScan() {
	if fp.isScanning {
		// Cancel current scan
		if fp.scanCancel != nil {
			fp.scanCancel()
		}
		return
	}

	// Parse dates from DateEntry widgets
	if fp.dateFromPicker.Date == nil || fp.dateToPicker.Date == nil {
		fp.showError("Please select both dates")
		return
	}
	dateFrom := *fp.dateFromPicker.Date
	dateTo := *fp.dateToPicker.Date
	// Normalize to date boundaries for dt=YYYY-MM-DD matching
	dateFrom = time.Date(dateFrom.Year(), dateFrom.Month(), dateFrom.Day(), 0, 0, 0, 0, dateFrom.Location())
	dateTo = time.Date(dateTo.Year(), dateTo.Month(), dateTo.Day(), 23, 59, 59, 0, dateTo.Location())

	// Handle symbol selection for "All" categories
	symbol := fp.symbolSelect.Selected
	if strings.HasPrefix(fp.categorySelect.Selected, "All ") {
		symbol = "ALL"
	} else if symbol == "" {
		fp.showError("Please select a symbol")
		return
	}

	// Prepare scan parameters (Filter removed as requested)
	params := domain.ScanParams{
		BasePath: fp.cfg.Storage.BasePath,
		Exchange: "ALL", // Support multiple exchanges in data/ directory
		Source:   fp.sourceSelect.Selected,
		Category: fp.categorySelect.Selected,
		Symbol:   symbol,
		DateFrom: dateFrom,
		DateTo:   dateTo,
		Hour:     fp.hourSelect.Selected,
		Ext:      fp.typeSelect.Selected,
		// Filter field removed - filename filter not needed
	}

	// Start async scan
	fp.scanCtx, fp.scanCancel = context.WithCancel(context.Background())
	fp.isScanning = true
	fp.scanBtn.SetText("⏹ Cancel")
	fp.statusLabel.SetText("Scanning...")

	go func() {
		start := time.Now()
		files, err := fp.fileScanner.FindFiles(fp.scanCtx, params)

		// All UI updates must be done on the main thread
		fp.ui(func() {
			if err != nil && err != context.Canceled {
				fp.showError(fmt.Sprintf("Scan failed: %v", err))
			} else if err == context.Canceled {
				fp.statusLabel.SetText("Scan cancelled")
			} else {
				totalSize := int64(0)
				for _, file := range files {
					totalSize += file.Size
				}
				duration := time.Since(start)
				fp.statusLabel.SetText(fmt.Sprintf("Found %d files · %.1fMB · scanned in %v",
					len(files), float64(totalSize)/(1024*1024), duration))

				fp.state.FilteredFiles = files
				fp.filesList.Refresh()

				if len(files) > 0 {
					fp.loadBtn.Enable()
				}
			}

			fp.isScanning = false
			fp.scanBtn.SetText("🔍 Scan")
		})
	}()
}

// handleLoad handles the load button click
func (fp *FilesPanel) handleLoad() {
	if fp.state.SelectedFileIndex < 0 || fp.state.SelectedFileIndex >= len(fp.state.FilteredFiles) {
		fp.showError("Please select a file first")
		return
	}

	fileItem := fp.state.FilteredFiles[fp.state.SelectedFileIndex]

	// Use the domain-specific methods for FileController
	fp.fileController.HandleFileSelectionDomain(fileItem)
	fp.fileController.HandleFileDoubleClickDomain(fileItem)
}

// handleFileSelection handles file selection in the list
func (fp *FilesPanel) handleFileSelection(id widget.ListItemID) {
	fp.state.SelectedFileIndex = int(id)

	if id >= 0 && id < len(fp.state.FilteredFiles) {
		fp.loadBtn.Enable()
	} else {
		fp.loadBtn.Disable()
	}
}

// UI helper methods to safely update UI from goroutines
func (fp *FilesPanel) ui(f func()) {
	fyne.Do(f)
}

// showError shows an error dialog
func (fp *FilesPanel) showError(message string) {
	fp.logger.Error("Files panel error", zap.String("message", message))
	fp.ui(func() {
		dialog.ShowError(fmt.Errorf(message), fp.window)
	})
}

// Refresh refreshes the files list display
func (fp *FilesPanel) Refresh() {
	fp.ui(func() {
		fp.filesList.Refresh()
	})
}

// onSourceChanged handles source selection changes
func (fp *FilesPanel) onSourceChanged(selected string) {
	fp.refreshSymbols()
}

// onCategoryChanged handles category selection changes
func (fp *FilesPanel) onCategoryChanged(selected string) {
	fp.refreshSymbols()
}

// refreshSymbols refreshes the symbol dropdown based on current source and category
func (fp *FilesPanel) refreshSymbols() {
	src := fp.sourceSelect.Selected
	cat := fp.categorySelect.Selected

	if src == "" || cat == "" {
		return
	}

	// All books / All trades のときはシンボル一覧を使わない
	if strings.HasPrefix(cat, "All ") {
		fp.ui(func() {
			fp.symbolSelect.SetOptions([]string{"ALL"})
			fp.symbolSelect.SetSelected("ALL")
			fp.symbolSelect.Disable()
		})
		return
	}

	go func() {
		// 多取引所対応: data/ 直下の全取引所を走査
		syms := make(map[string]struct{})
		dataRoot := filepath.Join(fp.cfg.Storage.BasePath, "data")

		// Get all exchanges (bitfinex, otherexchange, etc.)
		exchanges, err := os.ReadDir(dataRoot)
		if err != nil {
			fp.ui(func() {
				fp.symbolSelect.SetOptions([]string{"no data"})
				fp.symbolSelect.SetSelected("no data")
				fp.symbolSelect.Disable()
			})
			return
		}

		for _, ex := range exchanges {
			if !ex.IsDir() {
				continue
			}

			// Check all source aliases (websocket, ws, v2, restapi, restv2)
			for _, alias := range sourceAliases[src] {
				base := filepath.Join(dataRoot, ex.Name(), alias, cat)
				entries, err := os.ReadDir(base)
				if err != nil {
					continue // Skip if directory doesn't exist
				}

				// Add all subdirectories as symbols
				for _, e := range entries {
					if e.IsDir() {
						syms[e.Name()] = struct{}{}
					}
				}
			}
		}

		// UI更新
		fp.ui(func() {
			if len(syms) == 0 {
				fp.symbolSelect.SetOptions([]string{"no data"})
				fp.symbolSelect.SetSelected("no data")
				fp.symbolSelect.Disable()
			} else {
				list := make([]string, 0, len(syms))
				for s := range syms {
					list = append(list, s)
				}
				sort.Strings(list)

				// Add "ALL" at the beginning for full category search
				list = append([]string{"ALL"}, list...)
				fp.symbolSelect.SetOptions(list)
				fp.symbolSelect.Enable()

				// Restore previous selection or select ALL
				if fp.symbolRemember != "" {
					found := false
					for _, sym := range list {
						if sym == fp.symbolRemember {
							fp.symbolSelect.SetSelected(fp.symbolRemember)
							found = true
							break
						}
					}
					if !found {
						fp.symbolSelect.SetSelected("ALL")
					}
				} else {
					fp.symbolSelect.SetSelected("ALL")
				}
			}
		})
	}()
}

// UpdateFiles updates the files display (for compatibility)
func (fp *FilesPanel) UpdateFiles(filteredFiles []domain.FileItem) {
	fp.state.FilteredFiles = filteredFiles
	fp.Refresh()
}