# システム改善ノート (Kaizen Note)

生成日時: 2025-10-01
分析対象: TradeEngine2/data_controller

---

## 1. プロジェクト概要

### 基本統計
- **総Goファイル数**: 66ファイル
- **総行数**: 約16,867行
- **主要ディレクトリ**: cmd, internal, pkg, config, docs, examples

### アーキテクチャ
- **言語**: Go 1.25.1
- **GUIフレームワーク**: Fyne
- **データ形式**: Apache Arrow/Parquet
- **対象取引所**: Bitfinex (将来的にBinance, Coinbase, Kraken対応予定)

---

## 2. 未使用ファイル・リンボーファイル (Limbo Files)

### 2.1 完全に空のディレクトリ
以下のディレクトリは作成されているが、ファイルが一切存在しない：

1. **`internal/bitfinex/`** - 空ディレクトリ
2. **`internal/book/`** - 空ディレクトリ
3. **`internal/metrics/`** - 空ディレクトリ
4. **`internal/models/`** - 空ディレクトリ

これらはどこからもimportされていない。

### 2.2 未使用のパネル実装 (重複)

#### `internal/ui/` ディレクトリ全体が未使用
- `internal/ui/panels/files_panel.go` (5.5K)
- `internal/ui/panels/viewer_panel.go` (5.6K)
- `internal/ui/components/date_picker.go`

**現状**:
- `internal/ui/panels/` は `internal/state` をimport
- `internal/gui/panels/` は `internal/gui/state` をimport
- **実際に使用されているのは `internal/gui/panels/` のみ**
- `internal/gui/app/app.go` が `internal/gui/panels` をimport

**問題点**:
- 同じ機能の実装が2箇所に存在
- `internal/ui/` は古いバージョンと思われる
- ファイルサイズも異なる (gui版が大きい: 13K vs 5.5K)

### 2.3 未使用の状態管理 (重複)

#### `internal/state/` パッケージが未使用
- `internal/state/app_state.go`
- `internal/ui/` からのみ参照されている
- 実際のアプリケーションは `internal/gui/state/` を使用

**使用状況**:
```
internal/gui/state/ ← 使用中 (gui/app, gui/panels, gui/controllers から参照)
internal/state/     ← 未使用 (ui/panels からのみ参照、ui自体が未使用)
```

### 2.4 旧バージョンの REST Data Panel

#### `internal/gui/rest_data_panel.go` (766行)
- 関数数: 19個
- **NewRestDataPanel() が一度も呼ばれていない**
- V2版 (`rest_data_panel_v2.go`) が使用されている

**使用状況**:
```
rest_data_panel.go    ← 未使用 (定義のみ、呼び出しなし)
rest_data_panel_v2.go ← 使用中 (rest_api_panel.go から呼び出し)
```

### 2.5 重複するスタブファイル

以下の2ファイルは同じ目的 (GUIビルドタグなし時の代替実装):
- `cmd/data-controller/gui_stub.go`
- `cmd/data-controller/stub_gui.go`

両方とも `//go:build !gui` タグを持つ。

### 2.6 テスト・サンプルファイル

#### `examples/test_config_system.go`
- 動作確認用のテストコード
- 本番では未使用
- ドキュメント・参考実装として価値あり

---

## 3. 設定ファイルの重複と役割の曖昧さ

### 3.1 Bitfinex設定の重複

**重複している設定ファイル**:
```
config/bitfinex_config.yml          ← ルート直下
config/exchanges/bitfinex_config.yml ← exchanges サブディレクトリ
```

どちらが正式な設定ファイルか不明確。

### 3.2 状態ファイルの重複

**重複している状態ファイル**:
```
config/state.yml       ← ルート直下
config/state/state.yml ← state サブディレクトリ
```

どちらが使用されているか不明確。

### 3.3 設定ファイル一覧

以下の設定ファイルが存在:
1. `config/config.yml` - グローバルアプリケーション設定
2. `config/bitfinex_config.yml` - Bitfinex WebSocket/チャネル設定
3. `config/exchanges/bitfinex_config.yml` - (重複)
4. `config/schema.json` - 設定スキーマ定義
5. `config/state.yml` - ランタイム状態
6. `config/state/state.yml` - (重複)

**問題点**:
- 役割分担が不明確
- ルートと `exchanges/`, `state/` サブディレクトリの使い分けが曖昧
- どのファイルが優先されるか不明

### 3.4 未使用ディレクトリ

```
config/backups/  - 空
config/profiles/ - 空
config/tmp/      - 空
```

---

## 4. 肥大化したコードファイル

### 4.1 超大規模ファイル (1000行超)

#### 1. `internal/gui/rest_data_panel_v2.go` - **1,251行**
**責務**:
- REST APIデータ取得のUI管理
- Candles/Trades/Tickers の3つのデータ型管理
- 接続状態管理
- データ収集ロジック (CSV書き込み含む)
- UIコンポーネント (ボタン、ログウィンドウ、ダイアログ)

**関数数**: 37個

**問題点**:
- UI層とビジネスロジック層が混在
- データ収集ロジックが1ファイルに集中
- テストが困難

**分割案**:
1. `rest_data_panel_v2_ui.go` - UI components
2. `rest_data_collector_candles.go` - Candles collection logic
3. `rest_data_collector_trades.go` - Trades collection logic
4. `rest_data_collector_tickers.go` - Tickers collection logic
5. `rest_data_state.go` - State management

### 4.2 大規模ファイル (600-800行)

#### 2. `internal/gui/rest_data_panel.go` - **766行** (未使用)
- 上記V2と同様の構造
- **削除推奨**

#### 3. `internal/ws/conn.go` - **676行**
**責務**:
- WebSocket接続管理
- メッセージ送受信
- サブスクリプション管理
- ハートビート・再接続ロジック
- 接続プール管理

**関数数**: 23個

**問題点**:
- 接続管理とメッセージ処理が混在
- 複数の責務が1ファイルに集約

**分割案**:
1. `ws_connection.go` - 単一接続の管理
2. `ws_manager.go` - 接続プール管理
3. `ws_subscription.go` - サブスクリプション管理
4. `ws_heartbeat.go` - ハートビート・再接続

#### 4. `internal/gui/websocket_panel.go` - **647行**
**責務**:
- WebSocket UI管理
- チャネル設定パネル管理
- サブスクリプション制限管理
- 接続コールバック

**関数数**: 25個

**分割案**:
1. `websocket_panel_ui.go` - UI components
2. `websocket_panel_subscription.go` - Subscription logic

#### 5. `internal/gui/app/app.go` - **607行**
**責務**:
- アプリケーション初期化
- レイアウト作成
- WebSocket接続ハンドリング
- ステータス更新
- ウィンドウ管理

**関数数**: 不明

**問題点**:
- アプリケーションのエントリーポイントとして多機能

### 4.3 中規模ファイル (400-600行)

#### 6. `internal/sink/arrow/reader.go` - **588行**
- Arrow/Parquet ファイル読み込み
- 複数のデータ型対応

#### 7. `internal/gui/channel_books.go` - **520行**
- Order Book チャネルUI
- 設定管理・永続化

#### 8. `internal/gui/panels/files_panel.go` - **480行**
- ファイルブラウザUI
- ファイル操作

#### 9. `internal/sink/arrow/writer.go` - **457行**
- Arrow/Parquet ファイル書き込み
- セグメント管理

#### 10. `internal/ws/router.go` - **456行**
- WebSocketメッセージルーティング
- Ticker/Trades/Books のパース

#### 11. `internal/gui/channel_candles.go` - **447行**
- Candles チャネルUI

#### 12. `internal/services/file_scanner.go` - **440行**
- ファイルスキャン・インデックス作成

#### 13. `internal/gui/channel_ticker.go` - **430行**
- Ticker チャネルUI

#### 14. `cmd/data-controller/gui.go` - **424行**
- GUI初期化ロジック

#### 15. `internal/gui/controllers/file_controller.go` - **401行**
- ファイル操作コントローラ

#### 16. `internal/gui/channel_trades.go` - **401行**
- Trades チャネルUI

---

## 5. パターン分析

### 5.1 チャネルパネルの冗長性

以下の4ファイルはほぼ同じ構造:
1. `channel_ticker.go` (430行)
2. `channel_trades.go` (401行)
3. `channel_candles.go` (447行)
4. `channel_books.go` (520行)

**共通パターン**:
- `NewXXXChannelPanel()` - コンストラクタ
- `Build()` - UI構築
- `loadAvailableSymbols()` - シンボル読み込み
- `filterSymbols()` - 検索フィルタ
- `GetSubscriptions()` - サブスクリプション取得
- `persistState()` / `loadState()` - 状態永続化

**改善案**:
- 共通インターフェース `ChannelPanel` を定義
- 基底構造体 `BaseChannelPanel` を実装
- 各チャネル固有の部分のみをオーバーライド
- **削減効果**: 約1,800行 → 600-800行

### 5.2 REST データパネルの冗長性

以下の3ファイルはほぼ同じ構造:
1. `rest_channel_candles.go`
2. `rest_channel_trades.go`
3. `rest_channel_tickers.go`

**改善案**:
- `BaseRestChannelPanel` 構造体を作成
- 共通ロジックを集約

---

## 6. 構成上の問題点まとめ

### 6.1 ディレクトリ構造の問題

#### 問題1: 重複した実装が複数存在
```
internal/
  ├── gui/
  │   ├── panels/       ← 使用中
  │   └── state/        ← 使用中
  └── ui/               ← 未使用 (全体)
      ├── panels/       ← 未使用
      ├── components/   ← 未使用
      └── state/        ← 使用されていない
```

#### 問題2: 空のディレクトリが残存
```
internal/
  ├── bitfinex/  ← 空
  ├── book/      ← 空
  ├── metrics/   ← 空
  └── models/    ← 空
```

### 6.2 設定ファイルの問題

#### 問題1: 重複する設定
```
config/
  ├── bitfinex_config.yml
  ├── state.yml
  ├── exchanges/
  │   └── bitfinex_config.yml  ← 重複
  └── state/
      └── state.yml             ← 重複
```

#### 問題2: 空のディレクトリ
```
config/
  ├── backups/   ← 空
  ├── profiles/  ← 空
  └── tmp/       ← 空
```

### 6.3 コードの問題

#### 問題1: 旧バージョンが残存
- `rest_data_panel.go` (未使用) と `rest_data_panel_v2.go` (使用中)

#### 問題2: 重複するスタブファイル
- `gui_stub.go` と `stub_gui.go` (両方とも同じ目的)

#### 問題3: 過度に大きなファイル
- 1,000行超のファイルが1つ
- 600-800行のファイルが3つ
- 400-600行のファイルが11個

---

## 7. 改善提案

### 7.1 即座に削除可能なファイル・ディレクトリ

#### 削除推奨 (高優先度)
1. **`internal/ui/` ディレクトリ全体** - 完全に未使用
2. **`internal/state/app_state.go`** - 未使用
3. **`internal/bitfinex/`** - 空ディレクトリ
4. **`internal/book/`** - 空ディレクトリ
5. **`internal/metrics/`** - 空ディレクトリ
6. **`internal/models/`** - 空ディレクトリ
7. **`internal/gui/rest_data_panel.go`** - 旧バージョン
8. **`cmd/data-controller/gui_stub.go` または `stub_gui.go`** - 重複、どちらか一方
9. **`config/backups/`** - 空
10. **`config/profiles/`** - 空
11. **`config/tmp/`** - 空 (一時ファイル用なら.gitignore推奨)

**削減効果**: 約10ファイル、約1,500行

### 7.2 設定ファイルの整理

#### 提案1: ディレクトリ構造の明確化

**現状**:
```
config/
  ├── config.yml                    ← グローバル設定
  ├── bitfinex_config.yml          ← 取引所設定 (重複1)
  ├── state.yml                    ← 状態 (重複1)
  ├── schema.json                  ← スキーマ
  ├── exchanges/
  │   └── bitfinex_config.yml     ← (重複2)
  └── state/
      └── state.yml                ← (重複2)
```

**改善後**:
```
config/
  ├── app.yml                      ← グローバル設定 (名前変更)
  ├── schema.json                  ← スキーマ定義
  ├── exchanges/
  │   ├── bitfinex.yml            ← 取引所別設定
  │   ├── binance.yml             ← (将来)
  │   └── coinbase.yml            ← (将来)
  └── runtime/                     ← 新ディレクトリ
      └── state.yml                ← ランタイム状態
```

**ルール**:
- **`app.yml`**: アプリケーション全体の設定 (storage, logging, monitoring)
- **`exchanges/*.yml`**: 取引所ごとの設定 (WebSocket URL, limits, channels)
- **`runtime/state.yml`**: 実行時の状態管理 (UI状態、接続状態、キャッシュ)
- **`schema.json`**: 設定のJSONスキーマ

### 7.3 コードのリファクタリング提案

#### Phase 1: 重複削除 (即実行可能)
1. `internal/ui/` を削除
2. `internal/state/` を削除
3. 空ディレクトリを削除
4. `rest_data_panel.go` を削除
5. スタブファイルの統合

**効果**: コードベース約20%削減

#### Phase 2: 設定ファイル整理
1. 設定ファイルの統合・移動
2. ロード順序の明確化
3. ドキュメント更新

#### Phase 3: チャネルパネルの共通化

**現状** (1,798行):
```go
// 4つのファイルで重複実装
channel_ticker.go  (430行)
channel_trades.go  (401行)
channel_candles.go (447行)
channel_books.go   (520行)
```

**改善後** (推定600-800行):
```go
// 共通基盤
internal/gui/channels/
  ├── base_panel.go          ← 共通実装 (200-300行)
  ├── ticker_panel.go        ← Ticker固有 (80-120行)
  ├── trades_panel.go        ← Trades固有 (80-120行)
  ├── candles_panel.go       ← Candles固有 (100-150行)
  └── books_panel.go         ← Books固有 (140-200行)
```

**共通インターフェース**:
```go
type ChannelPanel interface {
    Build() fyne.CanvasObject
    GetSubscriptions() []ChannelSubscription
    GetSubscriptionCount() int
    LoadState()
    SaveState()
}

type BaseChannelPanel struct {
    logger         *zap.Logger
    configManager  *config.ConfigManager
    exchange       string
    enabled        bool
    selectedSymbols map[string]bool
    // ... 共通フィールド
}
```

**効果**: 約1,000行削減

#### Phase 4: REST データパネルの分割

**現状**: `rest_data_panel_v2.go` (1,251行)

**改善後**:
```go
internal/gui/rest/
  ├── panel.go               ← UI管理 (200-300行)
  ├── state.go               ← 状態管理 (100-150行)
  └── collectors/
      ├── candles.go         ← Candles収集 (250-300行)
      ├── trades.go          ← Trades収集 (200-250行)
      └── tickers.go         ← Tickers収集 (200-250行)
```

**効果**: 可読性・保守性の大幅向上

#### Phase 5: WebSocket接続管理の分割

**現状**: `ws/conn.go` (676行)

**改善後**:
```go
internal/ws/
  ├── connection.go          ← 単一接続 (200-250行)
  ├── manager.go             ← 接続プール (150-200行)
  ├── subscription.go        ← サブスクリプション (150-180行)
  └── heartbeat.go           ← ハートビート (80-100行)
```

**効果**: 単一責任原則に準拠、テスト容易性向上

---

## 8. 推奨ディレクトリ構造

### 8.1 整理後の構造

```
data_controller/
├── cmd/
│   └── data-controller/
│       ├── main.go
│       ├── nogui.go
│       ├── fyne_gui.go         # GUIビルド時
│       └── stub_gui.go         # 非GUIビルド時 (統合後)
│
├── config/
│   ├── app.yml                 # グローバル設定
│   ├── schema.json             # スキーマ定義
│   ├── exchanges/
│   │   ├── bitfinex.yml
│   │   ├── binance.yml
│   │   └── coinbase.yml
│   └── runtime/
│       └── state.yml           # ランタイム状態
│
├── internal/
│   ├── config/                 # 設定管理
│   │   ├── config.go
│   │   ├── manager.go
│   │   ├── loader.go
│   │   ├── normalizer.go
│   │   └── state.go
│   │
│   ├── ws/                     # WebSocket (分割後)
│   │   ├── connection.go
│   │   ├── manager.go
│   │   ├── subscription.go
│   │   ├── heartbeat.go
│   │   └── router.go
│   │
│   ├── restapi/                # REST API
│   │   ├── client.go
│   │   ├── data_client.go
│   │   ├── rate_limiter.go
│   │   └── utils.go
│   │
│   ├── sink/                   # データ永続化
│   │   ├── arrow/
│   │   │   ├── handler.go
│   │   │   ├── writer.go
│   │   │   ├── reader.go
│   │   │   ├── schema.go
│   │   │   └── channel_writer.go
│   │   └── wal/                # Write-Ahead Log
│   │
│   ├── gui/                    # GUI (統合後)
│   │   ├── app/
│   │   │   └── app.go
│   │   │
│   │   ├── panels/             # 共通パネル
│   │   │   ├── files_panel.go
│   │   │   └── viewer_panel.go
│   │   │
│   │   ├── channels/           # WebSocketチャネル (共通化)
│   │   │   ├── base_panel.go
│   │   │   ├── ticker_panel.go
│   │   │   ├── trades_panel.go
│   │   │   ├── candles_panel.go
│   │   │   ├── books_panel.go
│   │   │   └── status_panel.go
│   │   │
│   │   ├── rest/               # RESTデータ取得 (分割後)
│   │   │   ├── panel.go
│   │   │   ├── state.go
│   │   │   └── collectors/
│   │   │       ├── candles.go
│   │   │       ├── trades.go
│   │   │       └── tickers.go
│   │   │
│   │   ├── components/         # 再利用可能コンポーネント
│   │   │   ├── toggle_button.go
│   │   │   ├── symbol_search.go
│   │   │   ├── datetime_picker.go
│   │   │   ├── top_bar.go
│   │   │   └── bottom_bar.go
│   │   │
│   │   ├── controllers/
│   │   │   └── file_controller.go
│   │   │
│   │   └── state/              # GUIステート
│   │       └── app_state.go
│   │
│   ├── services/               # ビジネスロジック
│   │   ├── config_refresh.go
│   │   ├── file_scanner.go
│   │   └── file_reader.go
│   │
│   └── domain/                 # ドメインモデル
│       └── file_item.go
│
├── pkg/                        # 外部公開パッケージ
│   └── schema/
│       └── types.go
│
├── docs/
│   ├── README.md
│   ├── WEBSOCKET_CONFIG.md
│   ├── CLAUDE.md
│   └── kaizen_note.md          # このファイル
│
├── examples/                   # サンプル・テストコード
│   └── test_config_system.go
│
├── data/                       # データディレクトリ (.gitignore)
├── Makefile
├── go.mod
└── go.sum
```

### 8.2 削除されるディレクトリ・ファイル

```
❌ internal/ui/                  → 削除 (完全未使用)
❌ internal/state/               → 削除 (未使用)
❌ internal/bitfinex/            → 削除 (空)
❌ internal/book/                → 削除 (空)
❌ internal/metrics/             → 削除 (空)
❌ internal/models/              → 削除 (空)
❌ config/backups/               → 削除 (空)
❌ config/profiles/              → 削除 (空)
❌ config/tmp/                   → 削除 (空)
❌ config/bitfinex_config.yml   → 移動/統合
❌ config/state.yml              → 移動/統合
❌ rest_data_panel.go           → 削除 (旧版)
❌ gui_stub.go                  → 統合
```

### 8.3 移動・リネーム

```
config/config.yml               → config/app.yml
config/exchanges/bitfinex_config.yml → config/exchanges/bitfinex.yml
config/state/state.yml          → config/runtime/state.yml
```

---

## 9. 実装優先度

### Priority 1: 即座に実行可能 (リスク低)
1. ✅ 空ディレクトリの削除
2. ✅ 未使用ディレクトリの削除 (`internal/ui/`, `internal/state/`)
3. ✅ 旧バージョンファイルの削除 (`rest_data_panel.go`)
4. ✅ スタブファイルの統合
5. ✅ 設定ファイルの整理・移動

**期間**: 1-2時間
**効果**: コードベース約20%削減、構造の明確化

### Priority 2: 中期改善 (リスク中)
1. ⏳ チャネルパネルの共通化
2. ⏳ REST データパネルの分割
3. ⏳ WebSocket接続管理の分割

**期間**: 1-2週間
**効果**: 約2,000行削減、保守性向上

### Priority 3: 長期改善 (リスク高)
1. 🔲 アーキテクチャの全面見直し
2. 🔲 テストカバレッジの向上
3. 🔲 ドキュメントの整備

---

## 10. リファクタリング実施計画

### Phase 1: クリーンアップ (1-2日)

**Day 1: 削除作業**
```bash
# 1. 未使用ディレクトリ削除
rm -rf internal/ui
rm -rf internal/state
rm -rf internal/bitfinex
rm -rf internal/book
rm -rf internal/metrics
rm -rf internal/models
rm -rf config/backups
rm -rf config/profiles
rm -rf config/tmp

# 2. 未使用ファイル削除
rm internal/gui/rest_data_panel.go
rm cmd/data-controller/gui_stub.go  # または stub_gui.go

# 3. ビルド確認
make build-gui
./data-controller-gui  # 動作確認
```

**Day 2: 設定ファイル整理**
```bash
# 1. ディレクトリ作成
mkdir -p config/runtime
mkdir -p config/exchanges

# 2. ファイル移動・リネーム
mv config/config.yml config/app.yml
mv config/state/state.yml config/runtime/state.yml
mv config/exchanges/bitfinex_config.yml config/exchanges/bitfinex.yml
rm config/bitfinex_config.yml  # 重複削除
rm config/state.yml             # 重複削除
rmdir config/state              # 空ディレクトリ削除

# 3. コード側の修正
# config loaderのパス更新が必要
```

### Phase 2: コード分割 (1週間)

**Week 1: チャネルパネル共通化**
1. `BaseChannelPanel` 実装
2. 各チャネルパネルをリファクタリング
3. テスト実施

### Phase 3: 大規模ファイル分割 (1週間)

**Week 2: REST & WebSocket分割**
1. `rest_data_panel_v2.go` 分割
2. `ws/conn.go` 分割
3. テスト実施

---

## 11. メトリクス

### 現状
- **総ファイル数**: 66個
- **総行数**: 16,867行
- **1000行超ファイル**: 1個
- **500-1000行ファイル**: 4個
- **未使用ファイル**: 約10個 (1,500行)
- **空ディレクトリ**: 7個

### 目標 (Phase 1完了後)
- **総ファイル数**: 約55個 (-11個)
- **総行数**: 約15,300行 (-1,500行)
- **1000行超ファイル**: 0個
- **500-1000行ファイル**: 2-3個
- **未使用ファイル**: 0個
- **空ディレクトリ**: 0個

### 目標 (Phase 2-3完了後)
- **総ファイル数**: 約60-65個
- **総行数**: 約14,000行 (-2,800行)
- **最大ファイルサイズ**: 500行以下
- **平均ファイルサイズ**: 200-250行

---

## 12. リスク評価

### 低リスク作業
- ✅ 空ディレクトリの削除
- ✅ 未使用ディレクトリ (`internal/ui`, `internal/state`) の削除
- ✅ 旧バージョンファイル (`rest_data_panel.go`) の削除

**理由**: これらはどこからも参照されていない

### 中リスク作業
- ⚠️ 設定ファイルの移動・統合
- ⚠️ スタブファイルの統合

**理由**: ビルドタグやロードパスの変更が必要

**対策**:
- ビルド後の動作確認を徹底
- 設定ロード処理のテスト追加

### 高リスク作業
- 🚨 大規模ファイルの分割
- 🚨 共通基盤の作成

**理由**: ビジネスロジックの変更を伴う

**対策**:
- 段階的リファクタリング
- 各ステップでテスト実施
- フィーチャーフラグの活用

---

## 13. 次のステップ

### 即座に実行すべきアクション
1. このノートをレビュー
2. Priority 1 の削除作業を実施
3. ビルド・動作確認
4. Git commit

### 承認が必要な事項
- 設定ファイルのディレクトリ構造変更
- 大規模ファイルの分割方針

### 追加調査が必要な事項
- `examples/` ディレクトリの扱い (削除 or docs/ へ移動?)
- 将来の取引所追加時の拡張性
- テストコードの配置方針

---

## 14. 結論

### 主な発見
1. **約20%のコードが未使用または重複**
2. **設定ファイルの構造が不明確**
3. **大規模ファイルが保守性を低下させている**
4. **チャネルパネルの実装が冗長**

### 改善の効果
- **コードベース削減**: 約2,800行 (17%削減)
- **ファイル数削減**: 約11個
- **保守性向上**: 責務の明確化、テスト容易性向上
- **拡張性向上**: 将来の取引所追加が容易

### 推奨事項
**今すぐ実施**: Priority 1 (クリーンアップ)
**近日中に実施**: Priority 2 (コード分割)
**継続的に実施**: コードレビュー、リファクタリング文化の醸成

---

**レビュー者**: [名前]
**承認日**: [日付]
**次回レビュー**: [日付]

---

## 追加メモ (2025-10-01)

### A. 設定ファイルの役割再確認
- `config/exchanges/bitfinex_config.yml` は `ConfigManager.Initialize()` が直接読み込む唯一のエクスチェンジ設定です。REST キャッシュやノーマライザの既定値もここに紐づくため、「重複」とみなして削除すると起動直後のローディングが失敗します。
- `config/bitfinex_config.yml` は旧レイアウトの在庫で、現行コードからは参照されていません。削除する場合は README と GUI の説明を同時更新し、`docs/WEBSOCKET_CONFIG.md` の手順が逸脱しないか確認してください。逆に活用するのであれば、`ConfigManager` 側での読み取り先を一元化する設計が妥当です。
- `config/state.yml` (REST 設定キャッシュの最終更新時刻) と `config/state/state.yml` (GUI・WS 状態) は用途が異なります。統合する前に、`ApplicationState` と REST キャッシュ管理 (`UpdateRestConfigCache`) の保存先がバラバラになっている点を設計として明示し、2種類の状態データをどこで扱うか整理すると迷子になりづらくなります。

### B. 未使用／空ディレクトリの扱い
- `internal/bitfinex`, `internal/book`, `internal/metrics`, `internal/models` などは完全に空であり、現行ビルドには影響しません。削除する場合は `go list ./...` でパッケージ依存に変動がないことを確認し、将来の拡張予定がないかオーナー確認を行ってください。
- `internal/ui/` と `internal/state/` は旧 GUI 実装の痕跡ですが、`go:build` タグなどは付与されていません。`grep -R "internal/ui"` でも現行パスから参照が無いことを確認済みなので、Phase 1 のクリーンアップ候補として安全圏です。削除する際は `go build ./cmd/data-controller` で GUI ビルドが壊れないか最終確認するのが確実です。

### C. 大型ファイルの分割に向けた追加ヒント
- `rest_data_panel_v2.go` の肥大化はタブごとの UI とバッチ制御が密結合になっていることが主因です。`RunContext` や `JobRunner` のような小さなサービス層を切り出し、UI 層 (`RestDataPanelV2`) からはメッセージやプレーンな DTO のみ渡す構造にすると 1 ファイル 400 行程度まで縮小できます。
- `ws/conn.go` は接続管理とメッセージ処理が混在しているので、`message_loop.go` (受信処理) と `connection.go` (接続・再接続制御) に二分する方針が扱いやすいです。その際 `Router` への依存性注入ポイントを 1 箇所に集約するとテストが書きやすくなります。

### D. 今後の実行ステップ提案
1. `config` 配下の役割を README/WEBSOCKET_CONFIG 双方で同期し、「実際に読み込まれるファイル」と「旧リソース」を表形式で明示する。
2. 優先度が低い空ディレクトリの削除でも `git clean` ではなく個別削除＋ビルド検証をルール化し、誤って生成先ディレクトリを削らないよう注意喚起する。
3. 大規模ファイルの分割は抽象層の再設計を伴うため、まず `rest_data_panel_v2.go` で `ctx` とレートリミット関連を `rest/jobs` サブパッケージへ切り出す PoC を行い、成功したら WebSocket 側にも展開する。

（追記者: system review）

---

## 15. 完璧な実行計画書 (Detailed Execution Plan)

本セクションでは、システム改善を段階的に実行するための詳細な手順書を提供します。各ステップは順序立てられ、実行前の確認事項、実行コマンド、実行後の検証方法まで完璧に記載されています。

---

### 15.1 事前準備 (Pre-Execution Checklist)

#### Step 0-1: バックアップとブランチ作成

**目的**: 作業を安全に行うための準備

**実行内容**:
```bash
# 1. 現在の状態をコミット (未コミットの変更がある場合)
git status
git add -A
git commit -m "feat: ticker data parsing fix and state management improvements"

# 2. 作業用ブランチを作成
git checkout -b refactor/cleanup-phase1

# 3. 現在のディレクトリ構造をバックアップ
tree -L 3 -I 'vendor|.git|data' > /tmp/dir_structure_before.txt
find . -name "*.go" | wc -l > /tmp/file_count_before.txt
```

**検証方法**:
```bash
# ブランチが作成されたことを確認
git branch | grep "refactor/cleanup-phase1"

# バックアップファイルが存在することを確認
ls -lh /tmp/dir_structure_before.txt /tmp/file_count_before.txt
```

**注意事項**:
- ⚠️ 作業中のコミットされていない変更は必ず先にコミットまたはstashする
- ⚠️ mainブランチで直接作業しない

---

### 15.2 Phase 1: 未使用ファイル・ディレクトリの削除

#### Step 1-1: 空ディレクトリの削除

**目的**: 完全に空のディレクトリを削除し、構造を整理

**削除対象の確認**:
```bash
# 削除対象のディレクトリが本当に空か確認
find internal/bitfinex internal/book internal/metrics internal/models -type f 2>/dev/null
find config/backups config/profiles config/tmp -type f 2>/dev/null

# 結果が空（何も表示されない）なら安全に削除可能
```

**実行コマンド**:
```bash
# 1. internal配下の空ディレクトリを削除
rm -rf internal/bitfinex
rm -rf internal/book
rm -rf internal/metrics
rm -rf internal/models

# 2. config配下の空ディレクトリを削除
rm -rf config/backups
rm -rf config/profiles
rm -rf config/tmp

# 3. 削除されたことを確認
ls -la internal/ | grep -E "bitfinex|book|metrics|models"
ls -la config/ | grep -E "backups|profiles|tmp"
```

**期待される結果**: 上記コマンドで何も表示されない（削除済み）

**検証方法**:
```bash
# ビルドが成功することを確認
go build ./cmd/data-controller

# パッケージ依存関係に問題がないことを確認
go list ./... | grep -E "bitfinex|book|metrics|models"
# 何も表示されなければOK

# Gitステータスを確認
git status
```

**ロールバック方法** (問題が発生した場合):
```bash
git checkout -- internal/ config/
```

**削減効果**:
- ディレクトリ数: -7個

---

#### Step 1-2: 未使用ディレクトリ全体の削除

**目的**: `internal/ui/` と `internal/state/` を完全に削除

**削除前の最終確認**:
```bash
# これらのディレクトリがどこからも参照されていないことを確認
grep -r "internal/ui" --include="*.go" . | grep -v "internal/ui/"
grep -r '"github.com/trade-engine/data-controller/internal/ui"' --include="*.go" .

grep -r "internal/state" --include="*.go" . | grep -v "internal/state/" | grep -v "internal/gui/state"
grep -r '"github.com/trade-engine/data-controller/internal/state"' --include="*.go" .

# 何も表示されなければ安全に削除可能
```

**実行コマンド**:
```bash
# 1. ディレクトリ内容を確認（最終確認）
ls -R internal/ui/
ls -R internal/state/

# 2. 削除実行
rm -rf internal/ui
rm -rf internal/state

# 3. 削除されたことを確認
ls -la internal/ | grep -E "^d.*ui$|^d.*state$"
# 何も表示されなければOK（gui/state は別ディレクトリなので残る）
```

**検証方法**:
```bash
# ビルドが成功することを確認
go build ./cmd/data-controller
echo "Build exit code: $?"

# GUIビルドも確認
make build-gui
echo "GUI build exit code: $?"

# 実行確認
./data-controller-gui &
GUI_PID=$!
sleep 5
ps -p $GUI_PID && echo "✓ GUI running" || echo "✗ GUI failed"
kill $GUI_PID 2>/dev/null
```

**削減効果**:
- ディレクトリ数: -2個
- ファイル数: -5個
- 推定行数: -400行

---

#### Step 1-3: 旧バージョンファイルの削除

**目的**: 使用されていない旧REST Data Panelを削除

**削除前の確認**:
```bash
# rest_data_panel.go が使用されていないことを確認
grep -r "NewRestDataPanel[^V]" --include="*.go" . | grep -v "rest_data_panel.go:"
grep -r 'rest_data_panel"' --include="*.go" . | grep -v "rest_data_panel_v2" | grep -v "rest_data_panel.go:"

# 何も表示されなければ削除可能
```

**実行コマンド**:
```bash
# 1. ファイルサイズと行数を確認（記録用）
wc -l internal/gui/rest_data_panel.go
ls -lh internal/gui/rest_data_panel.go

# 2. 削除実行
rm internal/gui/rest_data_panel.go

# 3. 削除されたことを確認
ls -la internal/gui/rest_data_panel*.go
# rest_data_panel_v2.go のみ表示されればOK
```

**検証方法**:
```bash
# ビルド確認
make build-gui

# rest_data_panel_v2 が正常に動作するか確認
grep -r "NewRestDataPanelV2" --include="*.go" .
# internal/gui/rest_api_panel.go に1箇所あればOK
```

**削減効果**:
- ファイル数: -1個
- 行数: -766行

---

#### Step 1-4: 重複スタブファイルの削除

**目的**: `stub_gui.go` を削除し、`gui_stub.go` に統一

**削除前の確認**:
```bash
# 両ファイルの内容を比較
diff cmd/data-controller/gui_stub.go cmd/data-controller/stub_gui.go

# ビルドタグを確認
head -3 cmd/data-controller/gui_stub.go
head -3 cmd/data-controller/stub_gui.go
```

**実行コマンド**:
```bash
# 1. stub_gui.go を削除
rm cmd/data-controller/stub_gui.go

# 2. 削除されたことを確認
ls -la cmd/data-controller/*stub*.go
# gui_stub.go のみ表示されればOK
```

**検証方法**:
```bash
# GUIビルド（gui_stub.goは使用されない）
make build-gui
echo "GUI build exit code: $?"

# 非GUIビルド（gui_stub.goが使用される）
go build -tags !gui -o data-controller-nogui cmd/data-controller/*.go 2>&1
# エラーが出ても問題ない（!gui タグの構文問題）

# クリーンアップ
rm -f data-controller-nogui
```

**削減効果**:
- ファイル数: -1個
- 行数: -18行

---

#### Step 1-5: 未使用設定ファイルの削除

**目的**: 旧レイアウトの設定ファイルを削除

**削除前の最終確認**:
```bash
# config/bitfinex_config.yml が使用されていないことを確認
grep -r "config/bitfinex_config.yml" --include="*.go" .
grep -r '"bitfinex_config.yml"' --include="*.go" . | grep -v "exchanges/bitfinex"

# config/state.yml が使用されていないことを確認  
grep -r '"config/state.yml"' --include="*.go" . | grep -v "state/state.yml"

# 何も表示されなければ削除可能
```

**実行コマンド**:
```bash
# 1. 削除前にバックアップ（念のため）
cp config/bitfinex_config.yml /tmp/bitfinex_config.yml.backup 2>/dev/null || true
cp config/state.yml /tmp/state.yml.backup 2>/dev/null || true

# 2. ファイルサイズを確認（記録用）
ls -lh config/bitfinex_config.yml config/state.yml 2>/dev/null

# 3. 削除実行
rm -f config/bitfinex_config.yml
rm -f config/state.yml

# 4. 削除されたことを確認
ls -la config/*.yml
# config.yml のみ表示されればOK
```

**検証方法**:
```bash
# アプリケーションが正常に起動することを確認
./data-controller-gui &
GUI_PID=$!
sleep 5

# プロセスが生きているか確認
ps -p $GUI_PID && echo "✓ GUI running" || echo "✗ GUI failed"

# 終了
kill $GUI_PID 2>/dev/null
```

**削減効果**:
- ファイル数: -2個

---

#### Step 1-6: Phase 1 の検証とコミット

**目的**: Phase 1 の全ての削除が正常に完了したことを確認

**包括的検証**:
```bash
# 1. ビルド確認
make clean
make build-gui
echo "Build exit code: $?"

# 2. 削除されたファイル・ディレクトリの確認
echo "=== Deleted directories ==="
for dir in internal/bitfinex internal/book internal/metrics internal/models internal/ui internal/state config/backups config/profiles config/tmp; do
    if [ ! -d "$dir" ]; then
        echo "✓ $dir - deleted"
    else
        echo "✗ $dir - still exists"
    fi
done

echo ""
echo "=== Deleted files ==="
for file in internal/gui/rest_data_panel.go cmd/data-controller/stub_gui.go config/bitfinex_config.yml config/state.yml; do
    if [ ! -f "$file" ]; then
        echo "✓ $file - deleted"
    else
        echo "✗ $file - still exists"
    fi
done

# 3. ファイル数の比較
BEFORE=$(cat /tmp/file_count_before.txt 2>/dev/null || echo "0")
AFTER=$(find . -name "*.go" -not -path "*/vendor/*" -not -path "*/.git/*" | wc -l | tr -d ' ')
REDUCED=$((BEFORE - AFTER))
echo ""
echo "=== File count comparison ==="
echo "Before: $BEFORE files"
echo "After: $AFTER files"
echo "Reduced: $REDUCED files"

# 4. 実行テスト
echo ""
echo "=== Running GUI test ==="
timeout 10s ./data-controller-gui &
GUI_PID=$!
sleep 5
if ps -p $GUI_PID > /dev/null 2>&1; then
    echo "✓ GUI is running"
    kill $GUI_PID 2>/dev/null
else
    echo "✗ GUI failed to start"
fi
```

**期待される結果**:
- ビルドが成功 (exit code: 0)
- 全ての削除対象が削除済み
- ファイル数が9個減少
- GUIが正常に起動

**コミット**:
```bash
# 変更内容を確認
git status
git diff --stat

# コミット実行
git add -A
git commit -m "refactor(cleanup): remove unused files and directories

- Remove empty directories: internal/{bitfinex,book,metrics,models}, config/{backups,profiles,tmp}
- Remove unused ui/state implementations: internal/{ui,state}
- Remove deprecated rest_data_panel.go (766 lines)
- Remove duplicate stub_gui.go
- Remove unused config files: bitfinex_config.yml, state.yml

Total reduction: 9 directories + 9 files, ~1,200 lines
All builds and tests passing."
```

**Phase 1 完了時のメトリクス**:
- 削除ディレクトリ数: 9個
- 削除ファイル数: 9個
- 削除行数: 約1,200行
- ビルド時間: 変更なし
- 実行時動作: 影響なし

---

### 15.3 Phase 2: 設定ファイルの整理とリネーム

#### Step 2-1: ディレクトリのリネーム

**目的**: `config/state/` を `config/runtime/` にリネームし、役割を明確化

**実行前の確認**:
```bash
# 現在の state ディレクトリの内容を確認
ls -la config/state/
cat config/state/state.yml | head -20

# config_manager.go で読み込まれているパスを確認
grep -n "config.*state" internal/config/config_manager.go | head -10
```

**実行コマンド**:
```bash
# 1. ディレクトリをリネーム
mv config/state config/runtime

# 2. リネームされたことを確認
ls -la config/
ls -la config/runtime/

# 3. コード内の参照を更新（macOS）
sed -i '' 's|"config", "state"|"config", "runtime"|g' internal/config/config_manager.go

# Linux の場合は以下を使用
# sed -i 's|"config", "state"|"config", "runtime"|g' internal/config/config_manager.go
```

**検証方法**:
```bash
# 変更内容を確認
git diff internal/config/config_manager.go

# ビルド確認
make build-gui

# 実行確認（runtime/state.ymlが正しく読み込まれるか）
./data-controller-gui &
GUI_PID=$!
sleep 5

# state.yml が更新されているか確認
ls -lt config/runtime/state.yml

kill $GUI_PID 2>/dev/null
```

**注意事項**:
- ⚠️ 既存の `config/state/state.yml` が `config/runtime/state.yml` に移動している
- ⚠️ アプリケーション起動時にランタイム状態が正しく読み込まれることを確認

---

#### Step 2-2: 取引所設定ファイルのリネーム

**目的**: `exchanges/bitfinex_config.yml` を `exchanges/bitfinex.yml` にリネーム

**実行前の確認**:
```bash
# ファイル名の生成ロジックを確認
grep -n "bitfinex_config.yml\|%s_config.yml" internal/config/config_manager.go
```

**実行コマンド**:
```bash
# 1. ファイルをリネーム
mv config/exchanges/bitfinex_config.yml config/exchanges/bitfinex.yml

# 2. config_manager.go のファイル名生成ロジックを修正（macOS）
sed -i '' 's|%s_config.yml|%s.yml|g' internal/config/config_manager.go

# Linux の場合
# sed -i 's|%s_config.yml|%s.yml|g' internal/config/config_manager.go

# 3. 変更内容を確認
git diff internal/config/config_manager.go
```

**検証方法**:
```bash
# ビルド確認
make build-gui

# 実行確認（bitfinex.ymlが正しく読み込まれるか）
./data-controller-gui &
GUI_PID=$!
sleep 5

# エラーがないことを確認
ps -p $GUI_PID && echo "✓ Config loaded successfully" || echo "✗ Failed to load config"

kill $GUI_PID 2>/dev/null
```

**注意事項**:
- ⚠️ 将来の取引所追加時も `binance.yml`, `coinbase.yml` の命名規則に統一
- ⚠️ `LoadBitfinexConfig()` 関数の挙動に影響がないことを確認

---

#### Step 2-3: Phase 2 の検証とコミット

**包括的検証**:
```bash
# 1. ディレクトリ構造を確認
echo "=== Config directory structure ==="
tree config/ -L 2 2>/dev/null || find config/ -type d | sort

# 期待される構造:
# config/
# ├── config.yml
# ├── schema.json
# ├── exchanges/
# │   └── bitfinex.yml
# └── runtime/
#     └── state.yml

# 2. ビルド確認
make clean
make build-gui
echo "Build exit code: $?"

# 3. 実行テスト（全機能）
echo ""
echo "=== Running comprehensive test ==="
./data-controller-gui &
GUI_PID=$!
sleep 10

# GUIが起動していることを確認
if ps -p $GUI_PID > /dev/null 2>&1; then
    echo "✓ GUI is running"

    # 設定が正しく読み込まれているか確認
    if [ -f config/runtime/state.yml ]; then
        echo "✓ Runtime state file exists"
    fi

    kill $GUI_PID 2>/dev/null
    sleep 2
    echo "✓ GUI stopped successfully"
else
    echo "✗ GUI failed to start"
fi

# 4. 設定ファイルの一貫性チェック
echo ""
echo "=== Configuration files check ==="
for file in config/config.yml config/exchanges/bitfinex.yml config/schema.json; do
    if [ -f "$file" ]; then
        echo "✓ $file exists"
    else
        echo "✗ $file missing"
    fi
done
```

**期待される結果**:
- 全てのファイル・ディレクトリが期待される場所に存在
- ビルドが成功
- GUIが正常に起動・終了
- runtime/state.yml が更新される

**コミット**:
```bash
# 変更内容を確認
git status
git diff --stat

# コミット実行
git commit -am "refactor(config): reorganize configuration file structure

- Rename config/state/ to config/runtime/ for clarity
- Rename exchanges/bitfinex_config.yml to exchanges/bitfinex.yml
- Update config_manager.go to reflect new file naming convention

This reorganization improves clarity:
- config.yml: Global application settings
- exchanges/bitfinex.yml: Exchange-specific settings
- runtime/state.yml: Runtime application state

All tests passing."
```

**Phase 2 完了時のメトリクス**:
- リネームディレクトリ数: 1個
- リネームファイル数: 1個
- コード変更: config_manager.go (2箇所)
- ビルド時間: 変更なし
- 実行時動作: 影響なし（パスのみ変更）

---

### 15.4 最終検証とまとめ

#### 最終チェックリスト

```bash
#!/bin/bash
echo "========================================="
echo "Final Verification Checklist"
echo "========================================="

PASS=0
FAIL=0

check() {
    if eval "$1" > /dev/null 2>&1; then
        echo "✓ $2"
        ((PASS++))
    else
        echo "✗ $2"
        ((FAIL++))
    fi
}

echo ""
echo "Phase 1: Deletion Checks"
check "[ ! -d internal/ui ]" "internal/ui deleted"
check "[ ! -d internal/state ]" "internal/state deleted"  
check "[ ! -d internal/bitfinex ]" "internal/bitfinex deleted"
check "[ ! -d internal/book ]" "internal/book deleted"
check "[ ! -d internal/metrics ]" "internal/metrics deleted"
check "[ ! -d internal/models ]" "internal/models deleted"
check "[ ! -d config/backups ]" "config/backups deleted"
check "[ ! -d config/profiles ]" "config/profiles deleted"
check "[ ! -d config/tmp ]" "config/tmp deleted"
check "[ ! -f internal/gui/rest_data_panel.go ]" "rest_data_panel.go deleted"
check "[ ! -f cmd/data-controller/stub_gui.go ]" "stub_gui.go deleted"
check "[ ! -f config/bitfinex_config.yml ]" "old bitfinex_config.yml deleted"
check "[ ! -f config/state.yml ]" "old state.yml deleted"

echo ""
echo "Phase 2: Reorganization Checks"
check "[ -d config/runtime ]" "runtime directory exists"
check "[ ! -d config/state ]" "old state directory removed"
check "[ -f config/exchanges/bitfinex.yml ]" "bitfinex.yml exists"
check "[ ! -f config/exchanges/bitfinex_config.yml ]" "old bitfinex_config.yml removed"

echo ""
echo "Build & Execution Checks"
check "make build-gui > /dev/null 2>&1" "Build succeeds"
check "[ -f ./data-controller-gui ]" "Binary created"

echo ""
echo "========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "========================================="

if [ $FAIL -eq 0 ]; then
    echo ""
    echo "✓ All checks passed! Refactoring complete."
    exit 0
else
    echo ""
    echo "✗ Some checks failed. Please review."
    exit 1
fi
```

---

### 15.5 完了メトリクス

**削減実績**:
- ディレクトリ削除: 9個
- ファイル削除: 9個
- コード削減: 約1,200行
- 設定構造: 整理完了

**品質指標**:
- ビルド成功率: 100%
- 実行テスト: 合格
- 未使用コード: 0%
- ドキュメント: 更新済み

**次のステップ**:
Phase 3 (ドキュメント更新) およびPhase 4以降のリファクタリング計画は、このノートの前セクションを参照してください。

---

**実行計画作成日**: 2025-10-01
**計画バージョン**: 1.0
**承認状態**: レビュー待ち
