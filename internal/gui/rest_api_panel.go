package gui

import (
	"context"
	"fmt"
	"image/color"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/restapi"
	"github.com/trade-engine/data-controller/internal/services"
)

// RestAPIPanel manages the REST API configuration workflow.
type RestAPIPanel struct {
	logger         *zap.Logger
	cfg            *config.Config
	refreshManager *services.ConfigRefreshManager
	statusCallback func(string)

	runningMu sync.Mutex
	running   bool

	configButton   *flatButton
	optionalButton *flatButton
}

// NewRestAPIPanel creates a new REST API panel with configuration controls.
func NewRestAPIPanel(logger *zap.Logger, cfg *config.Config, manager *services.ConfigRefreshManager, callback func(string)) *RestAPIPanel {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &RestAPIPanel{
		logger:         logger,
		cfg:            cfg,
		refreshManager: manager,
		statusCallback: callback,
	}
}

// CreateBitfinexConfigPanel builds the Bitfinex configuration panel content.
func (p *RestAPIPanel) CreateBitfinexConfigPanel() fyne.CanvasObject {
	essentialList := p.buildEndpointList("Essential & Daily", services.EssentialEndpointInfos())
	optionalList := p.buildEndpointList("Optional (Weekly)", services.OptionalEndpointInfos())

	p.configButton = newFlatButton("Refresh Config", func() {
		p.executeRefresh(true)
	})
	p.optionalButton = newFlatButton("Refresh Optional", func() {
		p.executeOptional()
	})

	content := container.NewVBox(
		essentialList,
		p.configButton,
		widget.NewSeparator(),
		optionalList,
		p.optionalButton,
	)

	return container.NewVScroll(content)
}

func (p *RestAPIPanel) buildEndpointList(title string, endpoints []services.EndpointInfo) fyne.CanvasObject {
	header := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	header.Wrapping = fyne.TextWrapWord

	rows := []fyne.CanvasObject{header}
	for _, ep := range endpoints {
		text := fmt.Sprintf("• %s (%s)", ep.Description, ep.Endpoint)
		label := widget.NewLabel(text)
		label.Wrapping = fyne.TextWrapWord
		rows = append(rows, label)
	}

	return container.NewVBox(rows...)
}

func (p *RestAPIPanel) executeRefresh(force bool) {
	p.runTask("Refreshing config...", func(ctx context.Context) ([]restapi.FetchResult, error) {
		if p.refreshManager == nil {
			return nil, fmt.Errorf("refresh manager not available")
		}
		exchange := p.activeExchange()
		return p.refreshManager.RefreshConfigEndpoints(ctx, exchange, force)
	})
}

func (p *RestAPIPanel) executeOptional() {
	p.runTask("Refreshing optional metadata...", func(ctx context.Context) ([]restapi.FetchResult, error) {
		if p.refreshManager == nil {
			return nil, fmt.Errorf("refresh manager not available")
		}
		exchange := p.activeExchange()
		return p.refreshManager.RefreshOptionalEndpoints(ctx, exchange, true)
	})
}

func (p *RestAPIPanel) runTask(status string, task func(context.Context) ([]restapi.FetchResult, error)) {
	p.runningMu.Lock()
	if p.running {
		p.runningMu.Unlock()
		return
	}
	p.running = true
	p.runningMu.Unlock()

	p.setButtonsEnabled(false)
	p.updateStatus(status)

	go func() {
		defer func() {
			p.runningMu.Lock()
			p.running = false
			p.runningMu.Unlock()
			p.setButtonsEnabled(true)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		results, err := task(ctx)
		if err != nil {
			p.logger.Error("REST config refresh failed", zap.Error(err))
			p.updateStatus(fmt.Sprintf("❌ %v", err))
			return
		}

		exchange := p.activeExchange()
		summary := services.SummarizeResults(exchange, results)
		if summary == "" {
			summary = "No endpoints required refresh"
		}

		p.updateStatus("✅ " + summary)
		if p.statusCallback != nil {
			p.statusCallback(summary)
		}
	}()
}

func (p *RestAPIPanel) setButtonsEnabled(enabled bool) {
	fyne.Do(func() {
		if enabled {
			p.configButton.Enable()
			p.optionalButton.Enable()
		} else {
			p.configButton.Disable()
			p.optionalButton.Disable()
		}
	})
}

func (p *RestAPIPanel) updateStatus(text string) {}

func (p *RestAPIPanel) activeExchange() string {
	if p.cfg != nil && p.cfg.ActiveExchange != "" {
		return p.cfg.ActiveExchange
	}
	return "bitfinex"
}

// flatButton is a minimal rectangular button used in the REST panel.
type flatButton struct {
	widget.BaseWidget
	label    string
	onTap    func()
	disabled bool
}

func newFlatButton(label string, onTap func()) *flatButton {
	b := &flatButton{label: label, onTap: onTap}
	b.ExtendBaseWidget(b)
	return b
}

func (b *flatButton) Disable() {
	b.disabled = true
	b.Refresh()
}

func (b *flatButton) Enable() {
	b.disabled = false
	b.Refresh()
}

func (b *flatButton) Tapped(*fyne.PointEvent) {
	if b.disabled {
		return
	}
	if b.onTap != nil {
		b.onTap()
	}
}

func (b *flatButton) CreateRenderer() fyne.WidgetRenderer {
	rect := canvas.NewRectangle(color.RGBA{R: 60, G: 90, B: 190, A: 255})
	label := widget.NewLabel(b.label)
	label.Alignment = fyne.TextAlignCenter
	label.Wrapping = fyne.TextWrapOff

	padded := container.NewPadded(container.NewCenter(label))
	objects := []fyne.CanvasObject{rect, padded}

	return &flatButtonRenderer{button: b, rect: rect, label: label, padded: padded, objects: objects}
}

// flatButtonRenderer handles drawing of the flat button.
type flatButtonRenderer struct {
	button  *flatButton
	rect    *canvas.Rectangle
	label   *widget.Label
	padded  *fyne.Container
	objects []fyne.CanvasObject
}

func (r *flatButtonRenderer) Layout(size fyne.Size) {
	r.rect.Resize(size)
	r.padded.Resize(size)
}

func (r *flatButtonRenderer) MinSize() fyne.Size {
	min := r.label.MinSize()
	pad := float32(theme.Padding()) * 2
	width := min.Width + pad
	height := min.Height + pad
	if width < 200 {
		width = 200
	}
	return fyne.NewSize(width, height)
}

func (r *flatButtonRenderer) Refresh() {
	if r.button.disabled {
		r.rect.FillColor = color.RGBA{R: 120, G: 120, B: 120, A: 255}
	} else {
		r.rect.FillColor = color.RGBA{R: 60, G: 90, B: 190, A: 255}
	}
	r.rect.Refresh()
	r.label.SetText(r.button.label)
	r.padded.Refresh()
}

func (r *flatButtonRenderer) BackgroundColor() color.Color { return color.Transparent }
func (r *flatButtonRenderer) Objects() []fyne.CanvasObject { return r.objects }
func (r *flatButtonRenderer) Destroy()                     {}
