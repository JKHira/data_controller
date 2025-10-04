# 修正実装計画書

## 作成日: 2025-10-04
## ステータス: Phase 2完了、Phase 3実装中

---

## 修正概要

### 問題1: Symbol購読の不正な連動
**修正内容:** WebSocket接続時に各チャンネルが独自のsymbol選択を尊重するように修正

### 問題2: Disconnect時のArrow保存未完了
**修正内容:** handler.Stop()の前にFlushAll()を明示的に呼び出し

### UI改善: Flag連動
**修正内容:** Books/RawBooks enableに応じてBulk/Checksum flagをenable/disable

---

## 実装手順

### Phase 1: 問題2の修正 (優先度: 高)
データロスの可能性があるため、まずこれを修正

#### Step 1.1: handler.Stop()の修正
**ファイル:** `internal/sink/arrow/handler.go`
**変更箇所:** Stop()メソッド

**変更内容:**
```go
func (h *Handler) Stop() error {
	var err error
	h.stopOnce.Do(func() {
		h.logger.Info("Stopping Arrow handler")

		// Stop accepting new data
		h.mu.Lock()
		h.stopped = true
		close(h.stopCh)
		h.mu.Unlock()

		if h.flushTicker != nil {
			h.flushTicker.Stop()
		}

		// Wait for flush routine to finish
		h.wg.Wait()

		// 【追加】Final flush before closing
		h.logger.Info("Performing final flush before close")
		if flushErr := h.writer.FlushAll(); flushErr != nil {
			h.logger.Error("Failed to flush all data before close", zap.Error(flushErr))
		}

		// Close all files
		if closeErr := h.writer.Close(); closeErr != nil {
			h.logger.Error("Failed to close writer", zap.Error(closeErr))
			err = closeErr
		}

		h.logger.Info("Arrow handler stopped")
	})

	if !h.stopped {
		h.logger.Warn("Arrow handler already stopped")
	}

	return err
}
```

**理由:**
- FlushAll()を明示的に呼び出すことで、バッファに残っているデータを確実にflush
- close()前にflushすることで、.tmpファイルが確実に最終ファイルにrename

---

### Phase 2: UI改善 (優先度: 中)
UX改善のため、次にこれを実装

#### Step 2.1: websocket_panel.goの修正
**ファイル:** `internal/gui/websocket_panel.go`
**変更箇所:** buildUI()メソッド

**変更内容:**
1. Books/RawBooksパネルへの参照を追加
2. Checksum/Bulk checkboxをdisableで初期化
3. Books/RawBooksのOnStateChangeコールバックで制御

**詳細:**
```go
// buildUI constructs the UI components
func (p *WebSocketPanel) buildUI() {
	// ... (既存のコード)

	// Connection flag controls
	p.timestampCheck = widget.NewCheck("Timestamp (32768)", func(checked bool) {
		if p.restoring {
			return
		}
		p.updateConnectionFlags(func(flags *config.ConnectionFlags) {
			flags.Timestamp = checked
		})
	})

	p.sequenceCheck = widget.NewCheck("Sequence Numbers (65536)", func(checked bool) {
		if p.restoring {
			return
		}
		p.updateConnectionFlags(func(flags *config.ConnectionFlags) {
			flags.Sequence = checked
		})
	})

	p.checksumCheck = widget.NewCheck("Order Book Checksum (131072)", func(checked bool) {
		if p.restoring {
			return
		}
		p.updateConnectionFlags(func(flags *config.ConnectionFlags) {
			flags.Checksum = checked
		})
	})
	// 【追加】初期状態でdisable
	p.checksumCheck.Disable()

	p.bulkCheck = widget.NewCheck("Bulk Book Updates (536870912)", func(checked bool) {
		if p.restoring {
			return
		}
		p.updateConnectionFlags(func(flags *config.ConnectionFlags) {
			flags.Bulk = checked
		})
	})
	// 【追加】初期状態でdisable
	p.bulkCheck.Disable()

	// ... (既存のコード)
}
```

#### Step 2.2: handleChannelStateChangeの拡張
**ファイル:** `internal/gui/websocket_panel.go`
**変更箇所:** handleChannelStateChange()メソッド

**変更内容:**
```go
// handleChannelStateChange recomputes the aggregate subscription count
func (p *WebSocketPanel) handleChannelStateChange() {
	totalSubs := p.tickerPanel.GetSubscriptionCount() +
		p.tradesPanel.GetSubscriptionCount() +
		p.booksPanel.GetSubscriptionCount() +
		p.candlesPanel.GetSubscriptionCount() +
		p.statusPanel.GetSubscriptionCount()

	p.subscriptionCount.Set(totalSubs)
	p.updateSubscriptionInfo()

	// 【追加】Check if books/raw_books are enabled
	p.updateFlagAvailability()
}

// 【新規】updateFlagAvailability updates checksum/bulk flag availability
func (p *WebSocketPanel) updateFlagAvailability() {
	booksEnabled := p.booksPanel.IsEnabled()

	if booksEnabled {
		p.checksumCheck.Enable()
		p.bulkCheck.Enable()
	} else {
		p.checksumCheck.Disable()
		p.bulkCheck.Disable()
		// Also uncheck when disabled
		if !p.restoring {
			p.checksumCheck.SetChecked(false)
			p.bulkCheck.SetChecked(false)
		}
	}
}
```

#### Step 2.3: BooksChannelPanelにIsEnabled()追加
**ファイル:** `internal/gui/channel_books.go`

**変更内容:**
```go
// 【新規】IsEnabled returns whether the books channel is enabled
func (p *BooksChannelPanel) IsEnabled() bool {
	return p.enabled
}
```

---

### Phase 3: 問題1の修正 (優先度: 高)
Symbol購読の問題を修正

#### Step 3.1: ws/conn.goの修正方針
**現状分析:**
- createConnection()は`symbols []string`を受け取る
- 各チャンネル(Ticker/Trades/Books/RawBooks)で全symbolsをループしてsubscribeQueue追加
- customSubscriptions(Candles等)は別途追加

**修正方針:**
1. ConnectionManagerにCustomSubscriptionsを保持
2. createConnection()でCustomSubscriptionsから各チャンネルのsymbolを抽出
3. 既存のcfg.Channelsフラグと組み合わせてsubscribeQueue生成

**詳細設計:**

**ファイル:** `internal/ws/conn.go`

**変更内容:**
```go
// ConnectionManager構造体にcustomSubscriptions追加
type ConnectionManager struct {
	cfg                 *config.Config
	logger              *zap.Logger
	connMutex           sync.RWMutex
	connections         map[string]*Connection
	router              *Router
	ctx                 context.Context
	cancel              context.CancelFunc
	customSubscriptions []SubscribeRequest  // 【変更】公開フィールドに
}

// createConnection()を修正
func (cm *ConnectionManager) createConnection(connID string, symbols []string) (*Connection, error) {
	conn := &Connection{
		ID:             connID,
		URL:            cm.cfg.WebSocket.URL,
		channels:       make(map[int32]*ChannelInfo),
		lastHeartbeat:  make(map[int32]time.Time),
		reconnectChan:  make(chan struct{}, 1),
		done:           make(chan struct{}),
		logger:         cm.logger.With(zap.String("conn_id", connID)),
		confFlags:      cm.cfg.WebSocket.ConfFlags,
		subscribeQueue: make([]SubscribeRequest, 0),
		router:         cm.router,
	}

	// 【修正】customSubscriptionsからチャンネル別symbolを抽出
	tickerSymbols := make(map[string]bool)
	tradesSymbols := make(map[string]bool)
	booksSubscriptions := []SubscribeRequest{}
	candlesSubscriptions := []SubscribeRequest{}

	for _, sub := range cm.customSubscriptions {
		switch sub.Channel {
		case "ticker":
			tickerSymbols[sub.Symbol] = true
		case "trades":
			tradesSymbols[sub.Symbol] = true
		case "book":
			booksSubscriptions = append(booksSubscriptions, sub)
		case "candles":
			candlesSubscriptions = append(candlesSubscriptions, sub)
		}
	}

	// Ticker subscriptions (from customSubscriptions only)
	if cm.cfg.Channels.Ticker.Enabled {
		for symbol := range tickerSymbols {
			conn.subscribeQueue = append(conn.subscribeQueue, SubscribeRequest{
				Event:   "subscribe",
				Channel: "ticker",
				Symbol:  symbol,
			})
		}
	}

	// Trades subscriptions (from customSubscriptions only)
	if cm.cfg.Channels.Trades.Enabled {
		for symbol := range tradesSymbols {
			conn.subscribeQueue = append(conn.subscribeQueue, SubscribeRequest{
				Event:   "subscribe",
				Channel: "trades",
				Symbol:  symbol,
			})
		}
	}

	// Books subscriptions (from customSubscriptions, with prec/freq/len)
	if cm.cfg.Channels.Books.Enabled {
		for _, bookSub := range booksSubscriptions {
			if bookSub.Prec != nil && *bookSub.Prec != "R0" {
				conn.subscribeQueue = append(conn.subscribeQueue, bookSub)
			}
		}
	}

	// RawBooks subscriptions (from customSubscriptions, with R0 precision)
	if cm.cfg.Channels.RawBooks.Enabled {
		for _, bookSub := range booksSubscriptions {
			if bookSub.Prec != nil && *bookSub.Prec == "R0" {
				conn.subscribeQueue = append(conn.subscribeQueue, bookSub)
			}
		}
	}

	// Candles subscriptions (from customSubscriptions)
	conn.subscribeQueue = append(conn.subscribeQueue, candlesSubscriptions...)

	return conn, nil
}
```

#### Step 3.2: app.goの修正
**ファイル:** `internal/gui/app/app.go`
**変更箇所:** handleWsConnectConfig()

**変更内容:**
- customSubscriptionsをConnectionManagerに渡す前処理を追加

```go
func (a *Application) handleWsConnectConfig(wsConfig *gui.WSConnectionConfig) error {
	// ... (既存のコード)

	// Store custom subscriptions for connection manager
	a.customSubscriptions = wsConfig.Channels

	// 【修正】Convert ChannelSubscription to ws.SubscribeRequest format
	wsSubscriptions := make([]ws.SubscribeRequest, 0, len(wsConfig.Channels))
	for _, sub := range wsConfig.Channels {
		wsSubscriptions = append(wsSubscriptions, ws.SubscribeRequest{
			Event:   "subscribe",
			Channel: sub.Channel,
			Symbol:  sub.Symbol,
			Key:     sub.Key,
			Prec:    &sub.Prec,
			Freq:    &sub.Freq,
			Len:     &sub.Len,
		})
	}

	return a.handleWsConnect(exchange, symbols, wsSubscriptions)
}
```

#### Step 3.3: handleWsConnectの修正
**ファイル:** `internal/gui/app/app.go`

**変更内容:**
```go
func (a *Application) handleWsConnect(exchange string, symbols []string, subscriptions []ws.SubscribeRequest) error {
	// ... (既存の初期化コード)

	// Set custom subscriptions to connection manager
	a.connectionManager.SetCustomSubscriptions(subscriptions)

	// Start connection
	if err := a.connectionManager.Start(); err != nil {
		return fmt.Errorf("failed to start websocket connection: %w", err)
	}

	// ... (既存のコード)
}
```

---

## テスト計画

### 問題2のテスト
1. Ticker以外のチャンネル(Trades/Books/Candles)を有効化
2. データを数秒間取得
3. Disconnectボタンをクリック
4. dataディレクトリを確認し、.tmpファイルが残っていないことを確認

### UI改善のテスト
1. Books channelをdisableにする
2. Checksum/Bulk flagがdisableになることを確認
3. Books channelをenableにする
4. Checksum/Bulk flagがenableになることを確認

### 問題1のテスト
1. Ticker: tBTCUSD選択
2. Trades: tETHUSD選択
3. Books: tBNBUSD選択
4. Connectして各チャンネルが正しいsymbolのみ購読することを確認
5. Arrowファイルでそれぞれのsymbolが正しく保存されることを確認

---

## リスク管理

### 高リスク項目
1. **問題1の修正 - 既存動作への影響**
   - リスク: 既存の動作が壊れる可能性
   - 対策: 段階的実装と十分なテスト

2. **問題2の修正 - パフォーマンス影響**
   - リスク: 追加のFlushAll()呼び出しによる遅延
   - 対策: disconnect時のみなので影響は限定的

### 注意事項
- 各修正を個別にテスト
- 問題2を優先的に修正(データロス防止)
- UI改善は最も安全なので最後に実装可能

---

**最終更新:** 2025-10-04
**次のアクション:** Phase 1 (問題2)の実装開始
