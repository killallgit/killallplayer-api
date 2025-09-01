package podcasts

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers podcast routes
// Rate limiting is applied at the route registration level
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies, episodesMiddleware, syncMiddleware gin.HandlerFunc) {
	// GET /api/v1/podcasts/:id/episodes - with general rate limiting
	router.GET("/:id/episodes", episodesMiddleware, GetEpisodes(deps))

	// POST /api/v1/podcasts/:id/episodes/sync - with strict rate limiting
	router.POST("/:id/episodes/sync", syncMiddleware, PostSync(deps))
}
