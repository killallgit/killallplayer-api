package podcasts

import (
	"net/http"
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByFeedURL fetches podcast information by feed URL
// @Summary      Get podcast by feed URL
// @Description  Get podcast information using its RSS feed URL from Podcast Index API
// @Tags         podcasts
// @Accept       json
// @Produce      json
// @Param        url query string true "RSS feed URL of the podcast" format(url) example("https://feeds.buzzsprout.com/123456.rss")
// @Success      200 {object} object{status=string,data=object} "Podcast information"
// @Failure      400 {object} object{error=string} "Bad request - missing or invalid feed URL"
// @Failure      500 {object} object{error=string} "Internal server error"
// @Router       /api/v1/podcasts/by-feed-url [get]
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
