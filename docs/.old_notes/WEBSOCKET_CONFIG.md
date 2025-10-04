# WebSocket Configuration System

## Overview

The WebSocket configuration system provides a comprehensive, state-managed interface for configuring and controlling WebSocket connections to cryptocurrency exchanges with the following features:

- **Tabbed channel configuration** (Ticker, Trades, Books, Candles, Status)
- **30-channel subscription limit** with real-time counter
- **Automatic config refresh** on connection
- **State persistence** across sessions
- **REST API integration** for dynamic symbol lists
- **Currency normalization** for consistent internal representation

## Directory Structure

```
config/
├── exchanges/                      # Exchange-specific configurations
│   └── bitfinex_config.yml        # Bitfinex endpoints, limits, defaults
├── profiles/                       # User-defined connection profiles (optional)
├── state/                          # Runtime state files
│   └── state.yml                  # Application state (auto-managed)
├── backups/                        # Automatic config backups
└── tmp/                            # Temporary files (locks, etc.)
```

## Core Components

### 1. ConfigManager (`internal/config/config_manager.go`)

Central manager for configuration lifecycle:

```go
// Initialize
configManager := config.NewConfigManager(logger, basePath, restClient)
err := configManager.Initialize("bitfinex")

// Refresh config on WebSocket connect
err = configManager.RefreshConfigOnConnect("bitfinex")

// Start periodic updates
configManager.StartPeriodicUpdates("bitfinex")

// Get available pairs
pairs, err := configManager.GetAvailablePairs("bitfinex", "exchange")

// Shutdown gracefully
err = configManager.Shutdown()
```

### 2. Normalizer (`internal/config/normalizer.go`)

Converts exchange-specific formats to internal representation:

```go
normalizer := configManager.GetNormalizer()

// Normalize pair
normalized, err := normalizer.NormalizePair("tBTCUSD")
// Result: Base="BTC", Quote="USD", Internal="BTC-USD"

// Denormalize back to exchange format
original, err := normalizer.DenormalizePair("BTC-USD", "bitfinex")
// Result: "tBTCUSD"

// Get full currency name
fullName := normalizer.GetCurrencyFullName("BTC")
// Result: "Bitcoin"
```

### 3. ApplicationState (`internal/config/state.go`)

Thread-safe state management:

```go
appState := configManager.GetApplicationState()

// Connection status
appState.UpdateConnectionStatus("bitfinex", "conn_1", "connected")

// Subscriptions
sub := &config.SubscriptionState{
    Channel: "ticker",
    Symbol:  "tBTCUSD",
    ChanID:  12345,
}
appState.AddSubscription("bitfinex", "conn_1", sub)

// Subscription count
count := appState.GetActiveSubscriptionCount("bitfinex")

// UI state
uiState := appState.GetUIState("bitfinex")
appState.UpdateUIState("bitfinex", uiState)

// Save to disk
err := appState.Save()
```

### 4. FileLock (`internal/config/file_lock.go`)

Prevents concurrent config modifications:

```go
lockDir := filepath.Join(basePath, "config", "tmp")
err := config.WithLock(lockDir, "update_operation", 30*time.Second, func() error {
    // Perform atomic config update
    return updateConfig()
})
```

## GUI Components

### WebSocketPanel (`internal/gui/websocket_panel.go`)

Main panel with tabbed channel interface:

```go
panel := NewWebSocketPanel(logger, configManager, "bitfinex")

// Set connection callbacks
panel.SetConnectCallback(func(wsConfig *WSConnectionConfig) error {
    // Handle connection
    return wsConnect(wsConfig)
})

panel.SetDisconnectCallback(func() error {
    // Handle disconnection
    return wsDisconnect()
})

// Build UI
ui := panel.Build()
```

### Channel Panels

Each channel type has its own panel:

- **TickerChannelPanel** - Symbol selection for ticker data
- **TradesChannelPanel** - Symbol selection for trade data
- **BooksChannelPanel** - Order book config (precision, frequency, length)
- **CandlesChannelPanel** - OHLC config (timeframe, symbols)
- **StatusChannelPanel** - Derivatives/liquidation feed

## Configuration Files

### Exchange Config (`config/exchanges/bitfinex_config.yml`)

```yaml
endpoints:
  ws_public: "wss://api-pub.bitfinex.com/ws/2"
  ws_auth: "wss://api.bitfinex.com/ws/2"
  rest_public: "https://api-pub.bitfinex.com/v2"

limits:
  ws_connections_per_minute: 20
  ws_max_subscriptions: 30
  rest_rate_limit: 90

defaults:
  book:
    prec: "P0"
    freq: "F0"
    len: "25"
  candles:
    timeframe: "1m"

rest_config_endpoints:
  - endpoint: "pub:list:pair:exchange"
    cache_duration: 3600  # 1 hour
    file: "list_pair_exchange.json"
  - endpoint: "pub:map:currency:label"
    cache_duration: 86400  # 24 hours
    file: "map_currency_label.json"
```

### State File (`config/state/state.yml`)

Auto-managed runtime state:

```yaml
exchanges:
  bitfinex:
    ws:
      connections:
        - id: "conn_1"
          status: "connected"
          connected_at: "2025-09-30T10:30:00Z"
          subscriptions:
            - channel: "ticker"
              symbol: "tBTCUSD"
              chanId: 12345
      ui_state:
        active_tab: "books"
        selected_symbols: ["tBTCUSD", "tETHUSD"]
        connection_flags:
          checksum: true
          bulk: false
          timestamp: true
          sequence: false
    rest_config_cache:
      last_updated:
        "pub:list:pair:exchange": "2025-09-30T10:00:00Z"
      next_update:
        "pub:list:pair:exchange": "2025-09-30T11:00:00Z"
```

## Features

### 1. Subscription Limit Counter

- Real-time counter showing current/max subscriptions
- Warning when approaching limit (25+/30)
- Prevents exceeding 30-channel limit
- Visual indicators in UI

### 2. Automatic Config Refresh

**On WebSocket Connect:**
```go
// Triggered automatically when Connect button is pressed
configManager.RefreshConfigOnConnect("bitfinex")
```

**Periodic Updates:**
```go
// Started automatically after initialization
// Updates based on cache_duration in config
configManager.StartPeriodicUpdates("bitfinex")
```

### 3. State Persistence

- UI state saved on tab changes
- Restores selected symbols on restart
- Preserves connection flags
- Maintains channel configurations

### 4. Currency Normalization

Consistent internal representation:

- `tBTCUSD` → `BTC-USD` (internal)
- `AVAX:BTC` → `AVAX-BTC` (internal)
- `fUSD` → `USD-USD` (funding)

## Usage Example

```go
package main

import (
    "github.com/trade-engine/data-controller/internal/config"
    "github.com/trade-engine/data-controller/internal/gui"
    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewDevelopment()
    basePath := "/path/to/data_controller"

    // Create REST fetcher
    restFetcher := config.NewBitfinexRESTFetcher("https://api-pub.bitfinex.com/v2")

    // Create config manager
    configManager := config.NewConfigManager(logger, basePath, restFetcher)
    configManager.Initialize("bitfinex")

    // Create WebSocket panel
    wsPanel := gui.NewWebSocketPanel(logger, configManager, "bitfinex")

    // Set callbacks
    wsPanel.SetConnectCallback(func(wsConfig *gui.WSConnectionConfig) error {
        logger.Info("Connecting WebSocket",
            zap.Strings("symbols", wsConfig.Symbols),
            zap.Int("channels", len(wsConfig.Channels)))

        // Trigger config refresh
        go configManager.RefreshConfigOnConnect("bitfinex")

        // Actual connection logic here
        return nil
    })

    // Start periodic updates
    configManager.StartPeriodicUpdates("bitfinex")

    // Build UI
    ui := wsPanel.Build()

    // ... show UI in Fyne window
}
```

## REST API Integration

### Fetched Config Data

Files cached in `data/<exchange>/restapi/config/`:

- `list_pair_exchange.json` - Spot trading pairs
- `list_pair_margin.json` - Margin trading pairs
- `list_pair_futures.json` - Futures contracts
- `map_currency_label.json` - Currency full names
- `map_currency_sym.json` - Currency symbols
- And more...

### Update Schedule

Configured in `exchange_config.yml`:

- **Pairs**: 1 hour cache (updated hourly)
- **Currency maps**: 24 hour cache (updated daily)
- **Custom**: Configurable per endpoint

## Error Handling

### Connection Errors

```go
wsPanel.SetConnectCallback(func(wsConfig *WSConnectionConfig) error {
    if err := validateConfig(wsConfig); err != nil {
        return fmt.Errorf("invalid config: %w", err)
    }

    if err := connect(wsConfig); err != nil {
        return fmt.Errorf("connection failed: %w", err)
    }

    return nil
})
```

### Lock Timeouts

```go
err := config.WithLock(lockDir, "operation", 30*time.Second, func() error {
    // If this takes >30s, lock will timeout
    return longOperation()
})
if err != nil {
    logger.Error("Lock timeout", zap.Error(err))
}
```

### Config Fetch Failures

Config manager handles fetch failures gracefully:
- Uses cached data if available
- Logs errors but continues
- Retries on next scheduled update

## Testing

Run the test program:

```bash
# Basic test (no network)
go run examples/test_config_system.go

# With network test
TEST_NETWORK=1 go run examples/test_config_system.go
```

## Thread Safety

All components are thread-safe:

- **ConfigManager**: Internal mutexes for timers
- **ApplicationState**: RWMutex for all operations
- **FileLock**: Flock-based file locking
- **GUI panels**: Fyne thread-safe updates via `fyne.Do()`

## Performance Considerations

- **Symbol lists**: Limited to 500 pairs per exchange (UI performance)
- **Search**: Incremental filtering (max 100 displayed)
- **State saves**: Debounced writes (on demand)
- **Config fetches**: Rate-limited per exchange rules

## Migration from Old System

Old files automatically moved to new structure:
- `config.yml` → `config/config.yml`
- `bitfinex_config.yml` → `config/exchanges/bitfinex_config.yml`
- `state.yml` → `config/state/state.yml`

State files are backward compatible and auto-migrated on first load.