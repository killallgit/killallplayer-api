package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Load loads configuration from ./config/settings.yaml
func Load() (*Config, error) {
	v := viper.New()

	// Set default values
	setDefaults(v)

	// Set up environment variable reading for overrides
	v.SetEnvPrefix("KILLALL")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Load config from fixed location
	configPath := "./config/settings.yaml"
	v.SetConfigFile(configPath)
	
	// Try to read the config file
	if err := v.ReadInConfig(); err != nil {
		// If the config file doesn't exist, just use defaults and env vars
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("error reading config file %s: %w", configPath, err)
		}
		// Config file doesn't exist, which is fine - we'll use defaults
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Validate the configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	if c.Database.Path == "" {
		return fmt.Errorf("database path cannot be empty")
	}

	if c.Processing.Workers <= 0 {
		c.Processing.Workers = 2
	}

	if c.Processing.MaxQueueSize <= 0 {
		c.Processing.MaxQueueSize = 100
	}

	return nil
}

// setDefaults sets default configuration values
func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.read_timeout", 30*time.Second)
	v.SetDefault("server.write_timeout", 30*time.Second)
	v.SetDefault("server.shutdown_timeout", 10*time.Second)
	v.SetDefault("server.max_header_bytes", 1048576)

	// WebSocket defaults
	v.SetDefault("websocket.heartbeat_interval", 30*time.Second)
	v.SetDefault("websocket.max_message_size", 10485760)
	v.SetDefault("websocket.read_buffer_size", 1024)
	v.SetDefault("websocket.write_buffer_size", 1024)
	v.SetDefault("websocket.handshake_timeout", 10*time.Second)
	v.SetDefault("websocket.enable_compression", true)

	// Database defaults
	v.SetDefault("database.path", "./data/podcast.db")
	v.SetDefault("database.max_connections", 10)
	v.SetDefault("database.max_idle_connections", 5)
	v.SetDefault("database.connection_max_lifetime", 30*time.Minute)
	v.SetDefault("database.enable_wal", true)
	v.SetDefault("database.enable_foreign_keys", true)
	v.SetDefault("database.log_queries", false)

	// Processing defaults
	v.SetDefault("processing.workers", 2)
	v.SetDefault("processing.max_queue_size", 100)
	v.SetDefault("processing.job_timeout", 30*time.Minute)
	v.SetDefault("processing.retry_attempts", 3)
	v.SetDefault("processing.retry_delay", 5*time.Second)
	v.SetDefault("processing.ffmpeg_path", "/usr/local/bin/ffmpeg")
	v.SetDefault("processing.ffprobe_path", "/usr/local/bin/ffprobe")
	v.SetDefault("processing.ffmpeg_timeout", 5*time.Minute)
	v.SetDefault("processing.audiowaveform_path", "/usr/local/bin/audiowaveform")
	v.SetDefault("processing.waveform_resolutions", []int{256, 512, 1024})
	v.SetDefault("processing.waveform_bits", 8)

	// Podcast Index defaults
	v.SetDefault("podcast_index.base_url", "https://api.podcastindex.org/api/1.0")
	v.SetDefault("podcast_index.timeout", 10*time.Second)
	v.SetDefault("podcast_index.retry_attempts", 3)
	v.SetDefault("podcast_index.rate_limit", 10)
	v.SetDefault("podcast_index.cache_ttl", 1*time.Hour)
	v.SetDefault("podcast_index.user_agent", "PodcastPlayerAPI/1.0")

	// Whisper defaults
	v.SetDefault("whisper.api_url", "https://api.openai.com/v1/audio/transcriptions")
	v.SetDefault("whisper.model", "whisper-1")
	v.SetDefault("whisper.temperature", 0)
	v.SetDefault("whisper.timeout", 5*time.Minute)
	v.SetDefault("whisper.max_file_size", 26214400)
	v.SetDefault("whisper.chunk_duration", 600)
	v.SetDefault("whisper.cost_per_minute", 0.006)
	v.SetDefault("whisper.monthly_quota", 100.0)

	// Storage defaults
	v.SetDefault("storage.temp_dir", "./tmp")
	v.SetDefault("storage.cache_dir", "./cache")
	v.SetDefault("storage.max_temp_age", 24*time.Hour)
	v.SetDefault("storage.cleanup_interval", 1*time.Hour)
	v.SetDefault("storage.max_cache_size", 10737418240)

	// Cache defaults
	v.SetDefault("cache.memory.default_ttl", 10*time.Minute)
	v.SetDefault("cache.memory.cleanup_interval", 5*time.Minute)
	v.SetDefault("cache.memory.max_entries", 1000)
	v.SetDefault("cache.api.search_ttl", 1*time.Hour)
	v.SetDefault("cache.api.podcast_ttl", 24*time.Hour)
	v.SetDefault("cache.api.episode_ttl", 24*time.Hour)

	// Streaming defaults
	v.SetDefault("streaming.buffer_size", 32768)
	v.SetDefault("streaming.enable_range_requests", true)
	v.SetDefault("streaming.max_concurrent_streams", 10)
	v.SetDefault("streaming.bandwidth_limit", 0)

	// Rate limiting defaults
	v.SetDefault("rate_limiting.enabled", true)
	v.SetDefault("rate_limiting.endpoints", map[string]int{
		"search":     60,
		"stream":     100,
		"processing": 10,
		"websocket":  100,
		"default":    120,
	})

	// Security defaults
	v.SetDefault("security.enable_cors", true)
	v.SetDefault("security.cors_origins", []string{"*"})
	v.SetDefault("security.cors_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	v.SetDefault("security.cors_headers", []string{"Content-Type", "Authorization", "Range"})
	v.SetDefault("security.enable_request_id", true)
	v.SetDefault("security.enable_recovery", true)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "stdout")
	v.SetDefault("logging.file_path", "./logs/app.log")
	v.SetDefault("logging.max_size", 100)
	v.SetDefault("logging.max_backups", 10)
	v.SetDefault("logging.max_age", 30)
	v.SetDefault("logging.compress", true)
	v.SetDefault("logging.enable_caller", false)
	v.SetDefault("logging.enable_stacktrace", true)

	// Monitoring defaults
	v.SetDefault("monitoring.enabled", false)
	v.SetDefault("monitoring.metrics_path", "/metrics")
	v.SetDefault("monitoring.health_path", "/health")
	v.SetDefault("monitoring.pprof_enabled", false)
	v.SetDefault("monitoring.pprof_path", "/debug/pprof")

	// Features defaults
	v.SetDefault("features.enable_transcription", true)
	v.SetDefault("features.enable_waveform", true)
	v.SetDefault("features.enable_tagging", true)
	v.SetDefault("features.enable_caching", true)
	v.SetDefault("features.enable_websocket", true)
	v.SetDefault("features.maintenance_mode", false)
}
