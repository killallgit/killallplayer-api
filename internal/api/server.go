package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/killallgit/player-api/internal/api/handlers"
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/services/podcastindex"
	"github.com/killallgit/player-api/pkg/config"
)

// Server represents the HTTP server
type Server struct {
	router     *mux.Router
	httpServer *http.Server
	db         *database.DB
}

// Router returns the server's router for testing
func (s *Server) Router() *mux.Router {
	return s.router
}

// NewServer creates a new HTTP server
func NewServer(addr string) *Server {
	router := mux.NewRouter()
	
	server := &Server{
		router: router,
		httpServer: &http.Server{
			Addr:           addr,
			Handler:        router,
			ReadTimeout:    30 * time.Second,
			WriteTimeout:   30 * time.Second,
			IdleTimeout:    30 * time.Second,
			MaxHeaderBytes: 1 << 20, // 1MB
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

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	// Health check endpoint
	s.router.HandleFunc("/health", s.healthHandler).Methods(http.MethodGet)
	
	// Version endpoint
	s.router.HandleFunc("/", s.versionHandler).Methods(http.MethodGet)
	
	// API v1 routes
	v1 := s.router.PathPrefix("/api/v1").Subrouter()
	
	// Search endpoint - create podcast client and handler
	cfg, err := config.GetConfig()
	if err == nil && cfg != nil {
		podcastClient := podcastindex.NewClient(podcastindex.Config{
			APIKey:    cfg.PodcastIndex.APIKey,
			APISecret: cfg.PodcastIndex.APISecret,
			BaseURL:   cfg.PodcastIndex.BaseURL,
		})
		searchHandler := handlers.NewSearchHandler(podcastClient)
		v1.Handle("/search", searchHandler).Methods(http.MethodPost)
		// Handle OPTIONS for CORS preflight
		v1.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
			// Headers are already set by middleware
			w.WriteHeader(http.StatusOK)
		}).Methods(http.MethodOptions)
	}
	
	// WebSocket endpoint
	s.router.HandleFunc("/ws", s.websocketHandler).Methods(http.MethodGet)
	
	// Not found handler
	s.router.NotFoundHandler = http.HandlerFunc(s.notFoundHandler)
}

// setupMiddleware configures middleware
func (s *Server) setupMiddleware() {
	// CORS middleware
	s.router.Use(s.corsMiddleware)
	
	// Logging middleware
	s.router.Use(s.loggingMiddleware)
	
	// Recovery middleware
	s.router.Use(s.recoveryMiddleware)
}

// corsMiddleware handles CORS headers
func (s *Server) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Max-Age", "86400")
		
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// loggingMiddleware logs HTTP requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: Add proper logging
		next.ServeHTTP(w, r)
	})
}

// recoveryMiddleware recovers from panics
func (s *Server) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// healthHandler handles health check requests
func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"database":  s.getDatabaseStatus(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// getDatabaseStatus returns the database status
func (s *Server) getDatabaseStatus() map[string]interface{} {
	if s.db == nil {
		return map[string]interface{}{
			"status": "not configured",
		}
	}
	
	if err := s.db.HealthCheck(); err != nil {
		return map[string]interface{}{
			"status": "unhealthy",
			"error":  err.Error(),
		}
	}
	
	return map[string]interface{}{
		"status": "healthy",
	}
}

// versionHandler handles version requests
func (s *Server) versionHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"name":        "Podcast Player API",
		"version":     "1.0.0",
		"description": "A comprehensive podcast streaming and processing API",
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// websocketHandler handles WebSocket connections
func (s *Server) websocketHandler(w http.ResponseWriter, r *http.Request) {
	// WebSocket upgrade requires additional headers
	http.Error(w, "WebSocket not yet implemented", http.StatusBadRequest)
}

// notFoundHandler handles 404 responses
func (s *Server) notFoundHandler(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":  "error",
		"message": "Resource not found",
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(response)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown() error {
	return s.httpServer.Close()
}