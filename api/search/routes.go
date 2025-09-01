package search

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers search routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// POST /api/v1/search (router already includes /search prefix)
	router.POST("", Post(deps))
}
