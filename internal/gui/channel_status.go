package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
)

// StatusChannelPanel manages status channel configuration
type StatusChannelPanel struct {
	logger        *zap.Logger
	configManager *config.ConfigManager
	exchange      string
	enableCheck   *widget.Check
	typeSelect    *widget.Select
	container     *fyne.Container
	enabled       bool
	statusType    string // "derivatives" or "liquidation"

	onStateChange func()
	limitChecker  func(delta int) bool
	updating      bool
}

func NewStatusChannelPanel(logger *zap.Logger, configManager *config.ConfigManager, exchange string) *StatusChannelPanel {
	return &StatusChannelPanel{
		logger:        logger,
		configManager: configManager,
		exchange:      exchange,
		statusType:    "derivatives",
	}
}

func (p *StatusChannelPanel) SetOnStateChange(fn func()) {
	p.onStateChange = fn
}

func (p *StatusChannelPanel) SetLimitChecker(fn func(delta int) bool) {
	p.limitChecker = fn
}

func (p *StatusChannelPanel) Build() fyne.CanvasObject {
	p.enableCheck = widget.NewCheck("Enable Status Channel", func(checked bool) {
		p.enabled = checked
		if checked {
			p.typeSelect.Enable()
		} else {
			p.typeSelect.Disable()
		}

		if p.updating {
			return
		}

		if checked {
			if p.limitChecker != nil && !p.limitChecker(1) {
				p.updating = true
				p.enableCheck.SetChecked(false)
				p.typeSelect.Disable()
				p.updating = false
				return
			}
		}

		p.persistState()
		p.notifyStateChange()
	})

	p.typeSelect = widget.NewSelect([]string{"derivatives", "liquidation"}, func(value string) {
		p.statusType = value
		if p.updating {
			return
		}
		p.persistState()
	})
	p.typeSelect.SetSelected("derivatives")
	p.typeSelect.Disable()

	infoLabel := widget.NewLabel("Status channel provides derivatives status or liquidation feed.")

	descLabel := widget.NewLabel(`
Derivatives: Provides status updates for derivatives contracts
Liquidation: Provides global liquidation feed (liq:global)

Note: This channel counts as 1 subscription regardless of type.
`)

	configForm := widget.NewForm(
		widget.NewFormItem("Status Type", p.typeSelect),
	)

	p.container = container.NewVBox(
		infoLabel,
		widget.NewSeparator(),
		p.enableCheck,
		configForm,
		descLabel,
	)

	return p.container
}

func (p *StatusChannelPanel) GetSubscriptions() []ChannelSubscription {
	if !p.enabled {
		return []ChannelSubscription{}
	}

	var key string
	if p.statusType == "liquidation" {
		key = "liq:global"
	} else {
		key = "deriv:tBTCF0:USTF0"
	}

	return []ChannelSubscription{
		{
			Channel: "status",
			Key:     key,
		},
	}
}

func (p *StatusChannelPanel) GetSubscriptionCount() int {
	if !p.enabled {
		return 0
	}
	return 1
}

func (p *StatusChannelPanel) LoadState(uiState *config.UIState) {
	if uiState == nil || uiState.ChannelStates == nil {
		return
	}
	if channelState, ok := uiState.ChannelStates["status"].(map[string]interface{}); ok {
		if enabled, ok := channelState["enabled"].(bool); ok {
			p.enabled = enabled
			if p.enableCheck != nil {
				p.updating = true
				p.enableCheck.SetChecked(enabled)
				p.updating = false
				if enabled {
					p.typeSelect.Enable()
				} else {
					p.typeSelect.Disable()
				}
			}
		}
		if statusType, ok := channelState["status_type"].(string); ok {
			p.statusType = statusType
			if p.typeSelect != nil {
				p.updating = true
				p.typeSelect.SetSelected(statusType)
				p.updating = false
			}
		}
	}
}

func (p *StatusChannelPanel) SaveState(uiState *config.UIState) {
	if uiState.ChannelStates == nil {
		uiState.ChannelStates = make(map[string]interface{})
	}
	uiState.ChannelStates["status"] = map[string]interface{}{
		"enabled":     p.enabled,
		"status_type": p.statusType,
	}
}

func (p *StatusChannelPanel) Reset() {
	p.enabled = false
	p.statusType = "derivatives"
	if p.enableCheck != nil {
		p.updating = true
		p.enableCheck.SetChecked(false)
		p.updating = false
		p.typeSelect.Disable()
	}
	if p.typeSelect != nil {
		p.updating = true
		p.typeSelect.SetSelected("derivatives")
		p.updating = false
	}

	p.persistState()
	p.notifyStateChange()
}

func (p *StatusChannelPanel) notifyStateChange() {
	if p.onStateChange != nil {
		p.onStateChange()
	}
}

func (p *StatusChannelPanel) persistState() {
	if p.configManager == nil {
		return
	}
	state := p.configManager.GetApplicationState()
	if state == nil {
		return
	}

	uiState := state.GetUIState(p.exchange)
	p.SaveState(uiState)
	state.UpdateUIState(p.exchange, uiState)
	if err := p.configManager.SaveState(); err != nil {
		p.logger.Warn("failed to persist status channel state", zap.Error(err))
	}
}
