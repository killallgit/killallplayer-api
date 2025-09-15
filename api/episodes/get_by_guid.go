package episodes

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByGUID returns episode by GUID
// @Summary      Get episode by GUID
// @Description  Retrieve a single episode by its GUID
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        guid query string true "Episode GUID"
// @Success      200 {object} types.SingleEpisodeResponse "Episode details"
// @Failure      400 {object} types.ErrorResponse "Bad request - missing GUID"
// @Failure      404 {object} types.ErrorResponse "Episode not found"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes/by-guid [get]
func GetByGUID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		guid := c.Query("guid")
		if guid == "" {
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "GUID parameter is required",
			})
			return
		}

		episode, err := deps.EpisodeService.GetEpisodeByGUID(c.Request.Context(), guid)
		if err != nil {
			if IsNotFound(err) {
				c.JSON(http.StatusNotFound, types.ErrorResponse{
					Status:  types.StatusError,
					Message: "Episode not found",
				})
			} else {
				log.Printf("[ERROR] Failed to fetch episode by GUID %s: %v", guid, err)
				c.JSON(http.StatusInternalServerError, types.ErrorResponse{
					Status:  types.StatusError,
					Message: "Failed to fetch episode",
					Details: err.Error(),
				})
			}
			return
		}

		// Convert to unified Episode format
		pieFormat := deps.EpisodeTransformer.ModelToPodcastIndex(episode)
		unifiedEpisode := types.FromServiceEpisode(&pieFormat)

		// Return unified response
		c.JSON(http.StatusOK, types.SingleEpisodeResponse{
			BaseResponse: types.BaseResponse{
				Status:  types.StatusOK,
				Message: "Episode found",
			},
			Episode: unifiedEpisode,
		})
	}
}
