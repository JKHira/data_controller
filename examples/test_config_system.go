package main

import (
	"log"
	"os"
	"path/filepath"

	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
)

func main() {
	// Setup logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal(err)
	}
	defer logger.Sync()

	// Get base path (project root)
	basePath := filepath.Join(os.Getenv("HOME"), "Trade", "TradeEngine2", "data_controller")
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		basePath = "/Volumes/SSD/AI/Trade/TradeEngine2/data_controller"
	}

	logger.Info("Testing config system", zap.String("base_path", basePath))

	// Create REST fetcher
	restFetcher := config.NewBitfinexRESTFetcher("https://api-pub.bitfinex.com/v2")

	// Create config manager
	configManager := config.NewConfigManager(logger, basePath, restFetcher)

	// Initialize for Bitfinex
	if err := configManager.Initialize("bitfinex"); err != nil {
		logger.Fatal("Failed to initialize config manager", zap.Error(err))
	}

	logger.Info("Config manager initialized successfully")

	// Test: Get exchange config
	exchangeConfig := configManager.GetExchangeConfig()
	logger.Info("Exchange config loaded",
		zap.String("ws_url", exchangeConfig.Endpoints.WSPublic),
		zap.Int("max_subscriptions", exchangeConfig.Limits.WSMaxSubscriptions))

	// Test: Get available pairs
	pairs, err := configManager.GetAvailablePairs("bitfinex", "exchange")
	if err != nil {
		logger.Error("Failed to get pairs", zap.Error(err))
	} else {
		logger.Info("Available pairs loaded", zap.Int("count", len(pairs)))
		if len(pairs) > 0 {
			logger.Info("Sample pairs", zap.Strings("pairs", pairs[:min(10, len(pairs))]))
		}
	}

	// Test: Normalizer
	normalizer := configManager.GetNormalizer()
	testPairs := []string{"tBTCUSD", "tETHUSD", "AVAX:BTC", "fUSD"}

	logger.Info("Testing normalizer...")
	for _, pairStr := range testPairs {
		normalized, err := normalizer.NormalizePair(pairStr)
		if err != nil {
			logger.Error("Normalization failed", zap.String("pair", pairStr), zap.Error(err))
			continue
		}

		logger.Info("Normalized pair",
			zap.String("original", normalized.Original),
			zap.String("internal", normalized.Internal),
			zap.String("base", normalized.Base),
			zap.String("quote", normalized.Quote),
			zap.String("base_full", normalized.BaseFull),
			zap.String("quote_full", normalized.QuoteFull),
			zap.String("market_type", normalized.MarketType))
	}

	// Test: Application state
	logger.Info("Testing application state...")
	appState := configManager.GetApplicationState()

	// Simulate connection
	appState.UpdateConnectionStatus("bitfinex", "conn_1", "connected")

	// Simulate subscription
	sub := &config.SubscriptionState{
		Channel: "ticker",
		Symbol:  "tBTCUSD",
		ChanID:  12345,
	}
	appState.AddSubscription("bitfinex", "conn_1", sub)

	// Get subscription count
	count := appState.GetActiveSubscriptionCount("bitfinex")
	logger.Info("Active subscriptions", zap.Int("count", count))

	// Test: UI state
	uiState := appState.GetUIState("bitfinex")
	logger.Info("UI state loaded",
		zap.String("active_tab", uiState.ActiveTab),
		zap.Bool("checksum", uiState.ConnectionFlags.Checksum))

	// Update UI state
	uiState.ActiveTab = "books"
	uiState.SelectedSymbols = []string{"tBTCUSD", "tETHUSD"}
	appState.UpdateUIState("bitfinex", uiState)

	// Test: Save state
	if err := configManager.SaveState(); err != nil {
		logger.Error("Failed to save state", zap.Error(err))
	} else {
		logger.Info("State saved successfully")
	}

	// Test: File lock
	logger.Info("Testing file lock...")
	lockDir := filepath.Join(basePath, "config", "tmp")
	err = config.WithLock(lockDir, "test_operation", 5, func() error {
		logger.Info("Lock acquired, performing operation...")
		return nil
	})
	if err != nil {
		logger.Error("Lock test failed", zap.Error(err))
	} else {
		logger.Info("Lock test passed")
	}

	// Test: Fetch config on connect (optional - requires network)
	if os.Getenv("TEST_NETWORK") == "1" {
		logger.Info("Testing config refresh on connect...")
		if err := configManager.RefreshConfigOnConnect("bitfinex"); err != nil {
			logger.Error("Refresh failed", zap.Error(err))
		} else {
			logger.Info("Refresh successful")

			// Check updated pairs
			pairs, err := configManager.GetAvailablePairs("bitfinex", "exchange")
			if err == nil {
				logger.Info("Pairs refreshed", zap.Int("count", len(pairs)))
			}
		}
	} else {
		logger.Info("Skipping network test (set TEST_NETWORK=1 to enable)")
	}

	logger.Info("All tests completed")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}