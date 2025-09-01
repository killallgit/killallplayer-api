package episodes

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetRecent returns recent episodes across all podcasts
func GetRecent(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		max, _ := strconv.Atoi(c.DefaultQuery("max", "20"))
		if max < 1 || max > 100 {
			max = 20
		}

		episodes, err := deps.EpisodeService.GetRecentEpisodes(c.Request.Context(), max)
		if err != nil {
			log.Printf("[ERROR] Failed to fetch recent episodes (limit %d): %v", max, err)
			c.JSON(http.StatusInternalServerError, deps.EpisodeTransformer.CreateErrorResponse("Failed to fetch recent episodes"))
			return
		}

		response := deps.EpisodeTransformer.CreateSuccessResponse(episodes, "Recent episodes across all podcasts")
		c.JSON(http.StatusOK, response)
	}
}
