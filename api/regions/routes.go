package regions

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers region management routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// POST /api/v1/regions - Save a new region
	router.POST("", SaveRegion(deps))

	// GET /api/v1/regions - Get regions for an episode
	router.GET("", GetRegions(deps))
}
