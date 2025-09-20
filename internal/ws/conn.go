package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"

	"github.com/trade-engine/data-controller/internal/config"
)

type ConnectionManager struct {
	cfg       *config.Config
	logger    *zap.Logger
	connMutex sync.RWMutex
	connections map[string]*Connection
	router    *Router
	ctx       context.Context
	cancel    context.CancelFunc
}

type Connection struct {
	ID              string
	URL             string
	conn            *websocket.Conn
	connMutex       sync.RWMutex
	channels        map[int32]*ChannelInfo
	channelsMutex   sync.RWMutex
	lastHeartbeat   map[int32]time.Time
	heartbeatMutex  sync.RWMutex
	reconnectChan   chan struct{}
	done            chan struct{}
	logger          *zap.Logger
	confFlags       int64
	isConnected     bool
	subscribeQueue  []SubscribeRequest
	queueMutex      sync.Mutex
	router          *Router
}

type ChannelInfo struct {
	ID       int32
	Channel  string
	Symbol   string
	Pair     string
	SubID    *int64
	SubReq   SubscribeRequest
}

type SubscribeRequest struct {
	Event   string  `json:"event"`
	Channel string  `json:"channel"`
	Symbol  string  `json:"symbol"`
	Prec    *string `json:"prec,omitempty"`
	Freq    *string `json:"freq,omitempty"`
	Len     *string `json:"len,omitempty"`
	SubID   *int64  `json:"subId,omitempty"`
}

type ConfMessage struct {
	Event string `json:"event"`
	Flags int64  `json:"flags"`
}

type InfoMessage struct {
	Event   string      `json:"event"`
	Version float64     `json:"version"`
	ServID  string      `json:"serverId"`
	Code    *int        `json:"code,omitempty"`
	Msg     *string     `json:"msg,omitempty"`
}

type SubscribeResponse struct {
	Event    string `json:"event"`
	Channel  string `json:"channel"`
	ChanID   int32  `json:"chanId"`
	Symbol   string `json:"symbol"`
	Pair     string `json:"pair"`
	Prec     string `json:"prec,omitempty"`
	Freq     string `json:"freq,omitempty"`
	Len      string `json:"len,omitempty"`
	SubID    *int64 `json:"subId,omitempty"`
}

func NewConnectionManager(cfg *config.Config, logger *zap.Logger, router *Router) *ConnectionManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &ConnectionManager{
		cfg:         cfg,
		logger:      logger,
		connections: make(map[string]*Connection),
		router:      router,
		ctx:         ctx,
		cancel:      cancel,
	}
}

func (cm *ConnectionManager) Start() error {
	cm.logger.Info("Starting connection manager")

	symbolsPerConn := make([][]string, 0)
	maxChannelsPerConn := 30 // Bitfinex limit
	channelsNeeded := 0

	for range cm.cfg.Symbols {
		if cm.cfg.Channels.Ticker.Enabled {
			channelsNeeded++
		}
		if cm.cfg.Channels.Trades.Enabled {
			channelsNeeded++
		}
		if cm.cfg.Channels.Books.Enabled {
			channelsNeeded++
		}
		if cm.cfg.Channels.RawBooks.Enabled {
			channelsNeeded++
		}
	}

	symbolsPerBatch := maxChannelsPerConn / 4
	if symbolsPerBatch == 0 {
		symbolsPerBatch = 1
	}

	for i := 0; i < len(cm.cfg.Symbols); i += symbolsPerBatch {
		end := i + symbolsPerBatch
		if end > len(cm.cfg.Symbols) {
			end = len(cm.cfg.Symbols)
		}
		symbolsPerConn = append(symbolsPerConn, cm.cfg.Symbols[i:end])
	}

	for i, symbols := range symbolsPerConn {
		connID := fmt.Sprintf("conn-%d", i)
		conn, err := cm.createConnection(connID, symbols)
		if err != nil {
			return fmt.Errorf("failed to create connection %s: %w", connID, err)
		}

		cm.connMutex.Lock()
		cm.connections[connID] = conn
		cm.connMutex.Unlock()

		go conn.run(cm.ctx)
	}

	return nil
}

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

	for _, symbol := range symbols {
		if cm.cfg.Channels.Ticker.Enabled {
			conn.subscribeQueue = append(conn.subscribeQueue, SubscribeRequest{
				Event:   "subscribe",
				Channel: "ticker",
				Symbol:  symbol,
			})
		}

		if cm.cfg.Channels.Trades.Enabled {
			conn.subscribeQueue = append(conn.subscribeQueue, SubscribeRequest{
				Event:   "subscribe",
				Channel: "trades",
				Symbol:  symbol,
			})
		}

		if cm.cfg.Channels.Books.Enabled {
			prec := cm.cfg.Channels.Books.Precision
			freq := cm.cfg.Channels.Books.Frequency
			len := fmt.Sprintf("%d", cm.cfg.Channels.Books.Length)
			subID := int64(time.Now().UnixNano())

			conn.subscribeQueue = append(conn.subscribeQueue, SubscribeRequest{
				Event:   "subscribe",
				Channel: "book",
				Symbol:  symbol,
				Prec:    &prec,
				Freq:    &freq,
				Len:     &len,
				SubID:   &subID,
			})
		}

		if cm.cfg.Channels.RawBooks.Enabled {
			prec := cm.cfg.Channels.RawBooks.Precision
			freq := cm.cfg.Channels.RawBooks.Frequency
			len := fmt.Sprintf("%d", cm.cfg.Channels.RawBooks.Length)
			subID := int64(time.Now().UnixNano() + 1)

			conn.subscribeQueue = append(conn.subscribeQueue, SubscribeRequest{
				Event:   "subscribe",
				Channel: "book",
				Symbol:  symbol,
				Prec:    &prec,
				Freq:    &freq,
				Len:     &len,
				SubID:   &subID,
			})
		}
	}

	return conn, nil
}

func (c *Connection) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Connection context cancelled")
			return
		case <-c.reconnectChan:
			c.logger.Info("Reconnect signal received")
		default:
		}

		if err := c.connect(); err != nil {
			c.logger.Error("Failed to connect", zap.Error(err))
			time.Sleep(5 * time.Second)
			continue
		}

		if err := c.sendConf(); err != nil {
			c.logger.Error("Failed to send conf", zap.Error(err))
			c.disconnect()
			continue
		}

		if err := c.subscribeAll(); err != nil {
			c.logger.Error("Failed to subscribe", zap.Error(err))
			c.disconnect()
			continue
		}

		go c.heartbeatMonitor(ctx)
		go c.pingRoutine(ctx)

		c.readLoop(ctx)
		c.disconnect()

		select {
		case <-ctx.Done():
			return
		case <-time.After(5 * time.Second):
			c.logger.Info("Reconnecting after 5 seconds")
		}
	}
}

func (c *Connection) connect() error {
	c.logger.Info("Connecting to WebSocket", zap.String("url", c.URL))

	dialer := &websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(c.URL, http.Header{})
	if err != nil {
		return fmt.Errorf("failed to dial: %w", err)
	}

	c.connMutex.Lock()
	c.conn = conn
	c.isConnected = true
	c.connMutex.Unlock()

	c.logger.Info("Connected successfully")
	return nil
}

func (c *Connection) disconnect() {
	c.connMutex.Lock()
	defer c.connMutex.Unlock()

	if c.conn != nil {
		c.logger.Info("Closing WebSocket connection")

		// Send close frame
		c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))

		// Close the connection
		c.conn.Close()
		c.conn = nil
	}
	c.isConnected = false
	c.logger.Info("Disconnected")
}

func (c *Connection) sendConf() error {
	confMsg := ConfMessage{
		Event: "conf",
		Flags: c.confFlags,
	}

	return c.sendMessage(confMsg)
}

func (c *Connection) subscribeAll() error {
	c.queueMutex.Lock()
	defer c.queueMutex.Unlock()

	for _, req := range c.subscribeQueue {
		if err := c.sendMessage(req); err != nil {
			return fmt.Errorf("failed to subscribe to %s:%s: %w", req.Channel, req.Symbol, err)
		}
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

func (c *Connection) sendMessage(msg interface{}) error {
	c.connMutex.RLock()
	defer c.connMutex.RUnlock()

	if c.conn == nil {
		return fmt.Errorf("connection not established")
	}

	return c.conn.WriteJSON(msg)
}

func (c *Connection) readLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("Read loop context cancelled")
			return
		case <-c.done:
			c.logger.Info("Read loop received done signal")
			return
		default:
		}

		c.connMutex.RLock()
		conn := c.conn
		c.connMutex.RUnlock()

		if conn == nil {
			c.logger.Info("Connection is nil, exiting read loop")
			return
		}

		// Set read deadline to allow graceful shutdown
		conn.SetReadDeadline(time.Now().Add(5 * time.Second))

		_, message, err := conn.ReadMessage()
		if err != nil {
			// Check if it's a timeout (expected during shutdown)
			if netErr, ok := err.(interface{ Timeout() bool }); ok && netErr.Timeout() {
				continue
			}
			c.logger.Error("Read error", zap.Error(err))
			return
		}

		if err := c.processMessage(message); err != nil {
			c.logger.Error("Failed to process message", zap.Error(err))
		}
	}
}

func (c *Connection) processMessage(data []byte) error {
	var rawMsg json.RawMessage
	if err := json.Unmarshal(data, &rawMsg); err != nil {
		return fmt.Errorf("failed to unmarshal raw message: %w", err)
	}

	var array []json.RawMessage
	if err := json.Unmarshal(rawMsg, &array); err != nil {
		var info InfoMessage
		if err := json.Unmarshal(rawMsg, &info); err == nil {
			return c.handleInfoMessage(&info)
		}

		var subResp SubscribeResponse
		if err := json.Unmarshal(rawMsg, &subResp); err == nil {
			return c.handleSubscribeResponse(&subResp)
		}

		return fmt.Errorf("unknown message format")
	}

	if len(array) < 2 {
		return fmt.Errorf("array message too short")
	}

	var chanID int32
	if err := json.Unmarshal(array[0], &chanID); err != nil {
		return fmt.Errorf("failed to unmarshal channel ID: %w", err)
	}

	var msgType string
	if err := json.Unmarshal(array[1], &msgType); err != nil {
		return c.handleDataMessage(chanID, array[1:])
	}

	switch msgType {
	case "hb":
		return c.handleHeartbeat(chanID)
	case "cs":
		if len(array) >= 3 {
			var checksum int32
			if err := json.Unmarshal(array[2], &checksum); err == nil {
				return c.handleChecksum(chanID, checksum)
			}
		}
	default:
		return c.handleDataMessage(chanID, array[1:])
	}

	return nil
}

func (c *Connection) handleInfoMessage(info *InfoMessage) error {
	c.logger.Info("Received info message",
		zap.String("event", info.Event),
		zap.Float64("version", info.Version),
		zap.String("server_id", info.ServID))

	if info.Code != nil {
		c.logger.Info("Info code received", zap.Int("code", *info.Code))

		if *info.Code == 20051 || *info.Code == 20060 || *info.Code == 20061 {
			c.logger.Info("Server maintenance or restart, triggering reconnect")
			select {
			case c.reconnectChan <- struct{}{}:
			default:
			}
		}
	}

	return nil
}

func (c *Connection) handleSubscribeResponse(resp *SubscribeResponse) error {
	c.logger.Info("Channel subscribed",
		zap.String("channel", resp.Channel),
		zap.Int32("chan_id", resp.ChanID),
		zap.String("symbol", resp.Symbol),
		zap.String("pair", resp.Pair))

	channelInfo := &ChannelInfo{
		ID:      resp.ChanID,
		Channel: resp.Channel,
		Symbol:  resp.Symbol,
		Pair:    resp.Pair,
		SubID:   resp.SubID,
	}

	c.channelsMutex.Lock()
	c.channels[resp.ChanID] = channelInfo
	c.channelsMutex.Unlock()

	c.heartbeatMutex.Lock()
	c.lastHeartbeat[resp.ChanID] = time.Now()
	c.heartbeatMutex.Unlock()

	return nil
}

func (c *Connection) handleHeartbeat(chanID int32) error {
	c.heartbeatMutex.Lock()
	c.lastHeartbeat[chanID] = time.Now()
	c.heartbeatMutex.Unlock()

	return nil
}

func (c *Connection) handleChecksum(chanID int32, checksum int32) error {
	c.logger.Debug("Received checksum",
		zap.Int32("chan_id", chanID),
		zap.Int32("checksum", checksum))

	return nil
}

func (c *Connection) handleDataMessage(chanID int32, data []json.RawMessage) error {
	c.channelsMutex.RLock()
	channelInfo, exists := c.channels[chanID]
	c.channelsMutex.RUnlock()

	if !exists {
		c.logger.Warn("Received data for unknown channel", zap.Int32("chan_id", chanID))
		return nil
	}

	c.logger.Debug("Received data message",
		zap.Int32("chan_id", chanID),
		zap.String("channel", channelInfo.Channel),
		zap.String("symbol", channelInfo.Symbol),
		zap.Int("data_length", len(data)))

	// Route message to router if available
	if c.router != nil {
		return c.router.RouteMessage(chanID, channelInfo, data, c.ID)
	}

	c.logger.Warn("No router available for data routing")
	return nil
}

func (c *Connection) heartbeatMonitor(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.checkHeartbeats()
		}
	}
}

func (c *Connection) checkHeartbeats() {
	now := time.Now()
	timeout := 45 * time.Second

	c.heartbeatMutex.RLock()
	for chanID, lastHB := range c.lastHeartbeat {
		if now.Sub(lastHB) > timeout {
			c.logger.Warn("Heartbeat timeout",
				zap.Int32("chan_id", chanID),
				zap.Duration("since_last", now.Sub(lastHB)))

			select {
			case c.reconnectChan <- struct{}{}:
			default:
			}
			break
		}
	}
	c.heartbeatMutex.RUnlock()
}

func (c *Connection) pingRoutine(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := c.ping(); err != nil {
				c.logger.Error("Failed to send ping", zap.Error(err))
			}
		}
	}
}

func (c *Connection) ping() error {
	pingMsg := map[string]interface{}{
		"event": "ping",
		"cid":   time.Now().UnixNano(),
	}

	return c.sendMessage(pingMsg)
}

func (cm *ConnectionManager) Stop() {
	cm.logger.Info("Stopping connection manager")
	cm.cancel()

	cm.connMutex.RLock()
	connections := make([]*Connection, 0, len(cm.connections))
	for _, conn := range cm.connections {
		connections = append(connections, conn)
	}
	cm.connMutex.RUnlock()

	// Gracefully disconnect all connections
	for _, conn := range connections {
		cm.logger.Info("Stopping connection", zap.String("conn_id", conn.ID))

		// Signal the connection to stop
		select {
		case conn.done <- struct{}{}:
		default:
		}

		// Give it a moment to close gracefully
		time.Sleep(100 * time.Millisecond)

		// Force disconnect if still connected
		conn.disconnect()
		cm.logger.Info("Connection stopped", zap.String("conn_id", conn.ID))
	}

	cm.logger.Info("All connections stopped")
}