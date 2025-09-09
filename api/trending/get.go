package trending

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// Get returns trending podcasts from Podcast Index API
// @Summary      Get trending podcasts
// @Description  Get a list of trending podcasts from the Podcast Index API
// @Tags         trending
// @Accept       json
// @Produce      json
// @Param        limit query int false "Number of podcasts to return (1-100)" minimum(1) maximum(100) default(20)
// @Success      200 {object} podcastindex.SearchResponse "Podcast Index trending response"
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

		// Return the full Podcast Index response
		c.JSON(http.StatusOK, trending)
	}
}
