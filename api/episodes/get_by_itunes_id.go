package episodes

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByiTunesID fetches episodes for a podcast by iTunes ID
// GET /api/v1/episodes/by-itunes-id?id=<itunes_id>&limit=<limit>
func GetByiTunesID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		itunesIDStr := c.Query("id")
		if itunesIDStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "iTunes ID parameter 'id' is required",
			})
			return
		}

		itunesID, err := strconv.ParseInt(itunesIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid iTunes ID format, must be a number",
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
		episodesResp, err := deps.PodcastClient.GetEpisodesByiTunesID(c.Request.Context(), itunesID, limit)
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
