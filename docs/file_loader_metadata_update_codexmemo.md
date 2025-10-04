# File Loader & Metadata Update Memo (Codex)

## 対応タスク概要
1. File Loader の結果リスト表示領域を Info Bar 直前まで拡張し、スキャン結果を十分な高さで閲覧可能にする。
2. WebSocket Conf Flags (timestamp / sequence / checksum / bulk) と Arrow メタデータ `*_flag` を正しく同期させる。
3. Books / RawBooks の `prec` `freq` `len` をレコードから排除し、ファイルメタデータへ移行。
4. メタデータの `key` は Candles のみ保存し、その他のチャネルでは不要とする。
5. Candles のレコードから `timeframe` フィールドを削除（メタデータで提供）。

## 影響箇所メモ
- `internal/gui/panels/files_panel.go` (UI レイアウト)
- `pkg/schema/types.go` 共通フィールド拡張・チャネルメタ構造体
- `internal/ws/router.go` チャネル別メタ情報付与
- `internal/sink/arrow/handler.go` メタデータ更新フック
- `internal/sink/arrow/writer.go` メタデータ生成 / flags & book parameters / conf flags更新
- `internal/sink/arrow/channel_writer.go` close 時処理
- `internal/sink/arrow/schema.go` スキーマ調整 (book/candle)
- `internal/sink/arrow/reader.go` メタデータ読み出し・レンジ補完
- `internal/gui/controllers/file_controller.go` メタ閲覧表示
- `internal/gui/app/app.go` WebSocket 設定変更時の conf flag 伝搬

## 検証計画
- gofmt / go test (cgo 依存は既知制約) の再実行。
- 各チャネル (Ticker/Trades/Books/RawBooks/Candles) について、生成 Arrow ファイルのメタデータが仕様通りとなるか手動確認予定。


## 実装メモ
- File Loader コンテンツを Border レイアウトへ変更し、フィルタ/UI を上部、結果リストをスクロール領域として残余高さに配置。
- CommonFields/ChannelMetadata を拡張して channel id/key/timeframe/book params を保持、Router で各チャネルのメタ情報をセット。
- Arrow Handler/Writer にメタデータ更新 API を追加し、WebSocket conf flags とチャネル属性をファイルメタデータに反映。Books/RawBooks の `prec/freq/len` はメタデータへ移動。
- Candle スキーマから `timeframe` フィールドを削除し、メタデータ `timeframe` へ移行。Books スキーマから `prec/freq/len` を除去。
- Reader summary でメタデータをマージし、`recv_ts` / `mts` から `datetime_start/end` を補完。Viewer メタカードは指定された基本情報 + 新メタキーを表示。

## 検証
- `go test ./...` は cgo (`gcc-14` が `-fobjc-arc` 非対応) により継続的に失敗。新規 diff 由来のエラーは確認されず。

