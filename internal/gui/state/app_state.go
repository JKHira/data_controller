package state

import (
	"fmt"

	"fyne.io/fyne/v2/data/binding"
	"github.com/trade-engine/data-controller/internal/domain"
)

// AppState holds the shared application state
type AppState struct {
	// Data bindings
	StatusBinding binding.String
	StatsBinding  binding.String
	ConfigStatusBinding binding.String

	// File browser state
	FilesData         []domain.FileItem
	FilteredFiles     []domain.FileItem
	SelectedFileIndex int

	// File viewer state
	CurrentFilePath string
	CurrentPage     int
	TotalPages      int
	PageSize        int

	// Connection state
	IsConnected bool
}

// NewAppState creates a new application state
func NewAppState() *AppState {
	return &AppState{
		StatusBinding:     binding.NewString(),
		StatsBinding:      binding.NewString(),
		ConfigStatusBinding: binding.NewString(),
		FilesData:         make([]domain.FileItem, 0),
		FilteredFiles:     make([]domain.FileItem, 0),
		SelectedFileIndex: -1,
		CurrentPage:       1,
		PageSize:          3000,
		IsConnected:       false,
	}
}

// SetConnected updates the connection state
func (s *AppState) SetConnected(connected bool) {
	s.IsConnected = connected
}

// SetCurrentFile updates the current file and resets pagination
func (s *AppState) SetCurrentFile(filePath string) {
	s.CurrentFilePath = filePath
	s.CurrentPage = 1
}

// SetPageInfo updates pagination information
func (s *AppState) SetPageInfo(current, total int) {
	s.CurrentPage = current
	s.TotalPages = total
}

// GetCurrentPageLabel returns formatted page label
func (s *AppState) GetCurrentPageLabel() string {
	if s.TotalPages == 0 {
		return "Page 0/0"
	}
	return fmt.Sprintf("Page %d/%d", s.CurrentPage, s.TotalPages)
}

// CanNavigatePrevious returns true if previous page navigation is possible
func (s *AppState) CanNavigatePrevious() bool {
	return s.CurrentPage > 1
}

// CanNavigateNext returns true if next page navigation is possible
func (s *AppState) CanNavigateNext() bool {
	return s.CurrentPage < s.TotalPages
}
