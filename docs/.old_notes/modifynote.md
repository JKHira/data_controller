# Arrow Schema & File Structure Modification Plan

## 概要
WebSocket Arrow保存における冗長フィールドの削除とメタデータへの移行を実施。
ファイル構造の簡素化と、それに伴うロジック全体の整合性を保つための修正計画。

---

## 変更要件

### 1. 完全削除するフィールド (全チャンネル共通)
- `conn_id`
- `line_no`
- `sub_id` (必要なのは`chan_id`のみ)
- `config_id` (現在未使用)

### 2. Books/RawBooksのみで使用するフィールド
- `batch_id` → Books/RawBooksのみ保持、他のチャンネル(Ticker/Trades/Candles)からは削除

### 3. メタデータへ移動するフィールド
以下のフィールドをArrowファイルの列から削除し、ファイルメタデータ(KeyValueMetadata)へ移動:

**文字列フィールド:**
- `exchange` → metadata key: `exchange`
- `channel` → metadata key: `channel` (values: tickers, trades, books, raw_books, candles, stats)
- `chan_id` → metadata key: `chan_id`
- `ingest_id` → metadata key: `ingest_id`
- `source_file` → metadata key: `data_source` (名称変更)

**フラグフィールド (true/false):**
- `conf_flags` → 以下の個別フラグに分解してメタデータへ
  - `timestamp_flag`: "true"/"false"
  - `sequence_flag`: "true"/"false"
  - `checksum_flag`: "true"/"false" (Books/RawBooksのみ)
  - `bulk_flag`: "true"/"false" (Books/RawBooksのみ)

### 4. ファイル構造の簡素化

**現在:**
```
{base}/bitfinex/websocket/{channel}/{symbol}/dt={YYYY-MM-DD}/hour={HH}/seg={start}--{end}--size~{MB}/
  └── part-{channel}-{symbol}-{timestamp}-seq.arrow
```

**変更後:**
```
{base}/bitfinex/websocket/{channel}/{symbol}/dt=YYYY-MM-DD/
  └── part-{timestamp}.arrow
```

**メタデータに含める情報:**
- exchange: "bitfinex"
- data_source: "websocket"
- pair_symbol: "tBTCEUR" など
- channel: "books", "tickers", "trades", "raw_books", "candles", "stats"
- chan_id: "12345"
- ingest_id: "uuid-string"
- datetime_start_collecting: ISO8601 UTC (例: "2025-10-04T10:30:00Z")
- datetime_end: ISO8601 UTC
- timestamp_flag: "true"/"false"
- sequence_flag: "true"/"false"
- checksum_flag: "true"/"false" (Books/RawBooksのみ)
- bulk_flag: "true"/"false" (Books/RawBooksのみ)

---

## 影響を受けるファイルとコンポーネント

### A. スキーマ定義
- `pkg/schema/types.go` - CommonFields構造体の修正
- `internal/sink/arrow/schema.go` - Arrow schema定義の修正

### B. Arrow書き込み
- `internal/sink/arrow/writer.go` - メタデータ付与、ファイルパス生成の修正
- `internal/sink/arrow/channel_writer.go` - 各チャンネル書き込みロジックの修正
- `internal/sink/arrow/handler.go` - データハンドリングの確認

### C. WebSocket接続とルーティング
- `internal/ws/conn.go` - 削除フィールドへの参照の確認と修正
- `internal/ws/router.go` - CommonFields設定部分の修正

### D. Arrow読み込み
- `internal/sink/arrow/reader.go` - 読み込みロジックの修正、メタデータ読み取り対応
- `internal/services/file_scanner.go` - ファイルパス構造変更への対応
- `internal/gui/file_viewer.go` - メタデータ表示への対応

### E. GUIとファイル操作
- `internal/gui/panels/files_panel.go` - ファイルリスト表示の修正
- `internal/gui/controllers/file_controller.go` - ファイル操作の確認

---

## 作業手順

### Phase 1: スキーマとデータ構造の修正
1. **pkg/schema/types.go の修正**
   - CommonFieldsから削除: `ConnID`, `LineNo`, `SubID`, `ConfFlags`, `SourceFile`
   - Ticker/Trade/Candleから`BatchID`削除
   - BookLevel/RawBookEventには`BatchID`を保持

2. **internal/sink/arrow/schema.go の修正**
   - GetCommonFields()から削除されるフィールドを除外
   - 新しい共通フィールド構成を定義
   - Books/RawBooksのみにBatchIDを含める

### Phase 2: Arrow書き込みロジックの修正
3. **internal/sink/arrow/writer.go の修正**
   - ファイルパス生成ロジックの簡素化
   - メタデータ構築ロジックの追加
   - セグメント管理の簡素化

4. **internal/sink/arrow/channel_writer.go の修正**
   - 各チャンネル書き込み関数から削除フィールドを除外
   - メタデータをFileWriterへ付与する処理追加
   - フィールドインデックスの再調整

### Phase 3: WebSocketロジックの修正
5. **internal/ws/router.go の修正**
   - CommonFields設定から削除フィールドを除外
   - `ConnID`, `SubID`, `ConfFlags`への参照削除

6. **internal/ws/conn.go の確認**
   - 削除フィールドへの参照がないか確認
   - 必要に応じて修正

### Phase 4: Arrow読み込みとファイルスキャンの修正
7. **internal/sink/arrow/reader.go の修正**
   - メタデータ読み取り機能の追加
   - 新しいスキーマでの読み込み対応

8. **internal/services/file_scanner.go の修正**
   - 新しいファイルパス構造への対応
   - パス解析ロジックの簡素化

### Phase 5: GUIとテスト
9. **GUIファイルビューアの修正**
   - メタデータ表示機能の追加
   - 新しいファイル構造での表示対応

10. **統合テスト**
    - 各チャンネルでのWebSocket接続テスト
    - データ書き込み・読み込みテスト
    - GUI表示確認
    - 切断・再接続時の動作確認

---

## チェックリスト

### Phase 1: スキーマとデータ構造
- [ ] pkg/schema/types.go - CommonFields修正
- [ ] pkg/schema/types.go - Ticker/Trade/Candle構造体からBatchID削除
- [ ] internal/sink/arrow/schema.go - GetCommonFields修正
- [ ] internal/sink/arrow/schema.go - GetTickerSchema修正
- [ ] internal/sink/arrow/schema.go - GetTradeSchema修正
- [ ] internal/sink/arrow/schema.go - GetCandleSchema修正
- [ ] internal/sink/arrow/schema.go - GetBookLevelSchema確認(BatchID保持)
- [ ] internal/sink/arrow/schema.go - GetRawBookEventSchema確認(BatchID保持)

### Phase 2: Arrow書き込み
- [ ] internal/sink/arrow/writer.go - メタデータ構造体の定義
- [ ] internal/sink/arrow/writer.go - ファイルパス生成の簡素化
- [ ] internal/sink/arrow/writer.go - createNewSegment修正
- [ ] internal/sink/arrow/channel_writer.go - writeTickerメタデータ追加
- [ ] internal/sink/arrow/channel_writer.go - writeTradeメタデータ追加
- [ ] internal/sink/arrow/channel_writer.go - writeCandleメタデータ追加
- [ ] internal/sink/arrow/channel_writer.go - writeBookLevelメタデータ追加
- [ ] internal/sink/arrow/channel_writer.go - writeRawBookEventメタデータ追加
- [ ] internal/sink/arrow/channel_writer.go - フィールドインデックスの再計算

### Phase 3: WebSocketロジック
- [ ] internal/ws/router.go - routeTicker修正
- [ ] internal/ws/router.go - routeTrades修正
- [ ] internal/ws/router.go - routeBooks修正
- [ ] internal/ws/router.go - routeRawBooks修正
- [ ] internal/ws/router.go - routeCandles修正
- [ ] internal/ws/conn.go - 削除フィールドへの参照確認

### Phase 4: Arrow読み込みとスキャン
- [ ] internal/sink/arrow/reader.go - メタデータ読み取り機能追加
- [ ] internal/sink/arrow/reader.go - 新スキーマ対応
- [ ] internal/services/file_scanner.go - パス解析の簡素化

### Phase 5: GUI
- [ ] internal/gui/file_viewer.go - メタデータ表示追加
- [ ] internal/gui/panels/files_panel.go - 新構造対応確認

### Phase 6: テスト
- [ ] Ticker WebSocket接続・書き込みテスト
- [ ] Trades WebSocket接続・書き込みテスト
- [ ] Books WebSocket接続・書き込みテスト
- [ ] RawBooks WebSocket接続・書き込みテスト
- [ ] Candles WebSocket接続・書き込みテスト
- [ ] メタデータ読み取りテスト
- [ ] ファイルビューア表示テスト
- [ ] WebSocket切断・再接続テスト
- [ ] セグメント自動ローテーションテスト

---

## リスク管理

### 高リスク項目
1. **WebSocket切断時のクラッシュ**
   - 対策: conn.goのdisconnect()処理を慎重に確認
   - 対策: writer.Close()のエラーハンドリング強化

2. **既存データとの互換性**
   - 対策: 新旧両スキーマに対応した読み込みロジック(必要に応じて)
   - 対策: 移行期間中のフォールバック処理

3. **フィールドインデックスのミスマッチ**
   - 対策: channel_writer.goのインデックス計算を慎重に確認
   - 対策: ユニットテストでの検証

### 注意事項
- 既存のデータファイルは触らない
- 新しいWebSocket接続からのみ新スキーマを適用
- GUIでの新旧ファイル両方の表示対応を検討
- 各Phase完了後に動作確認を実施
- 関係のないコードは変更しない

---

## 最終確認項目
- [ ] 全チャンネルでデータ書き込みが正常動作
- [ ] メタデータが正しく付与されている
- [ ] ファイルパスが新構造で正しく生成されている
- [ ] WebSocket切断・再接続時にクラッシュしない
- [ ] GUIでファイルとメタデータが正しく表示される
- [ ] 既存機能に影響がない
- [ ] ログ出力が適切
- [ ] エラーハンドリングが適切

---

**作成日**: 2025-10-04
**ステータス**: 計画中
