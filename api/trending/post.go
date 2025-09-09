package trending

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/podcastindex"
)

// PodcastTrending defines the interface for getting trending podcasts
type PodcastTrending interface {
	GetTrending(ctx context.Context, max, since int, categories []string, lang string, fullText bool) (*podcastindex.SearchResponse, error)
}

// Post handles trending podcasts requests with filters
// @Summary      Get trending podcasts with filters
// @Description  Get trending podcasts with optional category filtering and other parameters
// @Tags         trending
// @Accept       json
// @Produce      json
// @Param        request body models.TrendingRequest true "Trending parameters"
// @Success      200 {object} models.EnhancedTrendingResponse "Enhanced trending response with category summary"
// @Failure      400 {object} object{status=string,message=string,details=string} "Bad request - invalid parameters"
// @Failure      500 {object} object{status=string,message=string,details=string} "Internal server error"
// @Failure      504 {object} object{status=string,message=string} "Gateway timeout - trending request timed out"
// @Router       /api/v1/trending [post]
func Post(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse request body
		var req models.TrendingRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid request format",
				"details": err.Error(),
			})
			return
		}

		// Set defaults
		if req.Max == 0 {
			req.Max = 10
		}
		if req.Since == 0 {
			req.Since = 24 // Default to last 24 hours
		}

		// Validate limits
		if req.Max < 1 || req.Max > 100 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Max must be between 1 and 100",
			})
			return
		}
		if req.Since < 1 || req.Since > 720 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Since must be between 1 and 720 hours (30 days)",
			})
			return
		}

		// Get podcast client from dependencies
		podcastClient, ok := deps.PodcastClient.(PodcastTrending)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Trending service not available",
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		// Get trending podcasts
		results, err := podcastClient.GetTrending(ctx, req.Max, req.Since, req.Categories, req.Lang, req.FullText)
		if err != nil {
			// Check if it's a context timeout
			if ctx.Err() == context.DeadlineExceeded {
				c.JSON(http.StatusGatewayTimeout, gin.H{
					"status":  "error",
					"message": "Trending request timed out",
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"status":  "error",
				"message": "Failed to fetch trending podcasts",
				"details": err.Error(),
			})
			return
		}

		// Process results to enhance categories
		enhancedResults := make([]models.EnhancedPodcast, 0, len(results.Feeds))
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

			enhancedPodcast := models.EnhancedPodcast{
				Podcast:      &podcast,
				CategoryList: categoryList,
			}
			enhancedResults = append(enhancedResults, enhancedPodcast)
		}

		// Build enhanced response
		response := models.EnhancedTrendingResponse{
			Status:          results.Status,
			Results:         enhancedResults,
			CategorySummary: categorySummary,
			TotalCount:      results.Count,
			Since:           req.Since,
			Max:             req.Max,
			Description:     results.Description,
		}

		// Return the enhanced response
		c.JSON(http.StatusOK, response)
	}
}
