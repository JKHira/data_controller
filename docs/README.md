# Data Controller

暗号通貨AI自動取引システムの中核となるデータ管理・制御システム

---

## 📋 目次

- [概要](#概要)
- [システム構成](#システム構成)
- [ディレクトリ構造](#ディレクトリ構造)
- [設定ファイル](#設定ファイル)
- [主要コンポーネント](#主要コンポーネント)
- [WebSocket設定システム](#websocket設定システム)
- [GUI構成](#gui構成)
- [データ保存](#データ保存)
- [実装チェックリスト](#実装チェックリスト)
- [使用予定AIモデル](#使用予定aiモデル)
- [参考リンク](#参考リンク)

---

## 概要

**Data Controller**は、暗号通貨取引所からリアルタイムデータを取得し、AI予測モデルへの入力データを生成、取引判断を実行するための統合制御システムです。

### 主要機能

- **リアルタイムデータ取得**: WebSocketによる複数取引所からの並行データ取得
- **データ正規化**: 取引所ごとの差異を吸収する統一フォーマット
- **状態管理**: 接続状態・購読情報の永続化
- **GUI制御**: Fyne製の直感的なユーザーインターフェース
- **高性能保存**: Apache Arrowによる列指向データ保存
- **自動設定更新**: REST APIからの動的設定取得

### 開発環境

- **言語**: Go 1.25+
- **システム**: macOS (Apple Silicon) / M1 Max 64GB
- **GUI**: Fyne v2.6
- **データ形式**: Apache Arrow
- **設定**: YAML

---

## システム構成

### Trade Engine（常駐1プロセス）

```
┌─────────────────────────────────────────────────────────┐
│                     Data Controller                      │
├─────────────────────────────────────────────────────────┤
│ WS取込 → 正規化 → 特徴生成 → MLX/CPU計算 → トレード制御 │
│   ↓                                          ↓           │
│ Apache Arrow保存                     Freqtrade連携      │
└─────────────────────────────────────────────────────────┘
```

#### 主要処理フロー

1. **WS取込**: 取引所WebSocketから並行データ取得（自動再接続・ハートビート）
2. **正規化**: 取引所ごとの表記差異を統一（例: `tBTCUSD` → `BTC-USD`）
3. **特徴生成**: 1-5秒でOFI/スプレッド分位を固定長float32配列化
4. **MLX/CPU計算**:
   - 最速: cgoでCラッパ関数直接呼び出し
   - 次善: gRPC/UDSで常駐MLXサーバにストリーミング
5. **トレード制御**: Freqtrade REST APIでforcebuy/forcesell/start/stop
6. **保存**: Apache Arrow（生L2データ） + InfluxDB（特徴量・要約）

### 可視化

- **Grafana**: InfluxDB/Prometheusデータの可視化
- **Fyne GUI**: ユーザー入力・パラメータ変更用の軽量UI

### 起動・復旧

- **launchd**: 自動再起動・ログ回収（macOS）

---

## ディレクトリ構造

```
data_controller/
├── cmd/                              # エントリーポイント
│   └── data-controller/
│       ├── main.go                  # メイン起動ロジック
│       ├── fyne_gui.go              # GUI初期化
│       ├── gui.go                   # GUI有効化フラグ
│       └── nogui.go                 # CLI実行ロジック
│
├── config/                          # 設定ファイル
│   ├── config.yml                   # グローバル設定
│   ├── bitfinex_config.yml          # レガシーBitfinex設定
│   ├── exchanges/                   # 取引所固有設定
│   │   └── bitfinex_config.yml     # 新Bitfinex設定（エンドポイント・制限）
│   ├── profiles/                    # ユーザー定義プロファイル（オプション）
│   ├── state/                       # アプリケーション状態
│   │   └── state.yml               # 接続状態・購読情報・UI状態
│   ├── backups/                     # 自動設定バックアップ
│   └── tmp/                         # 一時ファイル・ロック
│       └── update.lock             # 設定更新時の排他制御
│
├── data/                            # データ保存先
│   └── bitfinex/
│       ├── websocket/               # WebSocketデータ
│       │   ├── trades/             # 取引データ（Arrow形式）
│       │   ├── books/              # 板データ（Arrow形式）
│       │   └── ticker/             # ティッカーデータ（Arrow形式）
│       └── restapi/                 # REST APIデータ
│           └── config/              # 取引所設定キャッシュ
│               ├── list_pair_exchange.json      # スポット取引ペア
│               ├── list_pair_margin.json        # マージン取引ペア
│               ├── list_pair_futures.json       # 先物契約
│               ├── map_currency_label.json      # 通貨正式名称
│               └── *.json                       # その他設定
│
├── internal/                        # 内部パッケージ
│   ├── config/                      # 設定管理
│   │   ├── config.go               # 設定構造体定義
│   │   ├── loader.go               # YAML設定読み込み
│   │   ├── config_manager.go       # 設定ライフサイクル管理
│   │   ├── state.go                # アプリケーション状態管理
│   │   ├── normalizer.go           # 通貨ペア正規化エンジン
│   │   ├── exchange_config.go      # 取引所設定構造体
│   │   ├── file_lock.go            # ファイルロック機構
│   │   └── rest_fetcher.go         # REST API取得アダプター
│   │
│   ├── domain/                      # ドメインモデル
│   │   └── file_item.go            # ファイル項目モデル
│   │
│   ├── gui/                         # GUIシステム
│   │   ├── app/                    # アプリケーション層
│   │   │   └── app.go              # メインアプリ構造
│   │   ├── controllers/            # コントローラー層
│   │   │   └── file_controller.go  # ファイル操作ロジック
│   │   ├── panels/                 # プレゼンテーション層
│   │   │   ├── files_panel.go      # ファイル一覧パネル
│   │   │   └── viewer_panel.go     # ファイルビューアー
│   │   ├── state/                  # GUI状態管理
│   │   │   └── app_state.go        # GUI状態
│   │   ├── panes_v2.go             # WebSocket/REST パネル構成
│   │   ├── rest_api_panel.go       # REST Config + Data タブ
│   │   ├── rest_data_panel.go      # RESTデータ取得UI
│   │   ├── websocket_panel.go      # WebSocket設定パネル
│   │   ├── channel_ticker.go       # Tickerチャンネル設定
│   │   ├── channel_trades.go       # Tradesチャンネル設定
│   │   ├── channel_books.go        # Booksチャンネル設定
│   │   ├── channel_candles.go      # Candlesチャンネル設定
│   │   ├── channel_status.go       # Statusチャンネル設定
│   │   ├── rest_api_panel.go       # REST API操作パネル
│   │   ├── top_bar.go              # 上部バー
│   │   ├── bottom_bar.go           # 下部ステータスバー
│   │   ├── data_files.go           # データファイルパネル
│   │   ├── file_viewer.go          # ファイル表示共通機能
│   │   └── live_stream.go          # リアルタイム表示
│   │
│   ├── ws/                          # WebSocket実装
│   │   ├── conn.go                 # WebSocket接続管理
│   │   └── router.go               # メッセージルーティング
│   │
│   ├── restapi/                     # REST APIクライアント
│   │   ├── bitfinex_client.go      # Bitfinex API実装
│   │   ├── arrow_storage.go        # Arrowストレージ
│   │   └── utils.go                # ユーティリティ
│   │
│   ├── sink/arrow/                  # データ保存エンジン
│   │   ├── writer.go               # Arrowライター
│   │   ├── reader.go               # Arrowリーダー
│   │   ├── schema.go               # スキーマ定義
│   │   ├── handler.go              # データハンドラー
│   │   └── channel_writer.go       # チャンネル別ライター
│   │
│   └── services/                    # ビジネスロジック
│       ├── file_reader.go          # ファイル読み込み
│       ├── file_scanner.go         # ファイルスキャン
│       └── config_refresh.go       # 設定更新サービス
│
├── examples/                        # サンプル・テスト
│   └── test_config_system.go       # 設定システムテスト
│
├── docs/                            # ドキュメント
│   └── WEBSOCKET_CONFIG.md         # WebSocket設定システムドキュメント
│
├── CLAUDE.md                        # AI開発指針
├── README.md                        # 本ファイル
├── go.mod                           # Go依存関係
└── Makefile                         # ビルド設定
```

---

## 設定ファイル

### グローバル設定 (`config/config.yml`)

アプリケーション全体の基本設定

```yaml
application:
  name: "data-controller"
  version: "1.0.0"
  log_level: "debug"

storage:
  base_path: "/path/to/data"
  compression: "zstd"

gui:
  title: "Data Controller"
  width: 800
  height: 600
  theme: "dark"

exchanges:
  default: "bitfinex"
  entries:
    bitfinex:
      default_profile: "default"
      active_profile: "default"
```

**主要項目:**
- `application`: アプリ名・バージョン・ログレベル
- `storage`: データ保存先パス・圧縮設定
- `gui`: GUI設定（サイズ・テーマ）
- `exchanges`: 取引所プロファイル管理

### 取引所設定 (`config/exchanges/bitfinex_config.yml`)

取引所固有の仕様・制限・デフォルト値

```yaml
endpoints:
  ws_public: "wss://api-pub.bitfinex.com/ws/2"
  rest_public: "https://api-pub.bitfinex.com/v2"

limits:
  ws_max_subscriptions: 30      # 最大購読チャンネル数
  rest_rate_limit: 90           # REST APIレート制限（req/分）

defaults:
  book:
    prec: "P0"                  # デフォルト精度
    freq: "F0"                  # デフォルト頻度
    len: "25"                   # デフォルト長さ

rest_config_endpoints:
  - endpoint: "pub:list:pair:exchange"
    cache_duration: 3600        # キャッシュ有効期間（秒）
    file: "list_pair_exchange.json"
```

**主要項目:**
- `endpoints`: WebSocket/REST APIのURL
- `limits`: 接続制限・レート制限
- `defaults`: チャンネルごとのデフォルト値
- `rest_config_endpoints`: 自動取得する設定エンドポイント

### 状態ファイル (`config/state/state.yml`)

アプリケーションの実行時状態（自動管理）

```yaml
exchanges:
  bitfinex:
    ws:
      connections:
        - id: "conn_1"
          status: "connected"
          subscriptions:
            - channel: "ticker"
              symbol: "tBTCUSD"
              chanId: 12345
      ui_state:
        active_tab: "books"
        selected_symbols: ["tBTCUSD", "tETHUSD"]
        connection_flags:
          checksum: true
          timestamp: true
    rest_config_cache:
      last_updated:
        "pub:list:pair:exchange": "2025-09-30T10:00:00Z"
```

**主要項目:**
- `connections`: WebSocket接続状態・購読情報
- `ui_state`: UIの状態（アクティブタブ・選択シンボル）
- `rest_config_cache`: REST設定の更新時刻

---

## 主要コンポーネント

### 1. 設定管理 (`internal/config/`)

#### ConfigManager (`config_manager.go`)
設定のライフサイクル全体を管理

**主要機能:**
- 設定ファイルの読み込み・初期化
- REST APIからの自動設定取得
- 周期的な設定更新スケジューリング
- 状態の保存・復元

**使用例:**
```go
configManager := config.NewConfigManager(logger, basePath, restClient)
configManager.Initialize("bitfinex")
configManager.RefreshConfigOnConnect("bitfinex")
```

#### Normalizer (`normalizer.go`)
取引所ごとの表記差異を統一

**主要機能:**
- 通貨ペアの正規化（`tBTCUSD` → `BTC-USD`）
- 逆正規化（`BTC-USD` → `tBTCUSD`）
- 通貨の正式名称取得（`BTC` → `Bitcoin`）

**対応形式:**
- Trading pairs: `tBTCUSD`, `tETHUSD`
- Colon format: `AVAX:BTC`, `BTC:USD`
- Funding: `fUSD` → `USD-USD`
- Futures: `BTCF0:USTF0`

#### ApplicationState (`state.go`)
スレッドセーフな状態管理

**主要機能:**
- 接続状態の追跡
- 購読情報の管理
- UI状態の永続化
- REST設定キャッシュ管理

#### FileLock (`file_lock.go`)
設定ファイル更新の排他制御

**主要機能:**
- Flock-based ファイルロック
- タイムアウト付き排他制御
- ロック情報のJSON記録

### 2. WebSocket管理 (`internal/ws/`)

#### ConnectionManager (`conn.go`)
WebSocket接続の管理

**主要機能:**
- 自動再接続
- ハートビート監視
- 購読キュー管理
- チャンネル情報追跡

#### Router (`router.go`)
WebSocketメッセージのルーティング

**主要機能:**
- メッセージタイプ判定
- 適切なハンドラーへの振り分け
- エラーハンドリング

### 3. データ保存 (`internal/sink/arrow/`)

#### Writer (`writer.go`)
Apache Arrow形式でのデータ書き込み

**主要機能:**
- 高速列指向書き込み
- 自動ファイル分割（サイズ・時間ベース）
- 圧縮対応（zstd/gzip/snappy）

#### Reader (`reader.go`)
Apache Arrowファイルの読み込み

**主要機能:**
- メタデータスキャン
- ページング読み込み
- フィルタリング対応

### 4. REST API (`internal/restapi/`)

#### BitfinexClient (`bitfinex_client.go`)
Bitfinex REST APIクライアント

**主要機能:**
- 設定データ取得（ペア一覧・通貨情報）
- レート制限対応
- エラーハンドリング

---

## WebSocket設定システム

### 概要

タブ化されたUIで直感的にWebSocket購読を設定できるシステム

**主要機能:**
- 5つのチャンネルタイプ（Ticker/Trades/Books/Candles/Status）
- 30チャンネル購読制限のリアルタイム監視
- 接続時の自動設定更新（RESTキャッシュのホットリロード）
- 接続フラグ（Timestamp / Sequence / Checksum / Bulk）のワンクリック切り替え
- 状態の永続化（選択内容・タブ・接続設定）
- キャッシュ未取得時の「No data」ダイアログと即時取得

### GUI構成

#### WebSocketPanel (`internal/gui/websocket_panel.go`)
メインパネル - タブとサブスクリプションカウンター

- Bitfinexタブのデータソースは `config.ConfigManager` を介して `data/bitfinex/restapi/config/*.json` を参照
- 接続フラグ用チェックボックスを備え、合算値はBitfinexの `conf` メッセージへ反映
- キャッシュが空の場合、「No data. Do you want to fetch config data?」を提示しREST更新を即実行

#### 各チャンネルパネル

1. **TickerChannelPanel** (`channel_ticker.go`)
   - シンボル選択
   - リアルタイム価格更新

2. **TradesChannelPanel** (`channel_trades.go`)
   - シンボル選択
   - 約定情報取得

3. **BooksChannelPanel** (`channel_books.go`)
   - シンボル選択
   - 精度設定（P0-P4, R0）
   - 頻度設定（F0=リアルタイム, F1=2秒間隔）
   - 長さ設定（1/25/100/250レベル）

4. **CandlesChannelPanel** (`channel_candles.go`)
   - シンボル選択
   - 時間枠設定（1m/5m/15m/30m/1h/3h/6h/12h/1D/1W/14D/1M）

5. **StatusChannelPanel** (`channel_status.go`)
   - ステータスタイプ選択（Derivatives/Liquidation）

### 主要機能

#### 1. 購読制限カウンター
```
Subscriptions: 25 / 30 ⚠️ NEAR LIMIT
Subscriptions: 30 / 30 ⚠️ LIMIT REACHED
```

#### 2. 自動設定更新
`config.ConfigManager` を通じて WebSocket 接続時に以下を自動実行:
- 取引ペア／通貨マップの取得・更新
- 正規化ルール（シンボル → 内部表記）の再ロード
- `config/state/state.yml` への次回更新スケジュール格納

#### 3. 状態永続化
以下の情報をセッション間で保持:
- アクティブなタブ
- 選択中のシンボル
- チャンネル設定
- 接続フラグ
- RESTキャッシュの有効期限

#### 4. GUI ↔ 設定管理フロー

1. `gui/app/app.go` が起動時に `initialiseConfigManager` を呼び出し、`config/exchanges/bitfinex_config.yml` と REST キャッシュを読み込み。
2. `BuildExchangePanesV2` が `ConfigManager` と WebSocket パネルを接続し、タブUIに最新のシンボルリストを注入。
3. REST パネルは [Config|Data] の入れ子タブ構成となり、Config タブから従来のメタデータ更新、Data タブから Candles/Trades/Tickers History の取得ジョブを実行。
4. ユーザー操作で生成された `WSConnectionConfig` は `handleWsConnectConfig` に渡され、既存の `ConnectionManager` が理解できる `cfg.Symbols` / `cfg.Channels` に変換。
5. 接続成功後は `StartPeriodicUpdates` により REST キャッシュを自動更新、切断時には `StopPeriodicUpdates` と状態永続化を実施。
6. `handleWindowClose` でアプリ終了時に `ConfigManager.Shutdown()` を呼び出し、タイマー停止と `state.yml` 保存を保証。

詳細: [docs/WEBSOCKET_CONFIG.md](docs/WEBSOCKET_CONFIG.md)

---

## Bitfinex RESTデータ取得パネル

`rest_api_panel.go` と `rest_data_panel.go` で構成された REST パネルは、[Config|Data] の 2 層タブ構成に刷新されました。

- **Config タブ**: 既存のメタデータ取得ワークフロー。Essential/Daily/Optional の各エンドポイントを一括更新し、結果は `data/bitfinex/restapi/config/` に保存されます。
- **Data タブ**: Candles / Trades / Tickers History の履歴データを REST API から収集し、CSV として `storage.base_path/bitfinex/restapi/data/` 以下に保存します。

### Data タブの主な機能

| カテゴリ | 機能 |
| --- | --- |
| 対応エンドポイント | Candles (`/v2/candles/{key}/hist`), Trades (`/v2/trades/{symbol}/hist`), Tickers History (`/v2/tickers/hist`) |
| シンボル選択 | ConfigManager から読み込んだ最新のシンボルリストをマルチセレクトで提供 |
| タイムレンジ | UTC の `YYYY-MM-DD HH:MM:SS` 形式。未入力時は過去24時間〜現在を自動適用 |
| ページネーション | `sort=1`（昇順）でのウィンドウスクロールを採用。`Auto-pagination` を有効にすると連続取得 |
| レート制限 | Candles 30/min, Trades 15/min, Tickers 30/min に合わせた per-endpoint limiter を内蔵 |
| データ品質 | 重複排除、ギャップ検出（Candles: timeframe に基づく閾値）を設定可能 |
| 保存形式 | CSV（ヘッダ付き）。Candles/Trades は 1 ファイル/組合せ、Tickers はシンボル配列をまとめて保存 |
| ログ・進捗 | 実行状況をアクティビティログとステータスラベルに逐次表示。Stop ボタンで安全にキャンセル |

CSV 出力例:

- Candles: `candles_{symbol}_{timeframe}_{timestamp}.csv` （列: `mts,open,close,high,low,volume,symbol,timeframe`）
- Trades: `trades_{symbol}_{timestamp}.csv` （列: `id,mts,amount,price,symbol`）
- Tickers History: `tickers_{timestamp}.csv` （列: `symbol,bid,bid_size,ask,...,mts`）

実行ループは `restapi.BitfinexDataClient` の各エンドポイント呼び出しを利用し、コンテキストキャンセルに対応した安全な停止／再開が可能です。

---

## GUI構成

### メインウィンドウ構成

```
┌─────────────────────────────────────────────────┐
│ Top Bar (タイトル・ステータス)                    │
├─────────────────────────────────────────────────┤
│ ┌───────────────┐ ┌───────────────────────────┐ │
│ │ WebSocket     │ │ REST API                  │ │
│ │ ┌───────────┐ │ │ ┌───────────────────────┐ │ │
│ │ │ Ticker    │ │ │ │ Config Fetcher        │ │ │
│ │ │ Trades    │ │ │ │ - Pairs               │ │ │
│ │ │ Books     │ │ │ │ - Currencies          │ │ │
│ │ │ Candles   │ │ │ │ - Symbols             │ │ │
│ │ │ Status    │ │ │ └───────────────────────┘ │ │
│ │ └───────────┘ │ │                           │ │
│ │ [Connect]     │ │ [Fetch Config]            │ │
│ └───────────────┘ └───────────────────────────┘ │
├─────────────────────────────────────────────────┤
│ ┌───────────────┐ ┌───────────────────────────┐ │
│ │ Data Files    │ │ File Viewer               │ │
│ │ - Filter      │ │ - 3000 records display    │ │
│ │ - Search      │ │ - Read-only view          │ │
│ │ - Select      │ │ - Metadata info           │ │
│ └───────────────┘ └───────────────────────────┘ │
├─────────────────────────────────────────────────┤
│ Bottom Bar (情報・警告表示)                       │
└─────────────────────────────────────────────────┘
```

### GUI層の構造

**アプリケーション層** (`gui/app/`)
- メインアプリケーション構造

**コントローラー層** (`gui/controllers/`)
- ビジネスロジック処理

**プレゼンテーション層** (`gui/panels/`)
- UI表示・ユーザー操作

**状態管理層** (`gui/state/`)
- GUI状態の管理

---

## データ保存

### Apache Arrow形式

**利点:**
- 列指向による高速読み込み
- ゼロコピー読み込み
- 圧縮対応
- 複数言語対応

### ディレクトリ構造

```
data/bitfinex/
├── websocket/
│   ├── trades/
│   │   └── tBTCUSD/
│   │       └── 2025-09-30_10-00-00.arrow
│   ├── books/
│   │   └── tBTCUSD/
│   │       └── 2025-09-30_10-00-00.arrow
│   └── ticker/
│       └── tBTCUSD/
│           └── 2025-09-30_10-00-00.arrow
└── restapi/
    └── config/
        ├── list_pair_exchange.json
        └── map_currency_label.json
```

### メタデータ

各Arrowファイルにメタデータを付与:
- `exchange`: 取引所名
- `symbol`: シンボル
- `channel`: チャンネルタイプ
- `start_time`: 開始時刻
- `end_time`: 終了時刻
- `record_count`: レコード数

---

## 実装チェックリスト

### 接続
- ✅ HTTPは接続プール + 短タイムアウト
- ✅ WSは自動再接続と再購読
- ✅ ハートビート監視

### 品質
- ✅ バッチ化（cgo/MLX呼び出し回数削減）
- ⏳ 二重送信の冪等化（client_cmd_id）

### バックプレッシャ
- ⏳ 保存キュー溢れ時のサンプリング率調整
- ⏳ 要約粒度の動的調整

### フェイルセーフ
- ⏳ MLX落ち時のモデル無しモード切替
- ⏳ 既存ポジション管理のみモード

### ログ/監査
- ⏳ 注文IDでREST↔WS整合性確認
- ⏳ 全決定に理由（特徴値・確率・EV）添付

凡例: ✅完了 / ⏳未実装 / 🚧作業中

---

## 使用予定AIモデル

| モデル群 | 非線形表現力 | 時系列順序扱い | ドリフト対応 | 推論速度 | 特徴依存 | 典型ユース |
|---------|-------------|---------------|-------------|----------|---------|-----------|
| SGD/FTRL/Passive-Aggressive | 低〜中 | 明示的なし | 強い | 最軽量（μs〜ms） | 高い | 低遅延・連続学習 |
| LightGBM/HistGBDT | 中〜高 | 暗黙的 | 中 | 軽い（ms台） | 中 | 日次再学習・解釈性 |
| TCN/1D-CNN（TinyLOB系） | 中 | ネイティブ | 中 | 軽い（ms〜数ms） | 低〜中 | 板パターン検出 |
| HMM/カルマン+ロジスティック | 低〜中 | レジーム間接 | 強い | 軽い（μs〜ms） | 中 | 市況別切替 |

**注**: 実装優先度は SGD/FTRL → LightGBM → TCN の順

---

## 参考リンク

### Kraken WebSocket APIs
- [Global Intro](https://docs.kraken.com/api/docs/guides/global-intro)
- [Spot WebSocket](https://docs.kraken.com/api/docs/guides/spot-ws-intro)
- [Market Data](https://docs.kraken.com/api/docs/websocket-v2/ticker)

### Kraken REST APIs
- [Spot REST Intro](https://docs.kraken.com/api/docs/guides/spot-rest-intro)
- [Tradable Asset Pairs](https://docs.kraken.com/api/docs/rest-api/get-tradable-asset-pairs)

### Bitfinex WebSocket APIs
- [General](https://docs.bitfinex.com/docs/ws-general)
- [Public Channels](https://docs.bitfinex.com/docs/ws-public)
- [Ticker](https://docs.bitfinex.com/reference/ws-public-ticker)
- [Trades](https://docs.bitfinex.com/reference/ws-public-trades)
- [Books](https://docs.bitfinex.com/reference/ws-public-books)
- [Candles](https://docs.bitfinex.com/reference/ws-public-candles)
- [Status](https://docs.bitfinex.com/reference/ws-public-status)

### Bitfinex REST APIs
- [Platform Status](https://docs.bitfinex.com/reference/rest-public-platform-status)
- [Tickers](https://docs.bitfinex.com/reference/rest-public-tickers)
- [Trades](https://docs.bitfinex.com/reference/rest-public-trades)
- [Book](https://docs.bitfinex.com/reference/rest-public-book)
- [Candles](https://docs.bitfinex.com/reference/rest-public-candles)
- [Config](https://docs.bitfinex.com/reference/rest-public-conf)

### Apache Arrow
- [Go Package](https://pkg.go.dev/github.com/apache/arrow/go/arrow)
- [Format Intro](https://arrow.apache.org/docs/format/Intro.html)
- [Columnar Format](https://arrow.apache.org/docs/format/Columnar.html)

### Fyne Documentation
- [API v2.6](https://docs.fyne.io/api/v2.6/)
- [Widget](https://docs.fyne.io/api/v2.6/widget/)
- [Container](https://docs.fyne.io/api/v2.6/container/)
- [Data Binding](https://docs.fyne.io/api/v2.6/data/binding/)

---

## ライセンス

プロジェクト固有のライセンスに準拠

## 追加ドキュメント

- [WebSocket設定システム詳細](docs/WEBSOCKET_CONFIG.md)
- [AI開発指針](CLAUDE.md)

---

**最終更新**: 2025-09-30
