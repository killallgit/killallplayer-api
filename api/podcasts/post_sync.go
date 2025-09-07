package podcasts

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// PostSync manually triggers episode sync from Podcast Index
// @Summary      Sync episodes for podcast
// @Description  Manually trigger synchronization of episodes from Podcast Index API for a specific podcast
// @Tags         podcasts
// @Accept       json
// @Produce      json
// @Param        id path int true "Podcast ID" minimum(1) example(6780065)
// @Param        max query int false "Maximum number of episodes to sync (1-1000)" minimum(1) maximum(1000) default(50)
// @Success      200 {object} episodes.PodcastIndexResponse "Episodes successfully synced"
// @Failure      400 {object} episodes.PodcastIndexErrorResponse "Bad request - invalid podcast ID"
// @Failure      500 {object} episodes.PodcastIndexErrorResponse "Internal server error"
// @Router       /api/v1/podcasts/{id}/episodes/sync [post]
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
