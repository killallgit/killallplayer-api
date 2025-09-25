package episodes

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	episodeService "github.com/killallgit/player-api/internal/services/episodes"
)

// GetByID returns a single episode by Podcast Index ID
// @Summary      Get episode by ID
// @Description  Retrieve a single episode by its Podcast Index ID
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode Podcast Index ID" minimum(1) example(123456789)
// @Success      200 {object} types.SingleEpisodeResponse "Episode details"
// @Failure      400 {object} types.ErrorResponse "Bad request - invalid ID"
// @Failure      404 {object} types.ErrorResponse "Episode not found"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes/{id} [get]
func GetByID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse Podcast Index ID (int64)
		podcastIndexID, ok := types.ParseInt64Param(c, "id")
		if !ok {
			return // Error response already sent by utility
		}

		// Fetch episode
		episode, err := deps.EpisodeService.GetEpisodeByPodcastIndexID(c.Request.Context(), podcastIndexID)
		if err != nil {
			if episodeService.IsNotFound(err) {
				log.Printf("[WARN] Episode not found - Podcast Index ID: %d, Error: %v", podcastIndexID, err)
				c.JSON(http.StatusNotFound, types.ErrorResponse{
					Status:  types.StatusError,
					Message: "Episode not found",
				})
			} else {
				log.Printf("[ERROR] Failed to fetch episode with Podcast Index ID %d: %v", podcastIndexID, err)
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
