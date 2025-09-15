package episodes

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers episode routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// GET /api/v1/episodes - Get all episodes with pagination
	router.GET("", GetAll(deps))

	// GET /api/v1/episodes/by-guid
	router.GET("/by-guid", GetByGUID(deps))

	// GET /api/v1/episodes/:id - Note: This must come after other specific routes
	router.GET("/:id", GetByID(deps))

	// Reviews endpoint
	// GET /api/v1/episodes/:id/reviews - Get podcast reviews
	router.GET("/:id/reviews", GetReviews(deps))

}
