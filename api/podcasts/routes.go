package podcasts

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers podcast routes
// Rate limiting is applied at the route registration level
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies, podcastMiddleware, episodesMiddleware gin.HandlerFunc) {
	// GET /api/v1/podcasts/:id - Get podcast details by Podcast Index ID
	router.GET("/:id", podcastMiddleware, GetPodcast(deps))

	// GET /api/v1/podcasts/:id/episodes - Get episodes for a podcast by feedId
	router.GET("/:id/episodes", episodesMiddleware, GetEpisodesForPodcast(deps))
}
