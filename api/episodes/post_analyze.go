package episodes

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// AnalysisResponse represents the response from volume analysis
type AnalysisResponse struct {
	EpisodeID    int64    `json:"episode_id" example:"12345"`
	ClipsCreated int      `json:"clips_created" example:"3"`
	ClipUUIDs    []string `json:"clip_uuids" example:"052f3b9b-cc02-418c-a9ab-8f49534c01c8,123e4567-e89b-12d3-a456-426614174000"`
	Message      string   `json:"message" example:"Successfully analyzed episode and created 3 clips from volume spikes"`
}

// @Summary Analyze episode for volume spikes
// @Description Scans the entire episode audio for volume anomalies (loud sections that may be ads or music) and automatically creates clips from detected spikes. Uses cached audio if available to avoid re-downloading. Created clips are labeled as 'volume_spike' for review.
// @Tags episodes
// @Produce json
// @Param id path int true "Podcast Index Episode ID"
// @Success 200 {object} AnalysisResponse "Analysis completed successfully with list of created clip UUIDs"
// @Failure 400 {object} types.ErrorResponse "Invalid episode ID"
// @Failure 404 {object} types.ErrorResponse "Episode not found"
// @Failure 500 {object} types.ErrorResponse "Analysis failed"
// @Router /api/v1/episodes/{id}/analyze [post]
func AnalyzeVolumeSpikes(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil {
			types.SendBadRequest(c, "Invalid episode ID")
			return
		}

		clipUUIDs, err := deps.EpisodeAnalysisService.AnalyzeAndCreateClips(c.Request.Context(), episodeID)
		if err != nil {
			types.SendInternalError(c, err.Error())
			return
		}

		message := "No volume spikes detected"
		if len(clipUUIDs) > 0 {
			message = "Successfully analyzed episode and created clips from volume spikes"
		}

		c.JSON(http.StatusOK, AnalysisResponse{
			EpisodeID:    episodeID,
			ClipsCreated: len(clipUUIDs),
			ClipUUIDs:    clipUUIDs,
			Message:      message,
		})
	}
}
