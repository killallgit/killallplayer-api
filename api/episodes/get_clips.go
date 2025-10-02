package episodes

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
	"gorm.io/gorm"
)

// ClipResponse represents a clip in API responses
// @Description Audio clip time range for skipping or ML training
type ClipResponse struct {
	UUID          string   `json:"uuid" example:"052f3b9b-cc02-418c-a9ab-8f49534c01c8" description:"Unique identifier"`
	StartTime     float64  `json:"start_time" example:"30.5" description:"Start time in seconds"`
	EndTime       float64  `json:"end_time" example:"45.2" description:"End time in seconds"`
	Label         string   `json:"label" example:"advertisement" description:"Clip label"`
	Confidence    *float64 `json:"confidence,omitempty" example:"0.85" description:"Auto-label confidence (0-1)"`
	AutoLabeled   bool     `json:"auto_labeled" example:"true" description:"Whether automatically detected"`
	UserConfirmed bool     `json:"user_confirmed" example:"false" description:"Whether user confirmed this clip"`
	Extracted     bool     `json:"extracted" example:"false" description:"Whether audio file has been extracted"`
	CreatedAt     string   `json:"created_at" example:"2025-10-01T12:00:00Z"`
}

// ClipsResponse represents the response for episode clips
// @Description Response containing clips for an episode
type ClipsResponse struct {
	types.BaseResponse
	EpisodeID int64          `json:"episode_id" example:"123" description:"Podcast Index Episode ID"`
	Clips     []ClipResponse `json:"clips" description:"Array of clips (empty if analysis pending)"`
	Progress  *int           `json:"progress,omitempty" example:"45" description:"Analysis progress 0-100 (only when processing)"`
}

// GetClips returns clips for an episode, triggering analysis if needed
// DEPRECATED: This handler is not currently registered. Use ListClipsForEpisode in clips_handlers.go instead.
func GetClips(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID (Podcast Index ID)
		podcastIndexID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || podcastIndexID <= 0 {
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Invalid Podcast Index Episode ID",
			})
			return
		}

		// Check if ClipService is available
		if deps.ClipService == nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Clip service not available",
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		log.Printf("[DEBUG] GetClips: PodcastIndexID=%d", podcastIndexID)

		// Get clips for this episode from database
		clipModels, err := deps.ClipService.GetClipsByEpisodeID(ctx, podcastIndexID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: fmt.Sprintf("Failed to retrieve clips: %v", err),
			})
			return
		}

		// If clips exist, return them
		if len(clipModels) > 0 {
			clips := make([]ClipResponse, len(clipModels))
			for i, clip := range clipModels {
				clips[i] = ClipResponse{
					UUID:          clip.UUID,
					StartTime:     clip.OriginalStartTime,
					EndTime:       clip.OriginalEndTime,
					Label:         clip.Label,
					Confidence:    clip.LabelConfidence,
					AutoLabeled:   clip.AutoLabeled,
					UserConfirmed: false, // TODO: Add UserConfirmed field to model
					Extracted:     clip.Extracted,
					CreatedAt:     clip.CreatedAt.Format(time.RFC3339),
				}
			}

			c.JSON(http.StatusOK, ClipsResponse{
				BaseResponse: types.BaseResponse{
					Status:  types.StatusOK,
					Message: fmt.Sprintf("Found %d clips for episode", len(clips)),
				},
				EpisodeID: podcastIndexID,
				Clips:     clips,
			})
			return
		}

		// No clips exist - enqueue analysis job if job service is available
		if deps.JobService != nil {
			// Get episode details to get audio URL
			if deps.EpisodeService == nil {
				c.JSON(http.StatusInternalServerError, types.ErrorResponse{
					Status:  types.StatusError,
					Message: "Episode service not available",
				})
				return
			}

			episode, err := deps.EpisodeService.GetEpisodeByID(ctx, uint(podcastIndexID))
			if err != nil {
				c.JSON(http.StatusInternalServerError, types.ErrorResponse{
					Status:  types.StatusError,
					Message: fmt.Sprintf("Failed to get episode details: %v", err),
				})
				return
			}

			// Enqueue analysis job - use unique key to prevent duplicates
			payload := models.JobPayload{
				"episode_id": podcastIndexID,
				"audio_url":  episode.AudioURL,
			}

			// Use EnqueueUniqueJob to prevent duplicate jobs for same episode
			uniqueKey := fmt.Sprintf("episode_analysis:%d", podcastIndexID)
			job, jobErr := deps.JobService.EnqueueUniqueJob(ctx, "episode_analysis", payload, uniqueKey)
			if jobErr != nil {
				log.Printf("[WARN] Failed to enqueue episode analysis job for episode %d: %v", podcastIndexID, jobErr)
				// Continue anyway - return queued status
			} else {
				log.Printf("[INFO] Enqueued episode analysis job %d for episode %d", job.ID, podcastIndexID)
			}
		}

		// Return 202 Accepted with empty clips
		c.JSON(http.StatusAccepted, ClipsResponse{
			BaseResponse: types.BaseResponse{
				Status:  types.StatusQueued,
				Message: "Episode analysis has been queued",
			},
			EpisodeID: podcastIndexID,
			Clips:     []ClipResponse{},
			Progress:  nil,
		})
	}
}
