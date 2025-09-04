package podcasts

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers podcast routes
// Rate limiting is applied at the route registration level
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies, episodesMiddleware, syncMiddleware gin.HandlerFunc) {
	// Existing routes
	// GET /api/v1/podcasts/:id/episodes - with general rate limiting
	router.GET("/:id/episodes", episodesMiddleware, GetEpisodes(deps))

	// POST /api/v1/podcasts/:id/episodes/sync - with strict rate limiting
	router.POST("/:id/episodes/sync", syncMiddleware, PostSync(deps))

	// New podcast metadata routes - with general rate limiting
	// GET /api/v1/podcasts/by-feed-url?url=<feed_url>
	router.GET("/by-feed-url", episodesMiddleware, GetByFeedURL(deps))

	// GET /api/v1/podcasts/by-feed-id?id=<feed_id>
	router.GET("/by-feed-id", episodesMiddleware, GetByFeedID(deps))

	// GET /api/v1/podcasts/by-itunes-id?id=<itunes_id>
	router.GET("/by-itunes-id", episodesMiddleware, GetByiTunesID(deps))

	// POST /api/v1/podcasts/add - with strict rate limiting (similar to sync)
	router.POST("/add", syncMiddleware, PostAdd(deps))
}
