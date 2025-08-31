package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

var (
	once    sync.Once
	initErr error
)

// Init initializes the configuration system
// This should be called once at application startup
func Init() error {
	once.Do(func() {
		// Set default values
		setDefaults()

		// Set up environment variable reading for overrides
		viper.SetEnvPrefix("KILLALL")
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
		viper.AutomaticEnv()

		// Load config from fixed location (cleaned for safety)
		configPath := filepath.Clean("./config/settings.yaml")
		viper.SetConfigFile(configPath)

		// Try to read the config file
		if err := viper.ReadInConfig(); err != nil {
			// If the config file doesn't exist, just use defaults and env vars
			if !os.IsNotExist(err) {
				initErr = fmt.Errorf("error reading config file %s: %w", configPath, err)
				return
			}
			// Config file doesn't exist, which is fine - we'll use defaults
		}

		// Validate the configuration
		if err := validate(); err != nil {
			initErr = fmt.Errorf("invalid configuration: %w", err)
		}
	})

	return initErr
}

// Load loads configuration from ./config/settings.yaml
// Deprecated: Use Init() instead for better control
func Load() (*Config, error) {
	if err := Init(); err != nil {
		return nil, err
	}
	return GetConfig()
}

// GetConfig returns the current configuration as a struct
func GetConfig() (*Config, error) {
	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}
	return &config, nil
}

// Get returns a config value by key using Viper directly
func Get(key string) any {
	return viper.Get(key)
}

// GetString returns a string config value
func GetString(key string) string {
	return viper.GetString(key)
}

// GetInt returns an int config value
func GetInt(key string) int {
	return viper.GetInt(key)
}

// GetBool returns a bool config value
func GetBool(key string) bool {
	return viper.GetBool(key)
}

// GetDuration returns a duration config value
func GetDuration(key string) time.Duration {
	return viper.GetDuration(key)
}

// validate validates the configuration using Viper values
func validate() error {
	port := viper.GetInt("server.port")
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid server port: %d", port)
	}

	dbPath := viper.GetString("database.path")
	if dbPath == "" {
		// Database is optional, so we don't return an error
		// but we log a warning
		fmt.Println("Warning: No database path configured")
	}

	// Validate API keys aren't using placeholder values
	if err := validateAPIKeys(); err != nil {
		return err
	}

	// Auto-correct invalid worker count
	if viper.GetInt("processing.workers") <= 0 {
		viper.Set("processing.workers", 2)
	}

	// Auto-correct invalid queue size
	if viper.GetInt("processing.max_queue_size") <= 0 {
		viper.Set("processing.max_queue_size", 100)
	}

	return nil
}

// validateAPIKeys validates that API keys are not using placeholder values
func validateAPIKeys() error {
	// Check for production environment
	env := viper.GetString("environment")
	isProduction := env == "production" || env == "prod"

	// List of placeholder values that shouldn't be used
	placeholders := []string{
		"YOUR_KEY_HERE",
		"YOUR_SECRET_HERE",
		"YOUR_API_KEY",
		"YOUR_API_SECRET",
		"changeme",
		"CHANGEME",
		"",
	}

	// Check Podcast Index API credentials
	podcastKey := viper.GetString("podcast_index.api_key")
	podcastSecret := viper.GetString("podcast_index.api_secret")

	for _, placeholder := range placeholders {
		if podcastKey == placeholder || podcastSecret == placeholder {
			if isProduction {
				return fmt.Errorf("invalid Podcast Index API credentials: cannot use placeholder values in production")
			}
			fmt.Println("Warning: Podcast Index API credentials are using placeholder values")
			break
		}
	}

	// Check OpenAI API key
	openaiKey := viper.GetString("ai.openai_api_key")
	for _, placeholder := range placeholders {
		if openaiKey == placeholder {
			if isProduction {
				return fmt.Errorf("invalid OpenAI API key: cannot use placeholder values in production")
			}
			fmt.Println("Warning: OpenAI API key is using a placeholder value")
			break
		}
	}

	// Check JWT secret
	jwtSecret := viper.GetString("auth.jwt_secret")
	for _, placeholder := range placeholders {
		if jwtSecret == placeholder {
			if isProduction {
				return fmt.Errorf("invalid JWT secret: cannot use placeholder values in production")
			}
			fmt.Println("Warning: JWT secret is using a placeholder value - this is insecure!")
			break
		}
	}

	return nil
}

// Validate validates a Config struct (for testing)
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", c.Server.Port)
	}

	// Database is optional
	if c.Database.Path == "" {
		// Just log a warning in the struct validation too
		// This is mainly used for testing
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
func setDefaults() {
	// Environment defaults
	viper.SetDefault("environment", "development")

	// Server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.read_timeout", 30*time.Second)
	viper.SetDefault("server.write_timeout", 30*time.Second)
	viper.SetDefault("server.shutdown_timeout", 10*time.Second)
	viper.SetDefault("server.max_header_bytes", 1048576)

	// WebSocket defaults
	viper.SetDefault("websocket.heartbeat_interval", 30*time.Second)
	viper.SetDefault("websocket.max_message_size", 10485760)
	viper.SetDefault("websocket.read_buffer_size", 1024)
	viper.SetDefault("websocket.write_buffer_size", 1024)
	viper.SetDefault("websocket.handshake_timeout", 10*time.Second)
	viper.SetDefault("websocket.enable_compression", true)

	// Database defaults
	viper.SetDefault("database.path", "./data/podcast.db")
	viper.SetDefault("database.max_connections", 10)
	viper.SetDefault("database.max_idle_connections", 5)
	viper.SetDefault("database.connection_max_lifetime", 30*time.Minute)
	viper.SetDefault("database.enable_wal", true)
	viper.SetDefault("database.enable_foreign_keys", true)
	viper.SetDefault("database.log_queries", false)
	viper.SetDefault("database.verbose", false)

	// Processing defaults
	viper.SetDefault("processing.workers", 2)
	viper.SetDefault("processing.max_queue_size", 100)
	viper.SetDefault("processing.batch_size", 10)
	viper.SetDefault("processing.timeout", 5*time.Minute)
	viper.SetDefault("processing.retry_attempts", 3)
	viper.SetDefault("processing.retry_delay", 1*time.Second)

	// Podcast Index API defaults
	viper.SetDefault("podcast_index.api_key", "YOUR_KEY_HERE")
	viper.SetDefault("podcast_index.api_secret", "YOUR_SECRET_HERE")
	viper.SetDefault("podcast_index.api_url", "https://api.podcastindex.org/api/1.0")
	viper.SetDefault("podcast_index.timeout", 30*time.Second)
	viper.SetDefault("podcast_index.max_retries", 3)

	// AI defaults
	viper.SetDefault("ai.provider", "openai")
	viper.SetDefault("ai.openai_api_key", "YOUR_API_KEY")
	viper.SetDefault("ai.openai_model", "gpt-3.5-turbo")
	viper.SetDefault("ai.openai_max_tokens", 2000)
	viper.SetDefault("ai.openai_temperature", 0.7)
	viper.SetDefault("ai.request_timeout", 60*time.Second)

	// Transcription defaults
	viper.SetDefault("transcription.service", "whisper")
	viper.SetDefault("transcription.whisper_model", "whisper-1")
	viper.SetDefault("transcription.whisper_language", "en")
	viper.SetDefault("transcription.max_file_size", 25*1024*1024)
	viper.SetDefault("transcription.allowed_formats", []string{"mp3", "m4a", "wav", "ogg", "flac"})
	viper.SetDefault("transcription.output_format", "json")
	viper.SetDefault("transcription.enable_timestamps", true)
	viper.SetDefault("transcription.monthly_quota", 100.0)

	// FFmpeg defaults
	viper.SetDefault("ffmpeg.path", "ffmpeg")
	viper.SetDefault("ffmpeg.ffprobe_path", "ffprobe")
	viper.SetDefault("ffmpeg.max_processes", 5)
	viper.SetDefault("ffmpeg.timeout", 5*time.Minute)
	viper.SetDefault("ffmpeg.hardware_accel", "auto")
	viper.SetDefault("ffmpeg.output_format", "mp3")
	viper.SetDefault("ffmpeg.audio_codec", "libmp3lame")
	viper.SetDefault("ffmpeg.audio_bitrate", "128k")
	viper.SetDefault("ffmpeg.audio_sample_rate", 44100)
	viper.SetDefault("ffmpeg.audio_channels", 2)

	// Cache defaults
	viper.SetDefault("cache.type", "memory")
	viper.SetDefault("cache.ttl", 1*time.Hour)
	viper.SetDefault("cache.max_size", 100)
	viper.SetDefault("cache.cleanup_interval", 10*time.Minute)
	viper.SetDefault("cache.redis_addr", "localhost:6379")
	viper.SetDefault("cache.redis_password", "")
	viper.SetDefault("cache.redis_db", 0)
	viper.SetDefault("cache.redis_pool_size", 10)

	// Storage defaults
	viper.SetDefault("storage.type", "local")
	viper.SetDefault("storage.local_path", "./storage")
	viper.SetDefault("storage.temp_dir", "./tmp")
	viper.SetDefault("storage.max_file_size", 100*1024*1024)
	viper.SetDefault("storage.allowed_extensions", []string{"mp3", "m4a", "wav", "ogg", "flac", "aac"})
	viper.SetDefault("storage.s3_bucket", "")
	viper.SetDefault("storage.s3_region", "us-east-1")
	viper.SetDefault("storage.s3_access_key", "")
	viper.SetDefault("storage.s3_secret_key", "")
	viper.SetDefault("storage.s3_endpoint", "")

	// Security defaults
	viper.SetDefault("security.cors_enabled", true)
	viper.SetDefault("security.cors_origins", []string{"*"})
	viper.SetDefault("security.cors_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("security.cors_headers", []string{"Content-Type", "Authorization"})
	viper.SetDefault("security.cors_credentials", false)
	viper.SetDefault("security.cors_max_age", 86400)
	viper.SetDefault("security.rate_limit_enabled", true)
	viper.SetDefault("security.rate_limit_rps", 10)
	viper.SetDefault("security.rate_limit_burst", 20)
	viper.SetDefault("security.api_key_header", "X-API-Key")
	viper.SetDefault("security.api_key_required", false)

	// Auth defaults
	viper.SetDefault("auth.jwt_secret", "changeme")
	viper.SetDefault("auth.jwt_expiry", 24*time.Hour)
	viper.SetDefault("auth.jwt_refresh_expiry", 7*24*time.Hour)
	viper.SetDefault("auth.bcrypt_cost", 10)
	viper.SetDefault("auth.session_name", "podcast_session")
	viper.SetDefault("auth.session_lifetime", 24*time.Hour)
	viper.SetDefault("auth.oauth_google_client_id", "")
	viper.SetDefault("auth.oauth_google_client_secret", "")
	viper.SetDefault("auth.oauth_github_client_id", "")
	viper.SetDefault("auth.oauth_github_client_secret", "")

	// Monitoring defaults
	viper.SetDefault("monitoring.enabled", false)
	viper.SetDefault("monitoring.metrics_enabled", false)
	viper.SetDefault("monitoring.metrics_path", "/metrics")
	viper.SetDefault("monitoring.health_path", "/health")
	viper.SetDefault("monitoring.pprof_enabled", false)
	viper.SetDefault("monitoring.pprof_path", "/debug/pprof")
	viper.SetDefault("monitoring.tracing_enabled", false)
	viper.SetDefault("monitoring.tracing_service_name", "podcast-api")
	viper.SetDefault("monitoring.tracing_jaeger_endpoint", "http://localhost:14268/api/traces")

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.file_path", "./logs/app.log")
	viper.SetDefault("logging.file_max_size", 100)
	viper.SetDefault("logging.file_max_backups", 7)
	viper.SetDefault("logging.file_max_age", 30)
	viper.SetDefault("logging.enable_console", true)
	viper.SetDefault("logging.enable_file", false)

	// Feature flags
	viper.SetDefault("features.enable_transcription", true)
	viper.SetDefault("features.enable_waveform", true)
	viper.SetDefault("features.enable_chapters", true)
	viper.SetDefault("features.enable_tagging", true)
	viper.SetDefault("features.enable_search", true)
	viper.SetDefault("features.enable_recommendations", false)
	viper.SetDefault("features.enable_social", false)
	viper.SetDefault("features.enable_analytics", false)
	viper.SetDefault("features.maintenance_mode", false)
	viper.SetDefault("features.beta_features", false)
}
