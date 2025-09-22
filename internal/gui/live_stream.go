package gui

import (
	"fmt"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/trade-engine/data-controller/pkg/schema"
)

// LiveStreamData manages the live data stream display
type LiveStreamData struct {
	streamData       []string
	streamMutex      sync.Mutex
	maxStreamEntries int
	dataStreamList   *widget.List
}

// NewLiveStreamData creates a new live stream data manager
func NewLiveStreamData(maxEntries int) *LiveStreamData {
	lsd := &LiveStreamData{
		maxStreamEntries: maxEntries,
		streamData:       make([]string, 0, maxEntries),
	}

	// Create the list widget
	lsd.dataStreamList = widget.NewList(
		func() int { return len(lsd.streamData) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id < len(lsd.streamData) {
				label := obj.(*widget.Label)
				label.SetText(lsd.streamData[id])
			}
		},
	)

	return lsd
}

// CreateLiveStreamPanel creates the live data stream panel
func CreateLiveStreamPanel(liveStreamData *LiveStreamData) fyne.CanvasObject {
	return widget.NewCard("📡 Live Data Stream (Latest 20)", "", liveStreamData.dataStreamList)
}

// AddStreamData adds new data to the stream display
func (lsd *LiveStreamData) AddStreamData(dataType, symbol string, data interface{}) {
	lsd.streamMutex.Lock()
	defer lsd.streamMutex.Unlock()

	timestamp := time.Now().Format("15:04:05.000")
	var message string

	switch dataType {
	case "ticker":
		if ticker, ok := data.(*schema.Ticker); ok {
			message = fmt.Sprintf("[%s] 📈 %s: Bid=%.2f Ask=%.2f", timestamp, symbol, ticker.Bid, ticker.Ask)
		}
	case "trade":
		if trade, ok := data.(*schema.Trade); ok {
			side := "BUY"
			if trade.Amount < 0 {
				side = "SELL"
			}
			message = fmt.Sprintf("[%s] 💰 %s: %s %.6f @ %.2f", timestamp, symbol, side, abs(trade.Amount), trade.Price)
		}
	case "book":
		if book, ok := data.(*schema.BookLevel); ok {
			side := "BID"
			if book.Side == schema.SideAsk {
				side = "ASK"
			}
			message = fmt.Sprintf("[%s] 📚 %s: %s %.2f (%.4f)", timestamp, symbol, side, book.Price, book.Amount)
		}
	case "raw_book":
		if rawBook, ok := data.(*schema.RawBookEvent); ok {
			action := "UPDATE"
			if rawBook.Amount == 0 {
				action = "DELETE"
			}
			message = fmt.Sprintf("[%s] 📝 %s: %s %.2f", timestamp, symbol, action, rawBook.Price)
		}
	default:
		message = fmt.Sprintf("[%s] 📡 %s: %s", timestamp, symbol, dataType)
	}

	// Add to beginning of slice
	lsd.streamData = append([]string{message}, lsd.streamData...)

	// Keep only the latest entries
	if len(lsd.streamData) > lsd.maxStreamEntries {
		lsd.streamData = lsd.streamData[:lsd.maxStreamEntries]
	}

	// Update UI on main thread
	fyne.Do(func() {
		lsd.dataStreamList.Refresh()
	})
}

// GetList returns the widget list for external use
func (lsd *LiveStreamData) GetList() *widget.List {
	return lsd.dataStreamList
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}