package episodes

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByFeedURL fetches episodes for a podcast by feed URL
// @Summary      Get episodes by feed URL
// @Description  Get episodes for a specific podcast using its RSS feed URL
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        url query string true "RSS feed URL of the podcast" format(url) example("https://feeds.buzzsprout.com/123456.rss")
// @Param        limit query int false "Maximum number of episodes to return (1-100)" minimum(1) maximum(100) default(20)
// @Success      200 {object} podcastindex.SearchResponse "Podcast Index episodes response"
// @Failure      400 {object} object{error=string} "Bad request - missing or invalid feed URL"
// @Failure      500 {object} object{error=string} "Internal server error"
// @Router       /api/v1/episodes/by-feed-url [get]
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
