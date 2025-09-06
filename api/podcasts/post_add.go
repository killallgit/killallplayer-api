package podcasts

import (
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// PostAdd adds a podcast to the index by feed URL
// POST /api/v1/podcasts/add
// Body: {"url": "feed_url"}
func PostAdd(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		var request struct {
			URL string `json:"url" binding:"required"`
		}

		if err := c.ShouldBindJSON(&request); err != nil {
			types.SendBadRequest(c, "invalid request body, 'url' field is required")
			return
		}

		// Validate URL format
		if _, err := url.Parse(request.URL); err != nil {
			types.SendBadRequest(c, "invalid feed URL format")
			return
		}

		// Call Podcast Index API
		addResp, err := deps.PodcastClient.AddPodcastByFeedURL(c.Request.Context(), request.URL)
		if err != nil {
			types.SendInternalError(c, "failed to add podcast to index")
			return
		}

		types.SendSuccess(c, gin.H{
			"status":  "success",
			"message": "podcast added to index successfully",
			"data":    addResp,
		})
	}
}
