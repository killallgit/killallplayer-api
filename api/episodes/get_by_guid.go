package episodes

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByGUID returns episode by GUID with waveform status
// @Summary      Get episode by GUID
// @Description  Retrieve a single episode by its GUID with waveform status
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        guid query string true "Episode GUID"
// @Success      200 {object} episodes.EpisodeByGUIDResponse "Episode details with waveform"
// @Failure      400 {object} episodes.PodcastIndexErrorResponse "Bad request - missing GUID"
// @Failure      404 {object} episodes.PodcastIndexErrorResponse "Episode not found"
// @Failure      500 {object} episodes.PodcastIndexErrorResponse "Internal server error"
// @Router       /api/v1/episodes/by-guid [get]
func GetByGUID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		guid := c.Query("guid")
		if guid == "" {
			c.JSON(http.StatusBadRequest, deps.EpisodeTransformer.CreateErrorResponse("GUID parameter is required"))
			return
		}

		episode, err := deps.EpisodeService.GetEpisodeByGUID(c.Request.Context(), guid)
		if err != nil {
			if IsNotFound(err) {
				c.JSON(http.StatusNotFound, deps.EpisodeTransformer.CreateErrorResponse("Episode not found"))
			} else {
				log.Printf("[ERROR] Failed to fetch episode by GUID %s: %v", guid, err)
				c.JSON(http.StatusInternalServerError, deps.EpisodeTransformer.CreateErrorResponse("Failed to fetch episode"))
			}
			return
		}

		// Convert to Podcast Index format and enrich with waveform
		pieFormat := deps.EpisodeTransformer.ModelToPodcastIndex(episode)
		enricher := NewEpisodeEnricher(deps)
		enhanced := enricher.EnrichSingleEpisodeWithWaveform(c.Request.Context(), &pieFormat)

		// Wrap in standard response format
		response := EpisodeByGUIDResponse{
			Status:      "true",
			Episode:     enhanced,
			Description: "Episode found",
		}
		c.JSON(http.StatusOK, response)
	}
}
