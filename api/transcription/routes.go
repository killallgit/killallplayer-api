package transcription

import (
	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// RegisterRoutes registers all transcription-related routes
func RegisterRoutes(router *gin.RouterGroup, deps *types.Dependencies) {
	// Transcription endpoints
	router.POST("/:id/transcribe", TriggerTranscription(deps))
	router.GET("/:id/transcribe", GetTranscription(deps))
	router.GET("/:id/transcribe/status", GetTranscriptionStatus(deps))
}
