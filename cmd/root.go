package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/killallgit/player-api/pkg/config"
	"github.com/spf13/cobra"
)

var (
	cfgFile   string
	appConfig *config.Config
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "player-api",
	Short: "Podcast Player API server",
	Long: `Podcast Player API - A comprehensive podcast streaming and processing API

This API provides podcast discovery, audio streaming, real-time updates,
and audio processing capabilities including waveform generation and transcription.

Features:
  • Podcast discovery via Podcast Index API
  • Audio streaming with range request support
  • Real-time updates via WebSocket
  • Audio processing (waveform generation, transcription)
  • Audio tagging system`,
	SilenceUsage: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// NewRootCmd creates a new root command (exported for testing)
func NewRootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./config/config.yaml)")
	rootCmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().Bool("json-logs", false, "enable JSON formatted logs")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	// Determine config file path
	configPaths := []string{}

	if cfgFile != "" {
		// Use config file from the flag
		configPaths = append(configPaths, cfgFile)
	} else {
		// Default config locations
		configPaths = append(configPaths,
			"./config/config.yaml",
			"./config.yaml",
			filepath.Join(os.Getenv("HOME"), ".player-api.yaml"),
		)
	}

	// Try to find and load config
	configPath, err := config.GetConfigPath(configPaths...)
	if err != nil {
		// No config file found, will use defaults
		configPath = ""
	}

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	appConfig = cfg

	if configPath != "" {
		fmt.Fprintf(os.Stderr, "Using config file: %s\n", configPath)
	}
}
