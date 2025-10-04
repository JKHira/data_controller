# State永続化 修正計画書

## 作成日: 2025-10-04
## 問題: Books/Candles状態がアプリ終了時に保存されない

---

## 根本原因

**発見した問題**:
[app.go:handleWindowClose()](internal/gui/app/app.go#L518-L552)でstate保存を呼んでいない

**現在の動作**:
- 各チャンネルパネルは変更時にpersistState()を呼ぶ
- しかし、アプリ終了時に最終状態を保存する処理がない

**state.ymlに保存されている理由**:
- 前回のチャンネル変更時にpersistState()が実行された
- しかし、その後の変更(アプリ終了前の最終変更)は保存されていない可能性

---

## 修正方法

### app.goのhandleWindowClose()メソッドに状態保存を追加

**修正箇所**: [app.go:518-552](internal/gui/app/app.go#L518-L552)

**追加する処理**:
1. WebSocketPanelのsaveState()を呼ぶ
2. ConfigManagerのSaveState()を呼ぶ

---

## 実装

### 1. Application構造体にWebSocketPanelへの参照を追加

**確認**: Applicationがws_panelへのアクセスを持っているか

### 2. handleWindowClose()にstate保存を追加

```go
func (a *Application) handleWindowClose() {
    a.logger.Info("GUI: Window close requested")

    // Save current UI state before closing
    if a.wsPanel != nil {
        a.wsPanel.saveState()
    }

    // Stop connection if active
    if a.isRunning {
        // ...
    }

    // Save state to file
    if a.configManager != nil {
        if err := a.configManager.SaveState(); err != nil {
            a.logger.Warn("Failed to save state on close", zap.Error(err))
        }
    }

    // ... rest of cleanup
}
```

---

## テスト方法

1. アプリを起動
2. Books channelで設定変更:
   - Enable Books
   - Symbol選択
   - Precision/Frequency/Length変更
3. Candles channelで設定変更:
   - Enable Candles
   - Symbol選択
   - Timeframe変更
4. アプリを終了
5. config/runtime/state.ymlを確認
6. アプリを再起動
7. Books/Candlesの状態が復元されることを確認
