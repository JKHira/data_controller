# File Viewer Dynamic Display Memo (Codex)

## 目的
- File Viewer に Arrow ファイルのメタデータ専用領域を追加し、ロード時にメタデータを自動表示できるようにする。
- Arrow スキーマからフィールド名を動的に取得し、レコード表示をスキーマ順でレンダリングする仕組みに刷新する。

## 現状把握
- `ViewerPanel` は `widget.Card` 内にページングボタンと `fileViewer` エントリを縦配置している。メタデータ領域は存在しない。
- `FileController` は `fileViewer` のみを保持し、Arrow 読み込み結果を文字列にフォーマットしている。メタデータ取得は行っていない。
- `arrow.FileReader` が返す `PageData` は `FieldNames` を持たず、`display` 側はハードコードされたフィールド順（`channel` 列を前提）を使用。
- `ReadArrowFileSummary` でスキーマ情報を取得可能だが、UI では活用されていない。

## 影響範囲
- GUI: `internal/gui/panels/viewer_panel.go`, `internal/gui/file_viewer.go`, `internal/gui/controllers/file_controller.go`
- 状態管理: `internal/gui/state/app_state.go`
- Arrow 読み取り: `internal/sink/arrow/reader.go`, `internal/services/file_reader.go`

## 実装方針
1. ViewerPanel にメタデータ表示用の `widget.Entry`（読み取り専用）を追加し、200x200 程度の正方形領域としてカード内に配置する。Controller に新エントリを渡して更新させる。
2. AppState へ `CurrentFileSummary` と `CurrentFieldOrder` を追加。ファイル切替時に初期化、再読込時は流用。
3. Arrow FileReader の `PageData` に `FieldNames` を追加し、スキーマ順のフィールド名配列を返す。Service 層も同項目をパススルー。
4. FileController: Arrow 読み込み時に summary を取得して state とメタデータエントリに反映。レコード表示では `pageData.FieldNames` を利用して動的フォーマットを行う。
5. UI 表示ロジックはメタデータ部分とレコード部分を分離し、浮動小数点やタイムスタンプはそのまま `fmt.Sprintf("%v")` で出力する。

## 懸念点・確認事項
- メタデータ領域の最小サイズを Fyne の Scroll コンテナで調整し、指定通り「固定正方形」に近い挙動を確保する。
- 既存の `DisplayArrowData`（旧実装）との重複に注意し、必要に応じて内部利用メソッドへ統合。
- PageData のフィールド追加により影響を受ける他箇所がないかテスト実行で確認する。


## 検証
- `go test ./...` を実行したが、既存と同様に `fyne.io/systray` 等の cgo ビルドで `-fobjc-arc` 未対応エラーが発生しタイムアウト。今回の変更による追加エラーは発生していないことを確認。

