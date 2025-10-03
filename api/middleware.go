package api

import (
	"net/http"
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

func RequestSizeLimit() gin.HandlerFunc {
	return RequestSizeLimitWithSize(1024 * 1024)
}

func RequestSizeLimitWithSize(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodPost ||
			c.Request.Method == http.MethodPut ||
			c.Request.Method == http.MethodPatch {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		}
		c.Next()
	}
}

func PerClientRateLimit(rateLimiters *sync.Map, cleanupStop chan struct{}, cleanupInitialized *sync.Once, rps int, burst int) gin.HandlerFunc {
	cleanupInitialized.Do(func() {
		go cleanupOldRateLimiters(rateLimiters, cleanupStop)
	})

	return func(c *gin.Context) {
		clientIP := c.ClientIP()

		limiterInterface, _ := rateLimiters.LoadOrStore(clientIP, &clientLimiter{
			limiter:  rate.NewLimiter(rate.Every(time.Second/time.Duration(rps)), burst),
			lastSeen: time.Now(),
		})

		cl := limiterInterface.(*clientLimiter)
		cl.lastSeen = time.Now()

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
