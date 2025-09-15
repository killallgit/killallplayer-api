package trending

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers trending podcast routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// POST /api/v1/trending - Get trending podcasts with optional filters
	router.POST("", Post(deps))
}
