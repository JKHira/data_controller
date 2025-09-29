package gui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/services"
)

// ToggleButton: シンプルなトグル風ボタン（背景色とテキストを切替）
type ToggleButton struct {
	widget.BaseWidget
	connected   bool
	offText     string
	onText      string
	offColor    color.Color
	onColor     color.Color
	interactive bool
	OnChanged   func(bool)
}

func NewToggleButton(offText, onText string, offColor, onColor color.Color) *ToggleButton {
	tb := &ToggleButton{
		connected:   false,
		offText:     offText,
		onText:      onText,
		offColor:    offColor,
		onColor:     onColor,
		interactive: true,
	}
	tb.ExtendBaseWidget(tb)
	return tb
}

type toggleRenderer struct {
	tb   *ToggleButton
	rect *canvas.Rectangle
	txt  *canvas.Text
	obj  *fyne.Container
}

func (r *toggleRenderer) Layout(size fyne.Size) {
	r.obj.Resize(size)
	r.rect.Resize(size)
	r.txt.Move(fyne.NewPos(12, (size.Height-r.txt.MinSize().Height)/2))
}

func (r *toggleRenderer) MinSize() fyne.Size {
	return r.obj.MinSize()
}

func (r *toggleRenderer) Refresh() {
	if r.tb.connected {
		r.rect.FillColor = r.tb.onColor
		r.txt.Text = r.tb.onText
	} else {
		r.rect.FillColor = r.tb.offColor
		r.txt.Text = r.tb.offText
	}
	r.rect.Refresh()
	r.txt.Refresh()
}

func (r *toggleRenderer) BackgroundColor() color.Color { return color.Transparent }
func (r *toggleRenderer) Objects() []fyne.CanvasObject {
	return []fyne.CanvasObject{r.rect, r.txt}
}
func (r *toggleRenderer) Destroy() {}

func (tb *ToggleButton) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(tb.offColor)
	txt := canvas.NewText(tb.offText, color.White)
	txt.TextSize = 14
	cont := container.New(layout.NewMaxLayout(), rect, txt)
	return &toggleRenderer{tb: tb, rect: rect, txt: txt, obj: cont}
}

// Tapped toggles state and calls OnChanged
func (tb *ToggleButton) Tapped(*fyne.PointEvent) {
	if !tb.interactive {
		return
	}
	tb.connected = !tb.connected
	tb.Refresh()
	if tb.OnChanged != nil {
		tb.OnChanged(tb.connected)
	}
}

// DoubleTapped ignored
func (tb *ToggleButton) DoubleTapped(*fyne.PointEvent) {}

func (tb *ToggleButton) Set(connected bool) {
	tb.connected = connected
	tb.Refresh()
}

// SetInteractive enables or disables user interaction with the button
func (tb *ToggleButton) SetInteractive(interactive bool) {
	tb.interactive = interactive
}

// SetLabels updates the on/off text labels
func (tb *ToggleButton) SetLabels(offText, onText string) {
	tb.offText = offText
	tb.onText = onText
	tb.Refresh()
}

// BuildExchangePanes constructs the two side-by-side panes (Websocket / REST API)
// Returns the Websocket and REST panes as separate canvas objects.
func BuildExchangePanes(cfg *config.Config) (fyne.CanvasObject, fyne.CanvasObject) {
	return BuildExchangePanesWithHandlers(cfg, nil, nil, nil, nil, nil)
}

// BuildExchangePanesWithHandlers constructs the exchange panes with custom connection handlers
func BuildExchangePanesWithHandlers(
	cfg *config.Config,
	wsConnect func(exchange string, symbols []string) error,
	wsDisconnect func(exchange string) error,
	refreshManager *services.ConfigRefreshManager,
	statusCallback func(string),
	logger *zap.Logger,
) (fyne.CanvasObject, fyne.CanvasObject) {
	// Colors: disconnected = 柿色, connected = パントーングリーン
	orange := color.RGBA{R: 161, G: 93, B: 55, A: 255} // disconnected
	green := color.RGBA{R: 65, G: 204, B: 102, A: 255} // connected

	// --- Websocket pane ---
	wsToggle := NewToggleButton("Websocket Disconnected", "Websocket Connected", orange, green)
	wsToggle.SetInteractive(false)

	connectedExchanges := map[string]bool{
		"Bitfinex": false,
		"Binance":  false,
		"Coinbase": false,
		"Kraken":   false,
	}
	orderedExchanges := []string{"Bitfinex", "Binance", "Coinbase", "Kraken"}

	updateWSToggle := func() {
		connectedNames := make([]string, 0)
		for _, name := range orderedExchanges {
			if connectedExchanges[name] {
				connectedNames = append(connectedNames, name)
			}
		}

		if len(connectedNames) == 0 {
			wsToggle.SetLabels("Websocket Disconnected", "Websocket Connected")
			wsToggle.Set(false)
			return
		}

		label := fmt.Sprintf("%s Websocket Connected", strings.Join(connectedNames, " / "))
		wsToggle.SetLabels("Websocket Disconnected", label)
		wsToggle.Set(true)
	}

	bitfinexSymbols := []string{}
	if cfg != nil {
		bitfinexSymbols = append(bitfinexSymbols, cfg.Symbols...)
	}

	bitfinexChecks := make([]*widget.Check, 0, len(bitfinexSymbols))
	bitfinexList := make([]fyne.CanvasObject, 0, len(bitfinexSymbols))
	for _, symbol := range bitfinexSymbols {
		check := widget.NewCheck(symbol, nil)
		bitfinexChecks = append(bitfinexChecks, check)
		bitfinexList = append(bitfinexList, check)
	}

	if len(bitfinexList) == 0 {
		bitfinexList = append(bitfinexList, widget.NewLabel("No symbols configured"))
	}

	bitfinexChecksContainer := container.NewVBox(bitfinexList...)
	bitfinexScroll := container.NewVScroll(bitfinexChecksContainer)
	bitfinexScroll.SetMinSize(fyne.NewSize(400, 200))

	bitfinexButton := NewToggleButton("Connect Bitfinex Websocket", "Bitfinex Websocket Connected", orange, green)
	bitfinexButton.OnChanged = func(connected bool) {
		if connected {
			selected := make([]string, 0, len(bitfinexChecks))
			for _, check := range bitfinexChecks {
				if check.Checked {
					selected = append(selected, check.Text)
				}
			}

			if len(selected) == 0 {
				fmt.Println("Bitfinex Websocket connect requested without symbol selection")
				bitfinexButton.Set(false)
				return
			}

			if wsConnect != nil {
				if err := wsConnect("Bitfinex", selected); err != nil {
					fmt.Printf("Bitfinex Websocket connect failed: %v\n", err)
					bitfinexButton.Set(false)
					return
				}
			} else {
				fmt.Println("Bitfinex Websocket connect requested")
			}

			for _, check := range bitfinexChecks {
				check.Disable()
			}

			connectedExchanges["Bitfinex"] = true
			updateWSToggle()
		} else {
			if wsDisconnect != nil {
				if err := wsDisconnect("Bitfinex"); err != nil {
					fmt.Printf("Bitfinex Websocket disconnect failed: %v\n", err)
					bitfinexButton.Set(true)
					return
				}
			} else {
				fmt.Println("Bitfinex Websocket disconnect requested")
			}

			for _, check := range bitfinexChecks {
				check.Enable()
			}

			connectedExchanges["Bitfinex"] = false
			updateWSToggle()
		}
	}

	bitfinexContent := container.NewBorder(nil, bitfinexButton, nil, nil, bitfinexScroll)

	updateWSToggle()

	// Tabs for up to 4 exchanges (first is Bitfinex)
	wsTabs := container.NewAppTabs(
		container.NewTabItem("Bitfinex", bitfinexContent),
		container.NewTabItem("Binance", widget.NewLabel("設定をここに追加")),
		container.NewTabItem("Coinbase", widget.NewLabel("設定をここに追加")),
		container.NewTabItem("Kraken", widget.NewLabel("設定をここに追加")),
	)
	wsTabs.SetTabLocation(container.TabLocationTop)

	// top border: ensure toggle spans full width of pane
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
		container.NewTabItem("Binance", widget.NewLabel("設定をここに追加")),
		container.NewTabItem("Coinbase", widget.NewLabel("設定をここに追加")),
		container.NewTabItem("Kraken", widget.NewLabel("設定をここに追加")),
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
