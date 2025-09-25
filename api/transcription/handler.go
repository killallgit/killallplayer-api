package transcription

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
)

// TriggerTranscription manually triggers transcription generation for an episode
// @Summary      Trigger transcription generation
// @Description Manually trigger transcription generation for a specific episode. Will first check for existing transcripts at the episode's transcriptURL before using Whisper.
// @Tags         transcription
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode ID (Podcast Index ID)"
// @Success      200 {object} types.JobStatusResponse "Transcription already exists (source: 'fetched' or 'generated')"
// @Success      202 {object} types.JobStatusResponse "Transcription generation triggered"
// @Failure      400 {object} types.ErrorResponse "Invalid Podcast Index Episode ID"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes/{id}/transcribe [post]
func TriggerTranscription(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || episodeID < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Podcast Index Episode ID"})
			return
		}

		// Check if TranscriptionService is available
		if deps.TranscriptionService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Transcription service not available",
				"episode_id": episodeID,
			})
			return
		}

		// Check if JobService is available
		if deps.JobService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Job service not available",
				"episode_id": episodeID,
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Check if transcription already exists
		existingTranscription, err := deps.TranscriptionService.GetTranscription(ctx, uint(episodeID))
		if err == nil && existingTranscription != nil {
			c.JSON(http.StatusOK, gin.H{
				"message":    "Transcription already exists",
				"episode_id": episodeID,
				"status":     "completed",
				"progress":   100,
			})
			return
		}

		// Check if there's already a job for this episode
		existingJob, jobErr := deps.JobService.GetJobForTranscription(ctx, int64(episodeID))
		if jobErr == nil && existingJob != nil {
			// Job already exists, return status based on job state
			switch existingJob.Status {
			case models.JobStatusPending, models.JobStatusProcessing:
				c.JSON(http.StatusAccepted, gin.H{
					"message":    "Transcription generation already in progress",
					"episode_id": episodeID,
					"job_id":     existingJob.ID,
					"status":     string(existingJob.Status),
					"progress":   existingJob.Progress,
				})
				return
			case models.JobStatusCompleted:
				// Job completed but transcription not found? Try to return success anyway
				c.JSON(http.StatusOK, gin.H{
					"message":    "Transcription generation completed",
					"episode_id": episodeID,
					"job_id":     existingJob.ID,
					"status":     "completed",
					"progress":   100,
				})
				return
			case models.JobStatusFailed:
				// Job failed, allow creating a new one
				log.Printf("Previous transcription job %d failed, creating new job", existingJob.ID)
			}
		}

		// Create a new transcription generation job
		payload := models.JobPayload{
			"episode_id": episodeID,
		}

		job, err := deps.JobService.EnqueueJob(ctx, models.JobTypeTranscriptionGeneration, payload)
		if err != nil {
			log.Printf("Failed to enqueue transcription job for episode %d: %v", episodeID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to trigger transcription generation",
				"episode_id": episodeID,
			})
			return
		}

		log.Printf("Enqueued transcription generation job %d for episode %d", job.ID, episodeID)
		c.JSON(http.StatusAccepted, gin.H{
			"message":    "Transcription generation triggered",
			"episode_id": episodeID,
			"job_id":     job.ID,
			"status":     string(job.Status),
			"progress":   job.Progress,
		})
	}
}

// GetTranscription returns transcription data for an episode
// @Summary      Get transcription data
// @Description Retrieve the transcription text and metadata for an episode. Returns transcriptions that were either fetched from external URLs or generated via Whisper.
// @Tags         transcription
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode ID (Podcast Index ID)"
// @Success      200 {object} types.TranscriptionData "Transcription data (includes source: 'fetched' or 'generated')"
// @Failure      400 {object} types.ErrorResponse "Invalid Podcast Index Episode ID"
// @Failure      404 {object} types.ErrorResponse "Transcription not found"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes/{id}/transcribe [get]
func GetTranscription(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || episodeID < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Podcast Index Episode ID"})
			return
		}

		// Check if TranscriptionService is available
		if deps.TranscriptionService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Transcription service not available",
				"episode_id": episodeID,
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Get transcription from database
		transcriptionModel, err := deps.TranscriptionService.GetTranscription(ctx, uint(episodeID))
		if err != nil {
			// Check if it's a not found error
			if err.Error() == "transcription not found" {
				c.JSON(http.StatusNotFound, gin.H{
					"error":      "Transcription not found for episode",
					"episode_id": episodeID,
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to retrieve transcription",
				"episode_id": episodeID,
			})
			return
		}

		// Convert to response format
		transcriptionData := &types.TranscriptionData{
			EpisodeID: uint(episodeID),
			Text:      transcriptionModel.Text,
			Language:  transcriptionModel.Language,
			Duration:  transcriptionModel.Duration,
			Model:     transcriptionModel.Model,
			Cached:    true, // Always true since it's from database
		}

		c.JSON(http.StatusOK, transcriptionData)
	}
}

// GetTranscriptionStatus returns the processing status of a transcription
// @Summary      Get transcription generation status
// @Description Check the status of transcription generation for an episode
// @Tags         transcription
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode ID (Podcast Index ID)"
// @Success      200 {object} types.JobStatusResponse "Transcription status"
// @Failure      400 {object} types.ErrorResponse "Invalid Podcast Index Episode ID"
// @Failure      404 {object} types.JobStatusResponse "Transcription not available"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes/{id}/transcribe/status [get]
func GetTranscriptionStatus(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || episodeID < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Podcast Index Episode ID"})
			return
		}

		// Check if TranscriptionService is available
		if deps.TranscriptionService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Transcription service not available",
				"episode_id": episodeID,
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Check if transcription exists
		transcriptionModel, err := deps.TranscriptionService.GetTranscription(ctx, uint(episodeID))
		if err == nil && transcriptionModel != nil {
			status := gin.H{
				"episode_id": episodeID,
				"status":     "completed",
				"progress":   100,
				"message":    "Transcription ready",
			}
			c.JSON(http.StatusOK, status)
			return
		}

		// Check if there's a job in progress
		if deps.JobService != nil {
			job, jobErr := deps.JobService.GetJobForTranscription(ctx, int64(episodeID))
			if jobErr == nil && job != nil {
				status := gin.H{
					"episode_id": episodeID,
					"job_id":     job.ID,
					"status":     string(job.Status),
					"progress":   job.Progress,
					"message":    "Transcription generation in progress",
				}

				if job.Status == models.JobStatusFailed {
					status["message"] = "Transcription generation failed"
					status["error"] = job.Error
				}

				c.JSON(http.StatusOK, status)
				return
			}
		}

		status := gin.H{
			"episode_id": episodeID,
			"status":     "not_found",
			"progress":   0,
			"message":    "Transcription not available",
		}
		c.JSON(http.StatusNotFound, status)
	}
}
