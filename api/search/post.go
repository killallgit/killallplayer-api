package search

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/podcastindex"
)

// PodcastSearcher defines the interface for searching podcasts
type PodcastSearcher interface {
	Search(ctx context.Context, query string, limit int) (*podcastindex.SearchResponse, error)
}

// Post handles podcast search requests
// @Summary      Search for podcasts
// @Description  Search for podcasts by query string with optional result limit
// @Tags         search
// @Accept       json
// @Produce      json
// @Param        request body models.SearchRequest true "Search parameters"
// @Success      200 {object} podcastindex.SearchResponse "Podcast Index search response"
// @Failure      400 {object} object{status=string,message=string,details=string} "Bad request - invalid parameters"
// @Failure      500 {object} object{status=string,message=string,details=string} "Internal server error"
// @Failure      504 {object} object{status=string,message=string} "Gateway timeout - search request timed out"
// @Router       /api/v1/search [post]
func Post(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse request body
		var req models.SearchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid request format",
				"details": err.Error(),
			})
			return
		}

		// Validate query
		if req.Query == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Search query is required",
			})
			return
		}

		// Set default limit if not provided
		if req.Limit == 0 {
			req.Limit = 10
		}

		// Validate limit
		if req.Limit < 1 || req.Limit > 100 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Limit must be between 1 and 100",
			})
			return
		}

		// Get podcast client from dependencies
		podcastClient, ok := deps.PodcastClient.(PodcastSearcher)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Search service not available",
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		// Perform search
		results, err := podcastClient.Search(ctx, req.Query, req.Limit)
		if err != nil {
			// Check if it's a context timeout
			if ctx.Err() == context.DeadlineExceeded {
				c.JSON(http.StatusGatewayTimeout, gin.H{
					"status":  "error",
					"message": "Search request timed out",
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to search podcasts",
				"details": err.Error(),
			})
			return
		}

		// Return the full Podcast Index response
		c.JSON(http.StatusOK, results)
	}
}
