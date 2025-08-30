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
	once sync.Once
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

// GetConfig returns the current configuration as a struct
// Init() must be called before using this
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

// GetDuration returns a time.Duration config value
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
	viper.SetDefault("processing.job_timeout", 30*time.Minute)
	viper.SetDefault("processing.retry_attempts", 3)
	viper.SetDefault("processing.retry_delay", 5*time.Second)
	viper.SetDefault("processing.ffmpeg_path", "/usr/local/bin/ffmpeg")
	viper.SetDefault("processing.ffprobe_path", "/usr/local/bin/ffprobe")
	viper.SetDefault("processing.ffmpeg_timeout", 5*time.Minute)
	viper.SetDefault("processing.audiowaveform_path", "/usr/local/bin/audiowaveform")
	viper.SetDefault("processing.waveform_resolutions", []int{256, 512, 1024})
	viper.SetDefault("processing.waveform_bits", 8)

	// Podcast Index defaults
	viper.SetDefault("podcast_index.base_url", "https://api.podcastindex.org/api/1.0")
	viper.SetDefault("podcast_index.timeout", 10*time.Second)
	viper.SetDefault("podcast_index.retry_attempts", 3)
	viper.SetDefault("podcast_index.rate_limit", 10)
	viper.SetDefault("podcast_index.cache_ttl", 1*time.Hour)
	viper.SetDefault("podcast_index.user_agent", "PodcastPlayerAPI/1.0")

	// Whisper defaults
	viper.SetDefault("whisper.api_url", "https://api.openai.com/v1/audio/transcriptions")
	viper.SetDefault("whisper.model", "whisper-1")
	viper.SetDefault("whisper.temperature", 0)
	viper.SetDefault("whisper.timeout", 5*time.Minute)
	viper.SetDefault("whisper.max_file_size", 26214400)
	viper.SetDefault("whisper.chunk_duration", 600)
	viper.SetDefault("whisper.cost_per_minute", 0.006)
	viper.SetDefault("whisper.monthly_quota", 100.0)

	// Storage defaults
	viper.SetDefault("storage.temp_dir", "./tmp")
	viper.SetDefault("storage.cache_dir", "./cache")
	viper.SetDefault("storage.max_temp_age", 24*time.Hour)
	viper.SetDefault("storage.cleanup_interval", 1*time.Hour)
	viper.SetDefault("storage.max_cache_size", 10737418240)

	// Cache defaults
	viper.SetDefault("cache.memory.default_ttl", 10*time.Minute)
	viper.SetDefault("cache.memory.cleanup_interval", 5*time.Minute)
	viper.SetDefault("cache.memory.max_entries", 1000)
	viper.SetDefault("cache.api.search_ttl", 1*time.Hour)
	viper.SetDefault("cache.api.podcast_ttl", 24*time.Hour)
	viper.SetDefault("cache.api.episode_ttl", 24*time.Hour)

	// Streaming defaults
	viper.SetDefault("streaming.buffer_size", 32768)
	viper.SetDefault("streaming.enable_range_requests", true)
	viper.SetDefault("streaming.max_concurrent_streams", 10)
	viper.SetDefault("streaming.bandwidth_limit", 0)

	// Rate limiting defaults
	viper.SetDefault("rate_limiting.enabled", true)
	viper.SetDefault("rate_limiting.endpoints", map[string]int{
		"search":     60,
		"stream":     100,
		"processing": 10,
		"websocket":  100,
		"default":    120,
	})

	// Security defaults
	viper.SetDefault("security.enable_cors", true)
	viper.SetDefault("security.cors_origins", []string{"*"})
	viper.SetDefault("security.cors_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("security.cors_headers", []string{"Content-Type", "Authorization", "Range"})
	viper.SetDefault("security.enable_request_id", true)
	viper.SetDefault("security.enable_recovery", true)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
	viper.SetDefault("logging.file_path", "./logs/app.log")
	viper.SetDefault("logging.max_size", 100)
	viper.SetDefault("logging.max_backups", 10)
	viper.SetDefault("logging.max_age", 30)
	viper.SetDefault("logging.compress", true)
	viper.SetDefault("logging.enable_caller", false)
	viper.SetDefault("logging.enable_stacktrace", true)

	// Monitoring defaults
	viper.SetDefault("monitoring.enabled", false)
	viper.SetDefault("monitoring.metrics_path", "/metrics")
	viper.SetDefault("monitoring.health_path", "/health")
	viper.SetDefault("monitoring.pprof_enabled", false)
	viper.SetDefault("monitoring.pprof_path", "/debug/pprof")

	// Features defaults
	viper.SetDefault("features.enable_transcription", true)
	viper.SetDefault("features.enable_waveform", true)
	viper.SetDefault("features.enable_tagging", true)
	viper.SetDefault("features.enable_caching", true)
	viper.SetDefault("features.enable_websocket", true)
	viper.SetDefault("features.maintenance_mode", false)
}
