package gui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// SymbolSearchSelector provides a searchable multi-select symbol list
type SymbolSearchSelector struct {
	symbols      []string
	filteredSyms []string
	selected     map[string]bool
	onChanged    func([]string)

	searchEntry   *widget.Entry
	checkGroup    *widget.CheckGroup
	scrollContent *container.Scroll
	container     *fyne.Container
}

// NewSymbolSearchSelector creates a new symbol search selector with 300px height
func NewSymbolSearchSelector(symbols []string, onChange func([]string)) *SymbolSearchSelector {
	s := &SymbolSearchSelector{
		symbols:      symbols,
		filteredSyms: make([]string, len(symbols)),
		selected:     make(map[string]bool),
		onChanged:    onChange,
	}
	copy(s.filteredSyms, symbols)
	s.build()
	return s
}

// build creates the UI components
func (s *SymbolSearchSelector) build() {
	// Search entry at top
	s.searchEntry = widget.NewEntry()
	s.searchEntry.SetPlaceHolder("Search symbols... (e.g., BTC, ETH)")
	s.searchEntry.OnChanged = func(query string) {
		s.filterSymbols(query)
	}

	// CheckGroup for symbol selection
	s.checkGroup = widget.NewCheckGroup(s.filteredSyms, func(checked []string) {
		// Update selected map
		s.selected = make(map[string]bool)
		for _, sym := range checked {
			s.selected[sym] = true
		}

		if s.onChanged != nil {
			s.onChanged(checked)
		}
	})

	// Scrollable container with 300px height
	s.scrollContent = container.NewVScroll(s.checkGroup)
	s.scrollContent.SetMinSize(fyne.NewSize(0, 300))

	// Select All / Deselect All buttons
	selectAllBtn := widget.NewButton("Select All", func() {
		s.selectAll()
	})

	deselectAllBtn := widget.NewButton("Deselect All", func() {
		s.deselectAll()
	})

	btnContainer := container.NewHBox(
		selectAllBtn,
		deselectAllBtn,
	)

	// Layout: [Search] [Select/Deselect buttons] [Scrollable CheckGroup]
	s.container = container.NewBorder(
		container.NewVBox(s.searchEntry, btnContainer),
		nil,
		nil, nil,
		s.scrollContent,
	)
}

// Build returns the container for embedding in parent layouts
func (s *SymbolSearchSelector) Build() fyne.CanvasObject {
	return s.container
}

// filterSymbols filters the symbol list based on search query
func (s *SymbolSearchSelector) filterSymbols(query string) {
	query = strings.ToUpper(strings.TrimSpace(query))

	if query == "" {
		// Show first 100 symbols when no search query
		limit := 100
		if len(s.symbols) < limit {
			limit = len(s.symbols)
		}
		s.filteredSyms = make([]string, limit)
		copy(s.filteredSyms, s.symbols[:limit])
	} else {
		// Filter symbols containing query (show up to 100 matches)
		s.filteredSyms = []string{}
		for _, sym := range s.symbols {
			if strings.Contains(strings.ToUpper(sym), query) {
				s.filteredSyms = append(s.filteredSyms, sym)
				if len(s.filteredSyms) >= 100 {
					break
				}
			}
		}
	}

	// Update CheckGroup options
	s.checkGroup.Options = s.filteredSyms

	// Restore selected state for visible items
	selectedVisible := []string{}
	for _, sym := range s.filteredSyms {
		if s.selected[sym] {
			selectedVisible = append(selectedVisible, sym)
		}
	}
	s.checkGroup.SetSelected(selectedVisible)

	s.checkGroup.Refresh()
}

// selectAll selects all currently visible symbols
func (s *SymbolSearchSelector) selectAll() {
	for _, sym := range s.filteredSyms {
		s.selected[sym] = true
	}
	s.checkGroup.SetSelected(s.filteredSyms)

	if s.onChanged != nil {
		s.onChanged(s.GetSelected())
	}
}

// deselectAll deselects all symbols
func (s *SymbolSearchSelector) deselectAll() {
	s.selected = make(map[string]bool)
	s.checkGroup.SetSelected([]string{})

	if s.onChanged != nil {
		s.onChanged([]string{})
	}
}

// GetSelected returns all selected symbols (including those filtered out)
func (s *SymbolSearchSelector) GetSelected() []string {
	selected := []string{}
	for sym := range s.selected {
		selected = append(selected, sym)
	}
	return selected
}

// SetSelected sets the selected symbols
func (s *SymbolSearchSelector) SetSelected(symbols []string) {
	s.selected = make(map[string]bool)
	for _, sym := range symbols {
		s.selected[sym] = true
	}

	// Update visible checkboxes
	selectedVisible := []string{}
	for _, sym := range s.filteredSyms {
		if s.selected[sym] {
			selectedVisible = append(selectedVisible, sym)
		}
	}
	s.checkGroup.SetSelected(selectedVisible)
}

// SetSymbols updates the available symbols list
func (s *SymbolSearchSelector) SetSymbols(symbols []string) {
	s.symbols = symbols
	if s.searchEntry != nil {
		s.filterSymbols(s.searchEntry.Text)
	} else {
		s.filteredSyms = make([]string, len(symbols))
		copy(s.filteredSyms, symbols)
	}
}

// GetSymbolCount returns total and selected symbol counts
func (s *SymbolSearchSelector) GetSymbolCount() (total, selected int) {
	return len(s.symbols), len(s.selected)
}
