package arrow

import (
	"github.com/apache/arrow/go/v17/arrow"
)

// Common field indices for all schemas
const (
	// Common fields (present in all schemas)
	SymbolIdx = iota
	PairOrCurrencyIdx
	SeqIdx
	RecvTSIdx
)

// GetCommonFields returns the common fields used in all schemas
func GetCommonFields() []arrow.Field {
	return []arrow.Field{
		{Name: "symbol", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "pair_or_currency", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "seq", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "recv_ts", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
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

	// Add book level-specific fields (including batch_id for books)
	bookFields := []arrow.Field{
		{Name: "batch_id", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
		{Name: "price", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "count", Type: arrow.PrimitiveTypes.Int32, Nullable: false},
		{Name: "amount", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "side", Type: arrow.BinaryTypes.String, Nullable: false},
		{Name: "is_snapshot", Type: arrow.FixedWidthTypes.Boolean, Nullable: false},
	}

	fields = append(fields, bookFields...)
	return arrow.NewSchema(fields, nil)
}

// GetRawBookEventSchema returns the Arrow schema for raw book events
func GetRawBookEventSchema() *arrow.Schema {
	fields := GetCommonFields()

	// Add raw book event-specific fields (including batch_id for raw books)
	rawBookFields := []arrow.Field{
		{Name: "batch_id", Type: arrow.PrimitiveTypes.Int64, Nullable: true},
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

// GetCandleSchema returns the Arrow schema for candle data
func GetCandleSchema() *arrow.Schema {
	fields := GetCommonFields()

	// Add candle-specific fields
	candleFields := []arrow.Field{
		{Name: "mts", Type: arrow.PrimitiveTypes.Int64, Nullable: false},
		{Name: "open", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "close", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "high", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "low", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "volume", Type: arrow.PrimitiveTypes.Float64, Nullable: false},
		{Name: "is_snapshot", Type: arrow.FixedWidthTypes.Boolean, Nullable: false},
	}

	fields = append(fields, candleFields...)
	return arrow.NewSchema(fields, nil)
}
