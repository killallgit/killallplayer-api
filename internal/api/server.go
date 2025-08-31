package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/internal/api/handlers"
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/podcastindex"
	"github.com/killallgit/player-api/pkg/config"
	"golang.org/x/time/rate"
)

// Server represents the HTTP server
type Server struct {
	engine       *gin.Engine
	httpServer   *http.Server
	db           *database.DB
	episodeCache episodes.EpisodeCache // Store cache for proper cleanup
}

// Engine returns the server's gin engine for testing
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// NewServer creates a new HTTP server
func NewServer(addr string) *Server {
	// Set Gin mode based on environment
	if config.GetString("environment") == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	engine := gin.New()

	server := &Server{
		engine: engine,
		httpServer: &http.Server{
			Addr:           addr,
			Handler:        engine,
			ReadTimeout:    config.GetDuration("server.read_timeout"),
			WriteTimeout:   config.GetDuration("server.write_timeout"),
			IdleTimeout:    30 * time.Second,
			MaxHeaderBytes: config.GetInt("server.max_header_bytes"),
		},
	}

	server.setupMiddleware()
	server.setupRoutes()

	return server
}

// SetDatabase sets the database connection
func (s *Server) SetDatabase(db *database.DB) {
	s.db = db
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware() {
	// Recovery middleware - must be first
	s.engine.Use(gin.Recovery())

	// Logger middleware
	s.engine.Use(gin.Logger())

	// CORS middleware
	s.engine.Use(s.corsMiddleware())

	// Request size limiting middleware
	s.engine.Use(s.requestSizeLimitMiddleware())

	// Rate limiting middleware for API endpoints
	s.engine.Use(s.rateLimitMiddleware())
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.engine.GET("/health", s.healthHandler)

	// Version endpoint
	s.engine.GET("/", s.versionHandler)

	// API v1 routes
	v1 := s.engine.Group("/api/v1")
	{
		// Search endpoint - create podcast client and handler
		cfg, err := config.GetConfig()
		if err != nil {
			// Log error but don't fail server startup - search endpoint will be disabled
			fmt.Fprintf(gin.DefaultWriter, "Warning: Failed to load config, search endpoint disabled: %v\n", err)
		} else if cfg != nil {
			podcastClient := podcastindex.NewClient(podcastindex.Config{
				APIKey:    cfg.PodcastIndex.APIKey,
				APISecret: cfg.PodcastIndex.APISecret,
				BaseURL:   cfg.PodcastIndex.BaseURL,
			})
			searchHandler := handlers.NewSearchHandler(podcastClient)
			v1.POST("/search", searchHandler.HandleSearch)

			// Episode endpoints - only if database is configured
			if s.db != nil && s.db.DB != nil {
				// Create dependencies - reuse the same podcastindex client
				episodeFetcher := episodes.NewPodcastIndexAdapter(podcastClient)
				episodeRepo := episodes.NewRepository(s.db.DB)
				episodeCache := episodes.NewCache(time.Hour)
				s.episodeCache = episodeCache // Store for cleanup

				// Create service layer with configurable options
				maxConcurrentSync := config.GetInt("episodes.max_concurrent_sync")
				if maxConcurrentSync <= 0 {
					maxConcurrentSync = 5 // default
				}
				syncTimeout := config.GetDuration("episodes.sync_timeout")
				if syncTimeout <= 0 {
					syncTimeout = 30 * time.Second // default
				}

				episodeService := episodes.NewService(
					episodeFetcher,
					episodeRepo,
					episodeCache,
					episodes.WithMaxConcurrentSync(maxConcurrentSync),
					episodes.WithSyncTimeout(syncTimeout),
				)
				episodeTransformer := episodes.NewTransformer()

				// Use V3 handler with service layer
				episodeHandler := handlers.NewEpisodeHandlerV3(episodeService, episodeTransformer)

				// Episode routes (Podcast Index compatible)
				v1.GET("/episodes/byfeedid", func(c *gin.Context) {
					// Convert query param to path param for compatibility
					c.Params = append(c.Params, gin.Param{Key: "id", Value: c.Query("id")})
					episodeHandler.GetEpisodesByPodcastID(c)
				})
				v1.GET("/episodes/byguid", episodeHandler.GetEpisodeByGUID)
				v1.GET("/episodes/recent", episodeHandler.GetRecentEpisodes)

				// Additional routes for our API
				v1.GET("/podcasts/:id/episodes", episodeHandler.GetEpisodesByPodcastID)
				v1.POST("/podcasts/:id/episodes/sync", episodeHandler.SyncEpisodesFromPodcastIndex)
				v1.GET("/episodes/:id", episodeHandler.GetEpisodeByID)
				v1.PUT("/episodes/:id/playback", episodeHandler.UpdatePlaybackState)
			}
		}
	}

	// 404 handler
	s.engine.NoRoute(s.notFoundHandler)
}

// corsMiddleware returns CORS middleware
func (s *Server) corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}

// requestSizeLimitMiddleware returns request size limiting middleware
func (s *Server) requestSizeLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Limit request body size to 1MB for API endpoints
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut || c.Request.Method == http.MethodPatch {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024) // 1MB limit
		}
		c.Next()
	}
}

// rateLimitMiddleware returns rate limiting middleware
func (s *Server) rateLimitMiddleware() gin.HandlerFunc {
	// Create rate limiters for different endpoint categories
	generalLimiter := rate.NewLimiter(rate.Every(time.Second/10), 20) // 10 req/s, burst of 20
	syncLimiter := rate.NewLimiter(rate.Every(time.Second), 2)        // 1 req/s for sync endpoints, burst of 2

	return func(c *gin.Context) {
		// Skip rate limiting for health checks
		if c.Request.URL.Path == "/health" || c.Request.URL.Path == "/" {
			c.Next()
			return
		}

		// Apply stricter rate limiting for sync endpoints
		if c.Request.Method == http.MethodPost &&
			(c.Request.URL.Path == "/api/v1/episodes/sync" ||
				len(c.Request.URL.Path) > 20 && c.Request.URL.Path[len(c.Request.URL.Path)-5:] == "/sync") {
			if !syncLimiter.Allow() {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "Rate limit exceeded for sync operations. Please wait before trying again.",
				})
				c.Abort()
				return
			}
		} else {
			// Apply general rate limiting for other endpoints
			if !generalLimiter.Allow() {
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "Rate limit exceeded. Please slow down your requests.",
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// healthHandler handles health check requests
func (s *Server) healthHandler(c *gin.Context) {
	response := gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"database":  s.getDatabaseStatus(),
	}

	c.JSON(http.StatusOK, response)
}

// getDatabaseStatus returns the database status
func (s *Server) getDatabaseStatus() gin.H {
	if s.db == nil {
		return gin.H{
			"status": "not configured",
		}
	}

	if err := s.db.HealthCheck(); err != nil {
		return gin.H{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	}

	return gin.H{
		"status": "healthy",
	}
}

// versionHandler handles version requests
func (s *Server) versionHandler(c *gin.Context) {
	response := gin.H{
		"name":        "Podcast Player API",
		"version":     "1.0.0",
		"description": "A comprehensive podcast streaming and processing API",
	}

	c.JSON(http.StatusOK, response)
}

// notFoundHandler handles 404 responses
func (s *Server) notFoundHandler(c *gin.Context) {
	response := gin.H{
		"status":  "error",
		"message": "Resource not found",
	}

	c.JSON(http.StatusNotFound, response)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop the cache cleanup goroutine if it exists
	if s.episodeCache != nil {
		s.episodeCache.Stop()
	}

	return s.httpServer.Shutdown(ctx)
}
