package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// BitfinexConfig represents the Bitfinex exchange-specific configuration
type BitfinexConfig struct {
	Endpoints     BitfinexEndpoints     `yaml:"endpoints"`
	Limits        BitfinexLimits        `yaml:"limits"`
	Defaults      BitfinexDefaults      `yaml:"defaults"`
	Normalization NormalizationRules    `yaml:"normalization"`
	RestConfig    []RestConfigEndpoint  `yaml:"rest_config_endpoints"`
}

type BitfinexEndpoints struct {
	WSPublic   string `yaml:"ws_public"`
	WSAuth     string `yaml:"ws_auth"`
	RestPublic string `yaml:"rest_public"`
}

type BitfinexLimits struct {
	WSConnectionsPerMinute int `yaml:"ws_connections_per_minute"`
	WSMaxSubscriptions     int `yaml:"ws_max_subscriptions"`
	RestRateLimit          int `yaml:"rest_rate_limit"` // requests per minute
}

type BitfinexDefaults struct {
	Book    BookDefaults    `yaml:"book"`
	Candles CandlesDefaults `yaml:"candles"`
}

type BookDefaults struct {
	Prec string `yaml:"prec"`
	Freq string `yaml:"freq"`
	Len  string `yaml:"len"`
}

type CandlesDefaults struct {
	Timeframe string `yaml:"timeframe"`
}

type NormalizationRules struct {
	PairFormat string `yaml:"pair_format"`
	Uppercase  bool   `yaml:"uppercase"`
}

type RestConfigEndpoint struct {
	Endpoint      string `yaml:"endpoint"`
	CacheDuration int    `yaml:"cache_duration"` // seconds
	File          string `yaml:"file"`
}

// LoadBitfinexConfig loads the Bitfinex exchange configuration
func LoadBitfinexConfig(path string) (*BitfinexConfig, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("path must be absolute: %s", path)
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read bitfinex config: %w", err)
	}

	var cfg BitfinexConfig
	if err := yaml.Unmarshal(bytes, &cfg); err != nil {
		return nil, fmt.Errorf("unmarshal bitfinex config: %w", err)
	}

	return &cfg, nil
}

// SaveBitfinexConfig saves the Bitfinex configuration to disk
func SaveBitfinexConfig(path string, cfg *BitfinexConfig) error {
	bytes, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal bitfinex config: %w", err)
	}

	// Create backup before saving
	if err := createBackup(path); err != nil {
		// Log but don't fail on backup error
		fmt.Printf("Warning: failed to create backup: %v\n", err)
	}

	if err := os.WriteFile(path, bytes, 0644); err != nil {
		return fmt.Errorf("write bitfinex config: %w", err)
	}

	return nil
}

// createBackup creates a timestamped backup of the config file
func createBackup(path string) error {
	// Check if file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // No file to backup
	}

	// Read current content
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	// Create backup path
	dir := filepath.Dir(path)
	filename := filepath.Base(path)
	backupDir := filepath.Join(dir, "backups")
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return err
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(backupDir, fmt.Sprintf("%s_%s", filename, timestamp))

	// Write backup
	return os.WriteFile(backupPath, content, 0644)
}

// GetDefaultBitfinexConfig returns a default Bitfinex configuration
func GetDefaultBitfinexConfig() *BitfinexConfig {
	return &BitfinexConfig{
		Endpoints: BitfinexEndpoints{
			WSPublic:   "wss://api-pub.bitfinex.com/ws/2",
			WSAuth:     "wss://api.bitfinex.com/ws/2",
			RestPublic: "https://api-pub.bitfinex.com/v2",
		},
		Limits: BitfinexLimits{
			WSConnectionsPerMinute: 20,
			WSMaxSubscriptions:     30,
			RestRateLimit:          90,
		},
		Defaults: BitfinexDefaults{
			Book: BookDefaults{
				Prec: "P0",
				Freq: "F0",
				Len:  "25",
			},
			Candles: CandlesDefaults{
				Timeframe: "1m",
			},
		},
		Normalization: NormalizationRules{
			PairFormat: "base-quote",
			Uppercase:  true,
		},
		RestConfig: []RestConfigEndpoint{
			{
				Endpoint:      "pub:list:pair:exchange",
				CacheDuration: 3600, // 1 hour
				File:          "list_pair_exchange.json",
			},
			{
				Endpoint:      "pub:list:pair:margin",
				CacheDuration: 3600,
				File:          "list_pair_margin.json",
			},
			{
				Endpoint:      "pub:list:pair:futures",
				CacheDuration: 3600,
				File:          "list_pair_futures.json",
			},
			{
				Endpoint:      "pub:list:currency",
				CacheDuration: 86400, // 24 hours
				File:          "list_currency_margin.json",
			},
			{
				Endpoint:      "pub:map:currency:label",
				CacheDuration: 86400,
				File:          "map_currency_label.json",
			},
			{
				Endpoint:      "pub:map:currency:sym",
				CacheDuration: 86400,
				File:          "map_currency_sym.json",
			},
		},
	}
}