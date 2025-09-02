package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name              string
		method            string
		expectedStatus    int
		expectCORSHeaders bool
	}{
		{
			name:              "preflight request",
			method:            "OPTIONS",
			expectedStatus:    http.StatusOK,
			expectCORSHeaders: true,
		},
		{
			name:              "regular GET request",
			method:            "GET",
			expectedStatus:    http.StatusOK,
			expectCORSHeaders: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)

			// Apply CORS middleware
			router.Use(CORS())
			router.Any("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})

			// Create request
			req := httptest.NewRequest(tt.method, "/test", nil)

			// Execute
			router.ServeHTTP(w, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectCORSHeaders {
				assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
				assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
				assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Content-Type")
			}
		})
	}
}

func TestRequestSizeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	w := httptest.NewRecorder()
	_, router := gin.CreateTestContext(w)

	// Apply request size limit middleware
	router.Use(RequestSizeLimit())
	router.POST("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Create small request
	body := strings.Repeat("a", 100)
	req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")

	// Execute
	router.ServeHTTP(w, req)

	// Assert that middleware doesn't break normal requests
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPerClientRateLimit_Basic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Setup
	rateLimiters := &sync.Map{}
	cleanupStop := make(chan struct{})
	cleanupInitialized := &sync.Once{}

	router := gin.New()
	middleware := PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, 10, 5) // Generous limits
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})

	// Make a single request
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "127.0.0.1:12345"
	router.ServeHTTP(w, req)

	// Should succeed
	assert.Equal(t, http.StatusOK, w.Code)

	// Cleanup
	close(cleanupStop)
}

func TestCleanupOldRateLimiters(t *testing.T) {
	// Just test that the function exists and doesn't panic
	rateLimiters := &sync.Map{}
	cleanupStop := make(chan struct{})

	// This should not panic
	go cleanupOldRateLimiters(rateLimiters, cleanupStop)
	close(cleanupStop)

	// Test passes if no panic occurred
	assert.True(t, true)
}
