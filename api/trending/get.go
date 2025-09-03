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
// @Success      200 {object} object{status=string,podcasts=array,count=int,description=string} "List of trending podcasts"
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

		// Call Podcast Index trending endpoint
		trending, err := deps.PodcastClient.GetTrending(limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":      "false",
				"description": "Failed to fetch trending podcasts",
			})
			return
		}

		// Return the trending feeds directly from Podcast Index
		c.JSON(http.StatusOK, gin.H{
			"status":      "true",
			"podcasts":    trending.Feeds,
			"count":       len(trending.Feeds),
			"description": "Trending podcasts",
		})
	}
}
