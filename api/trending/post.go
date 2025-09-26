package trending

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/services/podcastindex"
)

// PodcastTrending defines the interface for getting trending podcasts
type PodcastTrending interface {
	GetTrending(ctx context.Context, max, since int, categories []string, lang string, fullText bool) (*podcastindex.SearchResponse, error)
}

// Post handles trending podcasts requests with filters
// @Summary      Get trending podcasts with optional filters
// @Description  Retrieve currently trending podcasts from Podcast Index based on recent activity and popularity.
// @Description  Results can be filtered by time period, categories, and language. Trending podcasts are determined
// @Description  by Podcast Index's algorithm which considers factors like new episodes, subscriber growth, and
// @Description  social media mentions. Use the returned podcast IDs (feedId) with /podcasts/{id}/episodes to get episodes.
// @Tags         trending
// @Accept       json
// @Produce      json
// @Param        request body types.TrendingRequest true "Filter parameters (all optional - defaults to 10 podcasts from last 24 hours)"
// @Success      200 {object} types.TrendingPodcastsResponse "List of trending podcasts with metadata"
// @Failure      400 {object} types.ErrorResponse "Invalid request format or parameter values"
// @Failure      500 {object} types.ErrorResponse "Failed to fetch trending data from Podcast Index API"
// @Failure      504 {object} types.ErrorResponse "Request timeout (limited to 10 seconds)"
// @Router       /api/v1/trending [post]
func Post(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse request body
		var req types.TrendingRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Invalid request format",
				Details: err.Error(),
			})
			return
		}

		// Set defaults
		if req.Max == 0 {
			req.Max = 10
		}
		if req.Since == 0 {
			req.Since = 24 // Default to last 24 hours
		}

		// Validate limits
		if req.Max < 1 || req.Max > 100 {
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Max must be between 1 and 100",
			})
			return
		}
		if req.Since < 1 || req.Since > 720 {
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Since must be between 1 and 720 hours (30 days)",
			})
			return
		}

		// Get podcast client from dependencies
		podcastClient, ok := deps.PodcastClient.(PodcastTrending)
		if !ok {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Trending service not available",
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
		defer cancel()

		// Get trending podcasts
		results, err := podcastClient.GetTrending(ctx, req.Max, req.Since, req.Categories, req.Lang, req.FullText)
		if err != nil {
			// Check if it's a context timeout
			if ctx.Err() == context.DeadlineExceeded {
				c.JSON(http.StatusGatewayTimeout, types.ErrorResponse{
					Status:  types.StatusError,
					Message: "Trending request timed out",
				})
				return
			}

			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Failed to fetch trending podcasts",
				Details: err.Error(),
			})
			return
		}

		// Transform Podcast Index results to our simplified format
		podcasts := types.FromPodcastIndexList(results.Feeds)

		// Return the TrendingPodcastsResponse
		c.JSON(http.StatusOK, types.TrendingPodcastsResponse{
			BaseResponse: types.BaseResponse{
				Status:  types.StatusOK,
				Message: "Fetched trending podcasts",
			},
			Podcasts: podcasts,
			Since:    req.Since,
			Count:    len(podcasts),
		})
	}
}
