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

		// Load config from multiple possible locations
		// First try environment variable, then working directory, then defaults
		configPath := os.Getenv("KILLALL_CONFIG_PATH")
		if configPath == "" {
			configPath = "./config/settings.yaml"
		}
		configPath = filepath.Clean(configPath)
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
	// Check if we're in test mode to suppress warnings
	isTestMode := os.Getenv("GO_TEST_MODE") == "1" || strings.Contains(os.Args[0], ".test")

	port := viper.GetInt("server.port")
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid server port: %d", port)
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
				return fmt.Errorf("invalid Podcast Index API credentials: cannot use placeholder values in production")
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
				return fmt.Errorf("invalid OpenAI API key: cannot use placeholder values in production")
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
				return fmt.Errorf("invalid JWT secret: cannot use placeholder values in production")
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

// setDefaults sets default configuration values for actually used features
func setDefaults() {
	// Core server defaults
	viper.SetDefault("server.host", "0.0.0.0")
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.shutdown_timeout", 10*time.Second)

	// Database defaults - optional but used
	viper.SetDefault("database.path", "./data/podcast.db")
	viper.SetDefault("database.verbose", false)

	// Podcast Index API defaults - required for core functionality
	viper.SetDefault("podcast_index.api_url", "https://api.podcastindex.org/api/1.0")
	viper.SetDefault("podcast_index.timeout", 30*time.Second)
	// API credentials should be provided via environment or config file

	// Episode service defaults
	viper.SetDefault("episodes.max_concurrent_sync", 5)
	viper.SetDefault("episodes.sync_timeout", 30*time.Second)

	// Basic security defaults that are actually used
	viper.SetDefault("security.cors_enabled", true)
	viper.SetDefault("security.cors_origins", []string{"*"})
	viper.SetDefault("security.cors_methods", []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"})
	viper.SetDefault("security.cors_headers", []string{"Content-Type", "Authorization"})
	viper.SetDefault("security.rate_limit_enabled", true)
	viper.SetDefault("security.rate_limit_rps", 10)
	viper.SetDefault("security.rate_limit_burst", 20)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")
}
