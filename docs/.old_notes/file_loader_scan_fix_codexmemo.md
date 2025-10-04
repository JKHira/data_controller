# File Loader Scan Fix Memo (Codex)

## 現状観察
- READMEの最新パス規則: `{base}/{exchange}/{source}/{channel}/{symbol}/dt=YYYY-MM-DD/{channel}-{timestamp}.arrow` に変更済み（`hour=`階層が無い）。
- 実データ確認でも `data/bitfinex/websocket/trades/tBTCGBP/dt=2025-10-04/trades-20251004T132846Z.arrow` のように `hour=` ディレクトリは存在しない。
- `internal/services/file_scanner.go` は `hour=` ディレクトリ前提で 24 時間分ループし、存在しないパスに対して `filepath.Walk` を試みているためファイルが検出されない。

## 影響範囲
- `services.FileScanner.FindFiles` / `generateHours` / `scanPath` が新階層に未対応。
- `FilesPanel` UI・`FileController` は `FileScanner` の結果に依存しており、scan に失敗すると一覧・Load が共に動作しない。
- 既存の旧フォーマット (`hour=` や `seg=`) への互換性は保持する必要がある。

## 修正方針
1. "All" 時の時間ループを `""` に統一し、`dt=` 直下を1回スキャンする形に変更（無限ループ防止＆新フォーマット対応）。
2. `scanPath` でファイル名から時刻情報を抽出（例: `...T132846Z` → `13`）し、必要に応じて `Hour` フィールドへセット。
3. 時刻フィルタ指定時は、`hour=` ディレクトリまたはファイル名の時刻に一致するもののみ残す。
4. メタデータ付与 (`files[i].Hour`) は既存値を尊重し、空時のみ上書きして双方向フォーマットへ対応。

## 追加確認事項
- `findAllCategoryFiles` など他の経路でも同じ時間ロジックを共有するため共通化／再利用が必要。
- 変更後、`FilesPanel` の表示や Load フロー、`FileController` の情報表示で副作用が無いかを確認する。
- 必要ならテストデータ(小規模)を用意してフィルタ動作を手動検証。


## 実装ノート
- `generateHours` を "All" 時に空値1件だけ返すよう変更し、DT直下スキャンに統一。
- `scanPath` でディレクトリ存在チェック・時刻抽出・時刻フィルタリングを追加。ファイル名から `T` 後の時間や `hour=` ディレクトリを解析して `FileItem.Hour` に格納。
- `FindFiles` / `findAllCategoryFiles` は `scanParams.Hour` を都度設定し、既存の `Hour` 情報が空の場合のみ上書きするように調整。

## 検証
- `go test ./internal/services -run TestDummy -c` でサービス層パッケージのビルドを実施しコンパイル確認（テストファイルなし）。
- ルートでの `go test ./...` は `fyne.io/systray` の cgo 依存で `-fobjc-arc` 未対応 GCC により停止（確認のみ、未解決）。

