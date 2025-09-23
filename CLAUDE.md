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

## 📁 プロジェクト構造の拡張

```
data_controller/
├── internal/
│   ├── features/              # 特徴生成エンジン
│   │   ├── calculator.go     # OFI・スプレッド計算
│   │   ├── buffer.go         # ローリングバッファ
│   │   └── pipeline.go       # 特徴量パイプライン
│   │
│   ├── mlx/                  # MLX連携
│   │   ├── cgo_bridge.go     # cgo直接呼び出し
│   │   ├── grpc_client.go    # gRPC/UDSクライアント
│   │   └── predictor.go      # 予測エンジン
│   │
│   ├── trading/              # 取引制御
│   │   ├── freqtrade.go      # Freqtrade API
│   │   ├── orders.go         # 注文管理
│   │   └── risk.go           # リスク管理
│   │
│   ├── monitoring/           # 監視
│   │   ├── metrics.go        # Prometheusメトリクス
│   │   ├── health.go         # ヘルスチェック
│   │   └── alerts.go         # アラート
│   │
│   ├── killswitch/           # 非常停止
│   │   ├── switch.go         # Kill-switch実装
│   │   └── triggers.go       # トリガー条件
│   │
│   └── storage/              # ストレージ拡張
│       ├── influx.go         # InfluxDB連携
│       └── timeseries.go     # 時系列データ管理
│
├── mlx_bridge/               # C++/MLXブリッジ
│   ├── mlx_bridge.h
│   ├── mlx_bridge.cpp
│   └── Makefile
│
└── docker/                   # Docker設定
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

## 📝 Next Steps

1. **Phase 1実装開始**: 特徴生成エンジンの詳細設計
2. **MLXブリッジ**: C++コードとcgo連携の実装方針確定
3. **InfluxDB環境**: Docker環境のセットアップ
4. **監視基盤**: Prometheus/Grafana環境構築

既存のBitfinexデータ収集システムが提供する堅固な基盤を活用し、段階的にAI自動取引システムへ発展させていく戦略です。