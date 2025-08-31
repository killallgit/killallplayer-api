package config

import "time"

// Config represents the complete application configuration
type Config struct {
	Server       ServerConfig       `mapstructure:"server"`
	Database     DatabaseConfig     `mapstructure:"database"`
	Processing   ProcessingConfig   `mapstructure:"processing"`
	PodcastIndex PodcastIndexConfig `mapstructure:"podcast_index"`
	Whisper      WhisperConfig      `mapstructure:"whisper"`
	Storage      StorageConfig      `mapstructure:"storage"`
	Cache        CacheConfig        `mapstructure:"cache"`
	Streaming    StreamingConfig    `mapstructure:"streaming"`
	RateLimiting RateLimitConfig    `mapstructure:"rate_limiting"`
	Security     SecurityConfig     `mapstructure:"security"`
	Logging      LoggingConfig      `mapstructure:"logging"`
	Monitoring   MonitoringConfig   `mapstructure:"monitoring"`
	Features     FeaturesConfig     `mapstructure:"features"`
}

// ServerConfig contains HTTP server settings
type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
	MaxHeaderBytes  int           `mapstructure:"max_header_bytes"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Path                  string        `mapstructure:"path"`
	MaxConnections        int           `mapstructure:"max_connections"`
	MaxIdleConnections    int           `mapstructure:"max_idle_connections"`
	ConnectionMaxLifetime time.Duration `mapstructure:"connection_max_lifetime"`
	EnableWAL             bool          `mapstructure:"enable_wal"`
	EnableForeignKeys     bool          `mapstructure:"enable_foreign_keys"`
	LogQueries            bool          `mapstructure:"log_queries"`
}

// ProcessingConfig contains audio processing settings
type ProcessingConfig struct {
	Workers             int           `mapstructure:"workers"`
	MaxQueueSize        int           `mapstructure:"max_queue_size"`
	JobTimeout          time.Duration `mapstructure:"job_timeout"`
	RetryAttempts       int           `mapstructure:"retry_attempts"`
	RetryDelay          time.Duration `mapstructure:"retry_delay"`
	FFmpegPath          string        `mapstructure:"ffmpeg_path"`
	FFprobePath         string        `mapstructure:"ffprobe_path"`
	FFmpegTimeout       time.Duration `mapstructure:"ffmpeg_timeout"`
	AudiowaveformPath   string        `mapstructure:"audiowaveform_path"`
	WaveformResolutions []int         `mapstructure:"waveform_resolutions"`
	WaveformBits        int           `mapstructure:"waveform_bits"`
}

// PodcastIndexConfig contains Podcast Index API settings
type PodcastIndexConfig struct {
	APIKey        string        `mapstructure:"api_key"`
	APISecret     string        `mapstructure:"api_secret"`
	BaseURL       string        `mapstructure:"base_url"`
	Timeout       time.Duration `mapstructure:"timeout"`
	RetryAttempts int           `mapstructure:"retry_attempts"`
	RateLimit     int           `mapstructure:"rate_limit"`
	CacheTTL      time.Duration `mapstructure:"cache_ttl"`
	UserAgent     string        `mapstructure:"user_agent"`
}

// WhisperConfig contains OpenAI Whisper API settings
type WhisperConfig struct {
	APIKey        string        `mapstructure:"api_key"`
	APIURL        string        `mapstructure:"api_url"`
	Model         string        `mapstructure:"model"`
	Language      string        `mapstructure:"language"`
	Temperature   float64       `mapstructure:"temperature"`
	Timeout       time.Duration `mapstructure:"timeout"`
	MaxFileSize   int64         `mapstructure:"max_file_size"`
	ChunkDuration int           `mapstructure:"chunk_duration"`
	CostPerMinute float64       `mapstructure:"cost_per_minute"`
	MonthlyQuota  float64       `mapstructure:"monthly_quota"`
}

// StorageConfig contains storage settings
type StorageConfig struct {
	TempDir         string        `mapstructure:"temp_dir"`
	CacheDir        string        `mapstructure:"cache_dir"`
	MaxTempAge      time.Duration `mapstructure:"max_temp_age"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	MaxCacheSize    int64         `mapstructure:"max_cache_size"`
}

// CacheConfig contains cache settings
type CacheConfig struct {
	Memory MemoryCacheConfig `mapstructure:"memory"`
	API    APICacheConfig    `mapstructure:"api"`
}

// MemoryCacheConfig contains in-memory cache settings
type MemoryCacheConfig struct {
	DefaultTTL      time.Duration `mapstructure:"default_ttl"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
	MaxEntries      int           `mapstructure:"max_entries"`
}

// APICacheConfig contains API response cache settings
type APICacheConfig struct {
	SearchTTL  time.Duration `mapstructure:"search_ttl"`
	PodcastTTL time.Duration `mapstructure:"podcast_ttl"`
	EpisodeTTL time.Duration `mapstructure:"episode_ttl"`
}

// StreamingConfig contains audio streaming settings
type StreamingConfig struct {
	BufferSize           int   `mapstructure:"buffer_size"`
	EnableRangeRequests  bool  `mapstructure:"enable_range_requests"`
	MaxConcurrentStreams int   `mapstructure:"max_concurrent_streams"`
	BandwidthLimit       int64 `mapstructure:"bandwidth_limit"`
}

// RateLimitConfig contains rate limiting settings
type RateLimitConfig struct {
	Enabled   bool           `mapstructure:"enabled"`
	Endpoints map[string]int `mapstructure:"endpoints"`
}

// SecurityConfig contains security settings
type SecurityConfig struct {
	EnableCORS      bool     `mapstructure:"enable_cors"`
	CORSOrigins     []string `mapstructure:"cors_origins"`
	CORSMethods     []string `mapstructure:"cors_methods"`
	CORSHeaders     []string `mapstructure:"cors_headers"`
	EnableRequestID bool     `mapstructure:"enable_request_id"`
	EnableRecovery  bool     `mapstructure:"enable_recovery"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level            string `mapstructure:"level"`
	Format           string `mapstructure:"format"`
	Output           string `mapstructure:"output"`
	FilePath         string `mapstructure:"file_path"`
	MaxSize          int    `mapstructure:"max_size"`
	MaxBackups       int    `mapstructure:"max_backups"`
	MaxAge           int    `mapstructure:"max_age"`
	Compress         bool   `mapstructure:"compress"`
	EnableCaller     bool   `mapstructure:"enable_caller"`
	EnableStacktrace bool   `mapstructure:"enable_stacktrace"`
}

// MonitoringConfig contains monitoring settings
type MonitoringConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	MetricsPath  string `mapstructure:"metrics_path"`
	HealthPath   string `mapstructure:"health_path"`
	PProfEnabled bool   `mapstructure:"pprof_enabled"`
	PProfPath    string `mapstructure:"pprof_path"`
}

// FeaturesConfig contains feature flags
type FeaturesConfig struct {
	EnableTranscription bool `mapstructure:"enable_transcription"`
	EnableWaveform      bool `mapstructure:"enable_waveform"`
	EnableTagging       bool `mapstructure:"enable_tagging"`
	EnableCaching       bool `mapstructure:"enable_caching"`
	MaintenanceMode     bool `mapstructure:"maintenance_mode"`
}
