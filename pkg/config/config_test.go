package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		setup   func()
		cleanup func()
		wantErr bool
	}{
		{
			name: "load from settings.yaml",
			setup: func() {
				// Create config directory
				_ = os.Mkdir("config", 0755)
				content := `
server:
  host: "127.0.0.1"
  port: 8080
database:
  path: "./test.db"
`
				_ = os.WriteFile("./config/settings.yaml", []byte(content), 0644)
			},
			cleanup: func() {
				os.Remove("./config/settings.yaml")
				os.Remove("config")
			},
			wantErr: false,
		},
		{
			name: "environment variable override",
			setup: func() {
				// Create config directory
				_ = os.Mkdir("config", 0755)
				content := `
server:
  host: "127.0.0.1"
  port: 8080
`
				_ = os.WriteFile("./config/settings.yaml", []byte(content), 0644)
				os.Setenv("PLAYER_API_SERVER_PORT", "9090")
			},
			cleanup: func() {
				os.Remove("./config/settings.yaml")
				os.Remove("config")
				os.Unsetenv("PLAYER_API_SERVER_PORT")
			},
			wantErr: false,
		},
		{
			name:    "missing config file with defaults",
			setup:   func() {},
			cleanup: func() {},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.cleanup()

			_, err := Load()
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
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
			name: "valid configuration",
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
			name: "empty database path",
			config: &Config{
				Server: ServerConfig{
					Host: "localhost",
					Port: 8080,
				},
				Database: DatabaseConfig{
					Path: "",
				},
			},
			wantErr: true,
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

