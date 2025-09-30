Claude AI へ　このファイルは変更をしてはいけません。

暗号通貨のAI自動取引プログラムの中継システムでデータのやり取りやプロセス、GUIのコントロールをするシステムであるData Controller作成をします。
構成は概ね次のように決まっています

** Trade Engine（常駐1プロセス）**

一部の例外を除いて大抵のシステム制作にはGo langを使用する。システムM1Max　64GBメモリ。


- **WS取込**：取引所WSをgoroutineで多重接続（再接続・心拍）    Bitfinexから開発開始。
- **特徴生成/バッチ**：1–5秒でOFI/スプレッド分位などを固定長float32配列に。
- **Mac MLX-NN計算/ブリッジ計算**：
    - 最速：**cgoでCラッパ関数を直呼び**（バッチ渡し、runtime.LockOSThread()）。
    - 次善：**gRPC/UDS**で常駐MLXサーバにストリーミング。
- **軽量の予測アルゴリズム計算**　go 言語でCPUを使用した計算。 
- **トレード制御**：**Freqtrade REST**でforcebuy/forcesell/start/stop等、**WS**で約定イベント購読    
- **保存**：大量のリアルタイムデータはArrow Apache-go（生L2）＋Influx/Prometheusは特微量や要約データ転送。
- **監視**：/healthz・/metrics（Prometheus）。
- **非常停止**：kill-switch（全発注停止/クローズ）を即時反映。
- **設定**：YAML/ENV＋**ホットリロード**（SIGHUP）。
    

**可視化**
- **Grafana**（ユーザーによる、Influx/Prometheusデータ表示やリアルタイムの状態可視化）
-  **Fyne/Go製の軽UI**（ユーザーによる入力、ボタン・パラメータ変更）


**起動・復旧**
- macなら **launchd**　自動再起動・ログ回収



- **InfluxDB は “だけ” Docker 駆動を推奨**
    
    - 理由：**バージョン固定・隔離・破棄/再作成が容易**、データはボリュームで永続化、**バックアップ自動化が楽**。
    - Mac（Apple Silicon）の Docker でも **この用途（特徴量・要約だけ）なら I/O/レイテンシのオーバーヘッドは実質許容**。生の L2 は Parquet 側なので問題なし。
    ###  InfluxDB を Docker に閉じ込める時の要点

- **データは named volume** に置く（**bind mount は不可**：Mac のファイル共有は I/O が遅い）。
- **バックアップ専用の bind mount** を別途用意（ホストに吐き出す先だけ bind mount）。
- **ローカル限定公開**：127.0.0.1:8086 にバインド。
- **ヘルスチェック／自動再起動**：restart: unless-stopped＋/health。
- バケット設計：
    - 例）rt-fast（7d保持、リアルタイム特徴）、agg-30s（30d保持、ダウンサンプル）、logs（90d保持、監査）。
- **Freqtrade は conda ネイティブでOK**。REST/WebSocket を **127.0.0.1 のみ**で公開し、Go から制御する形に。launchd で起動・監視を。

---

# **実装チェックリスト（要点だけ）**

- 接続：HTTPは**接続プール＋短タイムアウト**、WSは**自動再接続**と**再購読**。
- 品質：**バッチ化**（cgo/MLX呼び出し回数を削減）／**二重送信の冪等化**（client_cmd_id）。
- バックプレッシャ：保存キューが溜まったら**サンプリング率↑**や**要約粒度↑**。
- フェイルセーフ：MLX落ち→**モデル無しモード**（ノートレ/既存ポジのみ管理）に自動切替。
- ログ/監査：注文IDで**REST↔WS整合**、すべての決定に**理由（特徴値・確率・EV）**を添付して記録。


# 使用予定AIモデル
| **モデル群**                                      | **非線形の表現力**                 | **時系列の“順序”の扱い**                     | **ドリフト対応**                         | **推論/学習の軽さ（M1 Max想定）**  | **特徴エンジニアリング依存** | **典型ユース**                     |
| ------------------------------------------------- | ---------------------------------- | -------------------------------------------- | ---------------------------------------- | ---------------------------------- | ---------------------------- | ---------------------------------- |
| SGD / FTRL / Passive-Aggressive（オンライン線形） | 低〜中（拡張特徴で底上げ）         | 明示的には扱わない（ラグ・ローリングを自作） | 強い（逐次更新◎）                        | 最軽量（μs〜ms台）                 | **高い**                     | 低遅延、連続学習、ベースライン     |
| LightGBM / HistGBDT（小型GBDT）                   | 中〜高                             | 暗黙的（時系列特徴を入れればOK）             | 中（オンラインは不得手、バッチ更新向き） | 軽い（ms台、200〜500本の小規模樹） | **中**                       | 日次再学習で非線形吸収、解釈性     |
| TCN / 1D-CNN（TinyLOB系                           | 中（ローカル時系列パターンに強い） | ネイティブに扱う（causal/dilated conv）      | 中（再学習は必要、軽量化で実用）         | 軽い（小型構成でms〜数ms）         | **低〜中（生波形でも可）**   | 板の連続パターン検出、原系列学習   |
| HMM/カルマン＋ロジスティックのハイブリッド        | 低〜中（レジーム別で底上げ）       | レジームで間接的に扱う                       | 強い（レジーム自体がドリフト表現）       | 軽い（状態推定はμs〜ms）           | **中**                       | 市況別切替・安定化（ゲーティング） |



---
# 参考リンク集：

## Kraken Webocket APIs
https://docs.kraken.com/api/docs/guides/global-intro

User Trading:
https://docs.kraken.com/api/docs/guides/spot-ws-intro
https://docs.kraken.com/api/docs/websocket-v2/add_order
https://docs.kraken.com/api/docs/websocket-v2/amend_order
https://docs.kraken.com/api/docs/websocket-v2/cancel_order
https://docs.kraken.com/api/docs/websocket-v2/cancel_all
https://docs.kraken.com/api/docs/websocket-v2/cancel_after
https://docs.kraken.com/api/docs/websocket-v2/batch_add
https://docs.kraken.com/api/docs/websocket-v2/batch_cancel
https://docs.kraken.com/api/docs/websocket-v2/edit_order

User Data:
https://docs.kraken.com/api/docs/websocket-v2/executions
https://docs.kraken.com/api/docs/websocket-v2/balances

Market data:
https://docs.kraken.com/api/docs/websocket-v2/ticker
https://docs.kraken.com/api/docs/websocket-v2/book
https://docs.kraken.com/api/docs/websocket-v2/level3
https://docs.kraken.com/api/docs/websocket-v2/ohlc
https://docs.kraken.com/api/docs/websocket-v2/trade
https://docs.kraken.com/api/docs/websocket-v2/instrument

Admin:
https://docs.kraken.com/api/docs/websocket-v2/status
https://docs.kraken.com/api/docs/websocket-v2/heartbeat
https://docs.kraken.com/api/docs/websocket-v2/ping


# Kraken Rest APIs
https://docs.kraken.com/api/docs/guides/spot-rest-intro
https://docs.kraken.com/api/docs/rest-api/get-tradable-asset-pairs

---
## bitfinex websockets links:
https://docs.bitfinex.com/docs/ws-general
https://docs.bitfinex.com/docs/ws-public
https://docs.bitfinex.com/docs/ws-auth
https://docs.bitfinex.com/docs/ws-reading-the-documentation
https://docs.bitfinex.com/docs/ws-websocket-checksum
https://docs.bitfinex.com/docs/abbreviations-glossary
https://docs.bitfinex.com/docs/flag-values
https://docs.bitfinex.com/reference/ws-public-ticker
https://docs.bitfinex.com/reference/ws-public-trades
https://docs.bitfinex.com/reference/ws-public-books
https://docs.bitfinex.com/reference/ws-public-raw-books
https://docs.bitfinex.com/reference/ws-public-candles
https://docs.bitfinex.com/reference/ws-public-status

---
## bitfinex public rest api links:
https://docs.bitfinex.com/reference/rest-public-platform-status
https://docs.bitfinex.com/reference/rest-public-ticker
https://docs.bitfinex.com/reference/rest-public-tickers
https://docs.bitfinex.com/reference/rest-public-tickers-history
https://docs.bitfinex.com/reference/rest-public-trades
https://docs.bitfinex.com/reference/rest-public-book
https://docs.bitfinex.com/reference/rest-public-stats
https://docs.bitfinex.com/reference/rest-public-candles
https://docs.bitfinex.com/reference/rest-public-derivatives-status
https://docs.bitfinex.com/reference/rest-public-derivatives-status-history
https://docs.bitfinex.com/reference/rest-public-liquidations
https://docs.bitfinex.com/reference/rest-public-conf
post method...
https://docs.bitfinex.com/reference/rest-public-market-average-price
https://docs.bitfinex.com/reference/rest-public-foreign-exchange-rate

---
## apache arrow go docs links:
https://pkg.go.dev/github.com/apache/arrow/go/arrow
https://arrow.apache.org/docs/format/Intro.html
https://arrow.apache.org/docs/format/Columnar.html
https://arrow.apache.org/docs/format/Versioning.html

---
## Fyne documentation links:
https://docs.fyne.io/api/v2.6/widget/
https://docs.fyne.io/api/v2.6/
https://docs.fyne.io/api/v2.6/app/
https://docs.fyne.io/api/v2.6/canvas/
https://docs.fyne.io/api/v2.6/container/
https://docs.fyne.io/api/v2.6/data/binding/
https://docs.fyne.io/api/v2.6/driver/
https://docs.fyne.io/api/v2.6/driver/desktop/
https://docs.fyne.io/api/v2.6/driver/mobile/
https://docs.fyne.io/api/v2.6/driver/software/
https://docs.fyne.io/api/v2.6/layout/
https://docs.fyne.io/api/v2.6/storage/
https://docs.fyne.io/api/v2.6/storage/repository/
https://docs.fyne.io/api/v2.6/test/
https://docs.fyne.io/api/v2.6/theme/

---
