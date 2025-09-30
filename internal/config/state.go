package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ApplicationState represents the runtime state of the application
type ApplicationState struct {
	mu        sync.RWMutex
	filePath  string
	Exchanges map[string]*ExchangeState `yaml:"exchanges"`
}

// ExchangeState holds state for a single exchange
type ExchangeState struct {
	WS              *WSState              `yaml:"ws,omitempty"`
	RestConfigCache *RestConfigCacheState `yaml:"rest_config_cache,omitempty"`
}

// WSState holds WebSocket connection state
type WSState struct {
	Connections []*ConnectionState `yaml:"connections,omitempty"`
	UIState     *UIState           `yaml:"ui_state,omitempty"`
}

// ConnectionState represents a single WebSocket connection
type ConnectionState struct {
	ID            string                `yaml:"id"`
	Status        string                `yaml:"status"` // "connected", "disconnected", "reconnecting"
	ConnectedAt   time.Time             `yaml:"connected_at,omitempty"`
	DisconnectedAt time.Time            `yaml:"disconnected_at,omitempty"`
	Subscriptions []*SubscriptionState  `yaml:"subscriptions,omitempty"`
}

// SubscriptionState represents a channel subscription
type SubscriptionState struct {
	Channel  string  `yaml:"channel"`
	Symbol   string  `yaml:"symbol"`
	Prec     string  `yaml:"prec,omitempty"`
	Freq     string  `yaml:"freq,omitempty"`
	Len      string  `yaml:"len,omitempty"`
	Key      string  `yaml:"key,omitempty"` // For candles
	ChanID   int32   `yaml:"chanId,omitempty"`
	SubID    *int64  `yaml:"subId,omitempty"`
}

// UIState holds UI-specific state
type UIState struct {
	ActiveTab         string            `yaml:"active_tab"`
	SelectedSymbols   []string          `yaml:"selected_symbols,omitempty"`
	ConnectionFlags   ConnectionFlags   `yaml:"connection_flags,omitempty"`
	ChannelStates     map[string]interface{} `yaml:"channel_states,omitempty"`
}

// ConnectionFlags holds WebSocket configuration flags
type ConnectionFlags struct {
	Checksum  bool `yaml:"checksum"`
	Bulk      bool `yaml:"bulk"`
	Timestamp bool `yaml:"timestamp"`
	Sequence  bool `yaml:"sequence"`
}

// RestConfigCacheState holds REST API config cache information
type RestConfigCacheState struct {
	LastUpdated map[string]time.Time `yaml:"last_updated,omitempty"`
	NextUpdate  map[string]time.Time `yaml:"next_update,omitempty"`
}

// NewApplicationState creates a new application state
func NewApplicationState(filePath string) *ApplicationState {
	return &ApplicationState{
		filePath:  filePath,
		Exchanges: make(map[string]*ExchangeState),
	}
}

// Load loads the state from disk
func (s *ApplicationState) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if file exists
	if _, err := os.Stat(s.filePath); os.IsNotExist(err) {
		// Initialize with empty state
		return nil
	}

	bytes, err := os.ReadFile(s.filePath)
	if err != nil {
		return fmt.Errorf("read state file: %w", err)
	}

	if err := yaml.Unmarshal(bytes, s); err != nil {
		return fmt.Errorf("unmarshal state: %w", err)
	}

	return nil
}

// Save saves the state to disk
func (s *ApplicationState) Save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Ensure directory exists
	dir := filepath.Dir(s.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}

	bytes, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}

	if err := os.WriteFile(s.filePath, bytes, 0644); err != nil {
		return fmt.Errorf("write state file: %w", err)
	}

	return nil
}

// GetExchangeState returns state for an exchange, creating if needed
func (s *ApplicationState) GetExchangeState(exchange string) *ExchangeState {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.getExchangeStateLocked(exchange)
}

// getExchangeStateLocked returns state without locking (must be called with lock held)
func (s *ApplicationState) getExchangeStateLocked(exchange string) *ExchangeState {
	if s.Exchanges == nil {
		s.Exchanges = make(map[string]*ExchangeState)
	}

	if _, exists := s.Exchanges[exchange]; !exists {
		s.Exchanges[exchange] = &ExchangeState{
			WS: &WSState{
				Connections: []*ConnectionState{},
				UIState: &UIState{
					ActiveTab:       "ticker",
					SelectedSymbols: []string{},
					ConnectionFlags: ConnectionFlags{
						Checksum:  true,
						Bulk:      false,
						Timestamp: true,
						Sequence:  false,
					},
					ChannelStates: make(map[string]interface{}),
				},
			},
			RestConfigCache: &RestConfigCacheState{
				LastUpdated: make(map[string]time.Time),
				NextUpdate:  make(map[string]time.Time),
			},
		}
	}

	return s.Exchanges[exchange]
}

// UpdateConnectionStatus updates the status of a connection
func (s *ApplicationState) UpdateConnectionStatus(exchange, connID, status string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	exState := s.getExchangeStateLocked(exchange)
	if exState.WS == nil {
		return
	}

	for _, conn := range exState.WS.Connections {
		if conn.ID == connID {
			conn.Status = status
			if status == "connected" {
				conn.ConnectedAt = time.Now()
			} else if status == "disconnected" {
				conn.DisconnectedAt = time.Now()
			}
			return
		}
	}

	// Connection not found, create new one
	conn := &ConnectionState{
		ID:            connID,
		Status:        status,
		Subscriptions: []*SubscriptionState{},
	}
	if status == "connected" {
		conn.ConnectedAt = time.Now()
	}
	exState.WS.Connections = append(exState.WS.Connections, conn)
}

// AddSubscription adds a subscription to a connection
func (s *ApplicationState) AddSubscription(exchange, connID string, sub *SubscriptionState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	exState := s.getExchangeStateLocked(exchange)
	if exState.WS == nil {
		return
	}

	for _, conn := range exState.WS.Connections {
		if conn.ID == connID {
			conn.Subscriptions = append(conn.Subscriptions, sub)
			return
		}
	}
}

// GetActiveSubscriptionCount returns the total number of active subscriptions
func (s *ApplicationState) GetActiveSubscriptionCount(exchange string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exState, exists := s.Exchanges[exchange]
	if !exists || exState.WS == nil {
		return 0
	}

	count := 0
	for _, conn := range exState.WS.Connections {
		if conn.Status == "connected" {
			count += len(conn.Subscriptions)
		}
	}

	return count
}

// UpdateUIState updates the UI state for an exchange
func (s *ApplicationState) UpdateUIState(exchange string, uiState *UIState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	exState := s.getExchangeStateLocked(exchange)
	if exState.WS != nil {
		exState.WS.UIState = uiState
	}
}

// GetUIState returns the UI state for an exchange
func (s *ApplicationState) GetUIState(exchange string) *UIState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	exState, exists := s.Exchanges[exchange]
	if !exists || exState.WS == nil || exState.WS.UIState == nil {
		return &UIState{
			ActiveTab:       "ticker",
			SelectedSymbols: []string{},
			ConnectionFlags: ConnectionFlags{
				Checksum:  true,
				Bulk:      false,
				Timestamp: true,
				Sequence:  false,
			},
			ChannelStates: make(map[string]interface{}),
		}
	}

	return exState.WS.UIState
}

// UpdateRestConfigCache updates REST config cache timestamps
func (s *ApplicationState) UpdateRestConfigCache(exchange, endpoint string, lastUpdated, nextUpdate time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()

	exState := s.getExchangeStateLocked(exchange)
	if exState.RestConfigCache == nil {
		exState.RestConfigCache = &RestConfigCacheState{
			LastUpdated: make(map[string]time.Time),
			NextUpdate:  make(map[string]time.Time),
		}
	}

	exState.RestConfigCache.LastUpdated[endpoint] = lastUpdated
	exState.RestConfigCache.NextUpdate[endpoint] = nextUpdate
}