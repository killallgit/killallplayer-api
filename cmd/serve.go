package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/pkg/config"
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
	// Initialize config (lazy loading - only when serve command is run)
	if err := config.Init(); err != nil {
		return err
	}

	// Use config values if flags not provided
	if serverHost == "" {
		serverHost = config.GetString("server.host")
	}
	if serverPort == 0 {
		serverPort = config.GetInt("server.port")
	}

	// Initialize database with graceful fallback
	var db *database.DB
	dbPath := config.GetString("database.path")
	dbVerbose := config.GetBool("database.verbose")
	
	if dbPath != "" {
		var err error
		db, err = database.Initialize(dbPath, dbVerbose)
		if err != nil {
			// Log the error but continue without database
			fmt.Fprintf(os.Stderr, "Warning: Database initialization failed: %v\n", err)
			fmt.Println("Continuing without database functionality...")
		} else {
			// Run auto-migration for all models
			if err := db.AutoMigrate(
				&models.Podcast{},
				&models.Episode{},
				&models.User{},
				&models.Subscription{},
				&models.PlaybackState{},
			); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Database migration failed: %v\n", err)
				// Close the database connection if migration fails
				_ = db.Close()
				db = nil
			} else {
				// Ensure database is closed on shutdown
				defer func() {
					if db != nil {
						_ = db.Close()
					}
				}()
			}
		}
	} else {
		fmt.Println("No database path configured, running without database...")
	}

	// Log server startup
	fmt.Printf("Starting Podcast Player API server on %s:%d\n", serverHost, serverPort)

	// Create HTTP server
	srv := &http.Server{
		Addr:           fmt.Sprintf("%s:%d", serverHost, serverPort),
		Handler:        setupRoutes(db),
		ReadTimeout:    config.GetDuration("server.read_timeout"),
		WriteTimeout:   config.GetDuration("server.write_timeout"),
		MaxHeaderBytes: config.GetInt("server.max_header_bytes"),
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
	ctx, cancel := context.WithTimeout(context.Background(), config.GetDuration("server.shutdown_timeout"))
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
func setupRoutes(db *database.DB) http.Handler {
	mux := http.NewServeMux()

	// Add security headers middleware
	handler := addSecurityHeaders(mux)

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		
		health := map[string]any{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		
		// Check database health if available
		if db != nil {
			if err := db.HealthCheck(); err != nil {
				health["database"] = "unhealthy"
				health["database_error"] = err.Error()
			} else {
				health["database"] = "healthy"
			}
		} else {
			health["database"] = "not_configured"
		}
		
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf(`{"status":"%s","timestamp":"%s","database":"%s"}`, 
			health["status"], health["timestamp"], health["database"])
		_, _ = w.Write([]byte(response))
	})

	// Root endpoint with version info
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := fmt.Sprintf(`{"message":"Podcast Player API","version":"%s","commit":"%s"}`, Version, GitCommit)
		_, _ = w.Write([]byte(response))
	})

	return handler
}

// addSecurityHeaders adds security headers to all responses
func addSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy", "default-src 'self'")
		
		// Call the next handler
		next.ServeHTTP(w, r)
	})
}
