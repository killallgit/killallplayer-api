package handlers

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/internal/models"
)

// SearchHandler handles podcast search requests
type SearchHandler struct {
	podcastClient PodcastSearcher
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(client PodcastSearcher) *SearchHandler {
	return &SearchHandler{
		podcastClient: client,
	}
}


// HandleSearch handles the search endpoint using Gin
func (h *SearchHandler) HandleSearch(c *gin.Context) {
	// Parse request body
	var req models.SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Invalid request format",
		})
		return
	}

	// Validate request
	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Query parameter is required",
		})
		return
	}

	// Validate and set limit
	if req.Limit <= 0 {
		req.Limit = 10 // Default limit
	} else if req.Limit > 100 {
		c.JSON(http.StatusBadRequest, gin.H{
			"status":  "error",
			"message": "Limit must be between 1 and 100",
		})
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	// Search via Podcast Index API
	searchResp, err := h.podcastClient.Search(ctx, req.Query, req.Limit)
	if err != nil {
		// Log the actual error for debugging while returning generic message to client
		log.Printf("Search error for query '%s': %v", req.Query, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"status":  "error",
			"message": "Failed to search podcasts",
		})
		return
	}

	// Convert to our response format
	response := models.SearchResponse{
		Podcasts: make([]models.PodcastSearchResult, 0, len(searchResp.Feeds)),
	}

	for _, feed := range searchResp.Feeds {
		response.Podcasts = append(response.Podcasts, models.PodcastSearchResult{
			ID:          strconv.Itoa(feed.ID),
			Title:       feed.Title,
			Author:      feed.Author,
			Description: feed.Description,
			Image:       feed.Image,
			URL:         feed.URL,
		})
	}

	// Send response
	c.JSON(http.StatusOK, response)
}

