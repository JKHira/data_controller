package config

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// BitfinexRESTFetcher implements RestConfigFetcher for Bitfinex
type BitfinexRESTFetcher struct {
	baseURL    string
	httpClient *http.Client
}

// NewBitfinexRESTFetcher creates a new Bitfinex REST config fetcher
func NewBitfinexRESTFetcher(baseURL string) *BitfinexRESTFetcher {
	return &BitfinexRESTFetcher{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// FetchConfig fetches configuration data from Bitfinex REST API
func (f *BitfinexRESTFetcher) FetchConfig(endpoint string) ([]byte, error) {
	// Convert endpoint to URL
	// Format: "pub:list:pair:exchange" -> "/v2/conf/pub:list:pair:exchange"
	url := fmt.Sprintf("%s/conf/%s", f.baseURL, endpoint)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", "DataController/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := f.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return data, nil
}

// ParseEndpointFromFile converts a filename to an endpoint
// e.g., "list_pair_exchange.json" -> "pub:list:pair:exchange"
func ParseEndpointFromFile(filename string) string {
	// Remove .json extension
	name := strings.TrimSuffix(filename, ".json")

	// Convert underscores to colons
	endpoint := strings.ReplaceAll(name, "_", ":")

	// Add pub: prefix if not present
	if !strings.HasPrefix(endpoint, "pub:") && !strings.HasPrefix(endpoint, "calc:") {
		endpoint = "pub:" + endpoint
	}

	return endpoint
}