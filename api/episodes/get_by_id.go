package episodes

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByID returns a single episode by Podcast Index ID with waveform status
// @Summary      Get episode by ID with waveform status
// @Description  Retrieve a single episode by its Podcast Index ID including waveform processing status
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        id path int true "Episode Podcast Index ID" minimum(1) example(123456789)
// @Success      200 {object} EpisodeByGUIDEnhancedResponse "Episode details with waveform status"
// @Failure      400 {object} episodes.PodcastIndexErrorResponse "Bad request - invalid ID"
// @Failure      404 {object} episodes.PodcastIndexErrorResponse "Episode not found"
// @Failure      500 {object} episodes.PodcastIndexErrorResponse "Internal server error"
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
			if IsNotFound(err) {
				log.Printf("[WARN] Episode not found - Podcast Index ID: %d, Error: %v", podcastIndexID, err)
				types.SendNotFound(c, "Episode not found")
			} else {
				log.Printf("[ERROR] Failed to fetch episode with Podcast Index ID %d: %v", podcastIndexID, err)
				types.SendInternalError(c, "Failed to fetch episode")
			}
			return
		}

		// Convert to Podcast Index format and enrich with waveform
		pieFormat := deps.EpisodeTransformer.ModelToPodcastIndex(episode)
		enricher := NewEpisodeEnricher(deps)
		enhanced := enricher.EnrichSingleEpisodeWithWaveform(c.Request.Context(), &pieFormat)

		// Wrap in standard response format
		response := EpisodeByGUIDEnhancedResponse{
			Status:      "true",
			Episode:     enhanced,
			Description: "Episode found",
		}
		c.JSON(http.StatusOK, response)
	}
}

// IsNotFound checks if the error indicates a not found condition
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "episode not found" || err.Error() == "record not found"
}
