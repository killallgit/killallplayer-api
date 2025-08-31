package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewServer(t *testing.T) {
	tests := []struct {
		name       string
		addr       string
		checkSetup func(*testing.T, *Server)
	}{
		{
			name: "create server with default address",
			addr: ":8080",
			checkSetup: func(t *testing.T, s *Server) {
				assert.NotNil(t, s)
				assert.NotNil(t, s.router)
				assert.NotNil(t, s.httpServer)
				assert.Equal(t, ":8080", s.httpServer.Addr)
			},
		},
		{
			name: "create server with custom address",
			addr: "localhost:9090",
			checkSetup: func(t *testing.T, s *Server) {
				assert.NotNil(t, s)
				assert.Equal(t, "localhost:9090", s.httpServer.Addr)
			},
		},
		{
			name: "server has proper timeouts",
			addr: ":8080",
			checkSetup: func(t *testing.T, s *Server) {
				assert.Equal(t, 30*time.Second, s.httpServer.ReadTimeout)
				assert.Equal(t, 30*time.Second, s.httpServer.WriteTimeout)
				assert.Equal(t, 30*time.Second, s.httpServer.IdleTimeout)
				assert.Equal(t, 1<<20, s.httpServer.MaxHeaderBytes) // 1MB
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := NewServer(tt.addr)
			
			if tt.checkSetup != nil {
				tt.checkSetup(t, server)
			}
		})
	}
}

func TestServer_Routes(t *testing.T) {
	server := NewServer(":8080")
	
	// Get all registered routes
	routes := []struct {
		name       string
		method     string
		path       string
		wantStatus int
	}{
		{
			name:       "health endpoint",
			method:     http.MethodGet,
			path:       "/health",
			wantStatus: http.StatusOK,
		},
		{
			name:       "version endpoint",
			method:     http.MethodGet,
			path:       "/",
			wantStatus: http.StatusOK,
		},
		{
			name:       "search endpoint requires POST",
			method:     http.MethodGet,
			path:       "/api/v1/search",
			wantStatus: http.StatusMethodNotAllowed,
		},
		{
			name:       "websocket endpoint",
			method:     http.MethodGet,
			path:       "/ws",
			wantStatus: http.StatusBadRequest, // WebSocket upgrade required
		},
	}

	for _, tt := range routes {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			rr := httptest.NewRecorder()
			
			server.Router().ServeHTTP(rr, req)
			
			assert.Equal(t, tt.wantStatus, rr.Code)
		})
	}
}

func TestServer_HealthEndpoint(t *testing.T) {
	server := NewServer(":8080")
	
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	
	server.router.ServeHTTP(rr, req)
	
	assert.Equal(t, http.StatusOK, rr.Code)
	
	// Parse response
	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	
	// Check response structure
	assert.Equal(t, "ok", response["status"])
	assert.NotNil(t, response["timestamp"])
	assert.Contains(t, response, "database")
	
	// Database should be "not configured" when no DB is set up
	dbStatus := response["database"].(map[string]interface{})
	assert.Equal(t, "not configured", dbStatus["status"])
}

func TestServer_VersionEndpoint(t *testing.T) {
	server := NewServer(":8080")
	
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	
	server.router.ServeHTTP(rr, req)
	
	assert.Equal(t, http.StatusOK, rr.Code)
	
	// Parse response
	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	
	// Check response structure
	assert.Contains(t, response, "name")
	assert.Contains(t, response, "version")
	assert.Contains(t, response, "description")
	assert.Equal(t, "Podcast Player API", response["name"])
}

func TestServer_Middleware(t *testing.T) {
	server := NewServer(":8080")
	
	t.Run("CORS headers", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/search", nil)
		req.Header.Set("Origin", "http://localhost:3000")
		rr := httptest.NewRecorder()
		
		server.Router().ServeHTTP(rr, req)
		
		// Check CORS headers
		assert.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Origin"))
		assert.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Methods"))
		assert.NotEmpty(t, rr.Header().Get("Access-Control-Allow-Headers"))
	})
	
	t.Run("request logging", func(t *testing.T) {
		// This would normally check logs, but for now we just verify
		// the request completes successfully
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()
		
		server.Router().ServeHTTP(rr, req)
		
		assert.Equal(t, http.StatusOK, rr.Code)
	})
}

func TestServer_NotFoundHandler(t *testing.T) {
	server := NewServer(":8080")
	
	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	rr := httptest.NewRecorder()
	
	server.router.ServeHTTP(rr, req)
	
	assert.Equal(t, http.StatusNotFound, rr.Code)
	
	// Parse response
	var response map[string]interface{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	require.NoError(t, err)
	
	assert.Equal(t, "error", response["status"])
	assert.Contains(t, response["message"], "not found")
}

func TestServer_RoutePatterns(t *testing.T) {
	// This test verifies that our route patterns are correctly configured
	router := mux.NewRouter()
	
	// Register routes similar to how the server does it
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet)
	
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodGet)
	
	router.HandleFunc("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodPost)
	
	// Test route matching
	tests := []struct {
		path    string
		method  string
		matches bool
	}{
		{"/", http.MethodGet, true},
		{"/", http.MethodPost, false},
		{"/health", http.MethodGet, true},
		{"/health", http.MethodPost, false},
		{"/api/v1/search", http.MethodPost, true},
		{"/api/v1/search", http.MethodGet, false},
		{"/api/v2/search", http.MethodPost, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.path+"_"+tt.method, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			match := &mux.RouteMatch{}
			
			matched := router.Match(req, match)
			assert.Equal(t, tt.matches, matched)
		})
	}
}

func TestServer_ContentTypeHandling(t *testing.T) {
	server := NewServer(":8080")
	
	tests := []struct {
		name        string
		path        string
		contentType string
		wantStatus  int
	}{
		{
			name:        "JSON content type for API endpoints",
			path:        "/health",
			contentType: "application/json",
			wantStatus:  http.StatusOK,
		},
		{
			name:        "Any content type accepted for health",
			path:        "/health",
			contentType: "text/plain",
			wantStatus:  http.StatusOK,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.contentType != "" {
				req.Header.Set("Accept", tt.contentType)
			}
			rr := httptest.NewRecorder()
			
			server.Router().ServeHTTP(rr, req)
			
			assert.Equal(t, tt.wantStatus, rr.Code)
			// API responses should always be JSON
			assert.Contains(t, rr.Header().Get("Content-Type"), "application/json")
		})
	}
}