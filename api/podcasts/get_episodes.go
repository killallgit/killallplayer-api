package podcasts

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetEpisodesForPodcast returns episodes for a specific podcast
// @Summary      Get episodes for a podcast
// @Description  Retrieve all episodes for a specific podcast by its Podcast Index ID. This is the correct endpoint to use after getting podcast IDs from /trending.
// @Tags         podcasts
// @Accept       json
// @Produce      json
// @Param        id path int true "Podcast ID from trending or search results" minimum(1) example(6780065)
// @Param        max query int false "Maximum number of episodes to return (1-1000)" minimum(1) maximum(1000) default(20)
// @Success      200 {object} types.EpisodesResponse "List of episodes for the podcast"
// @Failure      400 {object} types.ErrorResponse "Bad request - invalid podcast ID"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/podcasts/{id}/episodes [get]
func GetEpisodesForPodcast(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse and validate podcast ID
		podcastID, ok := types.ParseInt64Param(c, "id")
		if !ok {
			return // Error response already sent by utility
		}

		// Parse pagination
		max, _ := strconv.Atoi(c.DefaultQuery("max", "20"))
		if max < 1 || max > 1000 {
			max = 20
		}

		// Fetch episodes from API (cache middleware will handle caching)
		apiResponse, err := deps.EpisodeService.FetchAndSyncEpisodes(c.Request.Context(), podcastID, max)
		if err != nil {
			log.Printf("[ERROR] Failed to fetch episodes for podcast %d: %v", podcastID, err)

			// Check if it's a configuration issue
			if err.Error() == "podcast API client not available - check Podcast Index API credentials" {
				c.JSON(http.StatusServiceUnavailable, types.ErrorResponse{
					Status:  types.StatusError,
					Message: "Podcast API service is not configured",
					Details: "The server is not properly configured to fetch podcast data. Please contact the administrator.",
				})
				return
			}

			// Return error to client
			c.JSON(http.StatusBadGateway, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Failed to fetch episodes from Podcast Index API",
				Details: err.Error(),
			})
			return
		}

		// Transform to unified response type
		episodes := types.FromServiceEpisodeList(apiResponse.Items)
		c.JSON(http.StatusOK, types.EpisodesResponse{
			BaseResponse: types.BaseResponse{
				Status:  types.StatusOK,
				Message: fmt.Sprintf("Fetched %d episodes for podcast", len(episodes)),
			},
			Episodes: episodes,
			Count:    len(episodes),
		})
	}
}
