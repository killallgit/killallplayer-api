package config

import (
	"os"
	"testing"

	"github.com/spf13/viper"
)

func TestConfig(t *testing.T) {
	tests := []struct {
		name    string
		setup   func()
		cleanup func()
		wantErr bool
		check   func(t *testing.T)
	}{
		{
			name: "load from config.yaml",
			setup: func() {
				// Reset viper for clean test
				viper.Reset()
				// Create config file in current directory
				content := `
server:
  host: "127.0.0.1"
  port: 8080
database:
  path: "./test.db"
`
				_ = os.WriteFile("./config.yaml", []byte(content), 0644)
			},
			cleanup: func() {
				_ = os.Remove("./config.yaml")
				viper.Reset()
			},
			wantErr: false,
			check: func(t *testing.T) {
				if GetInt("server.port") != 8080 {
					t.Errorf("Expected server.port to be 8080, got %d", GetInt("server.port"))
				}
			},
		},
		{
			name: "environment variable override",
			setup: func() {
				// Reset viper for clean test
				viper.Reset()
				// Create config file
				content := `
server:
  host: "127.0.0.1"
  port: 8080
`
				_ = os.WriteFile("./config.yaml", []byte(content), 0644)
				os.Setenv("KILLALL_SERVER_PORT", "9090")
			},
			cleanup: func() {
				_ = os.Remove("./config.yaml")
				os.Unsetenv("KILLALL_SERVER_PORT")
				viper.Reset()
			},
			wantErr: false,
			check: func(t *testing.T) {
				if GetInt("server.port") != 9090 {
					t.Errorf("Expected server.port to be overridden to 9090, got %d", GetInt("server.port"))
				}
			},
		},
		{
			name: "missing config file with defaults",
			setup: func() {
				// Reset viper for clean test
				viper.Reset()
				// No config file created
			},
			cleanup: func() {
				viper.Reset()
			},
			wantErr: false,
			check: func(t *testing.T) {
				// Should use defaults
				if GetInt("server.port") != 8080 {
					t.Errorf("Expected default server.port to be 8080, got %d", GetInt("server.port"))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.cleanup()

			err := Init()
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.check != nil && err == nil {
				tt.check(t)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				Database: DatabaseConfig{
					Path: "./data/podcast.db",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid port",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: 0,
				},
			},
			wantErr: true,
		},
		{
			name: "empty database path (now allowed)",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				Database: DatabaseConfig{
					Path: "",
				},
			},
			wantErr: false, // Database is now optional
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.config.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
