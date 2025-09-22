package gui

import (
	"fmt"
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

// ToggleButton: シンプルなトグル風ボタン（背景色とテキストを切替）
type ToggleButton struct {
	widget.BaseWidget
	connected bool
	offText   string
	onText    string
	offColor  color.Color
	onColor   color.Color
	OnChanged func(bool)
}

func NewToggleButton(offText, onText string, offColor, onColor color.Color) *ToggleButton {
	tb := &ToggleButton{
		connected: false,
		offText:   offText,
		onText:    onText,
		offColor:  offColor,
		onColor:   onColor,
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

// BuildExchangePanes constructs the two side-by-side panes (Websocket / REST API)
// Returns a fyne.CanvasObject to be embedded into the main window.
func BuildExchangePanes() fyne.CanvasObject {
	return BuildExchangePanesWithHandlers(nil, nil)
}

// BuildExchangePanesWithHandlers constructs the exchange panes with custom connection handlers
func BuildExchangePanesWithHandlers(wsHandler, restHandler func(bool)) fyne.CanvasObject {
	// Colors: disconnected = 柿色, connected = パントーングリーン
	orange := color.RGBA{R: 161, G: 93, B: 55, A: 255} // disconnected
	green := color.RGBA{R: 65, G: 204, B: 102, A: 255}  // connected

	// --- Websocket pane ---
	wsToggle := NewToggleButton("Websocket Disconnected", "Websocket Connected", orange, green)
	wsToggle.OnChanged = func(connected bool) {
		if wsHandler != nil {
			wsHandler(connected)
		} else {
			if connected {
				fmt.Println("Websocket: CONNECT requested")
			} else {
				fmt.Println("Websocket: DISCONNECT requested")
			}
		}
	}

	// Tabs for up to 4 exchanges (first is Bitfinex)
	wsTabs := container.NewAppTabs(
		container.NewTabItem("Bitfinex", widget.NewLabel("Bitfinex WS settings (保持)")),
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
		if restHandler != nil {
			restHandler(connected)
		} else {
			if connected {
				fmt.Println("REST API: CONNECT requested")
			} else {
				fmt.Println("REST API: DISCONNECT requested")
			}
		}
	}

	restTabs := container.NewAppTabs(
		container.NewTabItem("Bitfinex", widget.NewLabel("Bitfinex REST settings")),
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

	// Side-by-side panes, each uses available vertical space and share grid columns
	cols := container.New(layout.NewGridLayout(2), wsPane, restPane)
	wrapped := container.NewBorder(nil, nil, nil, nil, cols)
	return wrapped
}
