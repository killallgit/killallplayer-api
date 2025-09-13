package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/killallgit/player-api/pkg/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "killallplayer-api",
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
	// Set up configuration loading with proper initialization
	cobra.OnInitialize(initializeConfig)

	// Add persistent flags that are commonly used
	rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().Bool("json-logs", false, "Enable JSON log format")
	rootCmd.PersistentFlags().String("config", "", "Config file path (optional)")
}

// initializeConfig initializes the configuration system
// This is called lazily only when a command that needs config runs
func initializeConfig() {
	// Skip config loading for commands that don't need it
	if shouldSkipConfigLoading() {
		return
	}

	// Get config file path from flag if provided
	configFile, _ := rootCmd.PersistentFlags().GetString("config")
	if configFile != "" {
		// Set custom config file path via environment variable
		// This will be picked up by our Viper factory
		os.Setenv("KILLALL_CONFIG_PATH", configFile)
	}

	// Initialize the modern configuration system
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing configuration: %v\n", err)
		os.Exit(1)
	}

	// Apply command line flag overrides
	applyFlagOverrides()
}

// shouldSkipConfigLoading determines if config loading should be skipped
func shouldSkipConfigLoading() bool {
	cmd, _, _ := rootCmd.Find(os.Args[1:])
	if cmd == nil {
		return false
	}

	// Skip for version and help commands
	switch cmd.Name() {
	case "version", "help":
		return true
	default:
		// Check for help flag
		for _, arg := range os.Args[1:] {
			if arg == "--help" || arg == "-h" {
				return true
			}
		}
		return false
	}
}

// applyFlagOverrides applies command line flag values to configuration
func applyFlagOverrides() {
	// Apply log level override if set
	if rootCmd.PersistentFlags().Changed("log-level") {
		if logLevel, err := rootCmd.PersistentFlags().GetString("log-level"); err == nil {
			viper.Set("logging.level", logLevel)
		}
	}

	// Apply JSON logs override if set
	if rootCmd.PersistentFlags().Changed("json-logs") {
		if jsonLogs, err := rootCmd.PersistentFlags().GetBool("json-logs"); err == nil {
			if jsonLogs {
				viper.Set("logging.format", "json")
			} else {
				viper.Set("logging.format", "text")
			}
		}
	}

	log.Printf("Configuration initialized successfully")
}
