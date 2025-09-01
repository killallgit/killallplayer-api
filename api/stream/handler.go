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

		// Create HTTP client to fetch audio
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				// Allow up to 10 redirects
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}

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

		// Add user agent
		req.Header.Set("User-Agent", "PodcastPlayerAPI/1.0")

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

// HandleOptions handles preflight OPTIONS requests for CORS
func HandleOptions() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, HEAD, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Range, Content-Type")
		c.Header("Access-Control-Max-Age", "86400")
		c.Status(http.StatusNoContent)
	}
}