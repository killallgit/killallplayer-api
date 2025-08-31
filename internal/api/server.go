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

// NewServer creates a new HTTP server
func NewServer(address string) *Server {
	// Create Gin engine with default middleware (logger, recovery)
	engine := gin.New()
	engine.Use(gin.Recovery())

	server := &Server{
		engine: engine,
		httpServer: &http.Server{
			Addr:           address,
			Handler:        engine,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			IdleTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1 MB
		},
	}

	// Setup routes and middleware
	server.setupMiddleware()
	server.setupRoutes()

	return server
}

// SetDatabase sets the database connection
func (s *Server) SetDatabase(db *database.DB) {
	s.db = db
}

// Engine returns the Gin engine for testing
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// setupMiddleware configures global middleware
func (s *Server) setupMiddleware() {
	// Logger middleware
	s.engine.Use(gin.Logger())

	// Global CORS for preflight requests
	s.engine.Use(corsMiddleware())

	// Global request size limit
	s.engine.Use(requestSizeLimitMiddleware())
}

// setupRoutes configures all API routes with proper middleware
func (s *Server) setupRoutes() {
	// Public endpoints (no rate limiting)
	s.engine.GET("/health", s.healthHandler)
	s.engine.GET("/", s.versionHandler)

	// Create rate limiters
	generalLimiter := createRateLimiter(10, 20)    // 10 req/s, burst of 20
	syncLimiter := createRateLimiter(1, 2)         // 1 req/s, burst of 2
	searchLimiter := createRateLimiter(5, 10)      // 5 req/s, burst of 10

	// API v1 routes with rate limiting
	v1 := s.engine.Group("/api/v1")
	{
		// Load config once for all routes
		cfg, err := config.GetConfig()
		if err != nil {
			fmt.Fprintf(gin.DefaultWriter, "Warning: Failed to load config, some endpoints disabled: %v\n", err)
			return
		}

		if cfg == nil {
			fmt.Fprintf(gin.DefaultWriter, "Warning: Config is nil, API endpoints disabled\n")
			return
		}

		// Initialize podcast client
		podcastClient := podcastindex.NewClient(podcastindex.Config{
			APIKey:    cfg.PodcastIndex.APIKey,
			APISecret: cfg.PodcastIndex.APISecret,
			BaseURL:   cfg.PodcastIndex.BaseURL,
		})

		// Search endpoint with dedicated rate limiter
		searchHandler := handlers.NewSearchHandler(podcastClient)
		v1.POST("/search", rateLimitMiddleware(searchLimiter), searchHandler.HandleSearch)

		// Episode endpoints - only if database is configured
		if s.db != nil && s.db.DB != nil {
			// Initialize episode service
			episodeService, episodeTransformer := s.initializeEpisodeService(podcastClient)
			episodeHandler := handlers.NewEpisodeHandlerV3(episodeService, episodeTransformer)

			// Episode routes with general rate limiting
			episodesGroup := v1.Group("/episodes")
			episodesGroup.Use(rateLimitMiddleware(generalLimiter))
			{
				episodesGroup.GET("/byfeedid", func(c *gin.Context) {
					// Convert query param to path param for compatibility
					c.Params = append(c.Params, gin.Param{Key: "id", Value: c.Query("id")})
					episodeHandler.GetEpisodesByPodcastID(c)
				})
				episodesGroup.GET("/byguid", episodeHandler.GetEpisodeByGUID)
				episodesGroup.GET("/recent", episodeHandler.GetRecentEpisodes)
				episodesGroup.GET("/:id", episodeHandler.GetEpisodeByID)
				episodesGroup.PUT("/:id/playback", episodeHandler.UpdatePlaybackState)
			}

			// Podcast routes with mixed rate limiting
			podcastsGroup := v1.Group("/podcasts")
			{
				// Regular endpoints with general rate limiting
				podcastsGroup.GET("/:id/episodes", 
					rateLimitMiddleware(generalLimiter), 
					episodeHandler.GetEpisodesByPodcastID)
				
				// Sync endpoint with strict rate limiting
				podcastsGroup.POST("/:id/episodes/sync", 
					rateLimitMiddleware(syncLimiter), 
					episodeHandler.SyncEpisodesFromPodcastIndex)
			}
		}
	}

	// 404 handler
	s.engine.NoRoute(s.notFoundHandler)
}

// initializeEpisodeService creates and configures the episode service
func (s *Server) initializeEpisodeService(podcastClient *podcastindex.Client) (episodes.EpisodeService, episodes.EpisodeTransformer) {
	// Create dependencies
	episodeFetcher := episodes.NewPodcastIndexAdapter(podcastClient)
	episodeRepo := episodes.NewRepository(s.db.DB)
	episodeCache := episodes.NewCache(time.Hour)
	s.episodeCache = episodeCache // Store for cleanup

	// Get configuration
	maxConcurrentSync := config.GetInt("episodes.max_concurrent_sync")
	if maxConcurrentSync <= 0 {
		maxConcurrentSync = 5
	}
	syncTimeout := config.GetDuration("episodes.sync_timeout")
	if syncTimeout <= 0 {
		syncTimeout = 30 * time.Second
	}

	// Create service
	episodeService := episodes.NewService(
		episodeFetcher,
		episodeRepo,
		episodeCache,
		episodes.WithMaxConcurrentSync(maxConcurrentSync),
		episodes.WithSyncTimeout(syncTimeout),
	)

	return episodeService, episodes.NewTransformer()
}

// Middleware functions

// corsMiddleware handles CORS headers
func corsMiddleware() gin.HandlerFunc {
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

// requestSizeLimitMiddleware limits request body size
func requestSizeLimitMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only limit body size for methods that have a body
		if c.Request.Method == http.MethodPost || 
		   c.Request.Method == http.MethodPut || 
		   c.Request.Method == http.MethodPatch {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1024*1024) // 1MB
		}
		c.Next()
	}
}

// rateLimitMiddleware creates a rate limiting middleware for a specific limiter
func rateLimitMiddleware(limiter *rate.Limiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please slow down your requests.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// createRateLimiter creates a new rate limiter
func createRateLimiter(rps int, burst int) *rate.Limiter {
	return rate.NewLimiter(rate.Every(time.Second/time.Duration(rps)), burst)
}

// Handler functions

// healthHandler handles health check requests
func (s *Server) healthHandler(c *gin.Context) {
	response := gin.H{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	// Add database status - always include it for consistency
	response["database"] = s.getDatabaseStatus()

	c.JSON(http.StatusOK, response)
}

// getDatabaseStatus returns the database connection status
func (s *Server) getDatabaseStatus() gin.H {
	if s.db == nil || s.db.DB == nil {
		return gin.H{"status": "not configured"}
	}

	if err := s.db.HealthCheck(); err != nil {
		return gin.H{"status": "unhealthy", "error": err.Error()}
	}

	return gin.H{"status": "healthy"}
}

// versionHandler handles version requests
func (s *Server) versionHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"name":        "Podcast Player API",
		"version":     "1.0.0",
		"description": "API for managing and streaming podcasts",
		"status":      "running",
	})
}

// notFoundHandler handles 404 errors
func (s *Server) notFoundHandler(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{
		"status":  "error",
		"message": "The requested endpoint was not found",
		"path":    c.Request.URL.Path,
	})
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