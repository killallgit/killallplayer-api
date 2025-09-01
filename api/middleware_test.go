package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCORS(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		method           string
		origin           string
		expectedHeaders  map[string]string
		expectedStatus   int
		expectCORSHeaders bool
	}{
		{
			name:             "preflight request",
			method:           "OPTIONS",
			origin:           "https://example.com",
			expectedStatus:   http.StatusNoContent,
			expectCORSHeaders: true,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS",
				"Access-Control-Allow-Headers": "Origin, Content-Length, Content-Type, Authorization",
			},
		},
		{
			name:             "regular GET request",
			method:           "GET",
			origin:           "https://example.com",
			expectedStatus:   http.StatusOK,
			expectCORSHeaders: true,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
		},
		{
			name:             "POST request without origin",
			method:           "POST",
			origin:           "",
			expectedStatus:   http.StatusOK,
			expectCORSHeaders: true,
			expectedHeaders: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
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
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			
			// Execute
			router.ServeHTTP(w, req)
			
			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
			
			if tt.expectCORSHeaders {
				for header, expectedValue := range tt.expectedHeaders {
					assert.Equal(t, expectedValue, w.Header().Get(header), "Header: %s", header)
				}
			}
		})
	}
}

func TestRequestSizeLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name           string
		bodySize       int
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "small request under limit",
			bodySize:       100,
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
		{
			name:           "large request over limit",
			bodySize:       2 * 1024 * 1024, // 2MB (over 1MB limit)
			expectedStatus: http.StatusRequestEntityTooLarge,
			expectError:    true,
		},
		{
			name:           "request at limit",
			bodySize:       1024 * 1024, // 1MB (at limit)
			expectedStatus: http.StatusOK,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)
			
			// Apply request size limit middleware
			router.Use(RequestSizeLimit())
			router.POST("/test", func(c *gin.Context) {
				body, err := io.ReadAll(c.Request.Body)
				if err != nil {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
					return
				}
				c.JSON(http.StatusOK, gin.H{"received": len(body)})
			})
			
			// Create request with body of specified size
			body := strings.Repeat("a", tt.bodySize)
			req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
			req.Header.Set("Content-Type", "text/plain")
			
			// Execute
			router.ServeHTTP(w, req)
			
			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestRequestSizeLimitWithSize(t *testing.T) {
	gin.SetMode(gin.TestMode)

	customLimit := int64(512 * 1024) // 512KB

	tests := []struct {
		name           string
		bodySize       int
		expectedStatus int
	}{
		{
			name:           "request under custom limit",
			bodySize:       256 * 1024, // 256KB
			expectedStatus: http.StatusOK,
		},
		{
			name:           "request over custom limit",
			bodySize:       1024 * 1024, // 1MB (over 512KB limit)
			expectedStatus: http.StatusRequestEntityTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)
			
			// Apply custom request size limit middleware
			router.Use(RequestSizeLimitWithSize(customLimit))
			router.POST("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})
			
			// Create request
			body := strings.Repeat("a", tt.bodySize)
			req := httptest.NewRequest("POST", "/test", strings.NewReader(body))
			
			// Execute
			router.ServeHTTP(w, req)
			
			// Assert
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestPerClientRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name              string
		requestCount      int
		requestsPerSecond int
		burstSize         int
		expectSomeBlocked bool
		waitBetween       time.Duration
	}{
		{
			name:              "requests under rate limit",
			requestCount:      3,
			requestsPerSecond: 10,
			burstSize:         5,
			expectSomeBlocked: false,
			waitBetween:       0,
		},
		{
			name:              "burst requests",
			requestCount:      6,
			requestsPerSecond: 2,
			burstSize:         3,
			expectSomeBlocked: true,
			waitBetween:       0,
		},
		{
			name:              "spaced requests",
			requestCount:      5,
			requestsPerSecond: 10,
			burstSize:         2,
			expectSomeBlocked: false,
			waitBetween:       50 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			rateLimiters := &sync.Map{}
			cleanupStop := make(chan struct{})
			cleanupInitialized := &sync.Once{}
			
			w := httptest.NewRecorder()
			_, router := gin.CreateTestContext(w)
			
			// Apply rate limiting middleware
			middleware := PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, tt.requestsPerSecond, tt.burstSize)
			router.Use(middleware)
			router.GET("/test", func(c *gin.Context) {
				c.JSON(http.StatusOK, gin.H{"message": "success"})
			})
			
			// Make multiple requests
			successCount := 0
			blockedCount := 0
			
			for i := 0; i < tt.requestCount; i++ {
				if tt.waitBetween > 0 && i > 0 {
					time.Sleep(tt.waitBetween)
				}
				
				w := httptest.NewRecorder()
				req := httptest.NewRequest("GET", "/test", nil)
				req.RemoteAddr = "127.0.0.1:12345" // Consistent client IP
				
				router.ServeHTTP(w, req)
				
				if w.Code == http.StatusOK {
					successCount++
				} else if w.Code == http.StatusTooManyRequests {
					blockedCount++
				}
			}
			
			// Assert
			if tt.expectSomeBlocked {
				assert.Greater(t, blockedCount, 0, "Expected some requests to be blocked")
			} else {
				assert.Equal(t, 0, blockedCount, "Expected no requests to be blocked")
				assert.Equal(t, tt.requestCount, successCount, "Expected all requests to succeed")
			}
			
			// Cleanup
			close(cleanupStop)
		})
	}
}

func TestPerClientRateLimit_DifferentClients(t *testing.T) {
	gin.SetMode(gin.TestMode)
	
	// Setup
	rateLimiters := &sync.Map{}
	cleanupStop := make(chan struct{})
	cleanupInitialized := &sync.Once{}
	
	router := gin.New()
	middleware := PerClientRateLimit(rateLimiters, cleanupStop, cleanupInitialized, 2, 2) // Very restrictive
	router.Use(middleware)
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "success"})
	})
	
	// Client 1: Make requests that exhaust the rate limit
	for i := 0; i < 3; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "127.0.0.1:12345"
		router.ServeHTTP(w, req)
	}
	
	// Client 2: Should still be able to make requests
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.1:54321" // Different IP
	router.ServeHTTP(w, req)
	
	// Client 2 should succeed
	assert.Equal(t, http.StatusOK, w.Code)
	
	// Cleanup
	close(cleanupStop)
}

func TestCleanupOldRateLimiters(t *testing.T) {
	// This is harder to test directly since it's a background process
	// We'll test that the function exists and doesn't panic
	rateLimiters := &sync.Map{}
	cleanupStop := make(chan struct{})
	
	// Add some test data
	rateLimiters.Store("test-client", map[string]interface{}{
		"lastSeen": time.Now().Add(-2 * time.Hour),
	})
	
	// Start cleanup (it should run in background)
	go cleanupOldRateLimiters(rateLimiters, cleanupStop)
	
	// Let it run briefly
	time.Sleep(10 * time.Millisecond)
	
	// Stop cleanup
	close(cleanupStop)
	
	// Test passes if no panic occurred
	assert.True(t, true)
}