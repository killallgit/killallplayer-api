package api

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

// clientLimiter holds a rate limiter and its last accessed time
type clientLimiter struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// CORS handles CORS headers
func CORS() gin.HandlerFunc {
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

// RequestSizeLimit limits request body size to 1MB
func RequestSizeLimit() gin.HandlerFunc {
	return RequestSizeLimitWithSize(1024 * 1024) // 1MB default
}

// RequestSizeLimitWithSize limits request body size to specified bytes
func RequestSizeLimitWithSize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Only limit body size for methods that have a body
		if c.Request.Method == http.MethodPost ||
			c.Request.Method == http.MethodPut ||
			c.Request.Method == http.MethodPatch {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

// PerClientRateLimit creates a per-client rate limiting middleware
func PerClientRateLimit(rateLimiters *sync.Map, cleanupStop chan struct{}, cleanupInitialized *sync.Once, rps int, burst int) gin.HandlerFunc {
	// Start cleanup goroutine only once
	cleanupInitialized.Do(func() {
		go cleanupOldRateLimiters(rateLimiters, cleanupStop)
	})

	return func(c *gin.Context) {
		// Get client identifier (IP address)
		clientIP := c.ClientIP()

		// Load or create rate limiter for this client
		limiterInterface, _ := rateLimiters.LoadOrStore(clientIP, &clientLimiter{
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

// SwaggerAuthMiddleware provides simple static token authentication for Swagger UI
func SwaggerAuthMiddleware() gin.HandlerFunc {
	const swaggerToken = "swagger-api-token-2025"
	
	return func(c *gin.Context) {
		// Skip auth for OPTIONS requests
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}

		// Check for token in Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader != "" {
			// Check for Bearer token format
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if token == swaggerToken {
					c.Next()
					return
				}
			}
			// Check for direct token
			if authHeader == swaggerToken {
				c.Next()
				return
			}
		}

		// Check for token in query parameter (for Swagger UI auth)
		if token := c.Query("token"); token == swaggerToken {
			c.Next()
			return
		}

		// Return 401 if no valid token provided
		c.JSON(http.StatusUnauthorized, gin.H{
			"error":   "Unauthorized",
			"message": "Valid API token required for Swagger UI access",
			"token":   "Use Authorization header with token: " + swaggerToken,
		})
		c.Abort()
	}
}

// cleanupOldRateLimiters removes rate limiters that haven't been used for 10 minutes
func cleanupOldRateLimiters(rateLimiters *sync.Map, cleanupStop chan struct{}) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			rateLimiters.Range(func(key, value interface{}) bool {
				cl := value.(*clientLimiter)
				if now.Sub(cl.lastSeen) > 10*time.Minute {
					rateLimiters.Delete(key)
				}
				return true
			})
		case <-cleanupStop:
			return
		}
	}
}
