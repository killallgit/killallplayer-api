package annotations

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers annotation-related routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// Episode annotation endpoints
	router.POST("/:id/annotations", CreateAnnotation(deps))
	router.GET("/:id/annotations", GetAnnotations(deps))

	// Direct annotation endpoints (not nested under episodes)
	annotationsGroup := router.Group("/annotations")
	{
		annotationsGroup.PUT("/:id", UpdateAnnotation(deps))
		annotationsGroup.DELETE("/:id", DeleteAnnotation(deps))
	}
}
