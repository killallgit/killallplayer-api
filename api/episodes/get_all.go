package episodes

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetAll returns recent episodes (acts as the main episodes endpoint)
func GetAll(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse limit parameter
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		if limit < 1 || limit > 1000 {
			limit = 50
		}

		// For now, use GetRecentEpisodes as our "all episodes" endpoint
		// This returns the most recent episodes across all podcasts
		episodes, err := deps.EpisodeService.GetRecentEpisodes(c.Request.Context(), limit)
		if err != nil {
			log.Printf("[ERROR] Failed to fetch episodes (limit %d): %v", limit, err)
			c.JSON(http.StatusInternalServerError, deps.EpisodeTransformer.CreateErrorResponse("Failed to fetch episodes"))
			return
		}

		// Transform and return
		response := deps.EpisodeTransformer.CreateSuccessResponse(episodes, "All episodes")
		c.JSON(http.StatusOK, response)
	}
}
