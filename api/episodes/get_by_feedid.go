package episodes

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetByFeedID returns episodes by podcast/feed ID
// @Summary      Get episodes by feed ID
// @Description  Get episodes for a specific podcast using its feed ID (query parameter format for Podcast Index API compatibility)
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        id query int true "Podcast feed ID" minimum(1) example(6780065)
// @Param        max query int false "Maximum number of episodes to return (1-1000)" minimum(1) maximum(1000) default(20)
// @Success      200 {object} episodes.PodcastIndexResponse "List of episodes for the podcast"
// @Failure      400 {object} episodes.PodcastIndexErrorResponse "Bad request - missing or invalid feed ID"
// @Failure      500 {object} episodes.PodcastIndexErrorResponse "Internal server error"
// @Router       /api/v1/episodes/byfeedid [get]
func GetByFeedID(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get ID from query parameter (Podcast Index API compatibility)
		podcastIDStr := c.Query("id")
		if podcastIDStr == "" {
			c.JSON(http.StatusBadRequest, deps.EpisodeTransformer.CreateErrorResponse("Feed ID is required"))
			return
		}

		podcastID, err := strconv.ParseInt(podcastIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, deps.EpisodeTransformer.CreateErrorResponse("Invalid feed ID"))
			return
		}

		// Parse pagination
		max, _ := strconv.Atoi(c.DefaultQuery("max", "20"))
		if max < 1 || max > 1000 {
			max = 20
		}

		// Try to fetch fresh data from API and sync
		apiResponse, err := deps.EpisodeService.FetchAndSyncEpisodes(c.Request.Context(), podcastID, max)
		if err == nil && apiResponse != nil {
			c.JSON(http.StatusOK, apiResponse)
			return
		}

		// Fallback to database
		page := 1
		episodes, total, dbErr := deps.EpisodeService.GetEpisodesByPodcastID(c.Request.Context(), uint(podcastID), page, max)
		if dbErr != nil {
			log.Printf("[ERROR] Failed to fetch episodes from database for podcast %d: %v", podcastID, dbErr)
			c.JSON(http.StatusInternalServerError, deps.EpisodeTransformer.CreateErrorResponse("Failed to fetch episodes"))
			return
		}

		// Transform and return
		response := deps.EpisodeTransformer.CreateSuccessResponse(episodes, "")
		response.Query = podcastID

		if total > int64(len(episodes)) {
			response.Description = response.Description + ". Total available: " + strconv.FormatInt(total, 10)
		}

		c.JSON(http.StatusOK, response)
	}
}
