package episodes

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByiTunesID fetches episodes for a podcast by iTunes ID
// @Summary      Get episodes by iTunes ID
// @Description  Get episodes for a specific podcast using its iTunes/Apple Podcasts ID from Podcast Index API
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        id query int true "iTunes/Apple Podcasts ID" minimum(1) example(1234567890)
// @Param        limit query int false "Maximum number of episodes to return (1-100)" minimum(1) maximum(100) default(25)
// @Success      200 {object} object{status=string,data=object{episodes=[]object,count=int,limit=int}} "Episodes from iTunes podcast"
// @Failure      400 {object} object{error=string} "Bad request - missing or invalid iTunes ID"
// @Failure      500 {object} object{error=string} "Internal server error"
// @Router       /api/v1/episodes/by-itunes-id [get]
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
