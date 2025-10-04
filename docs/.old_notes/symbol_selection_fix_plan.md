# チャンネル別Symbol選択 修正計画書

## 作成日: 2025-10-04
## 目的: 各チャンネルが選択したsymbolだけを購読・保存するように修正

---

## 修正方針

Candlesチャンネルの仕組み(customSubscriptions経由)を全チャンネルに適用し、
チャンネル別のsymbol選択を実現する。

---

## 修正対象ファイル

### 1. internal/ws/conn.go
**修正内容**: createConnection()メソッドの購読生成ロジックを変更

**現在の問題** (186-236行):
- 統合symbolsリストの全symbolに対してTicker/Trades/Books/RawBooksを購読
- チャンネル別のsymbol選択が無視される

**修正方法**:
- 186-236行の全symbolループを削除
- customSubscriptionsのみを使用してsubscribeQueueを構築
- 239行の`conn.subscribeQueue = append(conn.subscribeQueue, cm.customSubscriptions...)`だけを残す

**修正前**:
```go
for _, symbol := range symbols {
    if cm.cfg.Channels.Ticker.Enabled {
        conn.subscribeQueue = append(conn.subscribeQueue, SubscribeRequest{
            Event:   "subscribe",
            Channel: "ticker",
            Symbol:  symbol,
        })
    }
    // ... Trades, Books, RawBooks も同様
}
conn.subscribeQueue = append(conn.subscribeQueue, cm.customSubscriptions...)
```

**修正後**:
```go
// Use custom subscriptions for all channels (including ticker, trades, books)
conn.subscribeQueue = append(conn.subscribeQueue, cm.customSubscriptions...)
```

---

### 2. internal/gui/app/app.go
**修正内容**: handleWsConnectConfig()メソッドの修正

**現在の問題** (354-362行):
- 全チャンネルのsymbolsを統合した配列を作成
- handleWsConnect(exchange, symbols)に渡している

**修正方法**:
- symbols統合ロジックを削除
- handleWsConnect()にはダミーの空配列を渡す(または引数を変更)
- customSubscriptionsに全チャンネルの情報を含める

**注意点**:
- 380-402行のチャンネル有効/無効の判定は**削除しない**
- Books channelのPrec/Freq/Lenパラメータは各subscriptionに含まれる
- 既存のcandles処理(397-402行)はそのまま維持

**修正前**:
```go
symbols := uniqueStrings(append([]string{}, wsConfig.Symbols...))
if len(symbols) == 0 {
    for _, sub := range wsConfig.Channels {
        if sub.Symbol != "" {
            symbols = append(symbols, sub.Symbol)
        }
    }
    symbols = uniqueStrings(symbols)
}

if len(symbols) == 0 {
    return fmt.Errorf("no symbols selected for connection")
}

a.cfg.Symbols = symbols
// ... チャンネル設定処理 ...
a.customSubscriptions = wsConfig.Channels

return a.handleWsConnect(exchange, symbols)
```

**修正後**:
```go
// Validate that we have subscriptions
if len(wsConfig.Channels) == 0 {
    return fmt.Errorf("no channels selected for connection")
}

// チャンネル設定処理はそのまま維持
// ...

a.customSubscriptions = wsConfig.Channels

// Pass custom subscriptions instead of unified symbols
return a.handleWsConnectWithSubscriptions(exchange)
```

---

### 3. internal/gui/app/app.go (handleWsConnect)
**修正内容**: handleWsConnect()メソッドの引数調整

**現在**:
```go
func (a *Application) handleWsConnect(exchange string, symbols []string) error
```

**修正後の選択肢**:

**Option A**: 既存メソッドを維持し、空配列を渡す
```go
return a.handleWsConnect(exchange, []string{})
```

**Option B**: 新しいメソッドを作成
```go
func (a *Application) handleWsConnectWithSubscriptions(exchange string) error {
    // customSubscriptionsを使用
}
```

**推奨**: Option A (最小限の変更)

---

## 修正手順

### Phase 1: conn.goの修正 ✅ 完了
1. ✅ symbol_selection_investigation.mdを参照
2. ✅ internal/ws/conn.goを開く
3. ✅ createConnection()の186-236行を削除
4. ✅ 239行だけを残す
5. ✅ コメントを追加して意図を明確化

**実装内容**: [conn.go:186-190](internal/ws/conn.go#L186-L190)
- 統合symbolsループを削除
- customSubscriptionsのみを使用するように変更

### Phase 2: app.goの修正 ✅ 完了
1. ✅ handleWsConnectConfig()メソッドを修正
2. ✅ symbols統合ロジック(354-366行)を削除
3. ✅ バリデーションをwsConfig.Channelsベースに変更
4. ✅ handleWsConnect()のsubscription変換ロジックを全チャンネル対応に拡張

**実装内容**:
- [app.go:354-368](internal/gui/app/app.go#L354-L368): symbols集約を削除、channelsベースのバリデーションに変更
- [app.go:300-333](internal/gui/app/app.go#L300-L333): 全チャンネルのcustomSubscriptions変換処理
  - Ticker, Trades, Books, RawBooks, Candlesすべてに対応
  - Books channelのSubID自動生成を追加

### Phase 3: ビルドとテスト
1. ✅ `go build`でコンパイル確認 - 成功
2. ⏳ GUIを起動して動作確認
3. ⏳ 各チャンネルで異なるsymbolを選択してテスト
4. ⏳ 保存されるファイルがチャンネル別に正しく分離されているか確認

---

## 期待される結果

### テストケース
選択:
- Ticker: BTC-EUR のみ
- Trades: BTC-GBP のみ
- Books: ETH-GBP のみ
- Candles: DOT-USD のみ

### 期待される保存結果
- `data/bitfinex/websocket/ticker/` → tBTCEUR/ のみ
- `data/bitfinex/websocket/trades/` → tBTCGBP/ のみ
- `data/bitfinex/websocket/books/` → tETHGBP/ のみ
- `data/bitfinex/websocket/candles/` → tDOTUSD/ のみ

---

## 影響範囲確認

### 変更により影響を受ける機能
- WebSocket接続処理
- チャンネル購読処理
- データ保存処理

### 影響を受けない機能
- UI表示
- State保存/復元
- REST API機能
- Arrow file reader

### リスク評価
- **低リスク**: 既存のCandles実装と同じパターンを適用
- **テスト必須**: 各チャンネルでのデータ保存確認

---

## 備考

この修正により、システムは完全に`customSubscriptions`ベースの購読管理に移行する。
各チャンネルパネルの`GetSubscriptions()`メソッドが購読の唯一の情報源となる。
