package stream

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers streaming routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// OPTIONS /api/v1/stream/:id - Handle CORS preflight
	router.OPTIONS("/:id", HandleOptions())
	
	// GET /api/v1/stream/:id - Stream episode audio
	router.GET("/:id", StreamEpisode(deps))
	
	// HEAD /api/v1/stream/:id - Get audio metadata without body
	router.HEAD("/:id", StreamEpisode(deps))
}