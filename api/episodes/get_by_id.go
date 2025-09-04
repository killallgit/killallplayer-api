package episodes

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByID returns a single episode by Podcast Index ID with waveform status
// @Summary      Get episode by ID
// @Description  Retrieve a single episode by its Podcast Index ID with waveform status
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        id path int true "Episode Podcast Index ID" minimum(1) example(123456789)
// @Success      200 {object} episodes.EpisodeByGUIDEnhancedResponse "Episode details with waveform"
// @Failure      400 {object} episodes.PodcastIndexErrorResponse "Bad request - invalid ID"
// @Failure      404 {object} episodes.PodcastIndexErrorResponse "Episode not found"
// @Failure      500 {object} episodes.PodcastIndexErrorResponse "Internal server error"
// @Router       /api/v1/episodes/{id} [get]
func GetByID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")
		log.Printf("[DEBUG] GetByID called with Podcast Index ID: %s", episodeIDStr)

		// Parse Podcast Index ID (int64)
		podcastIndexID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil {
			log.Printf("[ERROR] Invalid episode ID '%s': %v", episodeIDStr, err)
			c.JSON(http.StatusBadRequest, deps.EpisodeTransformer.CreateErrorResponse("Invalid episode ID"))
			return
		}

		log.Printf("[DEBUG] Fetching episode with Podcast Index ID: %d", podcastIndexID)
		episode, err := deps.EpisodeService.GetEpisodeByPodcastIndexID(c.Request.Context(), podcastIndexID)
		if err != nil {
			if IsNotFound(err) {
				log.Printf("[WARN] Episode not found - Podcast Index ID: %d, Error: %v", podcastIndexID, err)
				c.JSON(http.StatusNotFound, deps.EpisodeTransformer.CreateErrorResponse("Episode not found"))
			} else {
				log.Printf("[ERROR] Failed to fetch episode with Podcast Index ID %d: %v", podcastIndexID, err)
				c.JSON(http.StatusInternalServerError, deps.EpisodeTransformer.CreateErrorResponse("Failed to fetch episode"))
			}
			return
		}

		log.Printf("[DEBUG] Episode found - Podcast Index ID: %d, Title: %s, AudioURL: %s",
			podcastIndexID, episode.Title, episode.AudioURL)

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
