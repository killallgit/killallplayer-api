package recent

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetEpisodes fetches the most recent episodes globally
// GET /api/v1/recent/episodes?limit=<limit>
func GetEpisodes(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse limit parameter
		limit := 25 // default
		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		// Call Podcast Index API
		episodesResp, err := deps.PodcastClient.GetRecentEpisodes(c.Request.Context(), limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to fetch recent episodes",
			})
			return
		}

		// TODO: Add enrichment
		// - Merge with local playback progress data
		// - Add local user favorites/bookmarks
		// - Filter based on user preferences
		// - Add local episode availability status

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data": gin.H{
				"episodes": episodesResp.Items,
				"count":    episodesResp.Count,
				"limit":    limit,
			},
		})
	}
}
