# Data Controller

暗号通貨AI自動取引システムにおけるデータ取得・正規化・永続化を担う統合コントローラです。Bitfinexを皮切りに、WebSocketとRESTの両経路から高頻度データを吸い上げ、GUIからの操作で安全に蓄積・配信します。

---

## 📋 目次
- [概要](#概要)
- [システムアーキテクチャ](#システムアーキテクチャ)
- [ディレクトリ構成](#ディレクトリ構成)
- [設定](#設定)
- [主要コンポーネント](#主要コンポーネント)
- [GUIの構成](#guiの構成)
- [データストレージ](#データストレージ)
- [WebSocketメッセージフロー](#websocketメッセージフロー)
- [実装状況](#実装状況)
- [参考リンク](#参考リンク)

---

## 概要

### 役割
- **リアルタイム取得**: BitfinexのTicker / Trades / Books / Raw Books / Candles をWebSocketで同時取得し、自動再接続と状態復旧を実行
- **バッチ取得**: Candles / Trades / Tickers HistoryをREST API経由でダウンロード。GUIからのジョブ起動・監視に対応
- **正規化**: 通貨ペアやチャンネルごとの差異を統一スキーマにマッピングし、後段処理の共通化を実現
- **永続化**: Apache Arrow IPC形式の列指向ストレージへ保存し、後続分析・学習のデータレイクを構築
- **GUI制御**: Fyne v2.6で構築した専用GUIから、接続状態や実行ジョブを安全に制御・監視
- **設定管理**: ConfigManagerで設定の読み込み・自動更新・state永続化を実施

### 動作環境
- **言語**: Go 1.25.1
- **OS**: macOS (Apple Silicon) を想定
- **外部依存**:
  - Fyne v2.6.3 (GUI)
  - Apache Arrow Go v17 (データストレージ)
  - zap (ロギング)
  - golang.org/x/time/rate (レート制御)
- **データフォーマット**:
  - WebSocket → Arrow IPC (.arrow)
  - REST履歴 → CSV (.csv)
  - 設定 → YAML (.yml)
  - RESTキャッシュ → JSON (.json)

---

## システムアーキテクチャ

```
┌─────────────────────────────────────────────────────────────┐
│                        Data Controller                      │
├─────────────────────────────────────────────────────────────┤
│  WebSocket Manager     ─┐                                   │
│    ↳ Connection         │  realtime events                  │
│    ↳ Router             │  (ticker/trades/books/            │
│    ↳ SEQ_ALL Support    │   raw_books/candles)              │
│                         ├─► Handler ─► Arrow Writer         │
│  REST Data Orchestrator │  snapshots                        │
│    ↳ SafeRateLimiter    │  (candles/trades/tickers history) │
│    ↳ Backoff / Retry    ┘                                   │
│                                                             │
│  GUI (Fyne v2.6.3)                                          │
│    ↳ WebSocket Panel (5 tabs, flags control)                │
│    ↳ REST Data Panel v2 (3 tabs with datetime picker)       │
│    ↳ Files Viewer / Live Stream                             │
│                                                             │
│  Config & State Manager                                     │
│    ↳ YAML Loader / Exchange Profiles                        │
│    ↳ REST Config Fetcher / Auto-update                      │
│    ↳ State Persistence (GUI state snapshot)                 │
└─────────────────────────────────────────────────────────────┘
```

### データフロー

```
WebSocket:
Bitfinex WS → Connection (SEQ_ALL detection) → Router (channel routing)
  → Handler (buffering) → Arrow Writer (IPC format) → .arrow files

REST API:
GUI Job Trigger → SafeRateLimiter → BitfinexDataClient (retry/backoff)
  → CSV Writer → .csv files
```

---

## ディレクトリ構成

```
data_controller/
├── cmd/data-controller/         # エントリポイント
│   ├── main.go                  # GUI/CLI切り替え
│   ├── fyne_gui.go              # Fyneウィンドウ初期化
│   ├── gui.go                   # GUIビルド用実装
│   ├── gui_stub.go              # noguiビルド用スタブ
│   └── nogui.go                 # CLIモード実装
│
├── config/                      # 設定一式
│   ├── config.yml               # グローバル設定
│   ├── exchanges/
│   │   └── bitfinex.yml         # Bitfinex固有設定
│   ├── runtime/
│   │   └── state.yml            # 実行時状態（自動生成）
│   ├── state.yml                # GUI状態永続化
│   ├── schema.json              # 設定スキーマ定義
│   └── tmp/                     # 一時ファイル・ロック
│
├── data/bitfinex/               # 出力データ（.gitignore）
│   ├── websocket/               # WebSocketデータ（Arrow IPC形式）
│   │   ├── trades/
│   │   │   └── {symbol}/dt={date}/hour={HH}/seg={timestamp}/
│   │   │       └── part-trades-{symbol}-{timestamp}-seq.arrow
│   │   ├── books/
│   │   ├── raw_books/
│   │   ├── ticker/
│   │   └── candles/
│   └── restapi/
│       ├── data/                # REST履歴データ（CSV形式）
│       │   ├── candles/
│       │   ├── trades/
│       │   └── tickers/
│       └── config/              # REST設定キャッシュ（JSON）
│           ├── map_currency_label.json
│           ├── list_pair_exchange.json
│           └── conf_pub_info_{channel}.json
│
├── internal/
│   ├── config/                  # 設定管理
│   │   ├── config_manager.go   # メイン設定マネージャー
│   │   ├── exchange_config.go  # 取引所設定
│   │   ├── loader.go            # YAML読み込み
│   │   ├── normalizer.go        # 通貨ペア正規化
│   │   ├── rest_fetcher.go      # REST設定取得
│   │   ├── state.go             # 状態永続化
│   │   └── file_lock.go         # ファイルロック
│   │
│   ├── ws/                      # WebSocket処理
│   │   ├── conn.go              # 接続管理・メッセージ解析
│   │   └── router.go            # チャンネル別ルーティング
│   │
│   ├── sink/arrow/              # Arrow書き込み
│   │   ├── schema.go            # スキーマ定義（14共通+N固有フィールド）
│   │   ├── channel_writer.go   # チャンネル別ライター
│   │   ├── writer.go            # セグメント管理
│   │   ├── reader.go            # Arrow読み出し
│   │   └── handler.go           # イベントハンドラ
│   │
│   ├── restapi/                 # REST API処理
│   │   ├── bitfinex_data_client.go  # データ取得クライアント
│   │   ├── rate_limiter.go          # SafeRateLimiter実装
│   │   ├── arrow_storage.go         # Arrow保存（未使用）
│   │   └── utils.go                 # ユーティリティ
│   │
│   ├── gui/                     # GUI実装
│   │   ├── app/                 # アプリケーション構築
│   │   ├── websocket_panel.go   # WebSocketパネル
│   │   ├── rest_api_panel.go    # REST設定パネル
│   │   ├── rest_data_panel_v2.go    # RESTデータパネル
│   │   ├── rest_channel_*.go    # チャンネル別タブ
│   │   ├── channel_*.go         # WebSocketチャンネルタブ
│   │   ├── symbol_search.go     # シンボル検索UI
│   │   ├── datetime_picker.go   # 日時選択UI
│   │   ├── file_viewer.go       # ファイルビューア
│   │   ├── live_stream.go       # ライブストリーム
│   │   ├── data_files.go        # ファイル一覧
│   │   ├── controllers/         # ファイル操作コントローラ
│   │   ├── panels/              # 追加パネル
│   │   └── state/               # GUI状態管理
│   │
│   ├── services/                # サービス層
│   │   ├── config_refresh.go    # 設定自動更新
│   │   ├── file_scanner.go      # ファイルスキャン
│   │   └── file_reader.go       # ファイル読み出し
│   │
│   ├── domain/                  # ドメインモデル
│   │   └── file_item.go         # ファイルアイテム定義
│   │
│   └── metadata/                # メタデータ管理
│       └── refresh_state.go     # リフレッシュ状態
│
├── pkg/schema/                  # スキーマ定義
│   └── types.go                 # データ型定義
│
├── docs/                        # ドキュメント
│   ├── README.md               # 本ファイル
│   ├── WEBSOCKET_CONFIG.md     # WebSocket設定詳細
│   └── CLAUDE.md               # 開発メモ
│
├── examples/                    # サンプルコード
│   └── test_config_system.go   # 設定システムテスト
│
├── go.mod                       # Go依存管理
├── go.sum                       # 依存チェックサム
├── Makefile                     # ビルドコマンド
└── run.sh                       # 実行スクリプト
```

---

## 設定

### グローバル設定 (`config/config.yml`)
```yaml
app:
  title: "Data Controller"
  width: 2400
  height: 1300

storage:
  base_path: "./data"
  segment_size_mb: 256
  compression: "zstd"

logging:
  level: "debug"
  output: "stdout"

exchanges:
  default: "bitfinex"
  entries:
    - bitfinex
```

### Bitfinex設定 (`config/exchanges/bitfinex.yml`)
```yaml
websocket:
  url: "wss://api-pub.bitfinex.com/ws/2"
  conf_flags: 98304  # TIMESTAMP(32768) + SEQ_ALL(65536)

  channels:
    ticker:
      enabled: true
    trades:
      enabled: true
    books:
      enabled: true
      prec: "P0"
      freq: "F0"
      len: "25"
    raw_books:
      enabled: false
      prec: "R0"
      freq: "F0"
      len: "25"
    candles:
      enabled: true
      timeframe: "1m"

rest:
  base_url: "https://api-pub.bitfinex.com/v2"
  rate_limits:
    candles: 30  # req/min
    trades: 15
    tickers: 10
```

### 状態ファイル (`config/state.yml`)
- アクティブタブ、選択シンボル、接続フラグ、REST出力先、ウィンドウレイアウト等を永続化
- GUI終了時に自動保存
- 次回起動時に状態を復元

---

## 主要コンポーネント

### 1. WebSocket処理

#### Connection Manager (`internal/ws/conn.go`)
- **自動再接続**: 接続断時の指数バックオフ再接続
- **メッセージ解析**:
  - SEQ_ALL形式検出: `[CHANNEL_ID, DATA, SEQUENCE, TIMESTAMP]`
  - 通常形式: `[CHANNEL_ID, DATA]`
  - Heartbeat: `[CHANNEL_ID, "hb"]`
  - Checksum: `[CHANNEL_ID, "cs", CHECKSUM]` ⚠️ 検証未実装
- **チャンネル管理**: `chan_id` → チャンネル情報マッピング
- **サブスクリプションキュー**: 30枠制限の購読管理

#### Router (`internal/ws/router.go`)
- **チャンネル別ルーティング**:
  - `ticker` → Ticker
  - `trades` → Trade (snapshot/te/tu)
  - `book` (P0-P4) → BookLevel
  - `book` (R0) → RawBookEvent
  - `candles` → Candle
- **シーケンス番号管理**: SEQ_ALL対応
- **スナップショット検出**: 配列形式でのデータ判定

### 2. Arrow保存 (`internal/sink/arrow/`)

#### スキーマ設計

**共通フィールド (14フィールド):**
```go
exchange         string   // 取引所名
channel          string   // チャンネル名
symbol           string   // シンボル
pair_or_currency string   // ペア or 通貨
conn_id          string   // 接続ID
chan_id          int32    // チャンネルID（Bitfinex割り当て）
sub_id           *int64   // サブスクリプションID（optional）
conf_flags       int64    // 設定フラグ（TIMESTAMP/SEQ_ALL等）
seq              *int64   // シーケンス番号（SEQ_ALL時）
recv_ts          int64    // 受信タイムスタンプ（マイクロ秒）
batch_id         *int64   // バッチID（BULK時、未使用）
ingest_id        string   // 取り込みセッションID
source_file      string   // データソース（"websocket"固定）
line_no          *int64   // 行番号（ファイル再処理用、未使用）
```

**チャンネル固有フィールド:**
- **Ticker**: bid, bid_sz, ask, ask_sz, last, vol, high, low, daily_change, daily_change_rel
- **Trade**: trade_id, mts, amount, price, msg_type, is_snapshot
- **BookLevel**: price, count, amount, side, prec, freq, len, is_snapshot
- **RawBookEvent**: order_id, price, amount, op, side, is_snapshot
- **Candle**: mts, open, close, high, low, volume, timeframe, is_snapshot

#### ファイル構成
```
Hive-style partitioning:
{base_path}/{exchange}/{source}/{channel}/{symbol}/dt={YYYY-MM-DD}/hour={HH}/seg={start}--{end}--size~{MB}/
  └── part-{channel}-{symbol}-{timestamp}-seq.arrow
```

#### Writer (`writer.go`)
- セグメント単位でファイル管理（デフォルト256MB）
- 時間単位でディレクトリ分割
- zstd圧縮（設定可能）
- 一時ファイル → 最終ファイルのアトミックな書き込み

### 3. REST API処理

#### BitfinexDataClient (`internal/restapi/bitfinex_data_client.go`)
- **エンドポイント**:
  - `/v2/candles/trade:{timeframe}:{symbol}/hist`
  - `/v2/trades/{symbol}/hist`
  - `/v2/tickers/hist`
- **レート制限**: エンドポイント別の`rate.Limiter`
  - Candles: 30 req/min
  - Trades: 15 req/min
  - Tickers: 10 req/min
- **リトライ**: 指数バックオフ + `Retry-After`ヘッダー遵守（最大5回）
- **エラーハンドリング**: 429/ERR_RATE_LIMITで自動遅延

#### SafeRateLimiter (`rate_limiter.go`)
- 公式レート制限の80%で安全動作
- GUIジョブから利用

### 4. GUI (`internal/gui/`)

#### WebSocketパネル (`websocket_panel.go`)
- **5タブ構成**: Ticker / Trades / Books / Candles / Status
- **接続制御**:
  - Connect/Disconnectボタン
  - サブスクリプションカウンタ（30/30）
  - 接続状態表示
- **フラグ設定**:
  - Timestamp (32768)
  - Sequence (65536) - SEQ_ALL
  - Order Book Checksum (131072) - ⚠️ 検証未実装
  - Bulk Book Updates (536870912) - Books/RawBooks限定

#### RESTデータパネル v2 (`rest_data_panel_v2.go`)
- **3タブ構成**: Candles / Trades / Tickers
- **共通UI**:
  - シンボル検索（`symbol_search.go`）
  - 日時範囲選択（`datetime_picker.go`）
  - Limit/Sort設定
  - 実行ログ・進捗表示
- **ジョブ管理**: 同時実行・キャンセル対応

#### ファイルビューア (`file_viewer.go`)
- Arrow/CSVファイル一覧
- カテゴリ別フィルタ
- 最大3000レコード表示
- メタデータ表示

---

## データストレージ

### WebSocketデータ（Arrow IPC）

**保存形式**:
```
ファイル名save logicは次のようにカテゴリー(channel名ticker , books, raw_books etc)がファイル名の最初になるようにお願いします。:
{base=data}/{exchange}/{source}/{channel}/{symbol}/dt=YYYY-MM-DD/{channel}-{timestamp_started}.arrow

eg:'/Volumes/SSD/AI/Trade/TradeEngine2/data_controller/data/bitfinex/websocket/trades/tBTCGBP/dt=2025-10-04/trades-20251004T132846Z.arrow'
```

**特徴**:
- 列指向形式で高速クエリ
- zstd圧縮でストレージ効率化
- メタデータ付与（将来拡張予定）
- セグメント単位で256MB区切り

**フィールド最適化方針**:
- 接続固定値（`exchange`, `channel`, `conn_id`等）→ 将来的にメタデータ化を検討
- イベント固有値（`seq`, `recv_ts`, データフィールド）→ データとして保持

### RESTデータ（CSV）

**保存形式**:
```
data/bitfinex/restapi/data/{channel}/{job_timestamp}_{symbol}_{params}.csv
```

**ヘッダー**:
- **Candles**: `mts,open,close,high,low,volume`
- **Trades**: `id,mts,amount,price`
- **Tickers**: `symbol,bid,ask,mts`

---

## WebSocketメッセージフロー

### 通常メッセージ
```
[CHANNEL_ID, DATA]
  ↓
conn.go: handleMessage()
  ↓ chan_id → ChannelInfo lookup
router.go: RouteMessage()
  ↓ channel type routing
handler.go: Handle{Ticker|Trade|Book|...}()
  ↓
writer.go: Write{Ticker|Trade|Book|...}()
  ↓
channel_writer.go: write{ticker|trade|...}()
  ↓
.arrow file
```

### SEQ_ALL対応メッセージ
```
[CHANNEL_ID, DATA, SEQUENCE, TIMESTAMP]
  ↓
conn.go: SEQ_ALL detection (array length == 4 && array[2] is int64)
  ↓
handleDataMessageWithSeq(chanID, seq, data)
  ↓
router.go: RouteMessageWithSeq(seq)
  ↓
CommonFields.Seq = seq
  ↓
.arrow file (seq field populated)
```

### Checksum処理（⚠️ 未実装）
```
[CHANNEL_ID, "cs", CHECKSUM, SEQ, TIMESTAMP]
  ↓
conn.go: handleChecksum()
  ↓
⚠️ Debug log only - 検証ロジックなし
```

**必要な実装**:
1. Order Book状態管理
2. CRC-32計算
3. サーバーチェックサムとの比較
4. 不一致時の警告/再接続

---

## 実装状況

| 項目                                   | 状態 | 補足                                   |
| -------------------------------------- | ---- | -------------------------------------- |
| **WebSocket購読**                      |      |                                        |
| - Ticker/Trades/Books/RawBooks/Candles | ✅    | R0精度でRawBooksルーティング           |
| - SEQ_ALL対応                          | ✅    | シーケンス番号の検出・保存             |
| - Checksum検証                         | ⚠️    | メッセージ受信のみ、検証ロジック未実装 |
| - 自動再接続                           | ✅    | 指数バックオフ                         |
| **Arrow保存**                          |      |                                        |
| - チャンネル別スキーマ                 | ✅    | 14共通+N固有フィールド                 |
| - セグメント管理                       | ✅    | 256MB単位、時間別分割                  |
| - メタデータ活用                       | 🚧    | 将来的に冗長フィールドをメタデータ化   |
| **REST API**                           |      |                                        |
| - 設定キャッシュ更新                   | ✅    | 強制/自動更新対応                      |
| - 履歴取得（Candles/Trades/Tickers）   | ✅    | SafeRateLimiter + Backoff              |
| - Arrow保存                            | ❌    | CSV保存のみ実装                        |
| **GUI**                                |      |                                        |
| - WebSocketパネル                      | ✅    | 5タブ、フラグ制御                      |
| - RESTデータパネル v2                  | ✅    | 3タブ、datetime picker                 |
| - ファイルビューア                     | ✅    | 3000レコード表示                       |
| - 状態永続化                           | ✅    | state.yml自動保存                      |
| **その他**                             |      |                                        |
| - Freqtrade連携                        | 🚧    | 将来拡張予定                           |
| - 追加取引所サポート                   | 🚧    | Bitfinex完成後に拡張                   |

---

## 既知の問題と改善点

### 1. Checksum検証未実装
- Order Book状態管理が存在しない
- CRC-32計算ロジックがない
- チェックサムフラグON時も検証が行われない

### 2. 冗長フィールド
以下のフィールドは接続単位で固定のため、メタデータ化を検討：
- `exchange`, `channel`: ファイルパスに含まれる
- `conn_id`, `ingest_id`: セッション固定
- `conf_flags`: サブスクリプション設定固定
- `source_file`: 常に"websocket"固定
- `line_no`: 未使用（常にnull）
- `chan_id`: 接続セッション固有の一時ID

### 3. batch_id未使用
- BULK設定はBooks/RawBooksのみ対応
- Trades/Ticker/Candlesでは常にnull
- チャンネル別スキーマ化を検討

### 4. 複数バックグラウンドプロセス
- GUI終了時にプロセスが残留する問題
- 適切なシャットダウン処理の実装が必要

---

## 参考リンク

### Bitfinex API
- [WebSocket API ドキュメント](https://docs.bitfinex.com/docs/ws-public)
- [REST API v2 ドキュメント](https://docs.bitfinex.com/docs/rest-public)
- [WebSocket Checksum](https://docs.bitfinex.com/docs/ws-websocket-checksum)
- [Conf Flags](https://docs.bitfinex.com/docs/ws-conf-flags)

### 技術スタック
- [Apache Arrow Go](https://pkg.go.dev/github.com/apache/arrow/go/v17)
- [Fyne v2.6 API](https://docs.fyne.io/api/v2.6/)
- [Go Rate Limiter](https://pkg.go.dev/golang.org/x/time/rate)
- [Zap Logger](https://pkg.go.dev/go.uber.org/zap)

### プロジェクト
- [開発メモ (CLAUDE.md)](./CLAUDE.md)
- [WebSocket設定詳細 (WEBSOCKET_CONFIG.md)](./WEBSOCKET_CONFIG.md)

---

**最終更新**: 2025-10-03
**バージョン**: v0.2.0
**Go**: 1.25.1
**Fyne**: v2.6.3
