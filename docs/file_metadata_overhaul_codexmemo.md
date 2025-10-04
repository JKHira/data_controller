# File Metadata Overhaul Memo (Codex)

## 課題整理
- File Viewer のレコード表示領域がスクロールボックス内で高さを食い潰せず、1行分程度しか見えない。
- メタデータカードには Arrow スキーマ情報が並んでおり、利用者が欲しい非可変情報 (exchange / key / flags 等) が表示されない。
- Arrow writer 側では `datetime_end` や subscription `key`、正しい `chan_id` がメタデータに含まれておらず、books 系以外の旗も揃っていない。
- 既存メタデータキーが `datetime_start_collecting` など仕様とずれている。writer → reader → viewer の流れを Ticker / Books / Trades / RawBooks / Candles で横断的に確認して補正する必要がある。

## 対応方針
1. **UI 調整**: Metadata カードをトップに据えた Border レイアウトへ変更し、レコードスクロール領域がカード直下から Info Bar 直前まで伸びるようにする。
2. **メタデータ整備 (Writer)**:
   - Channel 情報 (chanId, key, pair など) を writer に伝達するため、Router→Handler→Writer の経路にメタデータ更新フックを追加。
   - Arrow ファイル作成時に `exchange`, `data_source`, `pair_symbol`, `channel`, `key`, `chan_id`, `ingest_id`, `datetime_start`, `timestamp_flag`, `sequence_flag` と books/raw_books 用の `checksum_flag`, `bulk_flag` を確実に書き込む。`datetime_end` は close 時に更新値を保持しつつ、reader 側で算出補填する。
3. **Reader / Summary 拡張**:
   - スキーマメタデータを map として取りこぼしなく抽出。
   - ファイル走査で `recv_ts` (存在しない場合は `mts`) の最小・最大値を計算し、ISO8601 文字列に変換して `datetime_start` / `datetime_end` を強化。
   - 既存ファイルとの互換のため旧キー (`datetime_start_collecting` 等) も読み替え。
4. **Viewer 表示**:
   - 基本情報 (File name, Size, Total Records, Record Batches, Columns) とメタデータを指定順で整形表示。
   - Flags は true/false 表示、存在しないキーは `-` 表示。

## 検証計画
- gofmt / go test (既知の cgo 制約は備考に明記)。
- 各チャネル種別でメタデータが埋まり、レコード表示領域がフルハイトで描画されるか手動確認予定。


## 実装メモ
- ViewerPanel / CreateFileViewerPanel を Border レイアウトに変更し、メタデータカードの直下でレコードスクロールが全面を占めるよう調整。
- `pkg/schema.CommonFields` に `ChanID` / `Channel` / `ChannelKey` を拡張、Router で各チャンネルの購読情報を詰めるようにした。
- Arrow Writer にチャネルメタデータキャッシュを追加し、キー・チャネルID・ペアなどをファイルメタデータへ反映。従来の `datetime_start_collecting` を `datetime_start` にリネームし、新たに `key` / `datetime_end` を埋め込む（`datetime_end` は reader で実データから補完）。
- Reader summary はスキーマメタデータを map で返しつつ、`recv_ts` / `mts` の min/max から ISO8601 の開始・終了時刻を算出。
- File Viewer のメタデータカードは要求された基本情報とメタデータ項目を表示するよう更新。既存のスキーマ一覧表示は削除。

## 検証
- `go test ./...` は引き続き macOS の cgo toolchain (`gcc-14` が `-fobjc-arc` 非対応) により失敗。新規変更による追加エラーは確認されず。

