package random

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
)

// Get returns random podcast episodes
// @Summary Get random podcast episodes
// @Description Returns random podcast episodes from Podcast Index with optional language and category filtering.
// @Description Useful for discovering new content. Episodes are randomly selected from recent additions to the index.
// @Tags random
// @Produce json
// @Param limit query int false "Number of episodes to return (1-100)" default(10) minimum(1) maximum(100)
// @Param lang query string false "Language code filter (e.g., 'en', 'es', 'fr')" default(en)
// @Param notcat query string false "Comma-separated categories to exclude (e.g., 'News,Politics')"
// @Success 200 {object} models.EpisodeResponse "Random episodes with metadata"
// @Failure 500 {object} types.ErrorResponse "Podcast Index API unavailable or communication failure"
// @Router /api/v1/random [get]
func Get(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse limit parameter
		limitStr := c.DefaultQuery("limit", "10")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 10
		}
		if limit > 100 {
			limit = 100
		}

		// Parse language parameter
		lang := c.DefaultQuery("lang", "en")

		// Parse notcat parameter
		var notCategories []string
		if notcat := c.Query("notcat"); notcat != "" {
			// Split by comma and trim spaces
			categories := strings.Split(notcat, ",")
			for _, cat := range categories {
				trimmed := strings.TrimSpace(cat)
				if trimmed != "" {
					notCategories = append(notCategories, trimmed)
				}
			}
		}

		// Call Podcast Index API
		episodes, err := deps.PodcastClient.GetRandomEpisodes(
			c.Request.Context(),
			limit,
			lang,
			notCategories,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Failed to fetch random episodes",
				Details: err.Error(),
			})
			return
		}

		// Build episode response with consistent format
		response := models.EpisodeResponse{
			Status:      episodes.Status,
			Results:     episodes.Items,
			TotalCount:  episodes.Count,
			Max:         strconv.Itoa(limit), // Convert to string to match PodcastIndex API format
			Lang:        lang,
			NotCat:      notCategories,
			Description: episodes.Description,
		}

		// Return the episode response
		c.JSON(http.StatusOK, response)
	}
}
