package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/internal/services/cache"
)

// CacheConfig holds configuration for cache middleware
type CacheConfig struct {
	Cache      cache.Cache
	DefaultTTL time.Duration
	TTLByPath  map[string]time.Duration // Path-specific TTLs
	Enabled    bool
}

// responseWriter captures response for caching
type responseWriter struct {
	gin.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (w *responseWriter) Write(data []byte) (int, error) {
	w.body.Write(data)
	return w.ResponseWriter.Write(data)
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// CacheMiddleware creates a cache middleware
func CacheMiddleware(config CacheConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !config.Enabled {
			c.Next()
			return
		}

		// Skip caching for non-GET requests
		if c.Request.Method != http.MethodGet {
			c.Next()
			return
		}

		// Check cache control headers from client
		if shouldBypassCache(c.Request) {
			c.Header("X-Cache", "BYPASS")
			c.Next()
			return
		}

		// Generate cache key
		key := generateCacheKey(c.Request)

		// Try to get from cache
		if cachedData, found := config.Cache.Get(context.Background(), key); found {
			// Parse cached response
			if response, err := parseCachedResponse(cachedData); err == nil {
				// Set cache headers
				c.Header("X-Cache", "HIT")
				c.Header("Age", fmt.Sprintf("%d", int(time.Since(response.CachedAt).Seconds())))

				// Set original headers
				for key, values := range response.Headers {
					for _, value := range values {
						c.Header(key, value)
					}
				}

				// Write cached response
				c.Data(response.Status, response.ContentType, response.Body)
				c.Abort()
				return
			}
		}

		// Cache MISS - capture response
		c.Header("X-Cache", "MISS")

		// Create response writer to capture response
		w := &responseWriter{
			ResponseWriter: c.Writer,
			body:           bytes.NewBuffer(nil),
			status:         http.StatusOK,
		}
		c.Writer = w

		// Process request
		c.Next()

		// Only cache successful responses
		if w.status == http.StatusOK && w.body.Len() > 0 {
			// Determine TTL
			ttl := config.DefaultTTL
			if pathTTL, exists := config.TTLByPath[c.Request.URL.Path]; exists {
				ttl = pathTTL
			} else {
				// Check for path prefix match
				for path, pathTTL := range config.TTLByPath {
					if strings.HasPrefix(c.Request.URL.Path, path) {
						ttl = pathTTL
						break
					}
				}
			}

			// Create cached response
			cachedResponse := CachedResponse{
				Status:      w.status,
				Headers:     c.Writer.Header(),
				Body:        w.body.Bytes(),
				ContentType: c.ContentType(),
				CachedAt:    time.Now(),
				ETag:        generateETag(w.body.Bytes()),
			}

			// Store in cache
			if data, err := serializeCachedResponse(cachedResponse); err == nil {
				_ = config.Cache.Set(context.Background(), key, data, ttl)
			}

			// Set ETag header
			c.Header("ETag", cachedResponse.ETag)
		}
	}
}

// CachedResponse represents a cached HTTP response
type CachedResponse struct {
	Status      int
	Headers     http.Header
	Body        []byte
	ContentType string
	CachedAt    time.Time
	ETag        string
}

// shouldBypassCache checks if cache should be bypassed based on request headers
func shouldBypassCache(req *http.Request) bool {
	cacheControl := req.Header.Get("Cache-Control")
	if cacheControl == "" {
		return false
	}

	// Parse cache control directives
	directives := strings.Split(strings.ToLower(cacheControl), ",")
	for _, directive := range directives {
		directive = strings.TrimSpace(directive)

		// Check for no-cache or no-store
		if directive == "no-cache" || directive == "no-store" {
			return true
		}

		// Check for max-age=0
		if strings.HasPrefix(directive, "max-age=") {
			if maxAge := strings.TrimPrefix(directive, "max-age="); maxAge == "0" {
				return true
			}
		}
	}

	// Also check Pragma header for backwards compatibility
	if req.Header.Get("Pragma") == "no-cache" {
		return true
	}

	return false
}

// generateCacheKey creates a unique key for the request
func generateCacheKey(req *http.Request) string {
	// Start with path
	parts := []string{req.URL.Path}

	// Add sorted query parameters
	if req.URL.RawQuery != "" {
		params := req.URL.Query()
		keys := make([]string, 0, len(params))
		for k := range params {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, k := range keys {
			for _, v := range params[k] {
				parts = append(parts, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	// Create key
	return "http:" + strings.Join(parts, ":")
}

// generateETag creates an ETag for the response body
func generateETag(body []byte) string {
	hash := sha256.Sum256(body)
	return fmt.Sprintf(`"%s"`, hex.EncodeToString(hash[:]))
}

// serializeCachedResponse serializes a cached response
func serializeCachedResponse(response CachedResponse) ([]byte, error) {
	var buf bytes.Buffer

	// Write metadata
	buf.WriteString(fmt.Sprintf("%d|%s|%d|%s\n",
		response.Status,
		response.ContentType,
		response.CachedAt.Unix(),
		response.ETag))

	// Write headers
	for key, values := range response.Headers {
		for _, value := range values {
			buf.WriteString(fmt.Sprintf("%s:%s\n", key, value))
		}
	}
	buf.WriteString("\n")

	// Write body
	buf.Write(response.Body)

	return buf.Bytes(), nil
}

// parseCachedResponse deserializes a cached response
func parseCachedResponse(data []byte) (*CachedResponse, error) {
	parts := bytes.SplitN(data, []byte("\n\n"), 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid cached response format")
	}

	lines := bytes.Split(parts[0], []byte("\n"))
	if len(lines) < 1 {
		return nil, fmt.Errorf("missing metadata")
	}

	// Parse metadata
	metadata := strings.Split(string(lines[0]), "|")
	if len(metadata) != 4 {
		return nil, fmt.Errorf("invalid metadata format")
	}

	status, _ := strconv.Atoi(metadata[0])
	cachedAt, _ := strconv.ParseInt(metadata[2], 10, 64)

	response := &CachedResponse{
		Status:      status,
		ContentType: metadata[1],
		CachedAt:    time.Unix(cachedAt, 0),
		ETag:        metadata[3],
		Headers:     make(http.Header),
		Body:        parts[1],
	}

	// Parse headers
	for i := 1; i < len(lines); i++ {
		if len(lines[i]) == 0 {
			break
		}
		headerParts := bytes.SplitN(lines[i], []byte(":"), 2)
		if len(headerParts) == 2 {
			response.Headers.Add(string(headerParts[0]), string(headerParts[1]))
		}
	}

	return response, nil
}
