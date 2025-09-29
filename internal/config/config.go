package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config represents the merged runtime configuration that combines
// global application settings with the currently active exchange profile.
type Config struct {
	Application Application
	Storage     Storage
	Metadata    Metadata
	Monitoring  Monitoring
	GUI         GUI
	Performance Performance
	Debug       Debug

	WebSocket WebSocket
	Symbols   []string
	Channels  Channels

	ActiveExchange     string
	ActiveProfile      string
	GlobalConfigPath   string
	ExchangeConfigPath string
	StatePath          string

	Exchanges ExchangesDefinition
}

type Application struct {
	Name     string `yaml:"name"`
	Version  string `yaml:"version"`
	LogLevel string `yaml:"log_level"`
}

type WebSocket struct {
	URL               string        `yaml:"url"`
	ReconnectInterval time.Duration `yaml:"reconnect_interval"`
	HeartbeatTimeout  time.Duration `yaml:"heartbeat_timeout"`
	PingInterval      time.Duration `yaml:"ping_interval"`
	MaxConnections    int           `yaml:"max_connections"`
	ConnectionTimeout time.Duration `yaml:"connection_timeout"`
	ConfFlags         int64         `yaml:"conf_flags"`
}

type Channels struct {
	Ticker   TickerConfig   `yaml:"ticker"`
	Trades   TradesConfig   `yaml:"trades"`
	Books    BooksConfig    `yaml:"books"`
	RawBooks RawBooksConfig `yaml:"raw_books"`
}

type TickerConfig struct {
	Enabled      bool          `yaml:"enabled"`
	SamplingRate time.Duration `yaml:"sampling_rate"`
}

type TradesConfig struct {
	Enabled bool   `yaml:"enabled"`
	MsgType string `yaml:"msg_type"`
}

type BooksConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Precision string `yaml:"precision"`
	Frequency string `yaml:"frequency"`
	Length    int    `yaml:"length"`
}

type RawBooksConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Precision string `yaml:"precision"`
	Frequency string `yaml:"frequency"`
	Length    int    `yaml:"length"`
}

type Storage struct {
	BasePath         string        `yaml:"base_path"`
	SegmentSizeMB    int           `yaml:"segment_size_mb"`
	Compression      string        `yaml:"compression"`
	CompressionLevel int           `yaml:"compression_level"`
	Parquet          ParquetConfig `yaml:"parquet"`
	WAL              WALConfig     `yaml:"wal"`
}

type ParquetConfig struct {
	RowGroupSizeMB int           `yaml:"row_group_size_mb"`
	FlushInterval  time.Duration `yaml:"flush_interval"`
	FlushRowCount  int           `yaml:"flush_row_count"`
}

type WALConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Compression    string `yaml:"compression"`
	RetentionHours int    `yaml:"retention_hours"`
}

type Metadata struct {
	SchemaVersion             string `yaml:"schema_version"`
	IncludeChecksumValidation bool   `yaml:"include_checksum_validation"`
	IncludeSequenceNumbers    bool   `yaml:"include_sequence_numbers"`
	IncludeTimestamps         bool   `yaml:"include_timestamps"`
}

type Monitoring struct {
	Prometheus  PrometheusConfig  `yaml:"prometheus"`
	HealthCheck HealthCheckConfig `yaml:"health_check"`
	Logging     LoggingConfig     `yaml:"logging"`
}

type PrometheusConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

type HealthCheckConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
}

type LoggingConfig struct {
	Format   string `yaml:"format"`
	Output   string `yaml:"output"`
	FilePath string `yaml:"file_path"`
}

type GUI struct {
	Title           string        `yaml:"title"`
	Width           int           `yaml:"width"`
	Height          int           `yaml:"height"`
	Theme           string        `yaml:"theme"`
	AutoStart       bool          `yaml:"auto_start"`
	ShowStatistics  bool          `yaml:"show_statistics"`
	RefreshInterval time.Duration `yaml:"refresh_interval"`
}

type Performance struct {
	BufferSize     int                  `yaml:"buffer_size"`
	WorkerCount    int                  `yaml:"worker_count"`
	MaxMemoryMB    int                  `yaml:"max_memory_mb"`
	GCInterval     time.Duration        `yaml:"gc_interval"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}

type CircuitBreakerConfig struct {
	Enabled          bool          `yaml:"enabled"`
	FailureThreshold int           `yaml:"failure_threshold"`
	ResetTimeout     time.Duration `yaml:"reset_timeout"`
}

type Debug struct {
	EnableProfiling       bool `yaml:"enable_profiling"`
	ProfilingPort         int  `yaml:"profiling_port"`
	VerboseLogging        bool `yaml:"verbose_logging"`
	SaveRawMessages       bool `yaml:"save_raw_messages"`
	SimulateNetworkIssues bool `yaml:"simulate_network_issues"`
}

// ExchangesDefinition tracks available exchanges and their profile metadata.
type ExchangesDefinition struct {
	Default string                      `yaml:"default"`
	Entries map[string]ExchangeSettings `yaml:"entries"`
}

// ExchangeSettings defines profile management information for a single exchange.
type ExchangeSettings struct {
	DefaultProfile  string                     `yaml:"default_profile"`
	ActiveProfile   string                     `yaml:"active_profile"`
	LastUsedProfile string                     `yaml:"last_used_profile"`
	Profiles        map[string]ExchangeProfile `yaml:"profiles"`
}

// ExchangeProfile describes a concrete profile location.
type ExchangeProfile struct {
	Path        string `yaml:"path"`
	Description string `yaml:"description,omitempty"`
	IsDefault   bool   `yaml:"is_default,omitempty"`
}

// exchangeProfileConfig represents the on-disk structure for an individual exchange profile.
type exchangeProfileConfig struct {
	WebSocket WebSocket `yaml:"websocket"`
	Symbols   []string  `yaml:"symbols"`
	Channels  Channels  `yaml:"channels"`
}

// globalConfig mirrors the persisted global configuration file.
type globalConfig struct {
	Application Application         `yaml:"application"`
	Storage     Storage             `yaml:"storage"`
	Metadata    Metadata            `yaml:"metadata"`
	Monitoring  Monitoring          `yaml:"monitoring"`
	GUI         GUI                 `yaml:"gui"`
	Performance Performance         `yaml:"performance"`
	Debug       Debug               `yaml:"debug"`
	Exchanges   ExchangesDefinition `yaml:"exchanges"`
}

// Load reads the global configuration file, resolves the active exchange profile,
// and returns a combined runtime configuration.
func Load(globalPath string) (*Config, error) {
	bytes, err := os.ReadFile(globalPath)
	if err != nil {
		return nil, fmt.Errorf("read global config: %w", err)
	}

	var globalCfg globalConfig
	if err := yaml.Unmarshal(bytes, &globalCfg); err != nil {
		return nil, fmt.Errorf("unmarshal global config: %w", err)
	}

	if globalCfg.Exchanges.Entries == nil || len(globalCfg.Exchanges.Entries) == 0 {
		return nil, fmt.Errorf("no exchanges configured in %s", globalPath)
	}

	activeExchange := globalCfg.Exchanges.Default
	if activeExchange == "" {
		for name := range globalCfg.Exchanges.Entries {
			activeExchange = name
			break
		}
	}
	exchangeSettings, ok := globalCfg.Exchanges.Entries[activeExchange]
	if !ok {
		return nil, fmt.Errorf("default exchange %q not found", activeExchange)
	}

	profileName := exchangeSettings.ActiveProfile
	if profileName == "" {
		profileName = exchangeSettings.LastUsedProfile
	}
	if profileName == "" {
		profileName = exchangeSettings.DefaultProfile
	}
	if profileName == "" {
		for name := range exchangeSettings.Profiles {
			profileName = name
			break
		}
	}
	if profileName == "" {
		return nil, fmt.Errorf("no profiles available for exchange %s", activeExchange)
	}

	profile, ok := exchangeSettings.Profiles[profileName]
	if !ok {
		return nil, fmt.Errorf("profile %q not defined for exchange %s", profileName, activeExchange)
	}
	if profile.Path == "" {
		return nil, fmt.Errorf("profile %q for exchange %s has empty path", profileName, activeExchange)
	}

	profilePath := profile.Path
	if !filepath.IsAbs(profilePath) {
		profilePath = filepath.Join(filepath.Dir(globalPath), profilePath)
	}

	profileBytes, err := os.ReadFile(profilePath)
	if err != nil {
		return nil, fmt.Errorf("read exchange profile %s: %w", profilePath, err)
	}

	var profileCfg exchangeProfileConfig
	if err := yaml.Unmarshal(profileBytes, &profileCfg); err != nil {
		return nil, fmt.Errorf("unmarshal exchange profile %s: %w", profilePath, err)
	}

	runtime := &Config{
		Application: globalCfg.Application,
		Storage:     globalCfg.Storage,
		Metadata:    globalCfg.Metadata,
		Monitoring:  globalCfg.Monitoring,
		GUI:         globalCfg.GUI,
		Performance: globalCfg.Performance,
		Debug:       globalCfg.Debug,
		WebSocket:   profileCfg.WebSocket,
		Symbols:     append([]string(nil), profileCfg.Symbols...),
		Channels:    profileCfg.Channels,

		ActiveExchange:     activeExchange,
		ActiveProfile:      profileName,
		GlobalConfigPath:   globalPath,
		ExchangeConfigPath: profilePath,
		StatePath:          filepath.Join(filepath.Dir(globalPath), "state.yml"),
		Exchanges:          globalCfg.Exchanges,
	}

	return runtime, nil
}

// Save is intentionally unsupported for the merged configuration to
// prevent accidental writes that discard profile metadata.
func (c *Config) Save(string) error {
	return fmt.Errorf("saving the merged configuration is not supported; update global and exchange profile files explicitly")
}
