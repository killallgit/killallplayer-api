# Config Package

This package handles configuration management for the Podcast Player API using Viper.

## Features
- Load configuration from YAML files
- Environment variable overrides
- Configuration validation
- Default values
- Hot reload support (optional)

## Structure
- `config.go` - Main configuration loader
- `types.go` - Configuration struct definitions
- `validation.go` - Configuration validation logic

## Configuration Sources (in order of precedence)
1. Environment variables
2. Configuration file (config.yaml)
3. Default values

## Usage
```go
cfg, err := config.Load("config/config.yaml")
if err != nil {
    log.Fatal(err)
}
```