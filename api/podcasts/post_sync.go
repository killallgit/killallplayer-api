package podcasts

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// PostSync manually triggers episode sync from Podcast Index
func PostSync(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		podcastIDStr := c.Param("id")
		podcastID, err := strconv.ParseInt(podcastIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, deps.EpisodeTransformer.CreateErrorResponse("Invalid podcast ID"))
			return
		}

		max, _ := strconv.Atoi(c.DefaultQuery("max", "50"))
		if max < 1 || max > 1000 {
			max = 50
		}

		// Fetch and sync
		response, err := deps.EpisodeService.FetchAndSyncEpisodes(c.Request.Context(), podcastID, max)
		if err != nil {
			log.Printf("[ERROR] Failed to sync episodes for podcast %d: %v", podcastID, err)
			c.JSON(http.StatusInternalServerError, deps.EpisodeTransformer.CreateErrorResponse("Failed to sync episodes"))
			return
		}

		c.JSON(http.StatusOK, response)
	}
}
