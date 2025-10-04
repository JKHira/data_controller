# File Loader & State Persistence 問題調査メモ

## 作成日: 2025-10-04

---

## 問題リスト

### 1. File Loader Scan問題
**症状**: 現在のファイル階層に対応していない可能性があり、Scanでファイル表示が正常にされない

**調査項目**:
- [ ] 現在のファイル階層構造を確認
- [ ] File Loaderのscanロジックを確認
- [ ] 期待される階層とscanロジックの差異を特定

---

### 2. Arrow File命名規則の変更
**現在**: 不明（調査必要）
**要求**: `{channel}-{timestamp_started}.arrow`

**例**: `ticker-20251004T130631Z.arrow`

**ファイルパス全体**:
```
{base=data}/{exchange}/{source}/{channel}/{symbol}/dt=YYYY-MM-DD/{channel}-{timestamp_started}.arrow
```

**調査項目**:
- [ ] 現在のファイル命名ロジックを確認
- [ ] writer.goまたはchannel_writer.goのファイル名生成箇所を特定
- [ ] 修正が必要な箇所をリストアップ

---

### 3. State永続化問題 (Books & Candles)
**症状**: アプリ終了→再起動時にBooks/Candlesの状態が復元されない

**調査項目**:
- [ ] runtime/state.ymlの現在の内容を確認
- [ ] state保存ロジックを確認
- [ ] state読み込みロジックを確認
- [ ] Books/Candlesの状態が保存されない原因を特定

---

### 4. UI変更
**要求**:
- File LoaderとFile Viewerを1つのタブに統合
- 画面幅をFile Loader幅(現在の半分)に合わせる
- File Viewer上部に正方形エリアを追加してメタデータ表示
- 右側のControls領域を広げる(File Viewer分)

**調査項目**:
- [ ] 現在のUI構成を確認
- [ ] File LoaderとFile Viewerのコードを確認
- [ ] レイアウト変更の影響範囲を確認

---

## 調査結果

### 現在のファイル階層

### File Loaderのscanロジック

### Arrow File命名規則

### State永続化ロジック

### UI構成

