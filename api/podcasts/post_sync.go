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
		podcastID, ok := types.ParseInt64Param(c, "id")
		if !ok {
			return // Error response already sent by utility
		}

		max, _ := strconv.Atoi(c.DefaultQuery("max", "50"))
		if max < 1 || max > 1000 {
			max = 50
		}

		// Fetch and sync
		response, err := deps.EpisodeService.FetchAndSyncEpisodes(c.Request.Context(), podcastID, max)
		if err != nil {
			log.Printf("[ERROR] Failed to sync episodes for podcast %d: %v", podcastID, err)
			types.SendInternalError(c, "Failed to sync episodes")
			return
		}

		c.JSON(http.StatusOK, response)
	}
}
