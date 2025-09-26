package clips

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers clip-related routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// Clip management endpoints
	router.POST("", CreateClip(deps))                 // Create new clip
	router.GET("", ListClips(deps))                   // List all clips
	router.GET("/:uuid", GetClip(deps))               // Get specific clip
	router.PUT("/:uuid/label", UpdateClipLabel(deps)) // Update clip label
	router.DELETE("/:uuid", DeleteClip(deps))         // Delete clip

	// Export endpoint
	router.GET("/export", ExportDataset(deps)) // Export dataset as ZIP
}
