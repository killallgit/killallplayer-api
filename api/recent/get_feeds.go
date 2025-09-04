package recent

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetFeeds fetches the most recently updated feeds
// GET /api/v1/recent/feeds?limit=<limit>
func GetFeeds(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse limit parameter
		limit := 25 // default
		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		// Call Podcast Index API
		feedsResp, err := deps.PodcastClient.GetRecentFeeds(c.Request.Context(), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to fetch recent feeds",
			})
			return
		}

		// TODO: Add enrichment
		// - Check if feeds are already in local database
		// - Add local subscription status for user
		// - Add local episode counts/statistics
		// - Filter based on user preferences

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"feeds": feedsResp.Feeds,
				"count": feedsResp.Count,
				"limit": limit,
			},
		})
	}
}
