package search

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/services/podcastindex"
)

// PodcastSearcher defines the interface for searching podcasts
type PodcastSearcher interface {
	Search(ctx context.Context, query string, limit int, fullText bool, val string, apOnly bool, clean bool) (*podcastindex.SearchResponse, error)
}

// Post handles podcast search requests
// @Summary      Search for podcasts by keyword
// @Description  Search the Podcast Index for podcasts matching the query string. Returns podcast metadata
// @Description  including titles, descriptions, feed URLs, and iTunes IDs. Results can be filtered by various
// @Description  criteria such as value4value support, iTunes availability, and explicit content. Search uses
// @Description  the Podcast Index API which indexes millions of podcasts from RSS feeds worldwide.
// @Tags         search
// @Accept       json
// @Produce      json
// @Param        request body types.SearchRequest true "Search parameters with query and optional filters"
// @Success      200 {object} types.PodcastSearchResponse "Matching podcasts with metadata (feedId can be used with /podcasts/{id}/episodes)"
// @Failure      400 {object} types.ErrorResponse "Invalid request format or missing required query field"
// @Failure      500 {object} types.ErrorResponse "Search service error or API communication failure"
// @Failure      504 {object} types.ErrorResponse "Request timeout (search limited to 10 seconds)"
// @Router       /api/v1/search [post]
func Post(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse request body
		var req types.SearchRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Invalid request format",
				Details: err.Error(),
			})
			return
		}

		// Validate query
		if req.Query == "" {
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Search query is required",
			})
			return
		}

		// Set default limit if not provided
		if req.Limit == 0 {
			req.Limit = 10
		}

		// Validate limit
		if req.Limit < 1 || req.Limit > 100 {
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Limit must be between 1 and 100",
			})
			return
		}

		// Get podcast client from dependencies
		podcastClient, ok := deps.PodcastClient.(PodcastSearcher)
		if !ok {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Search service not available",
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		// Perform search
		results, err := podcastClient.Search(ctx, req.Query, req.Limit, req.FullText, req.Val, req.ApOnly, req.Clean)
		if err != nil {
			// Check if it's a context timeout
			if ctx.Err() == context.DeadlineExceeded {
				c.JSON(http.StatusGatewayTimeout, types.ErrorResponse{
					Status:  types.StatusError,
					Message: "Search request timed out",
				})
				return
			}

			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Failed to search podcasts",
				Details: err.Error(),
			})
			return
		}

		// Transform Podcast Index results to our simplified format
		podcasts := types.FromPodcastIndexList(results.Feeds)

		// Return the search response
		c.JSON(http.StatusOK, types.PodcastSearchResponse{
			BaseResponse: types.BaseResponse{
				Status:  types.StatusOK,
				Message: "Search results retrieved successfully",
			},
			Podcasts: podcasts,
			Query:    req.Query,
			Count:    len(podcasts),
			Total:    results.Count,
		})
	}
}
