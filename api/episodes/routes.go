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

	// POST /api/v1/episodes/:id/analyze - Analyze episode for volume spikes
	router.POST("/:id/analyze", AnalyzeVolumeSpikes(deps))

	// Clip management endpoints (scoped to episode)
	router.POST("/:id/clips", CreateClipForEpisode(deps))          // Create clip for this episode
	router.GET("/:id/clips", ListClipsForEpisode(deps))            // List all clips for this episode
	router.GET("/:id/clips/:uuid", GetClipForEpisode(deps))        // Get specific clip
	router.PUT("/:id/clips/:uuid/label", UpdateClipLabel(deps))    // Update clip label
	router.PUT("/:id/clips/:uuid/approve", ApproveClip(deps))      // Approve clip for extraction
	router.DELETE("/:id/clips/:uuid", DeleteClipFromEpisode(deps)) // Delete clip
}
