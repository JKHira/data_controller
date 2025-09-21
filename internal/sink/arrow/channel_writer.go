package arrow

import (
	"fmt"
	"os"

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

	// Common fields
	builders[ExchangeIdx].(*array.StringBuilder).Append(string(event.Exchange))
	builders[ChannelIdx].(*array.StringBuilder).Append(string(event.Channel))
	builders[SymbolIdx].(*array.StringBuilder).Append(event.Symbol)
	builders[PairOrCurrencyIdx].(*array.StringBuilder).Append(event.PairOrCurrency)
	builders[ConnIDIdx].(*array.StringBuilder).Append(event.ConnID)
	builders[ChanIDIdx].(*array.Int32Builder).Append(event.ChanID)
	appendOptionalInt64(builders[SubIDIdx].(*array.Int64Builder), event.SubID)
	builders[TsMicrosIdx].(*array.Int64Builder).Append(event.TsMicros)
	builders[ConfFlagsIdx].(*array.Int64Builder).Append(event.ConfFlags)
	appendOptionalInt64(builders[SeqIdx].(*array.Int64Builder), event.Seq)
	appendOptionalInt64(builders[SrvMTSIdx].(*array.Int64Builder), event.SrvMTS)
	appendOptionalInt64(builders[WSTSIdx].(*array.Int64Builder), event.WSTS)
	builders[RecvTSIdx].(*array.Int64Builder).Append(event.RecvTS)
	appendOptionalInt64(builders[BatchIDIdx].(*array.Int64Builder), event.BatchID)
	builders[IngestIDIdx].(*array.StringBuilder).Append(event.IngestID)
	builders[SourceFileIdx].(*array.StringBuilder).Append(event.SourceFile)
	appendOptionalInt64(builders[LineNoIdx].(*array.Int64Builder), event.LineNo)

	// Raw book event specific fields
	builders[17].(*array.Int64Builder).Append(event.OrderID)
	builders[18].(*array.Float64Builder).Append(event.Price)
	builders[19].(*array.Float64Builder).Append(event.Amount)
	builders[20].(*array.StringBuilder).Append(string(event.Op))
	builders[21].(*array.StringBuilder).Append(string(event.Side))
	builders[22].(*array.BooleanBuilder).Append(event.IsSnapshot)

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

	// Common fields
	builders[ExchangeIdx].(*array.StringBuilder).Append(string(level.Exchange))
	builders[ChannelIdx].(*array.StringBuilder).Append(string(level.Channel))
	builders[SymbolIdx].(*array.StringBuilder).Append(level.Symbol)
	builders[PairOrCurrencyIdx].(*array.StringBuilder).Append(level.PairOrCurrency)
	builders[ConnIDIdx].(*array.StringBuilder).Append(level.ConnID)
	builders[ChanIDIdx].(*array.Int32Builder).Append(level.ChanID)
	appendOptionalInt64(builders[SubIDIdx].(*array.Int64Builder), level.SubID)
	builders[TsMicrosIdx].(*array.Int64Builder).Append(level.TsMicros)
	builders[ConfFlagsIdx].(*array.Int64Builder).Append(level.ConfFlags)
	appendOptionalInt64(builders[SeqIdx].(*array.Int64Builder), level.Seq)
	appendOptionalInt64(builders[SrvMTSIdx].(*array.Int64Builder), level.SrvMTS)
	appendOptionalInt64(builders[WSTSIdx].(*array.Int64Builder), level.WSTS)
	builders[RecvTSIdx].(*array.Int64Builder).Append(level.RecvTS)
	appendOptionalInt64(builders[BatchIDIdx].(*array.Int64Builder), level.BatchID)
	builders[IngestIDIdx].(*array.StringBuilder).Append(level.IngestID)
	builders[SourceFileIdx].(*array.StringBuilder).Append(level.SourceFile)
	appendOptionalInt64(builders[LineNoIdx].(*array.Int64Builder), level.LineNo)

	// Book level specific fields
	builders[17].(*array.Float64Builder).Append(level.Price)
	builders[18].(*array.Int32Builder).Append(level.Count)
	builders[19].(*array.Float64Builder).Append(level.Amount)
	builders[20].(*array.StringBuilder).Append(string(level.Side))
	builders[21].(*array.StringBuilder).Append(level.Prec)
	builders[22].(*array.StringBuilder).Append(level.Freq)
	builders[23].(*array.Int32Builder).Append(level.Len)
	builders[24].(*array.BooleanBuilder).Append(level.IsSnapshot)

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

	// Common fields
	builders[ExchangeIdx].(*array.StringBuilder).Append(string(trade.Exchange))
	builders[ChannelIdx].(*array.StringBuilder).Append(string(trade.Channel))
	builders[SymbolIdx].(*array.StringBuilder).Append(trade.Symbol)
	builders[PairOrCurrencyIdx].(*array.StringBuilder).Append(trade.PairOrCurrency)
	builders[ConnIDIdx].(*array.StringBuilder).Append(trade.ConnID)
	builders[ChanIDIdx].(*array.Int32Builder).Append(trade.ChanID)
	appendOptionalInt64(builders[SubIDIdx].(*array.Int64Builder), trade.SubID)
	builders[TsMicrosIdx].(*array.Int64Builder).Append(trade.TsMicros)
	builders[ConfFlagsIdx].(*array.Int64Builder).Append(trade.ConfFlags)
	appendOptionalInt64(builders[SeqIdx].(*array.Int64Builder), trade.Seq)
	appendOptionalInt64(builders[SrvMTSIdx].(*array.Int64Builder), trade.SrvMTS)
	appendOptionalInt64(builders[WSTSIdx].(*array.Int64Builder), trade.WSTS)
	builders[RecvTSIdx].(*array.Int64Builder).Append(trade.RecvTS)
	appendOptionalInt64(builders[BatchIDIdx].(*array.Int64Builder), trade.BatchID)
	builders[IngestIDIdx].(*array.StringBuilder).Append(trade.IngestID)
	builders[SourceFileIdx].(*array.StringBuilder).Append(trade.SourceFile)
	appendOptionalInt64(builders[LineNoIdx].(*array.Int64Builder), trade.LineNo)

	// Trade specific fields
	builders[17].(*array.Int64Builder).Append(trade.TradeID)
	builders[18].(*array.Int64Builder).Append(trade.MTS)
	builders[19].(*array.Float64Builder).Append(trade.Amount)
	builders[20].(*array.Float64Builder).Append(trade.Price)
	builders[21].(*array.StringBuilder).Append(string(trade.MsgType))
	builders[22].(*array.BooleanBuilder).Append(trade.IsSnapshot)

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

	// Common fields
	builders[ExchangeIdx].(*array.StringBuilder).Append(string(ticker.Exchange))
	builders[ChannelIdx].(*array.StringBuilder).Append(string(ticker.Channel))
	builders[SymbolIdx].(*array.StringBuilder).Append(ticker.Symbol)
	builders[PairOrCurrencyIdx].(*array.StringBuilder).Append(ticker.PairOrCurrency)
	builders[ConnIDIdx].(*array.StringBuilder).Append(ticker.ConnID)
	builders[ChanIDIdx].(*array.Int32Builder).Append(ticker.ChanID)
	appendOptionalInt64(builders[SubIDIdx].(*array.Int64Builder), ticker.SubID)
	builders[TsMicrosIdx].(*array.Int64Builder).Append(ticker.TsMicros)
	builders[ConfFlagsIdx].(*array.Int64Builder).Append(ticker.ConfFlags)
	appendOptionalInt64(builders[SeqIdx].(*array.Int64Builder), ticker.Seq)
	appendOptionalInt64(builders[SrvMTSIdx].(*array.Int64Builder), ticker.SrvMTS)
	appendOptionalInt64(builders[WSTSIdx].(*array.Int64Builder), ticker.WSTS)
	builders[RecvTSIdx].(*array.Int64Builder).Append(ticker.RecvTS)
	appendOptionalInt64(builders[BatchIDIdx].(*array.Int64Builder), ticker.BatchID)
	builders[IngestIDIdx].(*array.StringBuilder).Append(ticker.IngestID)
	builders[SourceFileIdx].(*array.StringBuilder).Append(ticker.SourceFile)
	appendOptionalInt64(builders[LineNoIdx].(*array.Int64Builder), ticker.LineNo)

	// Ticker specific fields
	builders[17].(*array.Float64Builder).Append(ticker.Bid)
	builders[18].(*array.Float64Builder).Append(ticker.BidSize)
	builders[19].(*array.Float64Builder).Append(ticker.Ask)
	builders[20].(*array.Float64Builder).Append(ticker.AskSize)
	builders[21].(*array.Float64Builder).Append(ticker.Last)
	builders[22].(*array.Float64Builder).Append(ticker.Vol)
	builders[23].(*array.Float64Builder).Append(ticker.High)
	builders[24].(*array.Float64Builder).Append(ticker.Low)
	builders[25].(*array.Float64Builder).Append(ticker.DailyChange)
	builders[26].(*array.Float64Builder).Append(ticker.DailyChangeRel)

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