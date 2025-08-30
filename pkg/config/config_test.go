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
			name: "load from valid config file",
			setup: func() {
				content := `
server:
  host: "127.0.0.1"
  port: 8080
database:
  path: "./test.db"
`
				_ = os.WriteFile("test_config.yaml", []byte(content), 0644)
			},
			cleanup: func() {
				os.Remove("test_config.yaml")
			},
			wantErr: false,
		},
		{
			name: "environment variable override",
			setup: func() {
				content := `
server:
  host: "127.0.0.1"
  port: 8080
`
				_ = os.WriteFile("test_config.yaml", []byte(content), 0644)
				os.Setenv("SERVER_PORT", "9090")
			},
			cleanup: func() {
				os.Remove("test_config.yaml")
				os.Unsetenv("SERVER_PORT")
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

			configPath := "test_config.yaml"
			if tt.name == "missing config file with defaults" {
				configPath = "nonexistent.yaml"
			}

			_, err := Load(configPath)
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

func TestGetConfigPath(t *testing.T) {
	tests := []struct {
		name    string
		paths   []string
		setup   func()
		cleanup func()
		want    string
		wantErr bool
	}{
		{
			name:  "find existing config",
			paths: []string{"./test_config.yaml", "./config/config.yaml"},
			setup: func() {
				_ = os.WriteFile("test_config.yaml", []byte("test"), 0644)
			},
			cleanup: func() {
				os.Remove("test_config.yaml")
			},
			want:    "./test_config.yaml",
			wantErr: false,
		},
		{
			name:    "no config found",
			paths:   []string{"./nonexistent1.yaml", "./nonexistent2.yaml"},
			setup:   func() {},
			cleanup: func() {},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			defer tt.cleanup()

			got, err := GetConfigPath(tt.paths...)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetConfigPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("GetConfigPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
