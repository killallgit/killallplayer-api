package random

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// Get returns random podcast episodes
// @Summary Get random podcast episodes
// @Description Returns random podcast episodes from Podcast Index
// @Tags Random
// @Produce json
// @Param limit query int false "Number of episodes to return (default 10, max 100)"
// @Param lang query string false "Language code (default 'en')"
// @Param notcat query string false "Comma-separated categories to exclude (e.g., 'News,Politics')"
// @Success 200 {object} podcastindex.EpisodesResponse
// @Failure 500 {object} object{status=string,description=string} "Internal server error"
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
			c.JSON(http.StatusInternalServerError, gin.H{
				"status":      "false",
				"description": "Failed to fetch random episodes",
			})
			return
		}

		// Return the full Podcast Index response
		c.JSON(http.StatusOK, episodes)
	}
}
