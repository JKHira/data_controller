package state

import (
	"context"
	"sync"
	"time"

	"fyne.io/fyne/v2/data/binding"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/services"
	"github.com/trade-engine/data-controller/internal/sink/arrow"
	"github.com/trade-engine/data-controller/internal/ws"
)

// AppState manages the entire application state
type AppState struct {
	cfg    *config.Config
	logger *zap.Logger
	ctx    context.Context
	cancel context.CancelFunc

	// Core services
	router            *ws.Router
	connectionManager *ws.ConnectionManager
	arrowHandler      *arrow.Handler
	fileScanner       *services.FileScanner
	FileReader        *services.FileReaderService

	// Connection state
	isRunning      bool
	isRunningMutex sync.RWMutex

	// UI bindings
	StatusBinding binding.String
	StatsBinding  binding.String

	// Stream data state
	StreamData        []string
	streamMutex       sync.Mutex
	maxStreamEntries  int
	streamCallbacks   []func()

	// File browser state
	FilesData         []string
	filesMutex        sync.RWMutex
	SelectedFileIndex int
	CurrentFilePath   string

	// File viewer state
	CurrentPage   int
	PageSize      int
	TotalPages    int
	ViewerContent string

	// Filter state
	FilterCriteria services.FileFilter
}

func NewAppState(cfg *config.Config, logger *zap.Logger) *AppState {
	ctx, cancel := context.WithCancel(context.Background())

	statusBinding := binding.NewString()
	statusBinding.Set("🔴 Disconnected")

	statsBinding := binding.NewString()
	statsBinding.Set("No data available")

	return &AppState{
		cfg:               cfg,
		logger:            logger,
		ctx:               ctx,
		cancel:            cancel,
		fileScanner:       services.NewFileScanner(logger, cfg.Storage.BasePath),
		FileReader:        services.NewFileReaderService(logger),
		StatusBinding:     statusBinding,
		StatsBinding:      statsBinding,
		maxStreamEntries:  20,
		PageSize:          100,
		CurrentPage:       1,
	}
}

// Connection management
func (s *AppState) IsRunning() bool {
	s.isRunningMutex.RLock()
	defer s.isRunningMutex.RUnlock()
	return s.isRunning
}

func (s *AppState) SetRunning(running bool) {
	s.isRunningMutex.Lock()
	defer s.isRunningMutex.Unlock()
	s.isRunning = running
}

// Stream data management
func (s *AppState) AddStreamData(data string) {
	s.streamMutex.Lock()
	defer s.streamMutex.Unlock()

	// Add to beginning of slice
	s.StreamData = append([]string{data}, s.StreamData...)

	// Keep only the latest entries
	if len(s.StreamData) > s.maxStreamEntries {
		s.StreamData = s.StreamData[:s.maxStreamEntries]
	}

	// Notify UI callbacks
	for _, callback := range s.streamCallbacks {
		callback()
	}
}

func (s *AppState) RegisterStreamCallback(callback func()) {
	s.streamMutex.Lock()
	defer s.streamMutex.Unlock()
	s.streamCallbacks = append(s.streamCallbacks, callback)
}

// File management
func (s *AppState) UpdateFilesList() error {
	var files []string
	var err error

	if s.FilterCriteria.StartDate.IsZero() && s.FilterCriteria.EndDate.IsZero() &&
		s.FilterCriteria.Channel == "" && s.FilterCriteria.Symbol == "" {
		// No filter applied
		files, err = s.fileScanner.GetAllFiles()
	} else {
		// Apply filter
		files, err = s.fileScanner.GetFilteredFiles(s.FilterCriteria)
	}

	if err != nil {
		return err
	}

	s.filesMutex.Lock()
	s.FilesData = files
	s.filesMutex.Unlock()

	return nil
}

func (s *AppState) GetFilesData() []string {
	s.filesMutex.RLock()
	defer s.filesMutex.RUnlock()
	return append([]string(nil), s.FilesData...) // Return copy
}

func (s *AppState) SetSelectedFile(index int, filePath string) {
	s.SelectedFileIndex = index
	s.CurrentFilePath = filePath
	s.CurrentPage = 1 // Reset to first page
	s.ViewerContent = ""
}

// Filter management
func (s *AppState) SetFilter(startDate, endDate time.Time, channel, symbol string) {
	s.FilterCriteria = services.FileFilter{
		StartDate: startDate,
		EndDate:   endDate,
		Channel:   channel,
		Symbol:    symbol,
	}
}

func (s *AppState) ClearFilter() {
	s.FilterCriteria = services.FileFilter{}
}

// Cleanup
func (s *AppState) Shutdown() {
	s.cancel()
}