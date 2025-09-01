package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
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
	engine             *gin.Engine
	httpServer         *http.Server
	db                 *database.DB
	episodeCache       episodes.EpisodeCache // Store cache for proper cleanup
	rateLimiters       *sync.Map            // Per-client rate limiters using sync.Map for concurrent access
	cleanupInitialized sync.Once           // Ensure cleanup goroutine runs only once
	cleanupStop        chan struct{}       // Channel to signal cleanup goroutine to stop
}

// clientLimiter holds a rate limiter and its last accessed time
type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// NewServer creates a new HTTP server
func NewServer(address string) *Server {
	// Create Gin engine with default middleware (logger, recovery)
	engine := gin.New()
	engine.Use(gin.Recovery())

	server := &Server{
		engine:       engine,
		rateLimiters: &sync.Map{},
		cleanupStop:  make(chan struct{}),
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

		// Search endpoint with dedicated rate limiter (5 req/s, burst of 10)
		searchHandler := handlers.NewSearchHandler(podcastClient)
		v1.POST("/search", s.perClientRateLimitMiddleware(5, 10), searchHandler.HandleSearch)

		// Episode endpoints - only if database is configured
		if s.db != nil && s.db.DB != nil {
			// Initialize episode service
			episodeService, episodeTransformer := s.initializeEpisodeService(podcastClient)
			episodeHandler := handlers.NewEpisodeHandlerV3(episodeService, episodeTransformer)

			// Episode routes with general rate limiting (10 req/s, burst of 20)
			episodesGroup := v1.Group("/episodes")
			episodesGroup.Use(s.perClientRateLimitMiddleware(10, 20))
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
				// Regular endpoints with general rate limiting (10 req/s, burst of 20)
				podcastsGroup.GET("/:id/episodes", 
					s.perClientRateLimitMiddleware(10, 20), 
					episodeHandler.GetEpisodesByPodcastID)
				
				// Sync endpoint with strict rate limiting (1 req/s, burst of 2)
				podcastsGroup.POST("/:id/episodes/sync", 
					s.perClientRateLimitMiddleware(1, 2), 
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

// perClientRateLimitMiddleware creates a per-client rate limiting middleware
func (s *Server) perClientRateLimitMiddleware(rps int, burst int) gin.HandlerFunc {
	// Start cleanup goroutine only once
	s.cleanupInitialized.Do(func() {
		go s.cleanupOldRateLimiters()
	})
	
	return func(c *gin.Context) {
		// Get client identifier (IP address)
		clientIP := c.ClientIP()
		
		// Load or create rate limiter for this client
		limiterInterface, _ := s.rateLimiters.LoadOrStore(clientIP, &clientLimiter{
			limiter:  rate.NewLimiter(rate.Every(time.Second/time.Duration(rps)), burst),
			lastSeen: time.Now(),
		})
		
		cl := limiterInterface.(*clientLimiter)
		cl.lastSeen = time.Now()
		
		// Check rate limit
		if !cl.limiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded. Please slow down your requests.",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// cleanupOldRateLimiters removes rate limiters that haven't been used for 10 minutes
func (s *Server) cleanupOldRateLimiters() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			s.rateLimiters.Range(func(key, value interface{}) bool {
				cl := value.(*clientLimiter)
				if now.Sub(cl.lastSeen) > 10*time.Minute {
					s.rateLimiters.Delete(key)
				}
				return true
			})
		case <-s.cleanupStop:
			return
		}
	}
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

	// Stop the rate limiter cleanup goroutine
	close(s.cleanupStop)

	return s.httpServer.Shutdown(ctx)
}