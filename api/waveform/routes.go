package waveform

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers all waveform-related routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// Waveform endpoints
	router.GET("/episodes/:id/waveform", GetWaveform(deps))
	router.GET("/episodes/:id/waveform/status", GetWaveformStatus(deps))
}