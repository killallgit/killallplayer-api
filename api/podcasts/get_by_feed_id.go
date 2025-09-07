package podcasts

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByFeedID fetches podcast information by feed ID
// @Summary      Get podcast by feed ID
// @Description  Get podcast information using its feed ID from Podcast Index API
// @Tags         podcasts
// @Accept       json
// @Produce      json
// @Param        id query int true "Podcast feed ID" minimum(1) example(6780065)
// @Success      200 {object} object{status=string,data=object} "Podcast information"
// @Failure      400 {object} object{error=string} "Bad request - missing or invalid feed ID"
// @Failure      500 {object} object{error=string} "Internal server error"
// @Router       /api/v1/podcasts/by-feed-id [get]
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
