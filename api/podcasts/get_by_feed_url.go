package podcasts

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByFeedURL fetches podcast information by feed URL
// GET /api/v1/podcasts/by-feed-url?url=<feed_url>
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

		// Call Podcast Index API
		podcastResp, err := deps.PodcastClient.GetPodcastByFeedURL(c.Request.Context(), feedURL)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "failed to fetch podcast information",
			})
			return
		}

		// TODO: Add enrichment - store in database, merge with local data, etc.
		// For now, return the Podcast Index response directly

		c.JSON(http.StatusOK, gin.H{
			"status": "success",
			"data":   podcastResp.Feed,
		})
	}
}
