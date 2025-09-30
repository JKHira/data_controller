# Data Controller: AI自動取引システム開発の勘所

## 🎯 プロジェクト概要

複数の取引所からのデータ収集システム基盤として、**数分〜2時間の短期取引**に特化したAI自動取引システムのData Controllerを構築する。

## 📋 現在の基盤と活用可能資産

### ✅ 既存の強固な基盤
- **Apache Arrow**: 高速データ保存・読み込み（L2/L3生データ対応済み）
- **WebSocket管理**: 自動再接続、ハートビート、メッセージルーティング
- **REST API**: レート制限対応、エラーハンドリング
- **Fyne GUI**: クロスプラットフォーム対応UI
- **設定管理**: YAML設定、ホットリロード（SIGHUP）
- **プロジェクト構造**: 内部パッケージの適切な分離

### 🔄 拡張が必要な領域
- **特徴生成エンジン**: OFI/スプレッド分位等のリアルタイム計算
- **MLX連携**: cgo経由でのニューラルネットワーク計算や得微量の作成
- **取引エンジン連携**: Freqtrade REST/WebSocket API
- **監視・アラート**: Prometheus metrics、ヘルスチェック
- **非常停止機能**: kill-switch実装

## 🏗 アーキテクチャ設計の勘所

### 1. データフロー設計
```
WebSocket (生L2/L3) → 特徴生成 → MLX/CPU計算 → 取引判断 → Freqtrade → 監視
                   ↓
              Apache Arrow (生データ)
                   ↓
              InfluxDB (特徴量・要約データ)
                   ↓
              Grafana (可視化)
```

### 2. プロセス設計原則
- **単一プロセス**: Trade Engine（Go）で全体制御
- **goroutine活用**: 各API接続を独立したgoroutineで並行処理
- **チャンネル通信**: goroutine間の安全なデータ受け渡し
- **リソース管理**: runtime.LockOSThread()でMLX計算専用スレッド確保

### 3. フォルト・トレラント設計
- **段階的縮退**: MLX落ち→CPU計算のみ→ポジション管理のみ
- **状態永続化**: 重要な状態をディスクに定期保存
- **自動復旧**: launchdでプロセス監視・自動再起動

## 🧮 特徴生成エンジン設計

### 1. リアルタイム特徴量計算（1-5秒間隔）
```go
type FeatureEngine struct {
    window       time.Duration  // 1-5秒の計算ウィンドウ
    buffer       *CircularBuffer // L2データのローリングバッファ
    calculator   *OFICalculator  // Order Flow Imbalance計算
    spreads      *SpreadAnalyzer // スプレッド分位計算
    output       chan []float32  // 固定長float32配列出力
}
```

### 2. 計算の最適化
- **SIMD活用**: Go 1.21+のSIMD命令でベクトル計算
- **メモリプール**: sync.Poolで配列の再利用
- **バッチ処理**: 複数ペアの特徴量を一括計算

### 3. 品質管理
- **データ品質チェック**: 欠損値、異常値の検出・補間
- **計算精度監視**: 特徴量の統計的妥当性チェック
- **レイテンシ監視**: 計算時間の計測・アラート

## 🔌 MLX連携の実装戦略

### 1. cgo直接呼び出し（最優先）
```go
// #cgo CFLAGS: -I./mlx_bridge
// #cgo LDFLAGS: -L./mlx_bridge -lmlx_bridge
// #include "mlx_bridge.h"
import "C"

func (m *MLXEngine) Predict(features []float32) ([]float32, error) {
    runtime.LockOSThread()
    defer runtime.UnlockOSThread()

    // C関数呼び出し
    result := C.mlx_predict((*C.float)(&features[0]), C.int(len(features)))
    return convertCArrayToGo(result), nil
}
```

### 2. gRPC/UDS フォールバック
```go
type MLXClient struct {
    conn   *grpc.ClientConn
    client pb.MLXServiceClient
    uds    string // Unix Domain Socket path
}
```

### 3. エラーハンドリング
- **接続監視**: ヘルスチェックで MLX プロセス状態監視
- **タイムアウト**: 推論計算の最大許容時間設定
- **フォールバック**: MLX障害時のCPU計算への自動切り替え

## 💱 Freqtrade連携設計

### 1. REST API制御
```go
type FreqtradeClient struct {
    baseURL    string
    httpClient *http.Client
    cmdTracker map[string]*Command // client_cmd_id追跡
}

// 主要操作
func (f *FreqtradeClient) ForceBuy(pair string, amount float64) error
func (f *FreqtradeClient) ForceSell(orderID string) error
func (f *FreqtradeClient) SetStrategy(strategy string) error
func (f *FreqtradeClient) EmergencyStop() error // kill-switch
```

### 2. WebSocket購読
```go
type FreqtradeWS struct {
    conn     *websocket.Conn
    orders   chan OrderEvent
    trades   chan TradeEvent
    status   chan StatusEvent
}
```

### 3. 注文管理
- **冪等性**: client_cmd_idで重複送信防止
- **整合性**: REST注文とWS約定イベントの照合
- **監査**: 全取引決定の理由（特徴値・確率・EV）をログ記録

## 📊 監視・アラートシステム

### 1. Prometheus メトリクス
```go
var (
    dataIngestRate = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "data_controller_ingest_rate",
            Help: "Data ingestion rate per second",
        },
        []string{"exchange", "channel"},
    )

    predictionLatency = prometheus.NewHistogram(
        prometheus.HistogramOpts{
            Name: "prediction_latency_seconds",
            Help: "MLX prediction latency",
        },
    )

    killSwitchActive = prometheus.NewGauge(
        prometheus.GaugeOpts{
            Name: "kill_switch_active",
            Help: "Kill switch status (1=active, 0=inactive)",
        },
    )
)
```

### 2. ヘルスチェック（/healthz）
```go
type HealthStatus struct {
    Overall    string            `json:"overall"`
    Components map[string]string `json:"components"`
    Timestamp  time.Time         `json:"timestamp"`
}

// チェック項目
// - WebSocket接続状態
// - MLX計算エンジン応答
// - Freqtrade接続
// - ディスク使用量
// - メモリ使用量
```

## 🛡 非常停止（Kill-Switch）設計

### 1. 多層防御
```go
type KillSwitch struct {
    active     atomic.Bool
    triggers   []TriggerCondition
    actions    []EmergencyAction
    notify     chan struct{}
}

// トリガー条件
// - 手動操作（GUI/API）
// - 異常損失検出
// - システムリソース枯渇
// - 外部API障害
```

### 2. 即座の実行
- **全発注停止**: Freqtrade強制停止
- **ポジションクローズ**: 市場注文での強制決済
- **システム停止**: データ収集・計算の安全な停止
- **通知**: アラート送信（GUI・ログ・外部通知）

## 📁 プロジェクト構造の現状と拡張

### 現在の実装済み構造
```
data_controller/
├── cmd/data-controller/          # エントリーポイント
│   ├── main.go                  # メイン起動ロジック
│   ├── fyne_gui.go              # GUI初期化（モジュラー化済み）
│   ├── gui.go                   # GUI有効化フラグ
│   ├── gui_stub.go              # ビルドタグ用GUIスタブ
│   └── nogui.go                 # CLI実行ロジック
│
├── internal/
│   ├── config/                  # 設定管理
│   │   ├── config.go           # 設定構造体定義
│   │   └── loader.go           # YAML設定読み込み
│   │
│   ├── domain/                  # ドメイン層（新規追加）
│   │   ├── file_item.go        # ファイル項目ドメインモデル
│   │   └── scan_params.go      # スキャンパラメーター定義
│   │
│   ├── gui/                    # GUI システム（モジュラー化完了）
│   │   ├── app/                # アプリケーション層
│   │   │   └── app.go          # メインアプリケーション構造
│   │   ├── controllers/        # コントローラー層
│   │   │   └── file_controller.go  # ファイル操作ロジック（domain対応済み）
│   │   ├── panels/             # プレゼンテーション層
│   │   │   ├── files_panel.go  # 高機能ファイル一覧パネル（フィルター・非同期スキャン対応）
│   │   │   └── viewer_panel.go # ファイルビューアーパネル
│   │   ├── state/              # 状態管理
│   │   │   └── app_state.go    # アプリケーション状態（domain.FileItem対応）
│   │   ├── file_viewer.go      # ファイル表示共通機能
│   │   ├── panes.go            # ペイン構成
│   │   ├── rest_api_panel.go   # REST API操作パネル
│   │   ├── top_bar.go          # 上部バー
│   │   ├── bottom_bar.go       # 下部ステータスバー
│   │   ├── data_files.go       # データファイルパネル
│   │   └── live_stream.go      # リアルタイム表示
│   │
│   ├── restapi/                # REST API クライアント
│   │   ├── bitfinex_client.go  # Bitfinex API実装
│   │   ├── arrow_storage.go    # Arrowストレージ（restapi対応）
│   │   └── utils.go            # ユーティリティ関数
│   │
│   ├── sink/arrow/             # データ保存エンジン
│   │   ├── writer.go           # Arrowライター（websocket対応）
│   │   ├── reader.go           # Arrowリーダー & メタデータスキャン（後方互換）
│   │   ├── schema.go           # スキーマ定義
│   │   ├── handler.go          # データハンドラー
│   │   └── channel_writer.go   # チャンネル別ライター
│   │
│   ├── ws/                     # WebSocket 実装
│   │   ├── conn.go             # WebSocket接続管理
│   │   └── router.go           # メッセージルーティング
│   │
│   └── services/               # ビジネスロジック
│       ├── file_reader.go      # ファイル読み込みサービス
│       └── file_scanner.go     # 高機能ファイルスキャン（レガシー対応・非同期処理）
│
├── data/                       # データ保存先
│   └── bitfinex/
│       ├── websocket/          # WebSocketデータ（旧v2から変更）
│       │   ├── trades/         # 取引データ
│       │   ├── books/          # 板データ
│       │   └── ticker/         # ティッカーデータ
│       └── restapi/            # REST APIデータ（旧restv2から変更）
│           └── basedata/       # ベースデータ
│               └── manifest.jsonl  # メタデータインデックス
│
├── config.yml                 # 設定ファイル
├── Makefile                   # ビルド設定
└── CLAUDE.md                  # 開発指針（本ファイル）
```

### 今後の拡張予定
```
data_controller/
├── internal/                   # 既存構造を拡張
│   ├── features/              # 特徴生成エンジン（新規）
│   │   ├── calculator.go     # OFI・スプレッド計算
│   │   ├── buffer.go         # ローリングバッファ
│   │   └── pipeline.go       # 特徴量パイプライン
│   │
│   ├── mlx/                  # MLX連携（新規）
│   │   ├── cgo_bridge.go     # cgo直接呼び出し
│   │   ├── grpc_client.go    # gRPC/UDSクライアント
│   │   └── predictor.go      # 予測エンジン
│   │
│   ├── trading/              # 取引制御（新規）
│   │   ├── freqtrade.go      # Freqtrade API
│   │   ├── orders.go         # 注文管理
│   │   └── risk.go           # リスク管理
│   │
│   ├── monitoring/           # 監視（新規）
│   │   ├── metrics.go        # Prometheusメトリクス
│   │   ├── health.go         # ヘルスチェック
│   │   └── alerts.go         # アラート
│   │
│   ├── killswitch/           # 非常停止（新規）
│   │   ├── switch.go         # Kill-switch実装
│   │   └── triggers.go       # トリガー条件
│   │
│   └── storage/              # ストレージ拡張（新規）
│       ├── influx.go         # InfluxDB連携
│       └── timeseries.go     # 時系列データ管理
│
├── mlx_bridge/               # C++/MLXブリッジ（新規）
│   ├── mlx_bridge.h
│   ├── mlx_bridge.cpp
│   └── Makefile
│
└── docker/                   # Docker設定（新規）
    └── influxdb/
        ├── docker-compose.yml
        └── init.sql
```

## 🚀 実装フェーズ

### Phase 1: 基盤強化
1. **特徴生成エンジン**: OFI・スプレッド計算の実装
2. **InfluxDB連携**: 特徴量の時系列保存
3. **Prometheus監視**: 基本メトリクス収集

### Phase 2: AI連携
1. **MLX bridge**: C++/cgoブリッジ実装
2. **予測パイプライン**: 特徴量→MLX→取引判断
3. **CPU計算フォールバック**: 軽量アルゴリズム実装

### Phase 3: 取引制御
1. **Freqtrade連携**: REST/WebSocket API実装
2. **注文管理**: 冪等性・整合性確保
3. **リスク管理**: ポジション・損失監視

### Phase 4: 運用強化
1. **Kill-switch**: 非常停止機能実装
2. **アラート**: 多チャンネル通知
3. **launchd**: 自動起動・監視設定

## ⚠️ 重要な考慮事項

### 1. パフォーマンス
- **レイテンシ**: WebSocket受信→取引判断を10ms以内
- **スループット**: 10,000+ ticks/秒の処理能力
- **メモリ**: 64GB環境でのリソース最適化

### 2. 信頼性
- **データ整合性**: L2/L3データの品質保証
- **計算精度**: 特徴量・予測の数値精度
- **取引安全性**: 誤発注防止・リスク管理

### 3. 運用性
- **監視**: リアルタイム状態監視
- **ログ**: 監査可能な詳細ログ
- **復旧**: 障害時の自動復旧手順

### 4. 開発効率
- **既存資産活用**: 現在のコードベース最大活用
- **段階的実装**: 段階的な機能追加・テスト
- **設定駆動**: YAMLでの柔軟な設定変更

## 🎯 最近の実装進捗

### ✅ 完了済み（2024年後期〜2025年前期）

#### GUI システムのモジュラー化
- **File Viewer 改善**: 灰色文字問題を修正（`Disable()` → `SetReadOnly(true)`）
- **表示件数増加**: ファイルビューアーで3000件のレコードを一度に表示可能
- **アーキテクチャ分割**: fyne_gui.goの巨大なファイルを責務ベースで分割
  - 状態管理: `internal/gui/state/app_state.go`
  - コントローラー: `internal/gui/controllers/file_controller.go`
  - プレゼンテーション: `internal/gui/panels/{files,viewer}_panel.go`
  - アプリケーション: `internal/gui/app/app.go`
- **ビルドシステム改善**: ビルドタグ対応のスタブファイル追加
- **UIスレッド安全性**: `fyne.Do()` による非同期UI操作の安全化

#### ファイル管理システムの大幅強化
- **ドメイン層の導入**: UI層とデータアクセス層の分離（`internal/domain/`）
- **高機能フィルタリング**: Source/Category/Symbol/Date範囲/Hour/Type による詳細フィルター
- **非同期スキャン**: コンテキストキャンセル対応の並行ファイル検索
- **ディレクトリ構造変更**: `v2` → `websocket`, `restv2` → `restapi` へ移行
- **後方互換性**: 旧ディレクトリ構造の自動検出・対応

#### REST API機能の拡張
- **垂直レイアウト**: REST APIパネルを垂直配置に変更して幅を削減
- **ベースデータ取得**: Bitfinex REST APIからの各種基本データ取得機能
- **進捗監視**: データ取得の進捗をリアルタイム表示
- **Apache Arrow統合**: REST APIデータのArrow形式保存

#### データ処理エンジンの改良
- **Apache Arrow最適化**: 大容量ファイルの効率的な読み込み
- **メタデータ強化**: ファイル一覧での詳細情報表示
- **パフォーマンス向上**: 3000件レコード表示の高速化
- **ファイルスキャナ強化**: 複雑な条件での高速ファイル検索

### 🏗 アーキテクチャの進化

#### 分離前（問題）
```go
// fyne_gui.go - 800+ 行の巨大ファイル
type FyneGUIApplication struct {
    // 50+ フィールドが混在
    // UI、状態、ロジックが全て混合
}
```

#### 分離後（解決）
```go
// 責務ベースの分離
├── state/app_state.go          # 状態管理のみ
├── controllers/file_controller.go  # ビジネスロジックのみ
├── panels/files_panel.go       # UI表示のみ
└── app/app.go                  # アプリケーション制御のみ
```

#### 得られた利点
- **テスタビリティ**: 各コンポーネントが独立してテスト可能
- **保守性**: 機能ごとにファイルが分離され、理解・修正が容易
- **拡張性**: 新機能追加時の影響範囲が限定的
- **再利用性**: 汎用コンポーネントとして他の機能でも利用可能

### 🔧 技術的な改善

#### File Viewer の品質向上
```go
// 問題: 灰色で読みにくい
a.fileViewer.Disable() // Read-only

// 解決: 通常の白文字を維持
if readOnlyEntry, ok := interface{}(a.fileViewer).(interface{ SetReadOnly(bool) }); ok {
    readOnlyEntry.SetReadOnly(true)
}
```

#### 非同期処理とUIスレッド安全性
```go
// 問題: goroutineからのUI操作でスレッドエラー
go func() {
    fp.statusLabel.SetText("更新中...") // エラー
}()

// 解決: fyne.Do()による安全なUI更新
func (fp *FilesPanel) ui(f func()) {
    fyne.Do(f)
}

go func() {
    fp.ui(func() {
        fp.statusLabel.SetText("更新中...") // 安全
    })
}()
```

#### ドメイン駆動設計の導入
```go
// 問題: UIレイヤーが直接Arrowファイル構造に依存
type AppState struct {
    FilteredFiles []arrow.FileInfo // 密結合
}

// 解決: ドメインモデルによる抽象化
type AppState struct {
    FilteredFiles []domain.FileItem // 疎結合
}

type FileItem struct {
    Path     string
    Exchange string
    Source   string
    Category string
    Symbol   string
    Date     string
    Size     int64
    ModTime  time.Time
}
```

#### 大量データ表示の最適化
```go
// 問題: 50件制限で使いにくい
maxRecords := min(50, len(pageData.Records))

// 解決: 3000件表示で実用的
maxRecords := min(a.pageSize, len(pageData.Records)) // pageSize = 3000
```

## 📝 Next Steps

### 短期目標（Phase 1 継続）
1. **パフォーマンス調整**: 2秒毎のファイルスキャン頻度の最適化
2. **エラーハンドリング強化**: 非同期処理でのエラー処理改善
3. **統合テスト**: リファクタリング後の機能確認
4. **メタデータキャッシュ**: 頻繁なファイルスキャンの効率化

### 中期目標（Phase 2 準備）
1. **特徴生成エンジン**: OFI・スプレッド計算の詳細設計
2. **MLXブリッジ**: C++コードとcgo連携の実装方針確定
3. **InfluxDB環境**: Docker環境のセットアップ
4. **監視基盤**: Prometheus/Grafana環境構築

### 長期目標（Phase 3-4）
1. **AI連携基盤**: MLX統合とFreqtrade連携
2. **運用監視**: Kill-switch、アラート、自動復旧
3. **本格運用**: launchd設定と24時間稼働体制

既存のBitfinexデータ収集システムが提供する堅固な基盤に加え、モジュラー化されたGUIアーキテクチャを基盤として、段階的にAI自動取引システムへ発展させていく戦略です。