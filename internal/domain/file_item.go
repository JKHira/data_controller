package domain

import "time"

// FileItem represents a file item for UI display
type FileItem struct {
	Path     string    `json:"path"`
	Size     int64     `json:"size"`
	ModTime  time.Time `json:"mod_time"`
	Exchange string    `json:"exchange"` // "bitfinex"
	Source   string    `json:"source"`   // "ws" | "restapi"
	Category string    `json:"category"` // "trades" | "ticker" | "books" | "raw_books"
	Symbol   string    `json:"symbol"`   // "tBTCUSD"
	Date     string    `json:"date"`     // "YYYY-MM-DD"
	Hour     string    `json:"hour"`     // "00".."23" or "All"
	Ext      string    `json:"ext"`      // "arrow" | "jsonl"
}

// ScanParams represents parameters for file scanning
type ScanParams struct {
	BasePath string    `json:"base_path"`
	Exchange string    `json:"exchange"`
	Source   string    `json:"source"`   // "websocket" | "restapi"
	Category string    `json:"category"` // "trades" | "ticker" | "books" | "raw_books"
	Symbol   string    `json:"symbol"`   // symbol filter ("ALL" | "no data" | specific symbol)
	DateFrom time.Time `json:"date_from"`
	DateTo   time.Time `json:"date_to"`
	Hour     string    `json:"hour"`     // "All" | "00".."23"
	Ext      string    `json:"ext"`      // "any" | "arrow" | "jsonl"
	// Filter removed - filename filter not needed as requested
}