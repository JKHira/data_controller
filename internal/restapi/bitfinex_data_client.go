package restapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// BitfinexDataClient provides access to public REST data endpoints (candles, trades, tickers history).
type BitfinexDataClient struct {
	baseURL   string
	client    *http.Client
	logger    *zap.Logger
	limiters  map[string]*rate.Limiter
	limiterMu sync.Mutex
}

const (
	candlesEndpointKey = "candles"
	tradesEndpointKey  = "trades"
	tickersEndpointKey = "tickers"
)

// NewBitfinexDataClient constructs a new BitfinexDataClient with sane defaults and per-endpoint rate limiting.
func NewBitfinexDataClient(logger *zap.Logger) *BitfinexDataClient {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &BitfinexDataClient{
		baseURL: "https://api-pub.bitfinex.com/v2",
		client: &http.Client{
			Timeout: 15 * time.Second,
		},
		logger: logger,
		limiters: map[string]*rate.Limiter{
			candlesEndpointKey: rate.NewLimiter(rate.Every(time.Minute/30), 1),
			tradesEndpointKey:  rate.NewLimiter(rate.Every(time.Minute/15), 1),
			tickersEndpointKey: rate.NewLimiter(rate.Every(time.Minute/10), 1),
		},
	}
}

// CandlesRequest defines the parameters for fetching historical candles.
type CandlesRequest struct {
	Symbol    string
	Timeframe string
	Section   string // hist or last
	Start     int64  // milliseconds
	End       int64  // milliseconds
	Limit     int
	Sort      int
}

// TradesRequest defines the parameters for fetching historical trades.
type TradesRequest struct {
	Symbol string
	Start  int64
	End    int64
	Limit  int
	Sort   int
}

// TickersHistoryRequest defines parameters for fetching tickers history snapshots.
type TickersHistoryRequest struct {
	Symbols []string
	Start   int64
	End     int64
	Limit   int
	Sort    int
}

// FetchCandles retrieves a single page of candles matching the request.
func (c *BitfinexDataClient) FetchCandles(ctx context.Context, req CandlesRequest) ([][6]float64, error) {
	if req.Section == "" {
		req.Section = "hist"
	}
	key := fmt.Sprintf("trade:%s:%s", req.Timeframe, req.Symbol)
	path := fmt.Sprintf("/candles/%s/%s", key, req.Section)

	query := url.Values{}
	if req.Start > 0 {
		query.Set("start", fmt.Sprintf("%d", req.Start))
	}
	if req.End > 0 {
		query.Set("end", fmt.Sprintf("%d", req.End))
	}
	if req.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", req.Limit))
	}
	if req.Sort != 0 {
		query.Set("sort", fmt.Sprintf("%d", req.Sort))
	}

	body, err := c.doRequest(ctx, candlesEndpointKey, path, query)
	if err != nil {
		return nil, err
	}

	var raw [][]float64
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode candles response: %w", err)
	}

	result := make([][6]float64, 0, len(raw))
	for _, row := range raw {
		if len(row) < 6 {
			continue
		}
		var entry [6]float64
		for i := 0; i < 6; i++ {
			entry[i] = row[i]
		}
		result = append(result, entry)
	}

	return result, nil
}

// FetchTrades retrieves a single page of trades for the given symbol.
func (c *BitfinexDataClient) FetchTrades(ctx context.Context, req TradesRequest) ([][]float64, error) {
	path := fmt.Sprintf("/trades/%s/hist", req.Symbol)
	query := url.Values{}
	if req.Start > 0 {
		query.Set("start", fmt.Sprintf("%d", req.Start))
	}
	if req.End > 0 {
		query.Set("end", fmt.Sprintf("%d", req.End))
	}
	if req.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", req.Limit))
	}
	if req.Sort != 0 {
		query.Set("sort", fmt.Sprintf("%d", req.Sort))
	}

	body, err := c.doRequest(ctx, tradesEndpointKey, path, query)
	if err != nil {
		return nil, err
	}

	var raw [][]float64
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode trades response: %w", err)
	}

	return raw, nil
}

// FetchTickersHistory retrieves hourly ticker snapshots for the requested symbols.
func (c *BitfinexDataClient) FetchTickersHistory(ctx context.Context, req TickersHistoryRequest) ([][]interface{}, error) {
	path := "/tickers/hist"
	query := url.Values{}
	if len(req.Symbols) > 0 {
		query.Set("symbols", strings.Join(req.Symbols, ","))
	}
	if req.Start > 0 {
		query.Set("start", fmt.Sprintf("%d", req.Start))
	}
	if req.End > 0 {
		query.Set("end", fmt.Sprintf("%d", req.End))
	}
	if req.Limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", req.Limit))
	}
	if req.Sort != 0 {
		query.Set("sort", fmt.Sprintf("%d", req.Sort))
	}

	body, err := c.doRequest(ctx, tickersEndpointKey, path, query)
	if err != nil {
		return nil, err
	}

	var raw [][]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("decode tickers response: %w", err)
	}

	return raw, nil
}

func (c *BitfinexDataClient) doRequest(ctx context.Context, limiterKey, path string, query url.Values) ([]byte, error) {
	const (
		maxRetries     = 5
		maxBackoff     = 30 * time.Second
		initialBackoff = time.Second
	)

	reqURL := c.baseURL + path
	if len(query) > 0 {
		reqURL += "?" + query.Encode()
	}

	for attempt := 0; attempt < maxRetries; attempt++ {
		if err := c.waitLimiter(ctx, limiterKey); err != nil {
			return nil, err
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
		if err != nil {
			return nil, err
		}

		req.Header.Set("User-Agent", "trade-engine-data-controller/1.0")

		resp, err := c.client.Do(req)
		if err != nil {
			return nil, err
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			delay := initialBackoff << attempt
			if retryAfter := parseRetryAfter(resp.Header.Get("Retry-After")); retryAfter > 0 {
				delay = retryAfter
			}
			if delay > maxBackoff {
				delay = maxBackoff
			}
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		return body, nil
	}

	return nil, fmt.Errorf("too many retries for %s", path)
}

func parseRetryAfter(header string) time.Duration {
	if header == "" {
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	if retryTime, err := http.ParseTime(header); err == nil {
		delay := time.Until(retryTime)
		if delay > 0 {
			return delay
		}
	}
	return 0
}

func (c *BitfinexDataClient) waitLimiter(ctx context.Context, key string) error {
	c.limiterMu.Lock()
	limiter, ok := c.limiters[key]
	c.limiterMu.Unlock()
	if !ok {
		return fmt.Errorf("no limiter configured for key %s", key)
	}
	return limiter.Wait(ctx)
}
