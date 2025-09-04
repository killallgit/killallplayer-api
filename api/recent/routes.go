package recent

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers recent content discovery routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// GET /api/v1/recent/episodes - Get recent episodes globally
	router.GET("/episodes", GetEpisodes(deps))

	// GET /api/v1/recent/feeds - Get recently updated feeds
	router.GET("/feeds", GetFeeds(deps))
}
