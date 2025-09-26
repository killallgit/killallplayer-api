package episodes

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	episodeService "github.com/killallgit/player-api/internal/services/episodes"
)

// GetByID returns a single episode by Podcast Index ID
// @Summary      Get episode details by Podcast Index ID
// @Description  Retrieve comprehensive episode information including title, description, audio URL, duration,
// @Description  and links to additional resources like transcripts and chapters. The episode data is fetched
// @Description  from the local database cache or Podcast Index API if not cached. Audio URLs are direct links
// @Description  suitable for streaming or download.
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode's Podcast Index ID (unique identifier from Podcast Index API)" minimum(1) example(16797088990)
// @Success      200 {object} types.SingleEpisodeResponse "Episode details including audio URL and metadata"
// @Failure      400 {object} types.ErrorResponse "Invalid ID format (must be positive integer)"
// @Failure      404 {object} types.ErrorResponse "Episode not found in database or Podcast Index API"
// @Failure      500 {object} types.ErrorResponse "Internal server error or API communication failure"
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
