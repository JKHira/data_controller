package gui

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
	"github.com/trade-engine/data-controller/internal/restapi"
)

// RestDataPanel provides UI and execution management for Bitfinex REST data acquisition.
type RestDataPanel struct {
	logger        *zap.Logger
	cfg           *config.Config
	configManager *config.ConfigManager
	dataClient    *restapi.BitfinexDataClient

	runningMu sync.Mutex
	running   bool
	cancel    context.CancelFunc

	// UI components
	dataTypeRadio *widget.RadioGroup
	symbolChecks  *widget.CheckGroup
	tfChecks      *widget.CheckGroup
	startEntry    *widget.Entry
	endEntry      *widget.Entry
	limitSlider   *widget.Slider
	limitValue    *widget.Label
	sortRadio     *widget.RadioGroup
	autoPaginate  *widget.Check
	dedupCheck    *widget.Check
	gapCheck      *widget.Check
	outputEntry   *widget.Entry
	connectBtn    *widget.Button
	disconnectBtn *widget.Button
	startBtn      *widget.Button
	stopBtn       *widget.Button
	progressLabel *widget.Label
	logBox        *widget.Entry

	symbolOptions []string
	connected     bool
}

func NewRestDataPanel(logger *zap.Logger, cfg *config.Config, manager *config.ConfigManager, dataClient *restapi.BitfinexDataClient) *RestDataPanel {
	if logger == nil {
		logger = zap.NewNop()
	}
	panel := &RestDataPanel{
		logger:        logger,
		cfg:           cfg,
		configManager: manager,
		dataClient:    dataClient,
	}
	panel.loadSymbols()
	return panel
}

func (p *RestDataPanel) loadSymbols() {
	if p.configManager == nil {
		p.symbolOptions = []string{"tBTCUSD", "tETHUSD"}
		return
	}
	symbols, err := p.configManager.GetAvailablePairs("bitfinex", "exchange")
	if err != nil {
		p.logger.Warn("Failed to load REST symbols", zap.Error(err))
		p.symbolOptions = []string{"tBTCUSD", "tETHUSD"}
		return
	}
	set := make(map[string]struct{})
	for _, sym := range symbols {
		if !strings.HasPrefix(sym, "t") && !strings.HasPrefix(sym, "f") {
			sym = "t" + sym
		}
		set[sym] = struct{}{}
	}
	p.symbolOptions = make([]string, 0, len(set))
	for sym := range set {
		p.symbolOptions = append(p.symbolOptions, sym)
	}
	sort.Strings(p.symbolOptions)
}

func (p *RestDataPanel) Build() fyne.CanvasObject {
	p.dataTypeRadio = widget.NewRadioGroup([]string{"Candles", "Trades", "Tickers History"}, nil)

	p.symbolChecks = widget.NewCheckGroup(p.symbolOptions, nil)
	p.symbolChecks.SetSelected([]string{"tBTCUSD"})

	timeframes := []string{"1m", "5m", "15m", "30m", "1h", "3h", "6h", "12h", "1D"}
	p.tfChecks = widget.NewCheckGroup(timeframes, nil)
	p.tfChecks.SetSelected([]string{"1m"})

	p.dataTypeRadio.SetSelected("Candles")
	p.dataTypeRadio.OnChanged = func(string) {
		p.refreshVisibility()
	}

	p.startEntry = widget.NewEntry()
	p.startEntry.SetPlaceHolder("2024-01-01 00:00:00")
	p.endEntry = widget.NewEntry()
	p.endEntry.SetPlaceHolder("2024-01-02 00:00:00")

	p.limitSlider = widget.NewSlider(100, 1000)
	p.limitSlider.Step = 100
	p.limitSlider.Value = 500
	p.limitValue = widget.NewLabel("500")
	p.limitSlider.OnChanged = func(f float64) {
		p.limitValue.SetText(fmt.Sprintf("%d", int(f)))
	}

	p.sortRadio = widget.NewRadioGroup([]string{"Ascending", "Descending"}, nil)
	p.sortRadio.SetSelected("Ascending")

	p.autoPaginate = widget.NewCheck("Auto-pagination", func(bool) {})
	p.autoPaginate.SetChecked(true)
	p.dedupCheck = widget.NewCheck("Remove duplicates", nil)
	p.dedupCheck.SetChecked(true)
	p.gapCheck = widget.NewCheck("Detect gaps", nil)

	defaultOutput := filepath.Join(p.cfg.Storage.BasePath, "bitfinex", "restapi", "data")
	p.outputEntry = widget.NewEntry()
	p.outputEntry.SetText(defaultOutput)

	p.connectBtn = widget.NewButton("Connect REST", func() {
		p.handleConnect()
	})
	p.disconnectBtn = widget.NewButton("Disconnect", func() {
		p.handleDisconnect()
	})
	p.disconnectBtn.Disable()

	p.startBtn = widget.NewButton("Start", func() {
		p.startJob()
	})
	p.startBtn.Disable()
	p.stopBtn = widget.NewButton("Stop", func() {
		p.stopJob()
	})
	p.stopBtn.Disable()

	p.progressLabel = widget.NewLabel("Disconnected")
	p.logBox = widget.NewMultiLineEntry()
	p.logBox.Disable()

	form := container.NewVBox(
		widget.NewLabelWithStyle("Data Type", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		p.dataTypeRadio,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Symbols", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewVScroll(p.symbolChecks),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Timeframes", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewVScroll(p.tfChecks),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Time Range", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewGridWithRows(2,
			container.NewGridWithColumns(2, widget.NewLabel("Start"), p.startEntry),
			container.NewGridWithColumns(2, widget.NewLabel("End"), p.endEntry),
		),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Request Options", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewVBox(
			container.NewGridWithColumns(2,
				widget.NewLabel("Limit"),
				container.NewBorder(nil, nil, nil, p.limitValue, p.limitSlider),
			),
			p.sortRadio,
			p.autoPaginate,
			p.dedupCheck,
			p.gapCheck,
		),
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Output", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewVBox(
			container.NewGridWithColumns(2, widget.NewLabel("Directory"), p.outputEntry),
		),
		widget.NewSeparator(),
		container.NewGridWithColumns(4, p.connectBtn, p.disconnectBtn, p.startBtn, p.stopBtn),
		p.progressLabel,
		widget.NewSeparator(),
		widget.NewLabelWithStyle("Activity Log", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewVScroll(p.logBox),
	)

	p.refreshVisibility()

	return container.NewVScroll(form)
}

func (p *RestDataPanel) refreshVisibility() {
	if p.tfChecks == nil {
		return
	}
	dataType := p.dataTypeRadio.Selected
	showTF := dataType == "Candles"
	p.tfChecks.Hidden = !showTF
	p.tfChecks.Refresh()
}

func (p *RestDataPanel) startJob() {
	p.runningMu.Lock()
	if p.running {
		p.runningMu.Unlock()
		return
	}
	if !p.connected {
		p.runningMu.Unlock()
		p.appendLog("Connect REST before starting a job")
		return
	}
	p.running = true
	p.runningMu.Unlock()

	symbols := p.symbolChecks.Selected
	if len(symbols) == 0 {
		p.appendLog("No symbols selected")
		p.setIdle()
		return
	}

	timeRange, err := p.parseTimeRange()
	if err != nil {
		p.appendLog(fmt.Sprintf("Invalid time range: %v", err))
		p.setIdle()
		return
	}

	limit := int(p.limitSlider.Value)
	sortVal := -1
	if p.sortRadio.Selected == "Ascending" {
		sortVal = 1
	}

	autoPaginate := p.autoPaginate.Checked
	dedup := p.dedupCheck.Checked
	gapDetect := p.gapCheck.Checked
	outputDir := strings.TrimSpace(p.outputEntry.Text)
	if outputDir == "" {
		p.appendLog("Output directory is required")
		p.setIdle()
		return
	}

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		p.appendLog(fmt.Sprintf("Failed to create output directory: %v", err))
		p.setIdle()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	p.cancel = cancel
	p.startBtn.Disable()
	p.stopBtn.Enable()
	p.progressLabel.SetText("Running...")

	go func() {
		var jobErr error
		switch p.dataTypeRadio.Selected {
		case "Candles":
			jobErr = p.runCandlesJob(ctx, symbols, limit, sortVal, autoPaginate, dedup, gapDetect, outputDir, timeRange)
		case "Trades":
			jobErr = p.runTradesJob(ctx, symbols, limit, sortVal, autoPaginate, dedup, outputDir, timeRange)
		case "Tickers History":
			jobErr = p.runTickersJob(ctx, symbols, limit, sortVal, autoPaginate, outputDir, timeRange)
		default:
			jobErr = fmt.Errorf("unsupported data type")
		}

		if jobErr != nil {
			p.appendLog(fmt.Sprintf("Job finished with error: %v", jobErr))
			p.progressLabel.SetText("Error")
		} else {
			p.appendLog("Job completed successfully")
			p.progressLabel.SetText("Completed")
		}
		p.setIdle()
	}()
}

func (p *RestDataPanel) stopJob() {
	p.runningMu.Lock()
	if !p.running {
		p.runningMu.Unlock()
		return
	}
	if p.cancel != nil {
		p.cancel()
	}
	p.runningMu.Unlock()
	p.appendLog("Stop requested")
}

func (p *RestDataPanel) setIdle() {
	p.runningMu.Lock()
	p.running = false
	p.cancel = nil
	p.runningMu.Unlock()
	fyne.Do(func() {
		if p.connected {
			p.startBtn.Enable()
		}
		p.stopBtn.Disable()
	})
}

func (p *RestDataPanel) appendLog(line string) {
	fyne.Do(func() {
		timestamp := time.Now().Format("15:04:05")
		p.logBox.SetText(p.logBox.Text + fmt.Sprintf("[%s] %s\n", timestamp, line))
		p.logBox.CursorRow = len(strings.Split(p.logBox.Text, "\n"))
	})
}

func (p *RestDataPanel) parseTimeRange() (timeRange [2]time.Time, err error) {
	startText := strings.TrimSpace(p.startEntry.Text)
	endText := strings.TrimSpace(p.endEntry.Text)
	layout := "2006-01-02 15:04:05"

	if startText == "" {
		timeRange[0] = time.Now().Add(-24 * time.Hour)
	} else {
		timeRange[0], err = time.ParseInLocation(layout, startText, time.UTC)
		if err != nil {
			return timeRange, err
		}
	}
	if endText == "" {
		timeRange[1] = time.Now()
	} else {
		timeRange[1], err = time.ParseInLocation(layout, endText, time.UTC)
		if err != nil {
			return timeRange, err
		}
	}
	if !timeRange[1].After(timeRange[0]) {
		return timeRange, fmt.Errorf("end time must be after start time")
	}
	return
}

func (p *RestDataPanel) runCandlesJob(ctx context.Context, symbols []string, limit int, sortVal int, autoPaginate, dedup, gapDetect bool, outputDir string, timeRange [2]time.Time) error {
	tfs := p.tfChecks.Selected
	if len(tfs) == 0 {
		return fmt.Errorf("no timeframes selected")
	}

	startMs := timeRange[0].UnixMilli()
	endMs := timeRange[1].UnixMilli()

	tfDurations := map[string]time.Duration{
		"1m":  time.Minute,
		"3m":  3 * time.Minute,
		"5m":  5 * time.Minute,
		"15m": 15 * time.Minute,
		"30m": 30 * time.Minute,
		"1h":  time.Hour,
		"3h":  3 * time.Hour,
		"6h":  6 * time.Hour,
		"12h": 12 * time.Hour,
		"1D":  24 * time.Hour,
		"7D":  7 * 24 * time.Hour,
		"14D": 14 * 24 * time.Hour,
		"1W":  7 * 24 * time.Hour,
		"1M":  30 * 24 * time.Hour,
	}

	for _, symbol := range symbols {
		for _, tf := range tfs {
			select {
			case <-ctx.Done():
				return context.Canceled
			default:
			}

			dur := tfDurations[tf]
			filePath := filepath.Join(outputDir, fmt.Sprintf("candles_%s_%s_%s.csv", strings.TrimPrefix(symbol, "t"), tf, time.Now().Format("20060102_150405")))
			if err := p.writeCandles(ctx, symbol, tf, limit, sortVal, autoPaginate, dedup, gapDetect, startMs, endMs, dur, filePath); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *RestDataPanel) writeCandles(ctx context.Context, symbol, timeframe string, limit, sortVal int, autoPaginate, dedup, gapDetect bool, startMs, endMs int64, tfDuration time.Duration, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	header := []string{"mts", "open", "close", "high", "low", "volume", "symbol", "timeframe"}
	if err := writer.Write(header); err != nil {
		return err
	}

	current := startMs
	var lastTimestamp int64 = -1
	var gapsDetected int

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		batch, err := p.dataClient.FetchCandles(ctx, restapi.CandlesRequest{
			Symbol:    symbol,
			Timeframe: timeframe,
			Section:   "hist",
			Start:     current,
			End:       endMs,
			Limit:     limit,
			Sort:      sortVal,
		})
		if err != nil {
			return err
		}

		if len(batch) == 0 {
			break
		}

		for _, entry := range batch {
			mts := int64(entry[0])
			if mts < startMs {
				continue
			}
			if mts > endMs {
				return nil
			}
			if dedup && mts == lastTimestamp {
				continue
			}
			if gapDetect && lastTimestamp > 0 && tfDuration > 0 {
				expected := lastTimestamp + tfDuration.Milliseconds()
				if mts > expected+tfDuration.Milliseconds()/2 {
					gapsDetected++
					p.appendLog(fmt.Sprintf("Gap detected for %s %s: %dms", symbol, timeframe, mts-expected))
				}
			}

			record := []string{
				fmt.Sprintf("%d", mts),
				formatFloat(entry[1]),
				formatFloat(entry[2]),
				formatFloat(entry[3]),
				formatFloat(entry[4]),
				formatFloat(entry[5]),
				symbol,
				timeframe,
			}
			if err := writer.Write(record); err != nil {
				return err
			}
			lastTimestamp = mts
		}

		writer.Flush()
		p.updateProgress(fmt.Sprintf("Candles %s %s: wrote %d rows", symbol, timeframe, len(batch)))

		if !autoPaginate {
			break
		}

		if sortVal == 1 {
			current = int64(batch[len(batch)-1][0]) + 1
		} else {
			current = int64(batch[len(batch)-1][0]) - 1
		}

		if sortVal == -1 || current >= endMs {
			break
		}
	}

	if gapsDetected > 0 {
		p.appendLog(fmt.Sprintf("%d gaps detected for %s %s", gapsDetected, symbol, timeframe))
	}

	return nil
}

func (p *RestDataPanel) runTradesJob(ctx context.Context, symbols []string, limit, sortVal int, autoPaginate, dedup bool, outputDir string, timeRange [2]time.Time) error {
	startMs := timeRange[0].UnixMilli()
	endMs := timeRange[1].UnixMilli()

	for _, symbol := range symbols {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		filePath := filepath.Join(outputDir, fmt.Sprintf("trades_%s_%s.csv", strings.TrimPrefix(symbol, "t"), time.Now().Format("20060102_150405")))
		if err := p.writeTrades(ctx, symbol, limit, sortVal, autoPaginate, dedup, startMs, endMs, filePath); err != nil {
			return err
		}
	}
	return nil
}

func (p *RestDataPanel) writeTrades(ctx context.Context, symbol string, limit, sortVal int, autoPaginate, dedup bool, startMs, endMs int64, filePath string) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	if err := writer.Write([]string{"id", "mts", "amount", "price", "symbol"}); err != nil {
		return err
	}

	current := startMs
	lastID := float64(0)

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		batch, err := p.dataClient.FetchTrades(ctx, restapi.TradesRequest{
			Symbol: symbol,
			Start:  current,
			End:    endMs,
			Limit:  limit,
			Sort:   sortVal,
		})
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}

		for _, row := range batch {
			if len(row) < 4 {
				continue
			}
			mts := int64(row[1])
			if mts < startMs {
				continue
			}
			if mts > endMs {
				return nil
			}
			if dedup && row[0] == lastID {
				continue
			}
			record := []string{
				formatFloat(row[0]),
				formatFloat(row[1]),
				formatFloat(row[2]),
				formatFloat(row[3]),
				symbol,
			}
			if err := writer.Write(record); err != nil {
				return err
			}
			lastID = row[0]
		}
		writer.Flush()
		p.updateProgress(fmt.Sprintf("Trades %s: wrote %d rows", symbol, len(batch)))

		if !autoPaginate {
			break
		}

		if sortVal == 1 {
			current = int64(batch[len(batch)-1][1]) + 1
		} else {
			current = int64(batch[len(batch)-1][1]) - 1
		}

		if sortVal == -1 || current >= endMs {
			break
		}
	}

	return nil
}

func (p *RestDataPanel) runTickersJob(ctx context.Context, symbols []string, limit, sortVal int, autoPaginate bool, outputDir string, timeRange [2]time.Time) error {
	startMs := timeRange[0].UnixMilli()
	endMs := timeRange[1].UnixMilli()

	if len(symbols) == 0 {
		return fmt.Errorf("no symbols selected")
	}

	filePath := filepath.Join(outputDir, fmt.Sprintf("tickers_%s.csv", time.Now().Format("20060102_150405")))
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	writer := csv.NewWriter(f)
	defer writer.Flush()

	header := []string{"symbol", "bid", "bid_size", "ask", "ask_size", "daily_change", "daily_change_rel", "last_price", "volume", "high", "low", "mts"}
	if err := writer.Write(header); err != nil {
		return err
	}

	current := startMs

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		batch, err := p.dataClient.FetchTickersHistory(ctx, restapi.TickersHistoryRequest{
			Symbols: symbols,
			Start:   current,
			End:     endMs,
			Limit:   limit,
			Sort:    sortVal,
		})
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			break
		}

		for _, row := range batch {
			if len(row) < 12 {
				continue
			}
			symbolVal := fmt.Sprintf("%v", row[0])
			mts := parseFloat(row[len(row)-1])
			if mts < float64(startMs) {
				continue
			}
			if mts > float64(endMs) {
				return nil
			}

			record := make([]string, len(header))
			record[0] = symbolVal
			for i := 1; i < len(header); i++ {
				record[i] = fmt.Sprintf("%v", row[i])
			}
			if err := writer.Write(record); err != nil {
				return err
			}
		}
		writer.Flush()
		p.updateProgress(fmt.Sprintf("Tickers: wrote %d rows", len(batch)))

		if !autoPaginate {
			break
		}

		if sortVal == 1 {
			last := batch[len(batch)-1]
			current = int64(parseFloat(last[len(last)-1])) + 1
		} else {
			current = int64(parseFloat(batch[len(batch)-1][len(batch[len(batch)-1])-1])) - 1
		}

		if sortVal == -1 || current >= endMs {
			break
		}
	}

	return nil
}

func (p *RestDataPanel) updateProgress(status string) {
	fyne.Do(func() {
		p.progressLabel.SetText(status)
	})
}

func (p *RestDataPanel) handleConnect() {
	p.runningMu.Lock()
	if p.connected {
		p.runningMu.Unlock()
		return
	}
	p.connected = true
	p.runningMu.Unlock()

	fyne.Do(func() {
		p.connectBtn.Disable()
		p.disconnectBtn.Enable()
		p.startBtn.Enable()
		p.progressLabel.SetText("Connected")
	})

	p.appendLog("REST session connected")
}

func (p *RestDataPanel) handleDisconnect() {
	p.runningMu.Lock()
	if !p.connected {
		p.runningMu.Unlock()
		return
	}
	p.connected = false
	cancel := p.cancel
	p.runningMu.Unlock()

	if cancel != nil {
		cancel()
	}

	fyne.Do(func() {
		p.connectBtn.Enable()
		p.disconnectBtn.Disable()
		p.startBtn.Disable()
		p.stopBtn.Disable()
		p.progressLabel.SetText("Disconnected")
	})

	p.appendLog("REST session disconnected")
}

func formatFloat(val float64) string {
	return strconv.FormatFloat(val, 'f', -1, 64)
}

func parseFloat(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		if f, err := strconv.ParseFloat(t, 64); err == nil {
			return f
		}
	}
	return 0
}
