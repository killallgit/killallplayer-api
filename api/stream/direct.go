package stream

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// StreamDirectURL handles audio streaming from a direct URL without database lookup
func StreamDirectURL() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get URL from query parameter
		audioURL := c.Query("url")
		if audioURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "URL parameter is required"})
			return
		}

		// Validate URL
		parsedURL, err := url.Parse(audioURL)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid URL format"})
			return
		}

		// Security checks
		if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Only HTTP and HTTPS URLs are allowed"})
			return
		}

		// Prevent access to private networks and local resources
		hostname := strings.ToLower(parsedURL.Hostname())
		if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" ||
			strings.HasPrefix(hostname, "192.168.") ||
			strings.HasPrefix(hostname, "10.") ||
			strings.HasPrefix(hostname, "172.") ||
			strings.HasSuffix(hostname, ".local") ||
			strings.HasSuffix(hostname, ".internal") {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access to private networks is not allowed"})
			return
		}

		// Prevent file:// and other potentially dangerous schemes were already handled above
		// Additional check for empty host
		if parsedURL.Host == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "URL must have a valid host"})
			return
		}

		// Limit URL length to prevent abuse
		if len(audioURL) > 2048 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "URL is too long"})
			return
		}

		log.Printf("[DEBUG] Direct stream request for URL: %s", audioURL)

		// Create HTTP client with connection timeout but no overall timeout
		// This allows long streaming but prevents hanging on connection
		client := &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second, // Connection timeout
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 30 * time.Second, // Time to receive headers
				ExpectContinueTimeout: 1 * time.Second,
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Allow up to 10 redirects
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				// Validate each redirect URL
				for _, r := range via {
					redirectURL := r.URL
					if redirectURL.Scheme != "http" && redirectURL.Scheme != "https" {
						return fmt.Errorf("invalid redirect scheme: %s", redirectURL.Scheme)
					}
				}
				return nil
			},
		}

		// Create request with range header if present
		req, err := http.NewRequestWithContext(c.Request.Context(), "GET", audioURL, nil)
		if err != nil {
			log.Printf("[ERROR] Failed to create request for audio URL: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stream audio"})
			return
		}

		// Copy range header if present
		rangeHeader := c.GetHeader("Range")
		if rangeHeader != "" {
			log.Printf("[DEBUG] Range request: %s", rangeHeader)
			req.Header.Set("Range", rangeHeader)
		}

		// Add headers to mimic a browser request
		req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "audio/*,video/*,*/*")
		req.Header.Set("Accept-Language", "en-US,en;q=0.9")
		req.Header.Set("Accept-Encoding", "identity")

		// Execute request
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("[ERROR] Failed to fetch audio: %v", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "Failed to fetch audio from source"})
			return
		}
		defer resp.Body.Close()

		// Log response status
		log.Printf("[DEBUG] Audio source responded with status: %d", resp.StatusCode)

		// Check if source returned an error
		if resp.StatusCode >= 400 {
			log.Printf("[ERROR] Audio source returned error status: %d", resp.StatusCode)
			c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("Audio source returned error: %d", resp.StatusCode)})
			return
		}

		// Copy important headers from source
		copyHeader := func(key string) {
			if value := resp.Header.Get(key); value != "" {
				c.Header(key, value)
			}
		}

		// Set content type
		contentType := resp.Header.Get("Content-Type")
		if contentType == "" {
			// Default to audio/mpeg if not specified
			contentType = "audio/mpeg"
		}
		
		// Check if we're getting HTML instead of audio/video
		if strings.Contains(strings.ToLower(contentType), "text/html") {
			log.Printf("[ERROR] Received HTML instead of audio. Content-Type: %s, URL: %s", contentType, audioURL)
			c.JSON(http.StatusBadGateway, gin.H{"error": "Audio source returned HTML instead of audio content"})
			return
		}
		
		c.Header("Content-Type", contentType)

		// Copy range-related headers
		copyHeader("Content-Length")
		copyHeader("Content-Range")
		copyHeader("Accept-Ranges")
		copyHeader("ETag")
		copyHeader("Last-Modified")
		copyHeader("Cache-Control")

		// Set CORS headers for audio streaming
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Range")
		c.Header("Access-Control-Expose-Headers", "Content-Length, Content-Range, Accept-Ranges")

		// Set appropriate status code
		if resp.StatusCode == http.StatusPartialContent {
			c.Status(http.StatusPartialContent)
			log.Printf("[DEBUG] Returning partial content (206)")
		} else {
			c.Status(http.StatusOK)
			log.Printf("[DEBUG] Returning full content (200)")
		}

		// Stream the audio data
		written, err := io.Copy(c.Writer, resp.Body)
		if err != nil {
			// Client might have disconnected, which is normal for streaming
			if !strings.Contains(err.Error(), "broken pipe") {
				log.Printf("[ERROR] Error streaming audio: %v", err)
			} else {
				log.Printf("[DEBUG] Client disconnected after %d bytes", written)
			}
		} else {
			log.Printf("[DEBUG] Successfully streamed %d bytes", written)
		}
	}
}