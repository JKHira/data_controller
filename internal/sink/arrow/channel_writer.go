package arrow

import (
	"fmt"
	"os"
	"time"

	"github.com/apache/arrow/go/v17/arrow"
	"github.com/apache/arrow/go/v17/arrow/array"
	"github.com/trade-engine/data-controller/pkg/schema"
)

func (cw *ChannelWriter) writeRawBookEvent(event *schema.RawBookEvent) error {
	cw.Mutex.Lock()
	defer cw.Mutex.Unlock()

	if !cw.IsOpen {
		return fmt.Errorf("writer is closed")
	}

	// Add data to builders
	builders := cw.Builder.builders

	// Common fields (4 fields total: symbol, pair_or_currency, seq, recv_ts)
	builders[SymbolIdx].(*array.StringBuilder).Append(event.Symbol)
	builders[PairOrCurrencyIdx].(*array.StringBuilder).Append(event.PairOrCurrency)
	appendOptionalInt64(builders[SeqIdx].(*array.Int64Builder), event.Seq)
	builders[RecvTSIdx].(*array.Int64Builder).Append(event.RecvTS)

	// Raw book event specific fields (starting at index 4)
	// batch_id, order_id, price, amount, op, side, is_snapshot
	appendOptionalInt64(builders[4].(*array.Int64Builder), event.BatchID)
	builders[5].(*array.Int64Builder).Append(event.OrderID)
	builders[6].(*array.Float64Builder).Append(event.Price)
	builders[7].(*array.Float64Builder).Append(event.Amount)
	builders[8].(*array.StringBuilder).Append(string(event.Op))
	builders[9].(*array.StringBuilder).Append(string(event.Side))
	builders[10].(*array.BooleanBuilder).Append(event.IsSnapshot)

	cw.RowCount++

	// Write record batch if we have enough rows
	if cw.RowCount%100 == 0 {
		return cw.writeRecordBatch()
	}

	return nil
}

func (cw *ChannelWriter) writeBookLevel(level *schema.BookLevel) error {
	cw.Mutex.Lock()
	defer cw.Mutex.Unlock()

	if !cw.IsOpen {
		return fmt.Errorf("writer is closed")
	}

	// Add data to builders
	builders := cw.Builder.builders

	// Common fields (4 fields total: symbol, pair_or_currency, seq, recv_ts)
	builders[SymbolIdx].(*array.StringBuilder).Append(level.Symbol)
	builders[PairOrCurrencyIdx].(*array.StringBuilder).Append(level.PairOrCurrency)
	appendOptionalInt64(builders[SeqIdx].(*array.Int64Builder), level.Seq)
	builders[RecvTSIdx].(*array.Int64Builder).Append(level.RecvTS)

	// Book level specific fields (starting at index 4)
	// batch_id, price, count, amount, side, is_snapshot
	appendOptionalInt64(builders[4].(*array.Int64Builder), level.BatchID)
	builders[5].(*array.Float64Builder).Append(level.Price)
	builders[6].(*array.Int32Builder).Append(level.Count)
	builders[7].(*array.Float64Builder).Append(level.Amount)
	builders[8].(*array.StringBuilder).Append(string(level.Side))
	builders[9].(*array.BooleanBuilder).Append(level.IsSnapshot)

	cw.RowCount++

	// Write record batch if we have enough rows
	if cw.RowCount%100 == 0 {
		return cw.writeRecordBatch()
	}

	return nil
}

func (cw *ChannelWriter) writeTrade(trade *schema.Trade) error {
	cw.Mutex.Lock()
	defer cw.Mutex.Unlock()

	if !cw.IsOpen {
		return fmt.Errorf("writer is closed")
	}

	// Add data to builders
	builders := cw.Builder.builders

	// Common fields (4 fields total: symbol, pair_or_currency, seq, recv_ts)
	builders[SymbolIdx].(*array.StringBuilder).Append(trade.Symbol)
	builders[PairOrCurrencyIdx].(*array.StringBuilder).Append(trade.PairOrCurrency)
	appendOptionalInt64(builders[SeqIdx].(*array.Int64Builder), trade.Seq)
	builders[RecvTSIdx].(*array.Int64Builder).Append(trade.RecvTS)

	// Trade specific fields (starting at index 4)
	// trade_id, mts, amount, price, msg_type, is_snapshot
	builders[4].(*array.Int64Builder).Append(trade.TradeID)
	builders[5].(*array.Int64Builder).Append(trade.MTS)
	builders[6].(*array.Float64Builder).Append(trade.Amount)
	builders[7].(*array.Float64Builder).Append(trade.Price)
	builders[8].(*array.StringBuilder).Append(string(trade.MsgType))
	builders[9].(*array.BooleanBuilder).Append(trade.IsSnapshot)

	cw.RowCount++

	// Write record batch if we have enough rows
	if cw.RowCount%100 == 0 {
		return cw.writeRecordBatch()
	}

	return nil
}

func (cw *ChannelWriter) writeTicker(ticker *schema.Ticker) error {
	cw.Mutex.Lock()
	defer cw.Mutex.Unlock()

	if !cw.IsOpen {
		return fmt.Errorf("writer is closed")
	}

	// Add data to builders
	builders := cw.Builder.builders

	// Common fields (4 fields total: symbol, pair_or_currency, seq, recv_ts)
	builders[SymbolIdx].(*array.StringBuilder).Append(ticker.Symbol)
	builders[PairOrCurrencyIdx].(*array.StringBuilder).Append(ticker.PairOrCurrency)
	appendOptionalInt64(builders[SeqIdx].(*array.Int64Builder), ticker.Seq)
	builders[RecvTSIdx].(*array.Int64Builder).Append(ticker.RecvTS)

	// Ticker specific fields (starting at index 4)
	// bid, bid_sz, ask, ask_sz, last, vol, high, low, daily_change, daily_change_rel
	builders[4].(*array.Float64Builder).Append(ticker.Bid)
	builders[5].(*array.Float64Builder).Append(ticker.BidSize)
	builders[6].(*array.Float64Builder).Append(ticker.Ask)
	builders[7].(*array.Float64Builder).Append(ticker.AskSize)
	builders[8].(*array.Float64Builder).Append(ticker.Last)
	builders[9].(*array.Float64Builder).Append(ticker.Vol)
	builders[10].(*array.Float64Builder).Append(ticker.High)
	builders[11].(*array.Float64Builder).Append(ticker.Low)
	builders[12].(*array.Float64Builder).Append(ticker.DailyChange)
	builders[13].(*array.Float64Builder).Append(ticker.DailyChangeRel)

	cw.RowCount++

	// Write record batch if we have enough rows
	if cw.RowCount%100 == 0 {
		return cw.writeRecordBatch()
	}

	return nil
}

func (cw *ChannelWriter) writeCandle(candle *schema.Candle) error {
	cw.Mutex.Lock()
	defer cw.Mutex.Unlock()

	if !cw.IsOpen {
		return fmt.Errorf("writer is closed")
	}

	// Add data to builders
	builders := cw.Builder.builders

	// Common fields (4 fields total: symbol, pair_or_currency, seq, recv_ts)
	builders[SymbolIdx].(*array.StringBuilder).Append(candle.Symbol)
	builders[PairOrCurrencyIdx].(*array.StringBuilder).Append(candle.PairOrCurrency)
	appendOptionalInt64(builders[SeqIdx].(*array.Int64Builder), candle.Seq)
	builders[RecvTSIdx].(*array.Int64Builder).Append(candle.RecvTS)

	// Candle specific fields (starting at index 4)
	// mts, open, close, high, low, volume, is_snapshot
	builders[4].(*array.Int64Builder).Append(candle.MTS)
	builders[5].(*array.Float64Builder).Append(candle.Open)
	builders[6].(*array.Float64Builder).Append(candle.Close)
	builders[7].(*array.Float64Builder).Append(candle.High)
	builders[8].(*array.Float64Builder).Append(candle.Low)
	builders[9].(*array.Float64Builder).Append(candle.Volume)
	builders[10].(*array.BooleanBuilder).Append(candle.IsSnapshot)

	cw.RowCount++

	// Write record batch if we have enough rows
	if cw.RowCount%100 == 0 {
		return cw.writeRecordBatch()
	}

	return nil
}

func (cw *ChannelWriter) writeRecordBatch() error {
	if !cw.IsOpen {
		return nil
	}

	// Build arrays from builders
	columns := make([]arrow.Array, len(cw.Builder.builders))
	for i, builder := range cw.Builder.builders {
		columns[i] = builder.NewArray()
		defer columns[i].Release()
	}

	// Create record batch
	record := array.NewRecord(cw.Schema, columns, int64(columns[0].Len()))
	defer record.Release()

	// Write record batch
	if err := cw.Writer.Write(record); err != nil {
		return fmt.Errorf("failed to write record batch: %w", err)
	}

	// Reset builders for next batch
	for _, builder := range cw.Builder.builders {
		builder.Release()
	}
	cw.Builder.initBuilders()

	return nil
}

func (cw *ChannelWriter) flush() error {
	cw.Mutex.Lock()
	defer cw.Mutex.Unlock()

	if !cw.IsOpen {
		return nil
	}

	// Write any remaining data
	if cw.Builder.builders[0].Len() > 0 {
		if err := cw.writeRecordBatch(); err != nil {
			return err
		}
	}

	return nil
}

func (cw *ChannelWriter) close() error {
	cw.Mutex.Lock()
	defer cw.Mutex.Unlock()

	if !cw.IsOpen {
		return nil
	}

	cw.IsOpen = false

	// Write any remaining data
	if cw.Builder.builders[0].Len() > 0 {
		if err := cw.writeRecordBatch(); err != nil {
			return fmt.Errorf("failed to write final batch: %w", err)
		}
	}

	if cw.Metadata != nil {
		cw.Metadata.DatetimeEnd = time.Now().UTC().Format(time.RFC3339)
	}

	// Close Arrow writer
	if err := cw.Writer.Close(); err != nil {
		return fmt.Errorf("failed to close Arrow writer: %w", err)
	}

	// Close file
	if cw.File != nil {
		cw.File.Sync() // Ensure all data is written to disk
		cw.File.Close()
	}

	// Rename temp file to final file
	if err := os.Rename(cw.TempFilePath, cw.FilePath); err != nil {
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	// Release builders
	for _, builder := range cw.Builder.builders {
		builder.Release()
	}

	return nil
}

// Helper function to append optional int64 values
func appendOptionalInt64(builder *array.Int64Builder, value *int64) {
	if value != nil {
		builder.Append(*value)
	} else {
		builder.AppendNull()
	}
}
