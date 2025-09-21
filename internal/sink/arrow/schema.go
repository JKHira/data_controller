package arrow

import (
	"github.com/apache/arrow/go/v17/arrow"
)

// Common field indices for all schemas
const (
	// Common fields (present in all schemas)
	ExchangeIdx = iota
	ChannelIdx
	SymbolIdx
	PairOrCurrencyIdx
	ConnIDIdx
	ChanIDIdx
	SubIDIdx
	TsMicrosIdx
	ConfFlagsIdx
	SeqIdx
	SrvMTSIdx
	WSTSIdx
	RecvTSIdx
	BatchIDIdx
	IngestIDIdx
	SourceFileIdx
	LineNoIdx
)

// GetCommonFields returns the common fields used in all schemas
func GetCommonFields() []arrow.Field {
	return []arrow.Field{
		{Name: "exchange", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "channel", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "symbol", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "pair_or_currency", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "conn_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "chan_id", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "sub_id", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "ts_us", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "conf_flags", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "seq", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "srv_mts", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "ws_ts", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "recv_ts", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "batch_id", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "ingest_id", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "source_file", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "line_no", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
	}
}

// GetTickerSchema returns the Arrow schema for ticker data
func GetTickerSchema() *arrow.Schema {
	fields := GetCommonFields()

	// Add ticker-specific fields
	tickerFields := []arrow.Field{
		{Name: "bid", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "bid_sz", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "ask", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "ask_sz", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "last", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "vol", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "high", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "low", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "daily_change", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "daily_change_rel", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
	}

	fields = append(fields, tickerFields...)
	return arrow.NewSchema(fields, nil)
}

// GetTradeSchema returns the Arrow schema for trade data
func GetTradeSchema() *arrow.Schema {
	fields := GetCommonFields()

	// Add trade-specific fields
	tradeFields := []arrow.Field{
		{Name: "trade_id", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "mts", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "amount", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "price", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "msg_type", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "is_snapshot", Type: arrow.FixedWidthTypes.Boolean, Nullable: false},
	}

	fields = append(fields, tradeFields...)
	return arrow.NewSchema(fields, nil)
}

// GetBookLevelSchema returns the Arrow schema for book level data
func GetBookLevelSchema() *arrow.Schema {
	fields := GetCommonFields()

	// Add book level-specific fields
	bookFields := []arrow.Field{
		{Name: "price", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "count", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "amount", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "side", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "prec", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "freq", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "len", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "is_snapshot", Type: arrow.FixedWidthTypes.Boolean, Nullable: false},
	}

	fields = append(fields, bookFields...)
	return arrow.NewSchema(fields, nil)
}

// GetRawBookEventSchema returns the Arrow schema for raw book events
func GetRawBookEventSchema() *arrow.Schema {
	fields := GetCommonFields()

	// Add raw book event-specific fields
	rawBookFields := []arrow.Field{
		{Name: "order_id", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "price", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "amount", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "op", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "side", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "is_snapshot", Type: arrow.FixedWidthTypes.Boolean, Nullable: false},
	}

	fields = append(fields, rawBookFields...)
	return arrow.NewSchema(fields, nil)
}