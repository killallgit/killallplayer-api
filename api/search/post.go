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
	Search(ctx context.Context, query string, limit int, fullText bool) (*podcastindex.SearchResponse, error)
}

// Post handles podcast search requests
// @Summary      Search for podcasts
// @Description  Search for podcasts by query string with optional result limit and full text
// @Tags         search
// @Accept       json
// @Produce      json
// @Param        request body models.SearchRequest true "Search parameters"
// @Success      200 {object} models.PodcastSearchResponse "Podcast search response with category summary"
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
		results, err := podcastClient.Search(ctx, req.Query, req.Limit, req.FullText)
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

		// Process results to enhance categories
		podcastResults := make([]models.PodcastResponse, 0, len(results.Feeds))
		categorySummary := make(map[string]int)

		for _, podcast := range results.Feeds {
			// Convert categories map to list
			categoryList := make([]string, 0, len(podcast.Categories))
			for _, catName := range podcast.Categories {
				if catName != "" {
					categoryList = append(categoryList, catName)
					categorySummary[catName]++
				}
			}

			podcastResponse := models.PodcastResponse{
				Podcast:      &podcast,
				CategoryList: categoryList,
			}
			podcastResults = append(podcastResults, podcastResponse)
		}

		// Build search response
		response := models.PodcastSearchResponse{
			Status:          results.Status,
			Query:           req.Query,
			Results:         podcastResults,
			CategorySummary: categorySummary,
			TotalCount:      results.Count,
			Description:     results.Description,
		}

		// Return the enhanced response
		c.JSON(http.StatusOK, response)
	}
}
