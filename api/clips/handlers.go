package clips

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/services/clips"
)

// CreateClipRequest represents the request to create a clip
// @Description Request body for creating a new audio clip
type CreateClipRequest struct {
	PodcastIndexEpisodeID int64   `json:"podcast_index_episode_id" binding:"required,min=1" example:"12345" description:"Podcast Index Episode ID for clip organization"`
	SourceEpisodeURL      string  `json:"source_episode_url" binding:"required" example:"https://example.com/episode.mp3" description:"URL or file path to the source audio"`
	OriginalStartTime     float64 `json:"start_time" binding:"min=0" example:"30" description:"Start time in seconds (can be 0)"`
	OriginalEndTime       float64 `json:"end_time" binding:"required,gt=0" example:"45" description:"End time in seconds (must be > start_time)"`
	Label                 string  `json:"label" binding:"required,min=1" example:"advertisement" description:"Classification label for ML training"`
}

// ClipResponse represents a clip in API responses
// @Description Complete information about an audio clip
type ClipResponse struct {
	UUID              string   `json:"uuid" example:"052f3b9b-cc02-418c-a9ab-8f49534c01c8" description:"Unique identifier for the clip"`
	Label             string   `json:"label" example:"advertisement" description:"ML training label"`
	Status            string   `json:"status" example:"ready" description:"Processing status: processing, ready, or failed"`
	ClipFilename      *string  `json:"filename,omitempty" example:"clip_052f3b9b-cc02-418c-a9ab-8f49534c01c8.wav" description:"Generated filename (null for auto-detected clips)"`
	ClipDuration      *float64 `json:"duration,omitempty" example:"15" description:"Duration in seconds (null for auto-detected clips)"`
	ClipSizeBytes     *int64   `json:"size_bytes,omitempty" example:"480078" description:"File size in bytes (null for auto-detected clips)"`
	SourceEpisodeURL  string   `json:"source_episode_url" example:"https://example.com/episode.mp3" description:"Original audio source"`
	OriginalStartTime float64  `json:"original_start_time" example:"30" description:"Original start time in source"`
	OriginalEndTime   float64  `json:"original_end_time" example:"45" description:"Original end time in source"`
	ErrorMessage      string   `json:"error_message,omitempty" example:"failed to download source audio: HTTP 403" description:"Error details if status is failed"`
	CreatedAt         string   `json:"created_at" example:"2025-09-25T16:36:45Z" description:"Creation timestamp"`
	UpdatedAt         string   `json:"updated_at" example:"2025-09-25T16:36:47Z" description:"Last update timestamp"`
}

// UpdateLabelRequest represents the request to update a clip's label
// @Description Request body for updating a clip's label
type UpdateLabelRequest struct {
	Label string `json:"label" binding:"required,min=1" example:"music" description:"New label for the clip"`
}

// CreateClip handles clip creation
// @Summary Create a new audio clip for ML training
// @Description Extract a labeled audio segment from a podcast episode for machine learning training datasets.
// @Description The clip will be automatically converted to 16kHz mono WAV format, padded or cropped to 15 seconds,
// @Description and stored with the specified label. Processing is asynchronous - the clip status will be "processing"
// @Description initially and change to "ready" when extraction completes or "failed" if an error occurs.
// @Tags clips
// @Accept json
// @Produce json
// @Param request body CreateClipRequest true "Audio clip extraction parameters with source URL and time range in seconds"
// @Success 202 {object} ClipResponse "Clip creation accepted and processing started"
// @Failure 400 {object} types.ErrorResponse "Invalid request parameters (e.g., end_time <= start_time)"
// @Failure 500 {object} types.ErrorResponse "Internal server error during clip creation"
// @Router /api/v1/clips [post]
func CreateClip(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		// Create the clip
		clip, err := deps.ClipService.CreateClip(c.Request.Context(), clips.CreateClipParams{
			PodcastIndexEpisodeID: req.PodcastIndexEpisodeID,
			SourceEpisodeURL:      req.SourceEpisodeURL,
			OriginalStartTime:     req.OriginalStartTime,
			OriginalEndTime:       req.OriginalEndTime,
			Label:                 req.Label,
		})

		if err != nil {
			types.SendInternalError(c, fmt.Sprintf("Failed to create clip: %v", err))
			return
		}

		// Return accepted status since processing is async
		c.JSON(http.StatusAccepted, ClipResponse{
			UUID:              clip.UUID,
			Label:             clip.Label,
			Status:            clip.Status,
			ClipFilename:      clip.ClipFilename,
			ClipDuration:      clip.ClipDuration,
			ClipSizeBytes:     clip.ClipSizeBytes,
			SourceEpisodeURL:  clip.SourceEpisodeURL,
			OriginalStartTime: clip.OriginalStartTime,
			OriginalEndTime:   clip.OriginalEndTime,
			ErrorMessage:      clip.ErrorMessage,
			CreatedAt:         clip.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:         clip.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
}

// GetClip retrieves a specific clip
// @Summary Get clip details by UUID
// @Description Retrieve detailed information about a specific clip including its processing status,
// @Description audio properties, and label. Check the 'status' field to determine if the clip is ready for use.
// @Tags clips
// @Produce json
// @Param uuid path string true "Unique clip identifier (UUID format)"
// @Success 200 {object} ClipResponse "Clip details retrieved successfully"
// @Failure 404 {object} types.ErrorResponse "Clip with specified UUID not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /api/v1/clips/{uuid} [get]
func GetClip(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		c.JSON(http.StatusOK, ClipResponse{
			UUID:              clip.UUID,
			Label:             clip.Label,
			Status:            clip.Status,
			ClipFilename:      clip.ClipFilename,
			ClipDuration:      clip.ClipDuration,
			ClipSizeBytes:     clip.ClipSizeBytes,
			SourceEpisodeURL:  clip.SourceEpisodeURL,
			OriginalStartTime: clip.OriginalStartTime,
			OriginalEndTime:   clip.OriginalEndTime,
			ErrorMessage:      clip.ErrorMessage,
			CreatedAt:         clip.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:         clip.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
}

// UpdateClipLabel updates a clip's label
// @Summary Update a clip's label for re-categorization
// @Description Change the label of an existing clip to reorganize training datasets.
// @Description This operation moves the clip file to a new label directory in storage.
// @Description Labels can be any string value for flexible categorization (e.g., "advertisement", "music", "speech").
// @Tags clips
// @Accept json
// @Produce json
// @Param uuid path string true "Unique clip identifier (UUID format)"
// @Param request body UpdateLabelRequest true "New label for categorization"
// @Success 200 {object} ClipResponse "Label updated successfully"
// @Failure 400 {object} types.ErrorResponse "Invalid request (empty label or malformed JSON)"
// @Failure 404 {object} types.ErrorResponse "Clip with specified UUID not found"
// @Failure 500 {object} types.ErrorResponse "Internal server error or storage operation failed"
// @Router /api/v1/clips/{uuid}/label [put]
func UpdateClipLabel(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
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

		clip, err := deps.ClipService.UpdateClipLabel(c.Request.Context(), uuid, req.Label)
		if err != nil {
			if err.Error() == "clip not found" {
				types.SendNotFound(c, "Clip not found")
			} else {
				types.SendInternalError(c, fmt.Sprintf("Failed to update label: %v", err))
			}
			return
		}

		c.JSON(http.StatusOK, ClipResponse{
			UUID:              clip.UUID,
			Label:             clip.Label,
			Status:            clip.Status,
			ClipFilename:      clip.ClipFilename,
			ClipDuration:      clip.ClipDuration,
			ClipSizeBytes:     clip.ClipSizeBytes,
			SourceEpisodeURL:  clip.SourceEpisodeURL,
			OriginalStartTime: clip.OriginalStartTime,
			OriginalEndTime:   clip.OriginalEndTime,
			ErrorMessage:      clip.ErrorMessage,
			CreatedAt:         clip.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:         clip.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
}

// DeleteClip deletes a clip
// @Summary Delete a clip and its audio file
// @Description Permanently delete a clip from the database and remove its associated audio file from storage.
// @Description This operation cannot be undone. If the clip is already deleted, returns success (idempotent).
// @Tags clips
// @Param uuid path string true "Unique clip identifier (UUID format)"
// @Success 204 "Clip deleted successfully (no content returned)"
// @Failure 400 {object} types.ErrorResponse "Invalid UUID format"
// @Failure 500 {object} types.ErrorResponse "Internal server error during deletion"
// @Router /api/v1/clips/{uuid} [delete]
func DeleteClip(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		uuid := c.Param("uuid")
		if uuid == "" {
			types.SendBadRequest(c, "UUID is required")
			return
		}

		if err := deps.ClipService.DeleteClip(c.Request.Context(), uuid); err != nil {
			types.SendInternalError(c, fmt.Sprintf("Failed to delete clip: %v", err))
			return
		}

		c.Status(http.StatusNoContent)
	}
}

// ListClips lists clips with optional filters
// @Summary List clips with optional filtering
// @Description Retrieve a paginated list of clips with optional filtering by label and processing status.
// @Description Results are ordered by creation time (newest first). Use this endpoint to monitor clip processing
// @Description or to browse available training data by label.
// @Tags clips
// @Produce json
// @Param label query string false "Filter clips by exact label match (e.g., 'advertisement')"
// @Param status query string false "Filter by processing status" Enums(processing, ready, failed)
// @Param limit query int false "Maximum number of clips to return (1-1000)" default(100) minimum(1) maximum(1000)
// @Param offset query int false "Number of clips to skip for pagination" default(0) minimum(0)
// @Success 200 {array} ClipResponse "List of clips matching the filters"
// @Failure 500 {object} types.ErrorResponse "Internal server error"
// @Router /api/v1/clips [get]
func ListClips(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse query parameters
		label := c.Query("label")
		status := c.Query("status")

		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

		// List clips
		clipsList, err := deps.ClipService.ListClips(c.Request.Context(), clips.ListClipsFilters{
			Label:  label,
			Status: status,
			Limit:  limit,
			Offset: offset,
		})

		if err != nil {
			types.SendInternalError(c, fmt.Sprintf("Failed to list clips: %v", err))
			return
		}

		// Convert to response format
		response := make([]ClipResponse, len(clipsList))
		for i, clip := range clipsList {
			response[i] = ClipResponse{
				UUID:              clip.UUID,
				Label:             clip.Label,
				Status:            clip.Status,
				ClipFilename:      clip.ClipFilename,
				ClipDuration:      clip.ClipDuration,
				ClipSizeBytes:     clip.ClipSizeBytes,
				SourceEpisodeURL:  clip.SourceEpisodeURL,
				OriginalStartTime: clip.OriginalStartTime,
				OriginalEndTime:   clip.OriginalEndTime,
				ErrorMessage:      clip.ErrorMessage,
				CreatedAt:         clip.CreatedAt.Format("2006-01-02T15:04:05Z"),
				UpdatedAt:         clip.UpdatedAt.Format("2006-01-02T15:04:05Z"),
			}
		}

		c.JSON(http.StatusOK, response)
	}
}

// ExportDataset exports all clips as a dataset
// @Summary Export ML training dataset as ZIP
// @Description Export all clips with status "ready" as a ZIP archive for machine learning training.
// @Description The archive contains audio files organized by label directories and a JSONL manifest file
// @Description with metadata for each clip. Audio files are in 16kHz mono WAV format, suitable for
// @Description training models like Whisper or Wav2Vec2. The manifest includes clip UUID, label, duration,
// @Description source URL, and original time range for full traceability.
// @Tags clips
// @Produce application/zip
// @Success 200 {file} binary "ZIP archive containing labeled audio clips and manifest.jsonl"
// @Failure 500 {object} types.ErrorResponse "Internal server error during export"
// @Router /api/v1/clips/export [get]
func ExportDataset(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Create temporary directory for export
		tempDir, err := os.MkdirTemp("", "dataset_export_*")
		if err != nil {
			types.SendInternalError(c, fmt.Sprintf("Failed to create temp directory: %v", err))
			return
		}
		defer os.RemoveAll(tempDir) // Clean up

		// Export dataset to temp directory
		if err := deps.ClipService.ExportDataset(c.Request.Context(), tempDir); err != nil {
			types.SendInternalError(c, fmt.Sprintf("Failed to export dataset: %v", err))
			return
		}

		// Create ZIP file
		zipPath := filepath.Join(tempDir, "dataset.zip")
		if err := createZip(tempDir, zipPath); err != nil {
			types.SendInternalError(c, fmt.Sprintf("Failed to create ZIP: %v", err))
			return
		}

		// Send ZIP file
		c.Header("Content-Description", "File Transfer")
		c.Header("Content-Transfer-Encoding", "binary")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=dataset_%d.zip", time.Now().Unix()))
		c.Header("Content-Type", "application/zip")
		c.File(zipPath)
	}
}

// createZip creates a ZIP archive from a directory
func createZip(sourceDir, targetPath string) error {
	zipFile, err := os.Create(targetPath)
	if err != nil {
		return err
	}
	defer zipFile.Close()

	archive := zip.NewWriter(zipFile)
	defer archive.Close()

	// Walk through all files
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the ZIP file itself
		if path == targetPath {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		// Create header
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = relPath

		// Handle directories
		if info.IsDir() {
			header.Name += "/"
		}

		// Create writer
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return err
		}

		// If it's a file, copy contents
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			_, err = io.Copy(writer, file)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}
