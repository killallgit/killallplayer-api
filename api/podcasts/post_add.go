package podcasts

import (
	"net/url"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// PostAdd adds a podcast to the index by feed URL
// @Summary      Add podcast to index
// @Description  Add a new podcast to the Podcast Index by providing its RSS feed URL
// @Tags         podcasts
// @Accept       json
// @Produce      json
// @Param        request body object{url=string} true "Feed URL to add" example({"url": "https://feeds.example.com/podcast.rss"})
// @Success      200 {object} object{status=string,message=string,data=object} "Podcast added successfully"
// @Failure      400 {object} object{status=string,message=string} "Bad request - missing or invalid feed URL"
// @Failure      500 {object} object{status=string,message=string} "Internal server error"
// @Router       /api/v1/podcasts/add [post]
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
