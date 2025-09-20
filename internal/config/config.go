package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Application Application `yaml:"application"`
	WebSocket   WebSocket   `yaml:"websocket"`
	Symbols     []string    `yaml:"symbols"`
	Channels    Channels    `yaml:"channels"`
	Storage     Storage     `yaml:"storage"`
	Metadata    Metadata    `yaml:"metadata"`
	Monitoring  Monitoring  `yaml:"monitoring"`
	GUI         GUI         `yaml:"gui"`
	Performance Performance `yaml:"performance"`
	Debug       Debug       `yaml:"debug"`
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
	Enabled          bool `yaml:"enabled"`
	Compression      string `yaml:"compression"`
	RetentionHours   int `yaml:"retention_hours"`
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
	Enabled bool `yaml:"enabled"`
	Port    int  `yaml:"port"`
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
	BufferSize      int               `yaml:"buffer_size"`
	WorkerCount     int               `yaml:"worker_count"`
	MaxMemoryMB     int               `yaml:"max_memory_mb"`
	GCInterval      time.Duration     `yaml:"gc_interval"`
	CircuitBreaker  CircuitBreakerConfig `yaml:"circuit_breaker"`
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

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}