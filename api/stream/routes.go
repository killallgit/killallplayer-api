package stream

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers streaming routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// Direct streaming endpoints (must come before :id routes)
	// GET /api/v1/stream/direct - Stream audio from direct URL (no database)
	router.GET("/direct", StreamDirectURL())

	// HEAD /api/v1/stream/direct - Get audio metadata for direct URL
	router.HEAD("/direct", StreamDirectURL())

	// OPTIONS /api/v1/stream/direct - Handle CORS preflight for direct streaming
	router.OPTIONS("/direct", HandleOptions())

	// Episode ID streaming endpoints
	// OPTIONS /api/v1/stream/:id - Handle CORS preflight
	router.OPTIONS("/:id", HandleOptions())

	// GET /api/v1/stream/:id - Stream episode audio by ID (with database lookup)
	router.GET("/:id", StreamEpisode(deps))

	// HEAD /api/v1/stream/:id - Get audio metadata without body
	router.HEAD("/:id", StreamEpisode(deps))
}
