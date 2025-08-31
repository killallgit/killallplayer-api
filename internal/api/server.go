package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/internal/api/handlers"
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/services/podcastindex"
	"github.com/killallgit/player-api/pkg/config"
)

// Server represents the HTTP server
type Server struct {
	engine     *gin.Engine
	httpServer *http.Server
	db         *database.DB
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
		if err == nil && cfg != nil {
			podcastClient := podcastindex.NewClient(podcastindex.Config{
				APIKey:    cfg.PodcastIndex.APIKey,
				APISecret: cfg.PodcastIndex.APISecret,
				BaseURL:   cfg.PodcastIndex.BaseURL,
			})
			searchHandler := handlers.NewSearchHandler(podcastClient)
			v1.POST("/search", searchHandler.HandleSearch)
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
	return gin.CustomRecovery(func(c *gin.Context, recovered interface{}) {
		if recovered != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Internal server error",
			})
		}
	})
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
	return s.httpServer.Shutdown(ctx)
}