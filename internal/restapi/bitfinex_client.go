package restapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// BitfinexClient handles REST API calls to Bitfinex with rate limiting and
// JSON persistence for configuration endpoints.
type BitfinexClient struct {
	baseURL         string
	client          *http.Client
	logger          *zap.Logger
	confLimiter     *rate.Limiter
	storageBasePath string
}

// EndpointTask describes a single REST configuration endpoint to fetch and persist.
type EndpointTask struct {
	Endpoint string
	FileName string
}

// FetchResult represents the outcome of a single endpoint fetch.
type FetchResult struct {
	Endpoint  string    `json:"endpoint"`
	Success   bool      `json:"success"`
	Timestamp time.Time `json:"timestamp"`
	FilePath  string    `json:"file_path,omitempty"`
	Error     string    `json:"error,omitempty"`
	Count     int       `json:"count,omitempty"`
}

// NewBitfinexClient creates a new Bitfinex REST API client.
func NewBitfinexClient(logger *zap.Logger, storageBasePath string) *BitfinexClient {
	return &BitfinexClient{
		baseURL:         "https://api-pub.bitfinex.com/v2",
		client:          &http.Client{Timeout: 10 * time.Second},
		logger:          logger,
		confLimiter:     rate.NewLimiter(rate.Every(time.Minute/72), 1),
		storageBasePath: storageBasePath,
	}
}

// FetchAndStoreJSON fetches the endpoint defined in task and writes the JSON
// response to the configured storage location. The returned FetchResult includes
// the resolved file path and a best-effort element count.
func (c *BitfinexClient) FetchAndStoreJSON(ctx context.Context, exchange string, task EndpointTask) FetchResult {
	result := FetchResult{
		Endpoint:  task.Endpoint,
		Timestamp: time.Now().UTC(),
	}

	body, err := c.fetchConfRaw(ctx, task.Endpoint)
	if err != nil {
		result.Error = err.Error()
		c.logger.Error("Failed to fetch config endpoint",
			zap.String("endpoint", task.Endpoint),
			zap.Error(err))
		return result
	}

	filePath, err := c.persistJSON(exchange, task.FileName, body)
	if err != nil {
		result.Error = err.Error()
		c.logger.Error("Failed to persist config endpoint",
			zap.String("endpoint", task.Endpoint),
			zap.Error(err))
		return result
	}

	result.FilePath = filePath
	result.Success = true
	result.Count = countTopLevelElements(body)

	c.logger.Info("Config endpoint fetched",
		zap.String("endpoint", task.Endpoint),
		zap.String("file", filePath),
		zap.Int("count", result.Count))

	return result
}

// fetchConfRaw fetches the raw JSON response for a configuration endpoint.
func (c *BitfinexClient) fetchConfRaw(ctx context.Context, key string) ([]byte, error) {
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

	return body, nil
}

func (c *BitfinexClient) persistJSON(exchange, fileName string, data []byte) (string, error) {
	if c.storageBasePath == "" {
		return "", fmt.Errorf("storage base path is not configured")
	}

	if exchange == "" {
		exchange = "bitfinex"
	}

	dir := filepath.Join(c.storageBasePath, exchange, "restapi", "config")
	if err := createDirIfNotExists(dir); err != nil {
		return "", err
	}

	if fileName == "" {
		fileName = "config.json"
	}

	path := filepath.Join(dir, sanitizeFileName(fileName))

	var pretty bytes.Buffer
	if json.Valid(data) {
		if err := json.Indent(&pretty, data, "", "  "); err != nil {
			pretty.Write(data)
		}
	} else {
		pretty.Write(data)
	}

	if err := writeFile(path, pretty.Bytes()); err != nil {
		return "", err
	}

	return path, nil
}

func sanitizeFileName(name string) string {
	cleaned := strings.ReplaceAll(name, ":", "_")
	cleaned = strings.ReplaceAll(cleaned, " ", "_")
	cleaned = strings.ReplaceAll(cleaned, "-", "_")
	for strings.Contains(cleaned, "__") {
		cleaned = strings.ReplaceAll(cleaned, "__", "_")
	}
	return cleaned
}

func countTopLevelElements(data []byte) int {
	var generic interface{}
	if err := json.Unmarshal(data, &generic); err != nil {
		return 0
	}

	switch typed := generic.(type) {
	case []interface{}:
		return len(typed)
	case map[string]interface{}:
		return len(typed)
	default:
		return 1
	}
}
