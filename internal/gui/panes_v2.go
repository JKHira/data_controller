package gui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/services"
)

// BuildExchangePanesV2 constructs the exchange panes using the new WebSocketPanel
func BuildExchangePanesV2(
	cfg *config.Config,
	configManager *config.ConfigManager,
	wsConnect func(*WSConnectionConfig) error,
	wsDisconnect func() error,
	refreshManager *services.ConfigRefreshManager,
	statusCallback func(string),
	logger *zap.Logger,
) (fyne.CanvasObject, fyne.CanvasObject) {
	// Colors: disconnected = 柿色, connected = パントーングリーン
	orange := color.RGBA{R: 161, G: 93, B: 55, A: 255} // disconnected
	green := color.RGBA{R: 65, G: 204, B: 102, A: 255} // connected

	// --- Websocket pane with new tabbed interface ---
	wsToggle := NewToggleButton("Websocket Disconnected", "Websocket Connected", orange, green)
	wsToggle.SetInteractive(false)

	// Create WebSocket panel for each exchange
	bitfinexWSPanel := NewWebSocketPanel(logger, configManager, "bitfinex")

	// Set connection callbacks
	bitfinexWSPanel.SetConnectCallback(func(wsConfig *WSConnectionConfig) error {
		if wsConnect != nil {
			if err := wsConnect(wsConfig); err != nil {
				return err
			}
		}

		// Update toggle
		wsToggle.Set(true)
		wsToggle.SetLabels("Websocket Disconnected", "Bitfinex Websocket Connected")

		// Trigger config refresh on connect
		if configManager != nil {
			go func() {
				if err := configManager.RefreshConfigOnConnect("bitfinex"); err != nil {
					logger.Error("Failed to refresh config on connect", zap.Error(err))
				} else {
					logger.Info("Config refreshed on WebSocket connect")
				}
			}()
		}

		return nil
	})

	bitfinexWSPanel.SetDisconnectCallback(func() error {
		if wsDisconnect != nil {
			if err := wsDisconnect(); err != nil {
				return err
			}
		}

		// Update toggle
		wsToggle.Set(false)

		return nil
	})

	// Create tabs for multiple exchanges
	wsTabs := container.NewAppTabs(
		container.NewTabItem("Bitfinex", bitfinexWSPanel.Build()),
		container.NewTabItem("Binance", widget.NewLabel("Coming soon")),
		container.NewTabItem("Coinbase", widget.NewLabel("Coming soon")),
		container.NewTabItem("Kraken", widget.NewLabel("Coming soon")),
	)
	wsTabs.SetTabLocation(container.TabLocationTop)

	// Top border with toggle spanning full width
	wsTop := container.NewBorder(nil, nil, nil, nil, wsToggle)

	wsPane := container.NewBorder(
		wsTop,
		nil, nil, nil,
		container.NewMax(wsTabs),
	)

	// --- REST API pane ---
	restToggle := NewToggleButton("REST API Disconnected", "REST API Connected", orange, green)
	restToggle.OnChanged = func(connected bool) {
		if logger != nil {
			logger.Info("REST API toggle changed", zap.Bool("connected", connected))
		} else {
			fmt.Printf("REST API toggle changed: %v\n", connected)
		}
	}

	restAPIPanel := NewRestAPIPanel(logger, cfg, refreshManager, statusCallback)

	restTabs := container.NewAppTabs(
		container.NewTabItem("Bitfinex", restAPIPanel.CreateBitfinexConfigPanel()),
		container.NewTabItem("Binance", widget.NewLabel("Coming soon")),
		container.NewTabItem("Coinbase", widget.NewLabel("Coming soon")),
		container.NewTabItem("Kraken", widget.NewLabel("Coming soon")),
	)
	restTabs.SetTabLocation(container.TabLocationTop)

	restTop := container.NewBorder(nil, nil, nil, nil, restToggle)

	restPane := container.NewBorder(
		restTop,
		nil, nil, nil,
		container.NewMax(restTabs),
	)

	return wsPane, restPane
}