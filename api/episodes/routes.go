package episodes

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers episode routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// GET /api/v1/episodes/:id - Get episode details
	router.GET("/:id", GetByID(deps))

	// GET /api/v1/episodes/:id/reviews - Get iTunes reviews for the podcast
	router.GET("/:id/reviews", GetReviews(deps))
}
