package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	serverHost string
	serverPort int
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API server",
	Long: `Start the Podcast Player API server with the configured settings.

The server will listen for HTTP requests and WebSocket connections,
providing podcast discovery, audio streaming, and real-time updates.

Example:
  player-api serve
  player-api serve --port 9090
  player-api serve --host 0.0.0.0 --port 8080`,
	RunE: runServer,
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Server flags
	serveCmd.Flags().StringVar(&serverHost, "host", "", "server host (overrides config)")
	serveCmd.Flags().IntVar(&serverPort, "port", 0, "server port (overrides config)")
}

func runServer(cmd *cobra.Command, args []string) error {
	// Load config (lazy loading - only when serve command is run)
	if err := loadConfig(); err != nil {
		return err
	}

	// Use config values if flags not provided
	if serverHost == "" {
		serverHost = appConfig.Server.Host
	}
	if serverPort == 0 {
		serverPort = appConfig.Server.Port
	}

	// TODO: Initialize database and run GORM AutoMigrate here
	// db, err := database.Initialize(appConfig.Database)
	// if err != nil {
	//     return fmt.Errorf("failed to initialize database: %w", err)
	// }
	// if err := db.AutoMigrate(&models.Podcast{}, &models.Episode{}, ...); err != nil {
	//     return fmt.Errorf("failed to auto-migrate database: %w", err)
	// }

	// Log server startup
	fmt.Printf("Starting Podcast Player API server on %s:%d\n", serverHost, serverPort)

	// Create HTTP server
	srv := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", serverHost, serverPort),
		Handler:        setupRoutes(),
		ReadTimeout:    appConfig.Server.ReadTimeout,
		WriteTimeout:   appConfig.Server.WriteTimeout,
		MaxHeaderBytes: appConfig.Server.MaxHeaderBytes,
	}

	// Channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Channel to receive server errors
	serverErr := make(chan error, 1)

	// Start server in a goroutine
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- fmt.Errorf("server error: %w", err)
		}
	}()

	fmt.Printf("Server is ready to handle requests at %s:%d\n", serverHost, serverPort)

	// Wait for interrupt signal or server error
	select {
	case <-stop:
		fmt.Println("\nShutting down server...")
	case err := <-serverErr:
		fmt.Fprintf(os.Stderr, "\n%v\n", err)
		fmt.Println("Shutting down server...")
	}

	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), appConfig.Server.ShutdownTimeout)
	defer cancel()

	// Attempt graceful shutdown
	if err := srv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Server forced to shutdown: %v\n", err)
		return err
	}

	fmt.Println("Server gracefully stopped")
	return nil
}

// setupRoutes configures and returns the HTTP handler
// This is a placeholder that will be expanded when we implement the HTTP server
func setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Root endpoint with version info
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf(`{"message":"Podcast Player API","version":"%s","commit":"%s"}`, Version, GitCommit)
		_, _ = w.Write([]byte(response))
	})

	return mux
}
