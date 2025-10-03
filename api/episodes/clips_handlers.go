package episodes

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/clips"
)

// EpisodeClipResponse represents a clip in API responses
type EpisodeClipResponse struct {
	UUID              string   `json:"uuid" example:"a1b2c3d4-e5f6-4a7b-8c9d-0e1f2a3b4c5d"`
	Label             string   `json:"label" example:"advertisement"`
	Status            string   `json:"status" enums:"detected,queued,processing,ready,failed" example:"queued"`
	Approved          bool     `json:"approved" example:"true"`
	Extracted         bool     `json:"extracted" example:"false"`
	ClipFilename      *string  `json:"filename,omitempty" example:"clip_a1b2c3d4.wav"`
	ClipDuration      *float64 `json:"duration,omitempty" example:"15.0"`
	ClipSizeBytes     *int64   `json:"size_bytes,omitempty" example:"480332"`
	OriginalStartTime float64  `json:"original_start_time" example:"30.0"`
	OriginalEndTime   float64  `json:"original_end_time" example:"45.0"`
	AutoLabeled       bool     `json:"auto_labeled" example:"false"`
	LabelConfidence   *float64 `json:"label_confidence,omitempty" example:"0.95"`
	LabelMethod       string   `json:"label_method" enums:"manual,peak_detection" example:"manual"`
	ErrorMessage      string   `json:"error_message,omitempty" example:""`
	CreatedAt         string   `json:"created_at" example:"2025-10-02T13:00:00Z"`
	UpdatedAt         string   `json:"updated_at" example:"2025-10-02T13:00:00Z"`
}

// CreateClipRequest represents the request to create a clip for an episode
type CreateClipRequest struct {
	OriginalStartTime float64 `json:"start_time" binding:"min=0" example:"30"`
	OriginalEndTime   float64 `json:"end_time" binding:"required,gt=0" example:"45"`
	Label             string  `json:"label" binding:"required,min=1" example:"advertisement"`
}

// UpdateLabelRequest represents the request to update a clip's label
type UpdateLabelRequest struct {
	Label string `json:"label" binding:"required,min=1" example:"music"`
}

// @Summary Create clip for episode
// @Description Create a new audio clip from this episode at the specified time range. Manual clips are automatically approved and queued for extraction.
// @Tags episodes
// @Accept json
// @Produce json
// @Param id path int true "Episode ID"
// @Param request body CreateClipRequest true "Clip creation parameters"
// @Success 202 {object} EpisodeClipResponse "Clip created and queued for extraction (approved=true, status=queued)"
// @Failure 400 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/episodes/{id}/clips [post]
func CreateClipForEpisode(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse episode ID from path
		episodeIDStr := c.Param("id")
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil {
			types.SendBadRequest(c, "Invalid episode ID")
			return
		}

		var req CreateClipRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			types.SendBadRequest(c, err.Error())
			return
		}

		// Validate time range
		if req.OriginalEndTime <= req.OriginalStartTime {
			types.SendBadRequest(c, "end_time must be greater than start_time")
			return
		}

		// Create the clip (manual clips are automatically approved)
		clip, err := deps.ClipService.CreateClip(c.Request.Context(), clips.CreateClipParams{
			PodcastIndexEpisodeID: episodeID,
			OriginalStartTime:     req.OriginalStartTime,
			OriginalEndTime:       req.OriginalEndTime,
			Label:                 req.Label,
			Approved:              true, // Manual clips are pre-approved
		})

		if err != nil {
			types.SendInternalError(c, fmt.Sprintf("Failed to create clip: %v", err))
			return
		}

		c.JSON(http.StatusAccepted, toClipResponse(clip))
	}
}

// @Summary List clips for episode
// @Description Get all clips created for this episode with optional status and approval filters
// @Tags episodes
// @Produce json
// @Param id path int true "Episode ID"
// @Param status query string false "Filter by status" Enums(queued, processing, ready, failed, detected)
// @Param approved query boolean false "Filter by approval status (true/false)"
// @Success 200 {array} EpisodeClipResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/episodes/{id}/clips [get]
func ListClipsForEpisode(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse episode ID from path
		episodeIDStr := c.Param("id")
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil {
			types.SendBadRequest(c, "Invalid episode ID")
			return
		}

		// Optional status filter
		status := c.Query("status")

		// Optional approved filter
		var approvedFilter *bool
		if approvedStr := c.Query("approved"); approvedStr != "" {
			approved := approvedStr == "true"
			approvedFilter = &approved
		}

		// List clips for this episode
		clipsList, err := deps.ClipService.ListClips(c.Request.Context(), clips.ListClipsFilters{
			EpisodeID: &episodeID, // Filter by episode
			Status:    status,
			Approved:  approvedFilter,
			Limit:     1000, // Return all clips for episode
			Offset:    0,
		})

		if err != nil {
			types.SendInternalError(c, fmt.Sprintf("Failed to list clips: %v", err))
			return
		}

		// Convert to response format
		response := make([]EpisodeClipResponse, len(clipsList))
		for i, clip := range clipsList {
			response[i] = toClipResponse(clip)
		}

		c.JSON(http.StatusOK, response)
	}
}

// @Summary Get clip details
// @Description Get details of a specific clip for this episode
// @Tags episodes
// @Produce json
// @Param id path int true "Episode ID"
// @Param uuid path string true "Clip UUID"
// @Success 200 {object} EpisodeClipResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 404 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/episodes/{id}/clips/{uuid} [get]
func GetClipForEpisode(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse episode ID from path
		episodeIDStr := c.Param("id")
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil {
			types.SendBadRequest(c, "Invalid episode ID")
			return
		}

		uuid := c.Param("uuid")
		if uuid == "" {
			types.SendBadRequest(c, "UUID is required")
			return
		}

		clip, err := deps.ClipService.GetClip(c.Request.Context(), uuid)
		if err != nil {
			if err.Error() == "clip not found" {
				types.SendNotFound(c, "Clip not found")
			} else {
				types.SendInternalError(c, fmt.Sprintf("Failed to get clip: %v", err))
			}
			return
		}

		// Verify clip belongs to this episode
		if clip.PodcastIndexEpisodeID != episodeID {
			types.SendNotFound(c, "Clip not found for this episode")
			return
		}

		c.JSON(http.StatusOK, toClipResponse(clip))
	}
}

// @Summary Update clip label
// @Description Update the classification label for a clip
// @Tags episodes
// @Accept json
// @Produce json
// @Param id path int true "Episode ID"
// @Param uuid path string true "Clip UUID"
// @Param request body UpdateLabelRequest true "New label"
// @Success 200 {object} EpisodeClipResponse
// @Failure 400 {object} types.ErrorResponse
// @Failure 404 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/episodes/{id}/clips/{uuid}/label [put]
func UpdateClipLabel(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse episode ID from path
		episodeIDStr := c.Param("id")
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil {
			types.SendBadRequest(c, "Invalid episode ID")
			return
		}

		uuid := c.Param("uuid")
		if uuid == "" {
			types.SendBadRequest(c, "UUID is required")
			return
		}

		var req UpdateLabelRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			types.SendBadRequest(c, err.Error())
			return
		}

		// Get clip first to verify it belongs to episode
		clip, err := deps.ClipService.GetClip(c.Request.Context(), uuid)
		if err != nil {
			if err.Error() == "clip not found" {
				types.SendNotFound(c, "Clip not found")
			} else {
				types.SendInternalError(c, fmt.Sprintf("Failed to get clip: %v", err))
			}
			return
		}

		// Verify clip belongs to this episode
		if clip.PodcastIndexEpisodeID != episodeID {
			types.SendNotFound(c, "Clip not found for this episode")
			return
		}

		// Update label
		clip, err = deps.ClipService.UpdateClipLabel(c.Request.Context(), uuid, req.Label)
		if err != nil {
			types.SendInternalError(c, fmt.Sprintf("Failed to update label: %v", err))
			return
		}

		c.JSON(http.StatusOK, toClipResponse(clip))
	}
}

// @Summary Delete clip
// @Description Delete a clip and its audio file from this episode
// @Tags episodes
// @Param id path int true "Episode ID"
// @Param uuid path string true "Clip UUID"
// @Success 204 "Deleted successfully"
// @Failure 400 {object} types.ErrorResponse
// @Failure 404 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/episodes/{id}/clips/{uuid} [delete]
func DeleteClipFromEpisode(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse episode ID from path
		episodeIDStr := c.Param("id")
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil {
			types.SendBadRequest(c, "Invalid episode ID")
			return
		}

		uuid := c.Param("uuid")
		if uuid == "" {
			types.SendBadRequest(c, "UUID is required")
			return
		}

		// Get clip first to verify it belongs to episode
		clip, err := deps.ClipService.GetClip(c.Request.Context(), uuid)
		if err != nil {
			if err.Error() == "clip not found" {
				// Idempotent - already deleted
				c.Status(http.StatusNoContent)
				return
			}
			types.SendInternalError(c, fmt.Sprintf("Failed to get clip: %v", err))
			return
		}

		// Verify clip belongs to this episode
		if clip.PodcastIndexEpisodeID != episodeID {
			types.SendNotFound(c, "Clip not found for this episode")
			return
		}

		// Delete clip
		if err := deps.ClipService.DeleteClip(c.Request.Context(), uuid); err != nil {
			types.SendInternalError(c, fmt.Sprintf("Failed to delete clip: %v", err))
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// @Summary Approve clip for extraction
// @Description Mark a clip as approved for extraction. This is used for clips created by analysis (status=detected, approved=false) to trigger audio extraction. Sets approved=true and queues the clip for processing.
// @Tags episodes
// @Produce json
// @Param id path int true "Episode ID"
// @Param uuid path string true "Clip UUID"
// @Success 200 {object} EpisodeClipResponse "Clip approved and queued for extraction"
// @Failure 400 {object} types.ErrorResponse
// @Failure 404 {object} types.ErrorResponse
// @Failure 500 {object} types.ErrorResponse
// @Router /api/v1/episodes/{id}/clips/{uuid}/approve [put]
func ApproveClip(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse episode ID from path
		episodeIDStr := c.Param("id")
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil {
			types.SendBadRequest(c, "Invalid episode ID")
			return
		}

		uuid := c.Param("uuid")
		if uuid == "" {
			types.SendBadRequest(c, "UUID is required")
			return
		}

		// Get clip first to verify it belongs to episode
		clip, err := deps.ClipService.GetClip(c.Request.Context(), uuid)
		if err != nil {
			if err.Error() == "clip not found" {
				types.SendNotFound(c, "Clip not found")
				return
			}
			types.SendInternalError(c, fmt.Sprintf("Failed to get clip: %v", err))
			return
		}

		// Verify clip belongs to this episode
		if clip.PodcastIndexEpisodeID != episodeID {
			types.SendNotFound(c, "Clip not found for this episode")
			return
		}

		// Update approved status
		// Note: We'll need to add an ApproveClip method to the clip service
		// For now, we can update directly via GORM
		if err := deps.DB.DB.Model(&models.Clip{}).Where("uuid = ?", uuid).Update("approved", true).Error; err != nil {
			types.SendInternalError(c, fmt.Sprintf("Failed to approve clip: %v", err))
			return
		}

		// Fetch updated clip
		clip.Approved = true
		c.JSON(http.StatusOK, toClipResponse(clip))
	}
}

// Helper function to convert clip model to response
func toClipResponse(clip *models.Clip) EpisodeClipResponse {
	return EpisodeClipResponse{
		UUID:              clip.UUID,
		Label:             clip.Label,
		Status:            clip.Status,
		Approved:          clip.Approved,
		Extracted:         clip.Extracted,
		ClipFilename:      clip.ClipFilename,
		ClipDuration:      clip.ClipDuration,
		ClipSizeBytes:     clip.ClipSizeBytes,
		OriginalStartTime: clip.OriginalStartTime,
		OriginalEndTime:   clip.OriginalEndTime,
		AutoLabeled:       clip.AutoLabeled,
		LabelConfidence:   clip.LabelConfidence,
		LabelMethod:       clip.LabelMethod,
		ErrorMessage:      clip.ErrorMessage,
		CreatedAt:         clip.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:         clip.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
}
