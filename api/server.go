package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/services/episodes"
)

// Server represents the HTTP server
type Server struct {
	engine             *gin.Engine
	httpServer         *http.Server
	db                 *database.DB
	episodeCache       episodes.EpisodeCache
	rateLimiters       *sync.Map
	cleanupInitialized sync.Once
	cleanupStop        chan struct{}

	// Dependencies for handlers
	dependencies *types.Dependencies
}

// NewServer creates a new HTTP server
func NewServer(address string) *Server {
	// Create Gin engine with recovery middleware only
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

	return server
}

// SetDatabase sets the database connection
func (s *Server) SetDatabase(db *database.DB) {
	s.db = db
	if s.dependencies == nil {
		s.dependencies = &types.Dependencies{}
	}
	s.dependencies.DB = db
}

// SetDependencies sets all handler dependencies
func (s *Server) SetDependencies(deps *types.Dependencies) {
	s.dependencies = deps
}

// Engine returns the Gin engine for testing
func (s *Server) Engine() *gin.Engine {
	return s.engine
}

// Initialize sets up middleware and routes
func (s *Server) Initialize() error {
	// Setup global middleware
	s.setupMiddleware()

	// Setup routes
	if err := s.setupRoutes(); err != nil {
		return err
	}

	return nil
}

// setupMiddleware configures global middleware
func (s *Server) setupMiddleware() {
	// Logger middleware
	s.engine.Use(gin.Logger())

	// Global CORS
	s.engine.Use(CORS())

	// Global request size limit
	s.engine.Use(RequestSizeLimit())
}

// setupRoutes delegates to the main route registration
func (s *Server) setupRoutes() error {
	return RegisterRoutes(s.engine, s.dependencies, s.rateLimiters, s.cleanupStop, &s.cleanupInitialized)
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

// getDatabaseStatus returns the database connection status (helper for health check)
func (s *Server) getDatabaseStatus() map[string]interface{} {
	if s.db == nil || s.db.DB == nil {
		return map[string]interface{}{"status": "not configured"}
	}

	if err := s.db.HealthCheck(); err != nil {
		return map[string]interface{}{"status": "unhealthy", "error": err.Error()}
	}

	return map[string]interface{}{"status": "healthy"}
}
