package trending

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
)

// Get returns trending podcasts from Podcast Index API
// @Summary      Get trending podcasts
// @Description  Get a list of trending podcasts from the Podcast Index API
// @Tags         trending
// @Accept       json
// @Produce      json
// @Param        limit query int false "Number of podcasts to return (1-100)" minimum(1) maximum(100) default(20)
// @Success      200 {object} models.PodcastTrendingResponse "Podcast trending response with category summary"
// @Failure      500 {object} object{status=string,description=string} "Internal server error"
// @Router       /api/v1/trending [get]
func Get(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get limit from query params with default
		limitStr := c.DefaultQuery("limit", "20")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 20
		}
		if limit > 100 {
			limit = 100
		}

		// Call Podcast Index trending endpoint with defaults
		trending, err := deps.PodcastClient.GetTrending(c.Request.Context(), limit, 24, nil, "en", false)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":      "false",
				"description": "Failed to fetch trending podcasts",
			})
			return
		}

		// Process results to enhance categories
		podcastResults := make([]models.PodcastResponse, 0, len(trending.Feeds))
		categorySummary := make(map[string]int)

		for _, podcast := range trending.Feeds {
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

		// Build trending response
		response := models.PodcastTrendingResponse{
			Status:          trending.Status,
			Results:         podcastResults,
			CategorySummary: categorySummary,
			TotalCount:      trending.Count,
			Since:           24, // Hours parameter used
			Max:             limit,
			Description:     trending.Description,
		}

		// Return the enhanced response
		c.JSON(http.StatusOK, response)
	}
}
