package categories

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/services/podcastindex"
)

// CategoriesProvider defines the interface for getting categories
type CategoriesProvider interface {
	GetCategories() (*podcastindex.CategoriesResponse, error)
}

// Get returns all available podcast categories
// @Summary      Get all podcast categories
// @Description  Get a list of all available podcast categories from the Podcast Index API.
// @Description  Categories help filter search and trending results. Results are cached for 24 hours.
// @Tags         categories
// @Accept       json
// @Produce      json
// @Success      200 {object} podcastindex.CategoriesResponse "Categories response with ID and name for each category"
// @Failure      500 {object} types.ErrorResponse "Service unavailable or API communication failure"
// @Router       /api/v1/categories [get]
func Get(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get podcast client from dependencies
		podcastClient, ok := deps.PodcastClient.(CategoriesProvider)
		if !ok {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Categories service not available",
			})
			return
		}

		// Get categories
		categories, err := podcastClient.GetCategories()
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Failed to fetch categories",
				Details: err.Error(),
			})
			return
		}

		// Add cache headers (categories rarely change)
		c.Header("Cache-Control", "public, max-age=86400") // Cache for 24 hours
		c.Header("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		c.Header("ETag", "\"categories-v1\"")

		// Return the categories
		c.JSON(http.StatusOK, categories)
	}
}
