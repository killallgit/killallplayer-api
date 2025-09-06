package podcasts

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetEpisodes returns episodes for a specific podcast
// @Summary      Get episodes for a podcast
// @Description  Retrieve all episodes for a specific podcast by its Podcast Index ID. This is the correct endpoint to use after getting podcast IDs from /trending.
// @Tags         podcasts
// @Accept       json
// @Produce      json
// @Param        id path int true "Podcast ID from trending or search results" minimum(1) example(6780065)
// @Param        max query int false "Maximum number of episodes to return (1-1000)" minimum(1) maximum(1000) default(20)
// @Success      200 {object} episodes.PodcastIndexResponse "List of episodes for the podcast"
// @Failure      400 {object} episodes.PodcastIndexErrorResponse "Bad request - invalid podcast ID"
// @Failure      500 {object} episodes.PodcastIndexErrorResponse "Internal server error"
// @Router       /api/v1/podcasts/{id}/episodes [get]
func GetEpisodes(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse and validate podcast ID
		podcastIDStr := c.Param("id")
		podcastID, err := strconv.ParseInt(podcastIDStr, 10, 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, deps.EpisodeTransformer.CreateErrorResponse("Invalid podcast ID"))
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
