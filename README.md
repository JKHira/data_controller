# Trade Data Controller - TradeEngine2

暗号通貨AI自動取引プログラムのための(現在はBitfinex) WebSocketデータ収集・保存・データプロセシングシステム

## プロジェクト概要

このプログラムは、暗号通貨取引所 WebSocket APIからリアルタイム市場データを収集し、規定の方式で保存をしたり、特徴量を生成したりするGo言語製システム。データはMachine Learning訓練用に使用されたり、予測アルゴリズム、UIに状態を表示したりするために使われます。rawデータをApache Arrow形式で保存します。Fyne GUIインターフェースにより、ユーザーがWebSocket接続の開始・停止を制御し、リアルタイムデータストリームを監視、保存データを閲覧できます。

## フォルダーとファイル構造

```
/Volumes/SSD/AI/Trade/TradeEngine2/data_controller/
├── cmd/data-controller/           # メインアプリケーション
│   ├── main.go                   # エントリーポイント
│   ├── fyne_gui.go              # Fyne GUI実装（build tag: gui）
│   ├── stub_gui.go              # GUI無し版スタブ（build tag: !gui）
│   ├── nogui.go                 # ヘッドレス版実装
│   └── gui.go                   # ターミナルGUI実装
├── internal/                     # 内部パッケージ
│   ├── config/                  # 設定管理
│   │   └── config.go
│   ├── sink/arrow/              # Apache Arrowデータ保存
│   │   ├── schema.go            # Arrowスキーマ定義
│   │   ├── writer.go            # 基本Arrowライター
│   │   ├── channel_writer.go    # チャンネル別ライター
│   │   ├── handler.go           # データハンドラー
│   │   └── reader.go            # Arrowファイルリーダー
│   └── ws/                      # WebSocket接続管理
│       ├── conn.go
│       ├── router.go
│       └── connection_manager.go
├── pkg/schema/                   # データスキーマ定義
│   └── types.go
├── data/                         # データ保存ディレクトリ
│   └── bitfinex/v2/             # Bitfinexデータ（自動生成）
├── config.yml                   # 設定ファイル
├── Makefile                      # ビルド自動化
├── go.mod                        # Go依存関係管理
├── go.sum                        # Go依存関係チェックサム
└── README.md                     # このファイル
```

## 各ファイルの役割と機能

### メインアプリケーション (cmd/data-controller/)

#### main.go
- **役割**: プログラムのエントリーポイント
- **機能**:
  - コマンドライン引数の解析（-config, -nogui）
  - GUI版またはNoGUI版の選択・起動

#### fyne_gui.go (//go:build gui)
- **役割**: Fyne GUIアプリケーションの実装
- **機能**:
  - WebSocket接続・切断ボタン
  - 接続ステータス表示
  - リアルタイム統計情報表示
  - ファイルリスト表示・閲覧機能
  - **🆕 Arrowファイルビューア機能**（ページネーション付き）
  - **🆕 リアルタイムデータストリーム表示**（最新20件）
  - **🆕 ファイル内容表示のクローズ・ナビゲーション機能**
  - GUI要素のデータバインディング管理

#### stub_gui.go (//go:build !gui)
- **役割**: GUI機能無し版のスタブ実装
- **機能**:
  - ビルドタグによる条件コンパイル
  - GUI無しビルド時のダミー実装

#### nogui.go
- **役割**: ヘッドレス版（GUI無し）の実装
- **機能**:
  - 自動WebSocket接続・データ収集
  - ターミナル出力による状態表示
  - サーバー環境での無人実行

#### gui.go
- **役割**: ターミナルベースGUIの実装
- **機能**:
  - テキストベースインターフェース
  - 基本的な操作とステータス表示

### 内部パッケージ (internal/)

#### config/config.go
- **役割**: 設定管理システム
- **機能**:
  - YAML設定ファイルの読み込み
  - WebSocket接続設定
  - データ保存設定
  - 購読チャンネル・シンボル設定

#### sink/arrow/schema.go
- **役割**: Apache Arrowスキーマ定義
- **機能**:
  - 全データ型のArrowスキーマ定義
  - 共通フィールド構造（17フィールド）
  - データ型固有フィールド定義

#### sink/arrow/writer.go
- **役割**: Apache Arrowデータライター
- **機能**:
  - セグメント管理（自動ローテーション）
  - マルチチャンネル対応
  - ファイルライフサイクル管理
  - 統計情報収集

#### sink/arrow/channel_writer.go
- **役割**: チャンネル別Arrowライター
- **機能**:
  - バッチ書き込み（100件ずつ）
  - レコードバッチ生成・フラッシュ
  - データ型別ビルダー管理

#### sink/arrow/handler.go
- **役割**: データハンドラー・コーディネーター
- **機能**:
  - WebSocketデータ受信処理
  - **🆕 GUI向けリアルタイムコールバック機能**
  - 定期フラッシュ処理（設定可能間隔）
  - 統計情報管理

#### sink/arrow/reader.go
- **役割**: **🆕 Arrowファイルリーダー**
- **機能**:
  - Arrowファイル内容読み取り
  - ページネーション機能（メモリ効率化）
  - ファイル概要情報取得
  - 可読形式でのデータ表示

#### ws/connection_manager.go
- **役割**: WebSocket接続管理
- **機能**:
  - Bitfinex WebSocket API v2接続
  - チャンネル購読・購読解除
  - ハートビート監視・自動再接続
  - データルーティング・処理
  - 適切な切断処理

### スキーマ定義 (pkg/schema/)

#### types.go
- **役割**: データ型定義
- **機能**:
  - 共通データ構造定義
  - チャンネル・メッセージ型定数
  - データ変換インターフェース

### ビルドシステム

#### Makefile
- **役割**: **🆕 ビルド自動化**
- **機能**:
  - ヘッドレス版ビルド（`make build`）
  - GUI版ビルド（`make build-gui`）
  - 依存関係管理（`make deps`）
  - テスト実行（`make test`）
  - 実行ターゲット（`make run`, `make run-gui`）
  - クリーンアップ（`make clean`）

## 実装内容

### WebSocket接続機能
- Bitfinex WebSocket API v2との接続
- conf flags設定: TIMESTAMP, SEQ_ALL, OB_CHECKSUM, BULK_UPDATES
- チャンネル購読: ticker, trades, books, raw_books
- 自動ハートビート応答
- 接続エラー処理・自動再接続

### データ保存機能
- **🆕 Apache Arrow IPC形式での保存**（Parquetから変更）
- 効率的な列指向データ形式
- バッチ処理による高パフォーマンス
- 時間ベースディレクトリ構造
- セグメント管理による適切なファイルサイズ制御

### GUI機能
- Fyne v2フレームワーク使用
- ユーザー制御によるWebSocket接続・切断
- リアルタイム接続ステータス表示
- データ統計情報表示
- **🆕 Arrowファイルブラウザ・ビューアー**
- **🆕 ページネーション機能**（100レコード/ページ）
- **🆕 リアルタイムデータストリーム表示**（最新20件）
- **🆕 ファイル内容クローズ・RAM使用量管理**

## 基本動作の流れ

1. **ビルド**: `make build-gui` でGUI版、`make build` でヘッドレス版をビルド
2. **起動**: `./data-controller-gui` でGUI版、`./data-controller -nogui` でヘッドレス版を起動
3. **設定読み込み**: config.ymlから接続設定・購読チャンネル設定を読み込み
4. **GUI表示**: Fyneウィンドウが表示され、初期状態は「未接続」
5. **接続開始**: 「Connect」ボタンクリックでWebSocket接続開始
6. **チャンネル購読**: 設定されたシンボル・チャンネルに自動購読
7. **データ受信**: リアルタイムでマーケットデータを受信・処理
8. **データ保存**: Arrow形式でバッチ保存（100件ずつ、設定間隔フラッシュ）
9. **GUI更新**: 接続ステータス、統計情報、データストリームを表示更新
10. **ファイル閲覧**: 保存されたArrowファイルをページネーション機能で閲覧
11. **切断**: 「Disconnect」ボタンで適切な購読解除・切断処理
12. **終了**: ウィンドウクローズで全リソースの適切な解放

## GUIの表示内容

### メインウィンドウ構成
```
┌─────────────────────────────────────────────────────────────────┐
│ Data Controller                                        │
├─────────────────────────────────────────────────────────────────┤
│ [🔌 Connect] [⏹️ Disconnect]  Status: 🔴 Disconnected          │
├─────────────────────────────────────────────────────────────────┤
│ 📊 Statistics:               │ 📁 Data Files:                    │
│ ┌─────────────────────────┐   │ ┌─────────────────────────────┐   │
│ │ 📈 Tickers: 1,234       │   │ │ ticker_BTCUSD_20240101.arrow│   │
│ │ 💰 Trades: 567          │   │ │ trades_ETHUSD_20240101.arrow│   │
│ │ 📚 Book Levels: 8,901   │   │ │ books_BTCUSD_20240101.arrow │   │
│ │ 📝 Raw Books: 12,345    │   │ └─────────────────────────────┘   │
│ │ 🗂️ Segments: 5          │   │                                   │
│ │ ❌ Errors: 0            │   │ 👁️ File Viewer:                   │
│ └─────────────────────────┘   │ ┌─────────────────────────────┐   │
│                               │ │ ◀ Prev │ Next ▶ │ Page 1/10 │   │
│ 📡 Live Data Stream:          │ │ ✕ Close                     │   │
│ ┌─────────────────────────────┤ ├─────────────────────────────────┤
│ │ [15:04:23] 📈 BTCUSD: Bid=  │ │ 📊 Arrow File: ticker_BTCUSD... │
│ │ [15:04:22] 💰 BTCUSD: BUY   │ │ 📏 File Size: 2,456,789 bytes   │
│ │ [15:04:21] 📚 BTCUSD: BID   │ │ 📈 Total Records: 12,345        │
│ │ [15:04:20] 📝 BTCUSD: UPDATE│ │ ┌───────────────────────────────┤
│ │ ...                         │ │ │ Record #1:                    │
│ └─────────────────────────────┘ │ │   exchange: bitfinex          │
├─────────────────────────────────┤ │   symbol: BTCUSD              │
│ 🎯 Symbols: BTCUSD, ETHUSD     │ │   bid: 43250.50               │
│ 💾 Storage: data/bitfinex       │ │   ask: 43251.00               │
└─────────────────────────────────┘ └─────────────────────────────────┘
```

### 🆕 新しい表示要素
- **リアルタイムデータストリーム**: 受信データをリアルタイム表示（最新20件）
- **Arrowファイルビューア**: ファイル内容を可読形式で表示
- **ページネーション**: 大容量ファイルを100レコードずつ表示
- **ナビゲーション**: Previous/Next/Close ボタン
- **RAM管理**: ファイルクローズ時にメモリクリア

## 使用方法

### 1. 簡単ビルド（Makefile使用）
```bash
# 依存関係更新
make deps

# ヘッドレス版ビルド
make build

# GUI版ビルド
make build-gui

# テスト実行
make test

# データファイル確認
make check-data
```

### 2. 手動ビルド
```bash
go mod tidy

# ヘッドレス版
go build -o data-controller cmd/data-controller/main.go cmd/data-controller/nogui.go cmd/data-controller/gui.go cmd/data-controller/stub_gui.go

# GUI版
CC=clang CXX=clang++ go build -tags gui -o data-controller-gui cmd/data-controller/main.go cmd/data-controller/nogui.go cmd/data-controller/gui.go cmd/data-controller/fyne_gui.go
```

### 3. 設定
config.ymlで以下を設定:
- WebSocket接続設定（URL、タイムアウト）
- 購読シンボル（BTCUSD、ETHUSD等）
- 購読チャンネル（ticker、trades、books等）
- データ保存パス

### 4. 実行
```bash
# GUI版
make run-gui
# または
./data-controller-gui

# ヘッドレス版
make run
# または
./data-controller -nogui

# ターミナルGUI版
./data-controller

# 設定ファイル指定
./data-controller-gui -config custom_config.yml
```

### 5. 操作
- **GUI版**: 「Connect」ボタンでデータ収集開始、「Disconnect」で停止
- **ファイル閲覧**: Data Filesリストからファイル選択、ページネーションで閲覧
- **ライブストリーム**: 接続中に最新20件のデータをリアルタイム表示
- **ヘッドレス版**: 自動でデータ収集開始、Ctrl+Cで停止

## 保存フォーマットと構造

### ディレクトリ構造
```
data/bitfinex/v2/{channel}/{symbol}/dt={date}/
```

例:
```
data/bitfinex/v2/ticker/BTCUSD/dt=2024-01-15/
├── ticker_BTCUSD_20240115_143000_001.arrow
├── trades_BTCUSD_20240115_143000_001.arrow
└── books_BTCUSD_20240115_143000_001.arrow
```

### Apache Arrowスキーマ
```go
// 共通フィールド（17フィールド）
type CommonFields struct {
    Exchange        string  // 取引所名
    Channel         string  // チャンネル名
    Symbol          string  // シンボル
    TsMicros        int64   // タイムスタンプ（マイクロ秒）
    IngestID        string  // インジェストID
    SeqNum          int64   // シーケンス番号
    // ... その他共通フィールド
}

// ティッカーデータ
type Ticker struct {
    CommonFields
    Bid             float64 // 買い価格
    Ask             float64 // 売り価格
    Last            float64 // 最終取引価格
    Vol             float64 // 出来高
    High            float64 // 高値
    Low             float64 // 安値
    Change          float64 // 変動
    ChangePercent   float64 // 変動率
}

// 取引データ
type Trade struct {
    CommonFields
    TradeID         int64   // 取引ID
    Price           float64 // 価格
    Amount          float64 // 数量
}

// オーダーブックデータ
type BookLevel struct {
    CommonFields
    Price           float64 // 価格
    Amount          float64 // 数量
    Side            int     // サイド（0=bid, 1=ask）
    Count           int64   // 注文数
}
```

### データ形式
- **フォーマット**: Apache Arrow IPC
- **バッチサイズ**: 100レコード
- **フラッシュ間隔**: 設定可能（デフォルト2秒）
- **ファイルローテーション**: 時間ベース・サイズベース

## 現在の完成度と今後の実装

### ✅ 完成済み機能
- Bitfinex WebSocket接続・データ受信
- **🆕 Apache Arrow形式でのデータ保存**
- **🆕 Fyne GUI実装（ページネーション付きファイルビューア）**
- **🆕 リアルタイムデータストリーム表示**
- **🆕 Arrowファイル可読表示機能**
- **🆕 RAM使用量管理・メモリリーク防止**
- 適切なバッチ処理・フラッシュ機能
- 接続状態管理・統計表示
- **🆕 Makefile による統合ビルドシステム**
- **🆕 ビルドタグによる条件コンパイル**

### 🔄 改善が必要な項目
1. **エラーハンドリング強化**
   - WebSocket接続エラーの詳細表示
   - ファイル保存エラーの通知機能

2. **パフォーマンス最適化**
   - メモリ使用量監視
   - 大量データ処理時の最適化

3. **設定機能拡張**
   - GUI内での設定変更機能
   - 動的チャンネル購読・解除

### 🚀 今後実装するべき内容
1. **AI・ML統合機能**
   - 保存データのML前処理機能
   - リアルタイム予測モデル統合
   - 取引シグナル生成

2. **高度なデータ分析**
   - オーダーブック分析
   - 価格変動アラート
   - 統計指標計算・表示

3. **取引機能**
   - Bitfinex取引API統合
   - 自動取引戦略実装
   - リスク管理機能

4. **運用機能**
   - ログ管理・ローテーション
   - メトリクス・監視機能
   - データベース統合

5. **ユーザビリティ**
   - チャート表示機能
   - データエクスポート機能
   - 設定のインポート・エクスポート

## 技術的詳細

### 依存関係
- **Go**: 1.24.0以上
- **Fyne**: v2.6.3 (GUI)
- **🆕 Apache Arrow**: v17.0.0 (データ保存、Parquetから変更)
- **websocket**: v1.5.1 (WebSocket接続)
- **zap**: v1.27.0 (ログ出力)

### コンパイル要件
- **macOS**: Apple Clang (CC=clang, CXX=clang++)
- **Linux/Windows**: 標準Goコンパイラ
- **🆕 ビルドタグ**: GUI版（`-tags gui`）、ヘッドレス版（デフォルト）

### 🆕 重要な変更点
1. **Parquet → Apache Arrow移行**: より成熟したライブラリで安定性向上
2. **Live Data Stream実装**: リアルタイムデータ表示機能
3. **Arrowファイルビューア**: バイナリファイルの可読表示
4. **ページネーション**: 大容量ファイルのメモリ効率的な閲覧
5. **Makefile導入**: 開発効率向上
6. **ビルドタグ活用**: 条件コンパイルによる最適化

このプログラムは、暗号通貨取引のためのデータ収集基盤として機能し、今後のAI自動取引システム開発の土台となります。Apache Arrow形式での保存により、データ分析・ML処理の効率が大幅に向上しました。