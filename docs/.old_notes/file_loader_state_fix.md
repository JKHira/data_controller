# File Loader & State Persistence 修正メモ

## 作成日: 2025-10-04

---

## 問題1: File Loaderがファイルをロードできない

### 症状
- path規則変更前は動作していた
- ファイル命名を`part-{timestamp}.arrow` → `{channel}-{timestamp}.arrow`に変更後、ロードできない

### 調査項目
- [ ] File Scannerが新しいファイル名形式を認識できるか
- [ ] extractSymbolFromPath()が新しい命名規則に対応しているか
- [ ] ファイルリスト表示に問題があるか
- [ ] ファイルパス構築ロジックに問題があるか

---

## 問題2: Books/Candlesの状態がリアルタイム保存されない

### 症状
- Tradesは正常に動作
- Books/Candlesは状態が一時記録されない
- 保存先: runtime/state.yml

### 調査項目
- [ ] TradesチャンネルのpersistState()呼び出しタイミング
- [ ] Books/CandlesのpersistState()呼び出しタイミング
- [ ] UIイベントハンドラの違い
- [ ] コールバック設定の違い

---

## 調査結果

### File Loaderの問題

**根本原因**: extractSymbolFromPath()が古いファイル命名規則(`part-{symbol}-{timestamp}`)用のロジックを含んでいた

**修正内容**: [file_scanner.go:355-358](internal/services/file_scanner.go#L355-L358)
- 355-366行の古いロジック(part-プレフィックス除去、ファイル名からsymbol抽出)を削除
- Symbolはディレクトリパスから抽出するため、ファイル名処理は不要

**新しい命名規則**: `{channel}-{timestamp}.arrow`
- 例: `ticker-20251004T132846Z.arrow`
- Symbolはパス内: `data/bitfinex/websocket/ticker/tBTCEUR/dt=2025-10-04/`

---

### Tradesの状態保存ロジック

**persistState()呼び出しタイミング** [channel_trades.go]:
- 81行: enableCheck変更時
- 116行: symbolList変更時

**persistState()実装** [channel_trades.go:332-343]:
```go
func (p *TradesChannelPanel) persistState() {
    if p.configManager == nil {
        return
    }
    state := p.configManager.GetApplicationState()
    if state == nil {
        return
    }

    uiState := state.GetUIState(p.exchange)
    p.SaveState(uiState)
    state.UpdateUIState(p.exchange, uiState)
    if err := p.configManager.SaveState(); err != nil {
        p.logger.Warn("failed to persist trades channel state", zap.Error(err))
    }
}
```

---

### Books/Candlesとの違い

**結論**: Books/Candles/Tradesすべて同じロジックでpersistState()を実装している

**Books** [channel_books.go]:
- 99行: enableCheck変更時
- 134行: symbolList変更時
- 144行: precSelect変更時
- 154行: freqSelect変更時
- 164行: lenSelect変更時
- 431行: Reset()時
- persistState()実装: 488-503行 (Tradesと同じパターン)

**Candles** [channel_candles.go]:
- 88行: enableCheck変更時
- 123行: symbolList変更時
- 134行: timeframeSelect変更時
- 353行: Reset()時
- persistState()実装: 410-425行 (Tradesと同じパターン)

---

### 問題の真相

**state.ymlの現在の状態**:
- books: enabled=false, selected_symbols=[]
- candles: enabled=false, selected_symbols=[]
- trades: enabled=true, selected_symbols=[tBTCGBP]

**可能性1**: Reset()がpersistState()を呼ぶ
- Books/Candles Reset()は431行/353行でpersistState()を呼ぶ
- しかしDisconnect時にReset()は呼ばれていない（確認済み）

**可能性2**: 古いデータ
- state.ymlが最新の変更を反映していない
- アプリ起動→変更→終了の間にSaveState()が呼ばれていない可能性

**可能性3**: LoadState()の問題
- 状態は保存されているが、読み込み時に正しく復元されていない
- LoadState()のタイミングやロジックに問題がある可能性

---

## 修正方針

### 1. File Loader修正 ✅
- extractSymbolFromPath()から古いロジック削除

### 2. State永続化の確認
- アプリ終了時のSaveState()は追加済み [app.go:523-529]
- 各チャンネルのpersistState()は正しく実装されている
- 問題はLoadState()のタイミングまたはロジックの可能性

### 3. 次のステップ
- 実際にアプリを起動してBooks/Candlesを有効化
- Symbol選択
- state.ymlを確認
- アプリ再起動して復元を確認

