package stream

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
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
		if isPrivateOrLocalAddress(parsedURL.Hostname()) {
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
		if len(audioURL) > MaxURLLength {
			c.JSON(http.StatusBadRequest, gin.H{"error": "URL is too long"})
			return
		}

		log.Printf("[DEBUG] Direct stream request for URL: %s", audioURL)

		// Use shared HTTP client for better connection reuse and performance

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

		// Execute request using shared client
		resp, err := streamingClient.Do(req)
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

		// Add chunked transfer encoding for streaming
		c.Header("Transfer-Encoding", "chunked")

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

		// Type assert to get the flusher for immediate data transmission
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			log.Printf("[WARN] Response writer doesn't support flushing, falling back to buffered copy")
			// Fall back to io.CopyBuffer if flushing not supported
			buffer := make([]byte, StreamBuffer)
			written, err := io.CopyBuffer(c.Writer, resp.Body, buffer)
			if err != nil {
				// Client might have disconnected, which is normal for streaming
				if !strings.Contains(err.Error(), "broken pipe") && !strings.Contains(err.Error(), "connection reset") {
					log.Printf("[ERROR] Error streaming audio: %v", err)
				} else {
					log.Printf("[DEBUG] Client disconnected after %d bytes", written)
				}
			} else {
				log.Printf("[DEBUG] Successfully streamed %d bytes", written)
			}
			return
		}

		// Custom streaming loop with explicit flushing to prevent client timeouts
		var totalWritten int64

		// Use smaller initial buffer for faster first byte to client
		initialBuffer := make([]byte, 8*1024)      // 8KB for initial chunks
		streamBuffer := make([]byte, StreamBuffer) // 32KB for subsequent chunks

		// Read and write first chunk with smaller buffer for faster initial response
		n, err := resp.Body.Read(initialBuffer)
		if err != nil && err != io.EOF {
			log.Printf("[ERROR] Error reading initial audio data: %v", err)
			return
		}
		if n > 0 {
			written, writeErr := c.Writer.Write(initialBuffer[:n])
			if writeErr != nil {
				if !strings.Contains(writeErr.Error(), "broken pipe") && !strings.Contains(writeErr.Error(), "connection reset") {
					log.Printf("[ERROR] Error writing initial audio data: %v", writeErr)
				}
				return
			}
			totalWritten += int64(written)
			flusher.Flush() // Flush immediately for fast first byte to client
			log.Printf("[DEBUG] Sent initial %d bytes to client", written)
		}

		// Continue streaming with larger buffer for efficiency
		for {
			n, err := resp.Body.Read(streamBuffer)
			if n > 0 {
				written, writeErr := c.Writer.Write(streamBuffer[:n])
				if writeErr != nil {
					if !strings.Contains(writeErr.Error(), "broken pipe") && !strings.Contains(writeErr.Error(), "connection reset") {
						log.Printf("[ERROR] Error writing audio data: %v", writeErr)
					} else {
						log.Printf("[DEBUG] Client disconnected after %d bytes", totalWritten)
					}
					break
				}
				totalWritten += int64(written)

				// Flush after each chunk to prevent client timeout
				flusher.Flush()
			}

			if err != nil {
				if err == io.EOF {
					break
				}
				if !strings.Contains(err.Error(), "broken pipe") && !strings.Contains(err.Error(), "connection reset") {
					log.Printf("[ERROR] Error reading audio data: %v", err)
				}
				break
			}
		}

		log.Printf("[DEBUG] Successfully streamed %d bytes total", totalWritten)
	}
}

// Constants for security validation
const (
	MaxURLLength = 2048
	StreamBuffer = 32 * 1024 // 32KB buffer for streaming
)

// Shared HTTP client for streaming operations
// Reusing connections improves performance and reduces resource usage
var streamingClient = &http.Client{
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second, // Connection timeout
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second, // Time to receive headers
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,              // Connection pool size
		MaxIdleConnsPerHost:   10,               // Per-host connection limit
		IdleConnTimeout:       90 * time.Second, // Connection idle timeout
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

// isPrivateOrLocalAddress checks if a hostname points to a private or local address
// This prevents SSRF attacks by blocking access to private networks, localhost, and metadata services
func isPrivateOrLocalAddress(hostname string) bool {
	hostname = strings.ToLower(strings.TrimSpace(hostname))

	// Check for obviously local/private hostnames
	if hostname == "localhost" || hostname == "::1" ||
		strings.HasSuffix(hostname, ".local") ||
		strings.HasSuffix(hostname, ".internal") {
		return true
	}

	// Try to parse as IP address
	ip := net.ParseIP(hostname)
	if ip != nil {
		return isPrivateIP(ip)
	}

	// For non-IP hostnames, check common patterns
	// IPv4 patterns that might not parse as IPs
	if hostname == "127.0.0.1" || strings.HasPrefix(hostname, "127.") {
		return true
	}

	// IPv4 private ranges (string-based for hostname patterns)
	if strings.HasPrefix(hostname, "192.168.") {
		return true
	}
	if strings.HasPrefix(hostname, "10.") {
		return true
	}
	if strings.HasPrefix(hostname, "172.") {
		// Check if it's in the 172.16.0.0/12 range (172.16.x.x to 172.31.x.x)
		parts := strings.Split(hostname, ".")
		if len(parts) >= 2 {
			if second, err := strconv.Atoi(parts[1]); err == nil {
				return second >= 16 && second <= 31
			}
		}
	}

	// AWS/Cloud metadata service addresses
	if hostname == "169.254.169.254" || strings.HasPrefix(hostname, "169.254.") {
		return true
	}

	return false
}

// isPrivateIP checks if an IP address is private according to RFC 1918 and related standards
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check IPv4 private ranges
	if ip4 := ip.To4(); ip4 != nil {
		// 10.0.0.0/8
		if ip4[0] == 10 {
			return true
		}
		// 172.16.0.0/12
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		// 192.168.0.0/16
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
		// 169.254.0.0/16 (link-local/APIPA)
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}

	// Check IPv6 private ranges
	if ip.To4() == nil {
		// fc00::/7 (unique local address)
		if len(ip) >= 1 && (ip[0]&0xfe) == 0xfc {
			return true
		}
		// fe80::/10 (link-local)
		if len(ip) >= 2 && ip[0] == 0xfe && (ip[1]&0xc0) == 0x80 {
			return true
		}
	}

	return false
}
