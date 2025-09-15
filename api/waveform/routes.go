package waveform

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers all waveform-related routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// GET /api/v1/episodes/:id/waveform - Get waveform data with status
	router.GET("/:id/waveform", GetWaveform(deps))
}
