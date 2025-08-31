package cmd

import (
	"fmt"
	"os"

	"github.com/killallgit/player-api/pkg/config"
	"github.com/spf13/cobra"
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
	// Set up configuration loading with lazy initialization
	cobra.OnInitialize(loadConfig)
	
	// Add persistent flags for logging configuration
	rootCmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().Bool("json-logs", false, "enable JSON formatted logs")
}

// loadConfig loads the configuration when a command needs it
// This is called lazily only when a command that needs config runs
func loadConfig() {
	// Skip config loading for commands that don't need it
	cmd, _, _ := rootCmd.Find(os.Args[1:])
	if cmd != nil && (cmd.Name() == "version" || cmd.Name() == "help") {
		// Skip loading config for version and help commands
		if len(os.Args) > 2 && os.Args[2] == "--help" {
			return // Skip for subcommand help too
		}
		if cmd.Name() == "version" {
			return // Version command doesn't need config
		}
	}

	// Initialize the configuration
	if err := config.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing config: %v\n", err)
		os.Exit(1)
	}
}