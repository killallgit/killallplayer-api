package stream

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// StreamEpisode handles audio streaming with range request support
// @Summary      Stream episode audio
// @Description  Stream audio for an episode by its Podcast Index ID with HTTP range request support (seeking)
// @Tags         streaming
// @Accept       json
// @Produce      audio/mpeg
// @Param        id path int true "Episode Podcast Index ID" minimum(1) example(123456789)
// @Param        Range header string false "HTTP Range header for partial content requests" example("bytes=0-1023")
// @Success      200 "Full audio content"
// @Success      206 "Partial audio content (range request)"
// @Failure      400 {object} object{error=string} "Bad request - invalid episode ID"
// @Failure      404 {object} object{error=string} "Episode not found or no audio available"
// @Failure      502 {object} object{error=string} "Bad gateway - audio source error"
// @Failure      500 {object} object{error=string} "Internal server error"
// @Router       /api/v1/stream/{id} [get]
// @Router       /api/v1/stream/{id} [head]
func StreamEpisode(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")
		log.Printf("[DEBUG] Stream request for episode ID: %s", episodeIDStr)

		// Parse Podcast Index ID (int64)
		podcastIndexID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil {
			log.Printf("[ERROR] Invalid episode ID for streaming '%s': %v", episodeIDStr, err)
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode ID"})
			return
		}

		// Fetch episode from database using Podcast Index ID
		log.Printf("[DEBUG] Fetching episode with Podcast Index ID %d for streaming", podcastIndexID)
		episode, err := deps.EpisodeService.GetEpisodeByPodcastIndexID(c.Request.Context(), podcastIndexID)
		if err != nil {
			log.Printf("[ERROR] Episode not found for streaming - Podcast Index ID: %d, Error: %v", podcastIndexID, err)
			c.JSON(http.StatusNotFound, gin.H{"error": "Episode not found"})
			return
		}

		if episode.AudioURL == "" {
			log.Printf("[ERROR] Episode with Podcast Index ID %d has no audio URL", podcastIndexID)
			c.JSON(http.StatusNotFound, gin.H{"error": "Audio not available for this episode"})
			return
		}

		log.Printf("[DEBUG] Streaming audio from URL: %s", episode.AudioURL)

		// Use shared HTTP client for better connection reuse and performance
		// (client defined in direct.go)

		// Create request with range header if present
		req, err := http.NewRequestWithContext(c.Request.Context(), "GET", episode.AudioURL, nil)
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
		req.Header.Set("Referer", episode.Link) // Use episode link as referer if available

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
			log.Printf("[ERROR] Received HTML instead of audio. Content-Type: %s, URL: %s", contentType, episode.AudioURL)
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

		// Only set chunked encoding if no Content-Length header is present
		if resp.Header.Get("Content-Length") == "" {
			c.Header("Transfer-Encoding", "chunked")
		}

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

// HandleOptions handles preflight OPTIONS requests for CORS
// @Summary      Handle CORS preflight
// @Description  Handle CORS preflight requests for streaming endpoints
// @Tags         streaming
// @Accept       json
// @Produce      json
// @Success      204 "No content - CORS preflight successful"
// @Router       /api/v1/stream/{id} [options]
// @Router       /api/v1/stream/direct [options]
func HandleOptions() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Range, Content-Type")
		c.Header("Access-Control-Max-Age", "86400")
		c.Status(http.StatusNoContent)
	}
}
