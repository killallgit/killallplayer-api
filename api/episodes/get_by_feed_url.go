package episodes

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByFeedURL fetches episodes for a podcast by feed URL
// GET /api/v1/episodes/by-feed-url?url=<feed_url>&limit=<limit>
func GetByFeedURL(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		feedURL := c.Query("url")
		if feedURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "feed URL parameter 'url' is required",
			})
			return
		}

		// Validate URL format
		if _, err := url.Parse(feedURL); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid feed URL format",
			})
			return
		}

		// Parse limit parameter
		limit := 25 // default
		if limitStr := c.Query("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
				limit = l
			}
		}

		// Call Podcast Index API
		episodesResp, err := deps.PodcastClient.GetEpisodesByFeedURL(c.Request.Context(), feedURL, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to fetch episodes",
			})
			return
		}

		// TODO: Add enrichment - merge with local data, playback progress, etc.

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
