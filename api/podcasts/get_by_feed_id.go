package podcasts

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByFeedID fetches podcast information by feed ID
// GET /api/v1/podcasts/by-feed-id?id=<feed_id>
func GetByFeedID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		feedIDStr := c.Query("id")
		if feedIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "feed ID parameter 'id' is required",
			})
			return
		}

		feedID, err := strconv.ParseInt(feedIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid feed ID format, must be a number",
			})
			return
		}

		// Call Podcast Index API
		podcastResp, err := deps.PodcastClient.GetPodcastByFeedID(c.Request.Context(), feedID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to fetch podcast information",
			})
			return
		}

		// TODO: Add enrichment - store in database, merge with local data, etc.

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   podcastResp.Feed,
		})
	}
}
