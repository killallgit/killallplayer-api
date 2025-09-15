package episodes

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetAll returns recent episodes (acts as the main episodes endpoint)
// @Summary      Get all episodes
// @Description  Get recent episodes across all podcasts with optional limit and podcast_id parameters
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        limit query int false "Number of episodes to return (1-1000)" minimum(1) maximum(1000) default(50)
// @Param        podcast_id query int false "Filter episodes by podcast ID"
// @Success      200 {object} types.EpisodesResponse "List of episodes"
// @Failure      400 {object} types.ErrorResponse "Bad request - invalid parameters"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes [get]
func GetAll(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse limit parameter
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		if limit < 1 || limit > 1000 {
			limit = 50
		}

		// Check if podcast_id is provided for filtering
		podcastIDStr := c.Query("podcast_id")
		if podcastIDStr != "" {
			// Parse podcast ID
			podcastID, err := strconv.ParseUint(podcastIDStr, 10, 32)
			if err != nil {
				log.Printf("[ERROR] Invalid podcast_id parameter '%s': %v", podcastIDStr, err)
				c.JSON(http.StatusBadRequest, types.ErrorResponse{
					Status:  types.StatusError,
					Message: "Invalid podcast_id parameter",
				})
				return
			}

			// Get episodes for specific podcast from database
			page := 1
			episodes, total, err := deps.EpisodeService.GetEpisodesByPodcastID(c.Request.Context(), uint(podcastID), page, limit)
			if err != nil {
				log.Printf("[ERROR] Failed to fetch episodes for podcast %d (limit %d): %v", podcastID, limit, err)
				c.JSON(http.StatusInternalServerError, types.ErrorResponse{
					Status:  types.StatusError,
					Message: "Failed to fetch episodes",
					Details: err.Error(),
				})
				return
			}

			// Transform to unified response type
			internalResponse := deps.EpisodeTransformer.CreateSuccessResponse(episodes, "")
			unifiedEpisodes := types.FromServiceEpisodeList(internalResponse.Items)

			message := fmt.Sprintf("Fetched %d episodes for podcast", len(episodes))
			if total > int64(len(episodes)) {
				message = fmt.Sprintf("Fetched %d of %d total episodes for podcast", len(episodes), total)
			}

			c.JSON(http.StatusOK, types.EpisodesResponse{
				BaseResponse: types.BaseResponse{
					Status:  types.StatusOK,
					Message: message,
				},
				Episodes: unifiedEpisodes,
				Count:    len(unifiedEpisodes),
				Total:    int(total),
			})
			return
		}

		// No podcast_id provided, return recent episodes across all podcasts
		episodes, err := deps.EpisodeService.GetRecentEpisodes(c.Request.Context(), limit)
		if err != nil {
			log.Printf("[ERROR] Failed to fetch episodes (limit %d): %v", limit, err)
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Failed to fetch episodes",
				Details: err.Error(),
			})
			return
		}

		// Transform to unified response type
		internalResponse := deps.EpisodeTransformer.CreateSuccessResponse(episodes, "")
		unifiedEpisodes := types.FromServiceEpisodeList(internalResponse.Items)

		c.JSON(http.StatusOK, types.EpisodesResponse{
			BaseResponse: types.BaseResponse{
				Status:  types.StatusOK,
				Message: fmt.Sprintf("Fetched %d recent episodes", len(episodes)),
			},
			Episodes: unifiedEpisodes,
			Count:    len(unifiedEpisodes),
		})
	}
}
