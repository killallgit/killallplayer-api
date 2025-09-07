package episodes

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetRecent returns recent episodes across all podcasts
// @Summary      Get recent episodes
// @Description  Get the most recent episodes across all podcasts, sorted by publish date
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        max query int false "Maximum number of episodes to return (1-100)" minimum(1) maximum(100) default(20)
// @Success      200 {object} episodes.PodcastIndexResponse "List of recent episodes"
// @Failure      500 {object} episodes.PodcastIndexErrorResponse "Internal server error"
// @Router       /api/v1/episodes/recent [get]
func GetRecent(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		max, _ := strconv.Atoi(c.DefaultQuery("max", "20"))
		if max < 1 || max > 100 {
			max = 20
		}

		episodes, err := deps.EpisodeService.GetRecentEpisodes(c.Request.Context(), max)
		if err != nil {
			log.Printf("[ERROR] Failed to fetch recent episodes (limit %d): %v", max, err)
			types.SendInternalError(c, "Failed to fetch recent episodes")
			return
		}

		response := deps.EpisodeTransformer.CreateSuccessResponse(episodes, "Recent episodes across all podcasts")
		c.JSON(http.StatusOK, response)
	}
}
