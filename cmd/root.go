package cmd

import (
	"os"

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
	// Global flags (no config file flag since it's always ./config/settings.yaml)
	rootCmd.PersistentFlags().String("log-level", "info", "log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().Bool("json-logs", false, "enable JSON formatted logs")
}
