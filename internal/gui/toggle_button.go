package gui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

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

func (tb *ToggleButton) DoubleTapped(*fyne.PointEvent) {}

func (tb *ToggleButton) Set(connected bool) {
	tb.connected = connected
	tb.Refresh()
}

func (tb *ToggleButton) SetInteractive(interactive bool) {
	tb.interactive = interactive
}

func (tb *ToggleButton) SetLabels(offText, onText string) {
	tb.offText = offText
	tb.onText = onText
	tb.Refresh()
}
