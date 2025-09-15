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
// @Summary      Search for podcasts
// @Description  Search for podcasts by query string with optional filters for content type, iTunes presence, and explicit content
// @Tags         search
// @Accept       json
// @Produce      json
// @Param        request body types.SearchRequest true "Search parameters"
// @Success      200 {object} types.PodcastSearchResponse "Podcast search results"
// @Failure      400 {object} types.ErrorResponse "Bad request - invalid parameters"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Failure      504 {object} types.ErrorResponse "Gateway timeout - search request timed out"
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
