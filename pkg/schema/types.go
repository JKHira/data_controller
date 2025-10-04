package schema

import "time"

type Exchange string

const (
	ExchangeBitfinex Exchange = "bitfinex"
)

type Channel string

const (
	ChannelTicker   Channel = "ticker"
	ChannelTrades   Channel = "trades"
	ChannelBooks    Channel = "books"
	ChannelRawBooks Channel = "raw_books"
	ChannelCandles  Channel = "candles"
)

type MessageType string

const (
	MessageTypeTE MessageType = "te"
	MessageTypeTU MessageType = "tu"
	MessageTypeHB MessageType = "hb"
	MessageTypeCS MessageType = "cs"
)

type Side string

const (
	SideBid Side = "bid"
	SideAsk Side = "ask"
)

type Operation string

const (
	OperationUpsert Operation = "upsert"
	OperationDelete Operation = "delete"
)

type CommonFields struct {
	Symbol         string  `parquet:"symbol,plain"`
	PairOrCurrency string  `parquet:"pair_or_currency,plain"`
	Seq            *int64  `parquet:"seq,optional"`
	RecvTS         int64   `parquet:"recv_ts,plain"`
	ChanID         int32   `parquet:"-"`
	Channel        Channel `parquet:"-"`
	ChannelKey     string  `parquet:"-"`
	Timeframe      string  `parquet:"-"`
	BookPrec       string  `parquet:"-"`
	BookFreq       string  `parquet:"-"`
	BookLen        string  `parquet:"-"`
}

type ChannelMetadata struct {
	Channel   Channel
	Symbol    string
	Pair      string
	Key       string
	ChanID    int32
	Timeframe string
	BookPrec  string
	BookFreq  string
	BookLen   string
}

type RawBookEvent struct {
	CommonFields
	BatchID    *int64    `parquet:"batch_id,optional"`
	OrderID    int64     `parquet:"order_id,plain"`
	Price      float64   `parquet:"price,plain"`
	Amount     float64   `parquet:"amount,plain"`
	Op         Operation `parquet:"op,plain"`
	Side       Side      `parquet:"side,plain"`
	IsSnapshot bool      `parquet:"is_snapshot,plain"`
}

type BookLevel struct {
	CommonFields
	BatchID    *int64  `parquet:"batch_id,optional"`
	Price      float64 `parquet:"price,plain"`
	Count      int32   `parquet:"count,plain"`
	Amount     float64 `parquet:"amount,plain"`
	Side       Side    `parquet:"side,plain"`
	Prec       string  `parquet:"prec,plain"`
	Freq       string  `parquet:"freq,plain"`
	Len        int32   `parquet:"len,plain"`
	IsSnapshot bool    `parquet:"is_snapshot,plain"`
}

type Trade struct {
	CommonFields
	TradeID    int64       `parquet:"trade_id,plain"`
	MTS        int64       `parquet:"mts,plain"`
	Amount     float64     `parquet:"amount,plain"`
	Price      float64     `parquet:"price,plain"`
	MsgType    MessageType `parquet:"msg_type,plain"`
	IsSnapshot bool        `parquet:"is_snapshot,plain"`
}

type Ticker struct {
	CommonFields
	Bid            float64 `parquet:"bid,plain"`
	BidSize        float64 `parquet:"bid_sz,plain"`
	Ask            float64 `parquet:"ask,plain"`
	AskSize        float64 `parquet:"ask_sz,plain"`
	Last           float64 `parquet:"last,plain"`
	Vol            float64 `parquet:"vol,plain"`
	High           float64 `parquet:"high,plain"`
	Low            float64 `parquet:"low,plain"`
	DailyChange    float64 `parquet:"daily_change,plain"`
	DailyChangeRel float64 `parquet:"daily_change_rel,plain"`
}

type Candle struct {
	CommonFields
	MTS        int64   `parquet:"mts,plain"`
	Open       float64 `parquet:"open,plain"`
	Close      float64 `parquet:"close,plain"`
	High       float64 `parquet:"high,plain"`
	Low        float64 `parquet:"low,plain"`
	Volume     float64 `parquet:"volume,plain"`
	Timeframe  string  `parquet:"timeframe,plain"`
	IsSnapshot bool    `parquet:"is_snapshot,plain"`
}

type Control struct {
	CommonFields
	Type      string    `parquet:"type,plain"`
	Reason    string    `parquet:"reason,plain"`
	Checksum  *int32    `parquet:"checksum,optional"`
	LastSeq   *int64    `parquet:"last_seq,optional"`
	Timestamp time.Time `parquet:"timestamp,timestamp(millis)"`
}

type SegmentManifest struct {
	SchemaVersion  string            `json:"schema_version"`
	Exchange       string            `json:"exchange"`
	Channel        string            `json:"channel"`
	Symbol         string            `json:"symbol"`
	PairOrCurrency string            `json:"pair_or_currency"`
	WSURL          string            `json:"ws_url"`
	ConnID         string            `json:"conn_id"`
	ChanID         int32             `json:"chan_id"`
	SubID          *int64            `json:"sub_id,omitempty"`
	ConfFlags      int64             `json:"conf_flags"`
	Book           *BookSubscription `json:"book,omitempty"`
	Segment        SegmentInfo       `json:"segment"`
	Seq            *SeqInfo          `json:"seq,omitempty"`
	Quality        QualityMetrics    `json:"quality"`
}

type BookSubscription struct {
	Prec string `json:"prec"`
	Freq string `json:"freq"`
	Len  int    `json:"len"`
}

type SegmentInfo struct {
	BytesTarget int64     `json:"bytes_target"`
	UTCStart    time.Time `json:"utc_start"`
	UTCEnd      time.Time `json:"utc_end"`
	Files       []string  `json:"files"`
}

type SeqInfo struct {
	First int64 `json:"first"`
	Last  int64 `json:"last"`
}

type QualityMetrics struct {
	ChecksumMismatch        int `json:"checksum_mismatch"`
	HBMissed                int `json:"hb_missed"`
	Reconnects              int `json:"reconnects"`
	TradesDedupDropped      int `json:"trades_dedup_dropped"`
	BookUpdatesDedupDropped int `json:"book_updates_dedup_dropped"`
}
