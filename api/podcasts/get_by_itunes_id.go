package podcasts

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByiTunesID fetches podcast information by iTunes ID
// @Summary      Get podcast by iTunes ID
// @Description  Get podcast information using its iTunes/Apple Podcasts ID from Podcast Index API
// @Tags         podcasts
// @Accept       json
// @Produce      json
// @Param        id query int true "iTunes/Apple Podcasts ID" minimum(1) example(1234567890)
// @Success      200 {object} object{status=string,data=object} "Podcast information"
// @Failure      400 {object} object{status=string,message=string} "Bad request - missing or invalid iTunes ID"
// @Failure      500 {object} object{status=string,message=string} "Internal server error"
// @Router       /api/v1/podcasts/by-itunes-id [get]
func GetByiTunesID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		itunesIDStr := c.Query("id")
		if itunesIDStr == "" {
			types.SendBadRequest(c, "iTunes ID parameter 'id' is required")
			return
		}

		itunesID, err := strconv.ParseInt(itunesIDStr, 10, 64)
		if err != nil {
			types.SendBadRequest(c, "invalid iTunes ID format, must be a number")
			return
		}

		// Call Podcast Index API
		podcastResp, err := deps.PodcastClient.GetPodcastByiTunesID(c.Request.Context(), itunesID)
		if err != nil {
			types.SendInternalError(c, "failed to fetch podcast information")
			return
		}

		types.SendSuccess(c, gin.H{
			"status": "success",
			"data":   podcastResp.Feed,
		})
	}
}
