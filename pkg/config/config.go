package config

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Init initializes the configuration system using standard Viper practices
func Init() error {
	// Set defaults
	setDefaults()

	// Set config name and paths
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config/")
	viper.AddConfigPath("$HOME/.killall/")
	viper.AddConfigPath("/etc/killall/")

	// Environment variables
	viper.SetEnvPrefix("KILLALL")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			fmt.Println("No config file found, using defaults and environment variables")
		} else {
			// Config file was found but another error was produced
			return fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Validate configuration
	return validateConfig()
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

// Global configuration accessor functions (for backward compatibility)

// Get returns a config value by key
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

// validateConfig validates the configuration
func validateConfig() error {
	// Check if we're in test mode to suppress warnings
	isTestMode := os.Getenv("GO_TEST_MODE") == "1" || strings.Contains(os.Args[0], ".test")

	port := viper.GetInt("server.port")
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid server.port %d, must be between 1-65535", port)
	}

	dbPath := viper.GetString("database.path")
	if dbPath == "" {
		// Database is optional, so we don't return an error
		// but we log a warning unless in test mode
		if !isTestMode {
			fmt.Println("Warning: No database path configured")
		}
	}

	// Validate API keys aren't using placeholder values
	if err := validateAPIKeys(isTestMode); err != nil {
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
func validateAPIKeys(isTestMode bool) error {
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
				return fmt.Errorf("podcast_index.api_key cannot use placeholder values in production")
			}
			if !isTestMode {
				fmt.Println("Warning: Podcast Index API credentials are using placeholder values")
			}
			break
		}
	}

	// Check OpenAI API key
	openaiKey := viper.GetString("ai.openai_api_key")
	for _, placeholder := range placeholders {
		if openaiKey == placeholder {
			if isProduction {
				return fmt.Errorf("ai.openai_api_key cannot use placeholder values in production")
			}
			if !isTestMode {
				fmt.Println("Warning: OpenAI API key is using a placeholder value")
			}
			break
		}
	}

	// Check JWT secret
	jwtSecret := viper.GetString("auth.jwt_secret")
	for _, placeholder := range placeholders {
		if jwtSecret == placeholder {
			if isProduction {
				return fmt.Errorf("auth.jwt_secret cannot use placeholder values in production")
			}
			if !isTestMode {
				fmt.Println("Warning: JWT secret is using a placeholder value - this is insecure!")
			}
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

	// Database is optional - no validation needed for empty path

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
	// Core server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.shutdown_timeout", "10s")
	viper.SetDefault("server.read_timeout", "30s")
	viper.SetDefault("server.write_timeout", "30s")

	// Database defaults
	viper.SetDefault("database.path", "./data/podcast.db")
	viper.SetDefault("database.verbose", false)

	// Podcast Index API defaults
	viper.SetDefault("podcast_index.api_url", "https://api.podcastindex.org/api/1.0")
	viper.SetDefault("podcast_index.timeout", "30s")
	viper.SetDefault("podcast_index.user_agent", "PodcastPlayerAPI/1.0")

	// Episode service defaults
	viper.SetDefault("episodes.max_concurrent_sync", 5)
	viper.SetDefault("episodes.sync_timeout", "30s")

	// Security defaults
	viper.SetDefault("security.cors_enabled", true)
	viper.SetDefault("security.cors_origins", "*")
	viper.SetDefault("security.cors_methods", "GET,POST,PUT,DELETE,OPTIONS")
	viper.SetDefault("security.cors_headers", "Content-Type,Authorization")
	viper.SetDefault("security.rate_limit_enabled", true)
	viper.SetDefault("security.rate_limit_rps", 10)
	viper.SetDefault("security.rate_limit_burst", 20)

	// Processing defaults
	viper.SetDefault("processing.workers", 2)
	viper.SetDefault("processing.max_queue_size", 100)
	viper.SetDefault("processing.timeout", "5m")

	// Cache defaults
	viper.SetDefault("cache.type", "memory")
	viper.SetDefault("cache.ttl", "1h")
	viper.SetDefault("cache.max_size", 100)
	viper.SetDefault("cache.cleanup_interval", "10m")

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
	viper.SetDefault("logging.output", "stdout")
}
