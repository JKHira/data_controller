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
- [データ保存とレート制御](#データ保存とレート制御)
- [実装状況](#実装状況)
- [参考リンク](#参考リンク)

---

## 概要

### 役割
- **リアルタイム取得**: BitfinexのTicker / Trades / Books / Raw Books / StatusをWebSocketで同時取得し、自動再接続と状態復旧を行います。
- **バッチ取得**: Candles / Trades / Tickers HistoryをREST API経由でダウンロード。GUIからのジョブ起動・監視に対応。
- **正規化**: 通貨ペアやチャンネルごとの差異を統一スキーマにマッピングし、後段処理の共通化を図ります。
- **永続化**: Apache Arrowベースの列指向ストレージへ保存し、後続分析・学習のデータレイクを構築。
- **GUI制御**: Fyne 2.6で構築した専用GUIから、接続状態や実行ジョブを安全に制御・監視。
- **設定管理**: ConfigManagerとConfigRefreshManagerで設定の読み込み・自動更新・state永続化を実施。

### 動作環境
- **言語**: Go 1.25+
- **OS**: macOS (Apple Silicon) を想定
- **外部依存**: Fyne v2.6, Apache Arrow Go, zap, rate
- **データフォーマット**: Arrow (websocket), CSV (REST履歴), YAML (設定), JSON (RESTキャッシュ)

---

## システムアーキテクチャ

```
┌────────────────────────────────────────────────────────┐
│                        Data Controller                 │
├────────────────────────────────────────────────────────┤
│  WebSocket Manager    ─┐                                 │
│    ↳ Bitfinex WS       │  realtime events (ticker/trade/ │
│    ↳ Raw Books         │  books/raw_books/status)        │
│                         ├─► Router ─► Normalizer ─► Arrow│
│  REST Data Orchestrator│  snapshots (candles/trades/     │
│    ↳ SafeRateLimiter   │  tickers history)               │
│    ↳ Backoff / Retry   ┘                                 │
│                                                            │
│  GUI (Fyne)                                               │
│    ↳ WebSocket Panel (connect, flags, counter)            │
│    ↳ REST Data Panel v2 (Candles/Trades/Tickers tabs)     │
│    ↳ Files / Viewer / Live Stream                         │
│                                                            │
│  Config & State Manager                                   │
│    ↳ YAML Loader / Profiles                               │
│    ↳ REST Config Fetcher / Scheduler                      │
└────────────────────────────────────────────────────────┘
```

---

## ディレクトリ構成

```
data_controller/
├── cmd/data-controller/         # エントリポイント
│   ├── main.go                  # GUI/CLI切り替え
│   └── fyne_gui.go              # ウィンドウ初期化
├── config/                      # 設定一式
│   ├── config.yml               # グローバル設定
│   ├── bitfinex_config.yml      # Bitfinex向け詳細設定 (channels.raw_booksを含む)
│   ├── exchanges/               # 取引所別プロファイル
│   ├── state/state.yml          # GUI/接続状態のスナップショット
│   ├── backups/                 # 自動バックアップ
│   └── tmp/update.lock          # 排他制御用ロック
├── data/bitfinex/               # 出力データ
│   ├── websocket/               # Arrowファイル
│   │   ├── trades/
│   │   ├── books/
│   │   ├── raw_books/
│   │   └── ticker/
│   └── restapi/
│       ├── data/                # Candles/Trades/Tickers CSV
│       └── config/              # REST設定キャッシュ
├── internal/
│   ├── config/                  # ConfigManager / Normalizer / State管理
│   ├── gui/                     # GUI実装 (Fyne)
│   │   ├── app/                 # アプリ構築ロジック
│   │   ├── rest_data_panel_v2.go
│   │   ├── rest_channel_{candles,trades,tickers}.go
│   │   ├── symbol_search.go / datetime_picker.go
│   │   └── websocket_panel.go
│   ├── restapi/                 # RESTクライアント
│   │   ├── bitfinex_data_client.go
│   │   └── rate_limiter.go      # SafeRateLimiter
│   ├── sink/arrow/              # Arrow書き込み/読み出し
│   ├── services/                # ConfigRefreshManager 等
│   └── ws/                      # ConnectionManager / Router / RawBooks処理
├── docs/                        # ドキュメント
│   └── README.md (本ファイル)
├── go.mod                       # Go依存
└── Makefile                     # ビルド支援
```

---

## 設定

### グローバル設定 (`config/config.yml`)
- GUIタイトル・固定ウィンドウサイズ (2400x1300)
- データ保存パス・圧縮方式・ログレベル
- 取引所プロファイルの選択 (`exchanges.default` / `entries`)

### Bitfinex設定 (`config/bitfinex_config.yml`)
- `websocket.conf_flags`: TIMESTAMP, SEQ_ALL, OB_CHECKSUM, BULK_UPDATES の複合値
- `channels`:
  - `ticker`, `trades`, `books` の有効/頻度指定
  - `raw_books`: R0精度・F0頻度・長さを指定 (Raw Books購読を有効化)
- `symbols`: 初期購読リスト (例: `tBTCUSD`, `tETHUSD`)

### 状態ファイル (`config/state/state.yml`)
- アクティブタブ、選択シンボル、接続フラグ、REST出力先、ウィンドウレイアウト等を永続化
- GUI終了時に `ConfigManager.Shutdown()` が書き戻し

---

## 主要コンポーネント

### 1. Config & State Management
- `internal/config/config_manager.go`: YAML読み込み、状態保存、REST設定キャッシュ管理
- `services.ConfigRefreshManager`: REST設定エンドポイントの強制/自動更新と結果集計
- `normalizer.go`: 取引所固有の通貨ペアを内部形式へ相互変換

### 2. WebSocket Ingestion
- `ws.ConnectionManager`: 自動再接続、サブスクキュー、Raw Books/Books両対応 (`prec=R0`でRaw Books判定)
- `ws.Router`: Ticker/Trades/Books/RawBooks/Statusをチャンネル別に解釈し、共通スキーマへマッピング
- `sink/arrow.Handler`: Routerから渡されたイベントをArrowファイルへストリーム書き込み
- GUIは `websocket_panel.go` から接続要求を発行し、SubscribeRequestとConf Flagsを計算

### 3. REST Data Acquisition
- `gui/rest_api_panel.go`: ConfigタブとDataタブの2層構成。Configタブから設定キャッシュを更新。
- `gui/rest_data_panel_v2.go`: Candles / Trades / Tickers Historyを同時実行可能なタブUI
  - シンボル検索 (`symbol_search.go`)、時間範囲入力 (`datetime_picker.go`)、Limit/Sort指定
  - 実行ログやアクティビティ表示、ジョブキャンセル、出力ディレクトリ変更に対応
- `restapi/bitfinex_data_client.go`: Candles/Trades/Tickers APIクライアント
  - エンドポイントごとの `rate.Limiter` (30/15/10 req/min) を定義
  - `429` や `ERR_RATE_LIMIT` 応答で指数バックオフ & `Retry-After` ヘッダー遵守
- `restapi/rate_limiter.go`: 安全率20%を加味した `SafeRateLimiter`。GUIジョブ実行時に使用

### 4. 永続化・ファイル操作
- `sink/arrow/`: チャンネルごとのセグメント管理、圧縮設定、メタデータ付与
- `services/file_scanner.go` / `file_reader.go`: GUIファイルブラウザ向け列挙・読み出し
- `internal/domain/file_item.go`: カテゴリ (trades/books/raw_books 等) を付与

---

## GUIの構成

### WebSocketパネル (`internal/gui/websocket_panel.go`)
- Ticker/Trades/Books/Candles/Statusの5タブをAppTabsで切り替え
- サブスクリプションカウンタ (30枠) とConnect/Disconnectボタンを下部に固定表示
- 接続フラグ (Timestamp / Sequence / Checksum / Bulk) をGUI上で切替可能
- Raw Booksを含むシンボル選択・制限チェック (`limitChecker`) による安全な購読制御
- ステータスメッセージは必要時のみ表示

### RESTデータパネル v2
- Candles / Trades / Tickers History の3タブ (有効化チェック付き)
- シンボル検索、時間範囲の即時バリデーション、Limitスライダ (Candles最大10,000, Tickers最大250)
- SafeRateLimiterでAPI呼び出しを制御しつつ、成功/失敗をアクティビティログに記録
- CSV出力は `storage.base_path/bitfinex/restapi/data/` 配下でジョブごとにタイムスタンプ保存

### ファイルパネル / ビューア
- Arrow/CSVファイルのカテゴリ別一覧とメタデータ表示
- ライブストリーム (`live_stream.go`) はRaw Booksを含めたリアルタイムイベントを簡易表示

---

## データ保存とレート制御
- **WebSocket**: Arrowファイルはチャンネル×シンボルでセグメント化し、メタデータ (`exchange`, `symbol`, `channel`, `start_time` 等) を付与
- **REST**: CSVヘッダはエンドポイントに合わせて最小構成に整理 (Tickersは `symbol,bid,ask,mts` のみ)
- **レート制限**: `SafeRateLimiter` が Candles 24/min, Trades 12/min, Tickers 8/min の安全ラインを提供
- **リトライ**: `BitfinexDataClient` の `doRequest` が指数バックオフ + `Retry-After` に対応、最大5回再試行

---

## 実装状況

| 項目 | 状態 | 補足 |
| ---- | ---- | ---- |
| WebSocket購読 (Ticker/Trades/Books/RawBooks/Status) | ✅ | Raw BooksはR0購読でルーティング済み |
| REST設定キャッシュ更新 | ✅ | Configタブから強制実行、結果はログ/GUIに集計 |
| REST履歴取得 (Candles/Trades/Tickers) | ✅ | GUIジョブ + SafeRateLimiter + Backoff |
| Arrow保存 & ファイルブラウザ | ✅ | trades/books/raw_books/tickerをカテゴリ分け表示 |
| Freqtrade等トレード制御 | 🚧 | API呼び出しは将来拡張予定 |
| 追加取引所サポート | 🚧 | Bitfinexをベースに拡張計画 |

---

## 参考リンク

- [Bitfinex WebSocket API](https://docs.bitfinex.com/docs/ws-public)
- [Bitfinex REST API](https://docs.bitfinex.com/docs/rest-public)
- [Apache Arrow](https://arrow.apache.org/)
- [Fyne Documentation](https://docs.fyne.io/api/v2.6/)

---

最終更新: 2025-10-01
