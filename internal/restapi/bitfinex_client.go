package restapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"golang.org/x/time/rate"
	"go.uber.org/zap"
)

// BitfinexClient handles REST API calls to Bitfinex with rate limiting
type BitfinexClient struct {
	baseURL    string
	client     *http.Client
	logger     *zap.Logger

	// Rate limiters for different endpoints
	confLimiter    *rate.Limiter  // 90 req/min
	tickersLimiter *rate.Limiter  // 30 req/min
	candlesLimiter *rate.Limiter  // 30 req/min
	tradesLimiter  *rate.Limiter  // 15 req/min
	bookLimiter    *rate.Limiter  // 240 req/min

	// Arrow storage handler
	arrowStorage *ArrowStorage
}

// BaseDataOptions represents the checkable options for base data fetching
type BaseDataOptions struct {
	// Listings
	SpotPairs      bool `json:"spot_pairs"`
	MarginPairs    bool `json:"margin_pairs"`
	FuturesPairs   bool `json:"futures_pairs"`
	Currencies     bool `json:"currencies"`
	MarginCurrencies bool `json:"margin_currencies"`

	// Mappings
	CurrencyLabels bool `json:"currency_labels"`
	CurrencySymbols bool `json:"currency_symbols"`
	CurrencyUnits  bool `json:"currency_units"`
	CurrencyUnderlying bool `json:"currency_underlying"`

	// Pair Info
	PairInfo        bool `json:"pair_info"`
	FuturesPairInfo bool `json:"futures_pair_info"`

	// Active Snapshot
	ActiveTickers bool `json:"active_tickers"`
}

// FetchResult represents the result of a fetch operation
type FetchResult struct {
	Endpoint  string    `json:"endpoint"`
	Success   bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
	FilePath  string    `json:"file_path,omitempty"`
	Error     string    `json:"error,omitempty"`
	Count     int       `json:"count,omitempty"`
}

// NewBitfinexClient creates a new Bitfinex REST API client
func NewBitfinexClient(logger *zap.Logger) *BitfinexClient {
	return &BitfinexClient{
		baseURL: "https://api-pub.bitfinex.com/v2",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		logger: logger,

		// Rate limiters (using 80% of official limits for safety)
		confLimiter:    rate.NewLimiter(rate.Every(time.Minute/72), 1),   // 72 req/min (80% of 90)
		tickersLimiter: rate.NewLimiter(rate.Every(time.Minute/24), 1),   // 24 req/min (80% of 30)
		candlesLimiter: rate.NewLimiter(rate.Every(time.Minute/24), 1),   // 24 req/min (80% of 30)
		tradesLimiter:  rate.NewLimiter(rate.Every(time.Minute/12), 1),   // 12 req/min (80% of 15)
		bookLimiter:    rate.NewLimiter(rate.Every(time.Minute/192), 1),  // 192 req/min (80% of 240)

		// Initialize Arrow storage
		arrowStorage: NewArrowStorage(logger),
	}
}

// fetchConfList fetches configuration list from Bitfinex
func (c *BitfinexClient) fetchConfList(ctx context.Context, key string) ([]string, error) {
	// Wait for rate limit
	if err := c.confLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	url := fmt.Sprintf("%s/conf/%s", c.baseURL, key)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "trade-engine-data-controller/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	// Response is [["BTCUSD","ETHUSD",...]]
	var outer [][]string
	if err := json.Unmarshal(body, &outer); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	if len(outer) == 0 {
		return nil, fmt.Errorf("empty response")
	}

	return outer[0], nil
}

// fetchTickers fetches all active tickers from Bitfinex
func (c *BitfinexClient) fetchTickers(ctx context.Context) ([]interface{}, error) {
	// Wait for rate limit
	if err := c.tickersLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limit wait failed: %w", err)
	}

	url := fmt.Sprintf("%s/tickers?symbols=ALL", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "trade-engine-data-controller/1.0")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tickers []interface{}
	if err := json.Unmarshal(body, &tickers); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	return tickers, nil
}

// FetchBaseData fetches base data according to the specified options
func (c *BitfinexClient) FetchBaseData(ctx context.Context, options BaseDataOptions, onProgress func(result FetchResult)) error {
	results := []FetchResult{}

	c.logger.Info("Starting Bitfinex base data fetch", zap.Any("options", options))

	// Create storage directory
	storageDir := "data/bitfinex/restv2/basedata"
	timestamp := time.Now().UTC()
	timestampStr := timestamp.Format("20060102T150405Z")

	// Fetch listings
	if options.SpotPairs {
		result := c.fetchAndSave(ctx, "pub:list:pair:exchange", "conf-list-pair-exchange", timestampStr, storageDir)
		results = append(results, result)
		if onProgress != nil {
			onProgress(result)
		}
	}

	if options.MarginPairs {
		result := c.fetchAndSave(ctx, "pub:list:pair:margin", "conf-list-pair-margin", timestampStr, storageDir)
		results = append(results, result)
		if onProgress != nil {
			onProgress(result)
		}
	}

	if options.FuturesPairs {
		result := c.fetchAndSave(ctx, "pub:list:pair:futures", "conf-list-pair-futures", timestampStr, storageDir)
		results = append(results, result)
		if onProgress != nil {
			onProgress(result)
		}
	}

	if options.Currencies {
		result := c.fetchAndSave(ctx, "pub:list:currency", "conf-list-currency", timestampStr, storageDir)
		results = append(results, result)
		if onProgress != nil {
			onProgress(result)
		}
	}

	if options.MarginCurrencies {
		result := c.fetchAndSave(ctx, "pub:list:currency:margin", "conf-list-currency-margin", timestampStr, storageDir)
		results = append(results, result)
		if onProgress != nil {
			onProgress(result)
		}
	}

	// Fetch mappings
	if options.CurrencyLabels {
		result := c.fetchAndSave(ctx, "pub:map:currency:label", "conf-map-currency-label", timestampStr, storageDir)
		results = append(results, result)
		if onProgress != nil {
			onProgress(result)
		}
	}

	if options.CurrencySymbols {
		result := c.fetchAndSave(ctx, "pub:map:currency:sym", "conf-map-currency-sym", timestampStr, storageDir)
		results = append(results, result)
		if onProgress != nil {
			onProgress(result)
		}
	}

	if options.CurrencyUnits {
		result := c.fetchAndSave(ctx, "pub:map:currency:unit", "conf-map-currency-unit", timestampStr, storageDir)
		results = append(results, result)
		if onProgress != nil {
			onProgress(result)
		}
	}

	if options.CurrencyUnderlying {
		result := c.fetchAndSave(ctx, "pub:map:currency:undl", "conf-map-currency-undl", timestampStr, storageDir)
		results = append(results, result)
		if onProgress != nil {
			onProgress(result)
		}
	}

	// Fetch active tickers
	if options.ActiveTickers {
		result := c.fetchTickersAndSave(ctx, timestampStr, storageDir)
		results = append(results, result)
		if onProgress != nil {
			onProgress(result)
		}
	}

	// Log summary
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	c.logger.Info("Bitfinex base data fetch completed",
		zap.Int("total", len(results)),
		zap.Int("success", successCount),
		zap.Int("failed", len(results)-successCount))

	return nil
}

// Helper function to fetch config data and save to file
func (c *BitfinexClient) fetchAndSave(ctx context.Context, confKey, filePrefix, timestamp, storageDir string) FetchResult {
	result := FetchResult{
		Endpoint:  confKey,
		Timestamp: time.Now().UTC(),
	}

	// Fetch data
	data, err := c.fetchConfList(ctx, confKey)
	if err != nil {
		result.Error = err.Error()
		c.logger.Error("Failed to fetch config", zap.String("key", confKey), zap.Error(err))
		return result
	}

	// Save to Arrow file
	filePath, err := c.arrowStorage.SaveBaseDataAsArrow(data, confKey, "bitfinex", result.Timestamp)
	if err != nil {
		result.Error = err.Error()
		c.logger.Error("Failed to save config", zap.String("key", confKey), zap.Error(err))
		return result
	}

	result.Success = true
	result.FilePath = filePath
	result.Count = len(data)

	c.logger.Info("Successfully fetched and saved config",
		zap.String("key", confKey),
		zap.String("file", filePath),
		zap.Int("count", len(data)))

	return result
}

// Helper function to fetch tickers and save to file
func (c *BitfinexClient) fetchTickersAndSave(ctx context.Context, timestamp, storageDir string) FetchResult {
	result := FetchResult{
		Endpoint:  "tickers?symbols=ALL",
		Timestamp: time.Now().UTC(),
	}

	// Fetch data
	data, err := c.fetchTickers(ctx)
	if err != nil {
		result.Error = err.Error()
		c.logger.Error("Failed to fetch tickers", zap.Error(err))
		return result
	}

	// Save to Arrow file
	filePath, err := c.arrowStorage.SaveBaseDataAsArrow(data, "tickers", "bitfinex", result.Timestamp)
	if err != nil {
		result.Error = err.Error()
		c.logger.Error("Failed to save tickers", zap.Error(err))
		return result
	}

	result.Success = true
	result.FilePath = filePath
	result.Count = len(data)

	c.logger.Info("Successfully fetched and saved tickers",
		zap.String("file", filePath),
		zap.Int("count", len(data)))

	return result
}

