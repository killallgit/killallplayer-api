package trending

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers trending podcast routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// GET /api/v1/trending
	router.GET("", Get(deps))
}