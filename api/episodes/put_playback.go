package episodes

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// PlaybackUpdateRequest represents a playback state update request
type PlaybackUpdateRequest struct {
	Position int  `json:"position"` // Playback position in seconds
	Played   bool `json:"played"`   // Whether the episode has been played
}

// PutPlayback updates playback position and played status
func PutPlayback(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")
		episodeID, err := strconv.ParseUint(episodeIDStr, 10, 32)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid episode ID",
			})
			return
		}

		var req PlaybackUpdateRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Invalid request body",
				"details": err.Error(),
			})
			return
		}

		// Validate position
		if req.Position < 0 {
			c.JSON(http.StatusBadRequest, gin.H{
				"status":  "error",
				"message": "Position cannot be negative",
			})
			return
		}

		// Update playback state
		err = deps.EpisodeService.UpdatePlaybackState(c.Request.Context(), uint(episodeID), req.Position, req.Played)
		if err != nil {
			if IsNotFound(err) {
				c.JSON(http.StatusNotFound, gin.H{
					"status":  "error",
					"message": "Episode not found",
				})
			} else {
				log.Printf("[ERROR] Failed to update playback state for episode %d: %v", episodeID, err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"status":  "error",
					"message": "Failed to update playback state",
				})
			}
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "Playback state updated",
			"data": gin.H{
				"episode_id": episodeID,
				"position":   req.Position,
				"played":     req.Played,
			},
		})
	}
}
