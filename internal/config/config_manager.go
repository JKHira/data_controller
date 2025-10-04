package config

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ConfigManager manages exchange configurations and their lifecycle
type ConfigManager struct {
	logger         *zap.Logger
	basePath       string
	exchangeConfig *BitfinexConfig
	appState       *ApplicationState
	normalizer     *Normalizer
	restClient     RestConfigFetcher
	updateTimers   map[string]*time.Timer
	timerMu        sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
}

// RestConfigFetcher interface for fetching REST config data
type RestConfigFetcher interface {
	FetchConfig(endpoint string) ([]byte, error)
}

// NewConfigManager creates a new configuration manager
func NewConfigManager(logger *zap.Logger, basePath string, restClient RestConfigFetcher) *ConfigManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConfigManager{
		logger:       logger,
		basePath:     basePath,
		restClient:   restClient,
		updateTimers: make(map[string]*time.Timer),
		ctx:          ctx,
		cancel:       cancel,
	}
}

// Initialize loads configuration and state
func (cm *ConfigManager) Initialize(exchange string) error {
	// Load exchange config
	exchangeConfigPath := filepath.Join(cm.basePath, "config", "exchanges", fmt.Sprintf("%s.yml", exchange))

	// Check if config exists, if not create default
	if _, err := os.Stat(exchangeConfigPath); os.IsNotExist(err) {
		return fmt.Errorf("exchange config not found: %s", exchangeConfigPath)
	}

	cfg, err := LoadBitfinexConfig(exchangeConfigPath)
	if err != nil {
		return fmt.Errorf("load exchange config: %w", err)
	}
	cm.exchangeConfig = cfg

	// Load application state
	stateDir := filepath.Join(cm.basePath, "config", "runtime")
	if err := os.MkdirAll(stateDir, 0755); err != nil {
		return fmt.Errorf("create runtime directory: %w", err)
	}

	statePath := filepath.Join(stateDir, "state.yml")
	cm.appState = NewApplicationState(statePath)
	if err := cm.appState.Load(); err != nil {
		cm.logger.Warn("Failed to load state, starting fresh", zap.Error(err))
	}

	// Initialize normalizer
	cm.normalizer = NewNormalizer(make(map[string]string))

	// Load currency labels if available
	if err := cm.loadCurrencyLabels(exchange); err != nil {
		cm.logger.Warn("Failed to load currency labels", zap.Error(err))
	}

	return nil
}

// loadCurrencyLabels loads currency label mappings from cache
func (cm *ConfigManager) loadCurrencyLabels(exchange string) error {
	configDir := filepath.Join(cm.basePath, "data", exchange, "restapi", "config")
	labelFile := filepath.Join(configDir, "map_currency_label.json")

	data, err := os.ReadFile(labelFile)
	if err != nil {
		return err
	}

	var labels [][][2]string
	if err := json.Unmarshal(data, &labels); err != nil {
		return err
	}

	if len(labels) > 0 {
		cm.normalizer.LoadCurrencyLabelsFromMap(labels[0])
	}

	return nil
}

// RefreshConfigOnConnect fetches and updates config data when WebSocket connects
func (cm *ConfigManager) RefreshConfigOnConnect(exchange string) error {
	cm.logger.Info("Refreshing config on WebSocket connect", zap.String("exchange", exchange))

	lockDir := filepath.Join(cm.basePath, "config", "tmp")
	return WithLock(lockDir, "refresh_on_connect", 30*time.Second, func() error {
		// Fetch all configured endpoints
		for _, endpoint := range cm.exchangeConfig.RestConfig {
			if err := cm.fetchAndCacheEndpoint(exchange, endpoint); err != nil {
				cm.logger.Error("Failed to fetch endpoint",
					zap.String("endpoint", endpoint.Endpoint),
					zap.Error(err))
				// Continue with other endpoints
			}
		}

		// Reload currency labels after update
		if err := cm.loadCurrencyLabels(exchange); err != nil {
			cm.logger.Warn("Failed to reload currency labels", zap.Error(err))
		}

		return nil
	})
}

// fetchAndCacheEndpoint fetches a single REST config endpoint and caches it
func (cm *ConfigManager) fetchAndCacheEndpoint(exchange string, endpoint RestConfigEndpoint) error {
	// Check if update is needed
	exState := cm.appState.GetExchangeState(exchange)
	if exState.RestConfigCache != nil {
		if lastUpdate, ok := exState.RestConfigCache.LastUpdated[endpoint.Endpoint]; ok {
			timeSinceUpdate := time.Since(lastUpdate)
			if timeSinceUpdate < time.Duration(endpoint.CacheDuration)*time.Second {
				cm.logger.Debug("Skipping fetch, cache still valid",
					zap.String("endpoint", endpoint.Endpoint),
					zap.Duration("age", timeSinceUpdate))
				return nil
			}
		}
	}

	// Fetch data
	cm.logger.Info("Fetching REST config", zap.String("endpoint", endpoint.Endpoint))
	data, err := cm.restClient.FetchConfig(endpoint.Endpoint)
	if err != nil {
		return fmt.Errorf("fetch config: %w", err)
	}

	// Save to cache file
	configDir := filepath.Join(cm.basePath, "data", exchange, "restapi", "config")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	cacheFile := filepath.Join(configDir, endpoint.File)
	if err := os.WriteFile(cacheFile, data, 0644); err != nil {
		return fmt.Errorf("write cache file: %w", err)
	}

	// Update state
	now := time.Now()
	nextUpdate := now.Add(time.Duration(endpoint.CacheDuration) * time.Second)
	cm.appState.UpdateRestConfigCache(exchange, endpoint.Endpoint, now, nextUpdate)

	// Schedule next update
	cm.scheduleUpdate(exchange, endpoint, nextUpdate)

	cm.logger.Info("REST config cached",
		zap.String("endpoint", endpoint.Endpoint),
		zap.String("file", cacheFile),
		zap.Time("next_update", nextUpdate))

	return nil
}

// scheduleUpdate schedules a timer for the next config update
func (cm *ConfigManager) scheduleUpdate(exchange string, endpoint RestConfigEndpoint, nextUpdate time.Time) {
	cm.timerMu.Lock()
	defer cm.timerMu.Unlock()

	timerKey := fmt.Sprintf("%s:%s", exchange, endpoint.Endpoint)

	// Cancel existing timer if any
	if timer, exists := cm.updateTimers[timerKey]; exists {
		timer.Stop()
	}

	// Calculate duration until next update
	duration := time.Until(nextUpdate)
	if duration < 0 {
		duration = 0
	}

	// Create new timer
	timer := time.AfterFunc(duration, func() {
		select {
		case <-cm.ctx.Done():
			return
		default:
			cm.logger.Info("Scheduled config update triggered",
				zap.String("exchange", exchange),
				zap.String("endpoint", endpoint.Endpoint))

			if err := cm.fetchAndCacheEndpoint(exchange, endpoint); err != nil {
				cm.logger.Error("Scheduled update failed",
					zap.String("endpoint", endpoint.Endpoint),
					zap.Error(err))
			}
		}
	})

	cm.updateTimers[timerKey] = timer
}

// StartPeriodicUpdates starts periodic updates for all configured endpoints
func (cm *ConfigManager) StartPeriodicUpdates(exchange string) {
	for _, endpoint := range cm.exchangeConfig.RestConfig {
		// Check when next update should occur
		exState := cm.appState.GetExchangeState(exchange)
		var nextUpdate time.Time

		if exState.RestConfigCache != nil {
			if nu, ok := exState.RestConfigCache.NextUpdate[endpoint.Endpoint]; ok {
				nextUpdate = nu
			}
		}

		if nextUpdate.IsZero() {
			// Never updated, schedule immediately
			nextUpdate = time.Now()
		}

		cm.scheduleUpdate(exchange, endpoint, nextUpdate)
	}
}

// StopPeriodicUpdates stops all periodic update timers
func (cm *ConfigManager) StopPeriodicUpdates() {
	cm.timerMu.Lock()
	defer cm.timerMu.Unlock()

	for key, timer := range cm.updateTimers {
		timer.Stop()
		delete(cm.updateTimers, key)
	}
}

// GetNormalizer returns the normalizer instance
func (cm *ConfigManager) GetNormalizer() *Normalizer {
	return cm.normalizer
}

// GetExchangeConfig returns the exchange configuration
func (cm *ConfigManager) GetExchangeConfig() *BitfinexConfig {
	return cm.exchangeConfig
}

// GetApplicationState returns the application state
func (cm *ConfigManager) GetApplicationState() *ApplicationState {
	return cm.appState
}

// SaveState saves the current application state to disk
func (cm *ConfigManager) SaveState() error {
	return cm.appState.Save()
}

// Shutdown gracefully shuts down the config manager
func (cm *ConfigManager) Shutdown() error {
	cm.cancel()
	cm.StopPeriodicUpdates()
	return cm.SaveState()
}

// GetAvailablePairs returns all available trading pairs from cache
func (cm *ConfigManager) GetAvailablePairs(exchange, pairType string) ([]string, error) {
	var filename string
	switch pairType {
	case "exchange", "spot":
		filename = "list_pair_exchange.json"
	case "margin":
		filename = "list_pair_margin.json"
	case "futures":
		filename = "list_pair_futures.json"
	default:
		return nil, fmt.Errorf("unknown pair type: %s", pairType)
	}

	configDir := filepath.Join(cm.basePath, "data", exchange, "restapi", "config")
	filePath := filepath.Join(configDir, filename)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read pairs file: %w", err)
	}

	var pairs [][]string
	if err := json.Unmarshal(data, &pairs); err != nil {
		return nil, fmt.Errorf("unmarshal pairs: %w", err)
	}

	if len(pairs) == 0 {
		return []string{}, nil
	}

	return pairs[0], nil
}
