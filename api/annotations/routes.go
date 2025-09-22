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

	// Direct annotation endpoints (UUID-based)
	annotationsGroup := router.Group("/annotations")
	{
		annotationsGroup.GET("/:uuid", GetAnnotationByUUID(deps))    // Get by UUID
		annotationsGroup.PUT("/:uuid", UpdateAnnotationByUUID(deps)) // Update by UUID
		annotationsGroup.DELETE("/:uuid", DeleteAnnotation(deps))    // Use UUID for consistency
	}
}
