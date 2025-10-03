package podcasts

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetPodcast returns podcast details by Podcast Index ID
// @Summary      Get podcast details
// @Description  Retrieve detailed information about a specific podcast using its Podcast Index ID.
// @Description  Data is fetched from the database if available, otherwise retrieved from Podcast Index API.
// @Description  Podcast metadata is automatically cached and refreshed if older than 24 hours.
// @Tags         podcasts
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Podcast's Podcast Index ID" minimum(1) example(6780065)
// @Success      200 {object} types.SinglePodcastResponse "Podcast details with full metadata"
// @Failure      400 {object} types.ErrorResponse "Invalid podcast ID format"
// @Failure      404 {object} types.ErrorResponse "Podcast not found"
// @Failure      500 {object} types.ErrorResponse "Failed to fetch podcast"
// @Router       /api/v1/podcasts/{id} [get]
func GetPodcast(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		podcastID, ok := types.ParseInt64Param(c, "id")
		if !ok {
			return
		}

		podcast, err := deps.PodcastService.GetPodcastByPodcastIndexID(c.Request.Context(), podcastID)
		if err != nil {
			log.Printf("[ERROR] Failed to get podcast %d: %v", podcastID, err)
			c.JSON(http.StatusNotFound, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Podcast not found",
				Details: err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, types.SinglePodcastResponse{
			BaseResponse: types.BaseResponse{
				Status:  types.StatusOK,
				Message: "Podcast retrieved successfully",
			},
			Podcast: types.FromModelPodcast(podcast),
		})
	}
}
