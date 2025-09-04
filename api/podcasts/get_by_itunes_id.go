package podcasts

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByiTunesID fetches podcast information by iTunes ID
// GET /api/v1/podcasts/by-itunes-id?id=<itunes_id>
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

		// Call Podcast Index API
		podcastResp, err := deps.PodcastClient.GetPodcastByiTunesID(c.Request.Context(), itunesID)
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
