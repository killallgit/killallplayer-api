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
// @Summary      Generate or fetch episode transcription
// @Description  Trigger transcription for a podcast episode. The system first checks if a transcript is available
// @Description  at the episode's transcriptURL (from RSS feed). If found, it fetches and stores it. Otherwise, if
// @Description  Whisper is configured, it generates a transcription using speech-to-text. Transcription is an async
// @Description  process that may take several minutes depending on episode duration. Poll the status endpoint with job_id to track progress.
// @Tags         transcription
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode's Podcast Index ID" minimum(1)
// @Success      200 {object} types.JobStatusResponse "Transcription already exists and is ready"
// @Success      202 {object} types.JobStatusResponse "Transcription job queued successfully (use job_id to track)"
// @Failure      400 {object} types.ErrorResponse "Invalid episode ID format"
// @Failure      500 {object} types.ErrorResponse "Service unavailable or configuration error"
// @Router       /api/v1/episodes/{id}/transcribe [post]
func TriggerTranscription(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || episodeID < 0 {
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Invalid Podcast Index Episode ID",
			})
			return
		}

		// Check if TranscriptionService is available
		if deps.TranscriptionService == nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Transcription service not available",
			})
			return
		}

		// Check if JobService is available
		if deps.JobService == nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Job service not available",
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Check if transcription already exists
		existingTranscription, err := deps.TranscriptionService.GetTranscription(ctx, episodeID)
		if err == nil && existingTranscription != nil {
			c.JSON(http.StatusOK, types.JobStatusResponse{
				EpisodeID: episodeID,
				Status:    "completed",
				Progress:  100,
				Message:   "Transcription already exists",
			})
			return
		}

		// Check if there's already a job for this episode
		existingJob, jobErr := deps.JobService.GetJobForTranscription(ctx, int64(episodeID))
		if jobErr == nil && existingJob != nil {
			// Job already exists, return status based on job state
			switch existingJob.Status {
			case models.JobStatusPending, models.JobStatusProcessing:
				c.JSON(http.StatusAccepted, types.JobStatusResponse{
					EpisodeID: episodeID,
					JobID:     existingJob.ID,
					Status:    string(existingJob.Status),
					Progress:  existingJob.Progress,
					Message:   "Transcription generation already in progress",
				})
				return
			case models.JobStatusCompleted:
				// Job completed but transcription not found? Try to return success anyway
				c.JSON(http.StatusOK, types.JobStatusResponse{
					EpisodeID: episodeID,
					JobID:     existingJob.ID,
					Status:    "completed",
					Progress:  100,
					Message:   "Transcription generation completed",
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
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Failed to trigger transcription generation",
				Details: err.Error(),
			})
			return
		}

		log.Printf("Enqueued transcription generation job %d for episode %d", job.ID, episodeID)
		c.JSON(http.StatusAccepted, types.JobStatusResponse{
			EpisodeID: episodeID,
			JobID:     job.ID,
			Status:    string(job.Status),
			Progress:  job.Progress,
			Message:   "Transcription generation triggered",
		})
	}
}

// GetTranscription returns transcription data for an episode
// @Summary      Get episode transcription text
// @Description  Retrieve the full transcription text for a podcast episode if available. Transcriptions may come
// @Description  from two sources: 'fetched' (downloaded from podcast RSS feed transcriptURL) or 'generated' (created
// @Description  using Whisper speech-to-text). The response includes the full text, source type, language, and timestamps.
// @Description  Use POST /episodes/{id}/transcribe first to trigger generation if transcription doesn't exist.
// @Tags         transcription
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode's Podcast Index ID" minimum(1)
// @Success      200 {object} types.TranscriptionData "Full transcription text with metadata"
// @Failure      400 {object} types.ErrorResponse "Invalid episode ID format"
// @Failure      404 {object} types.ErrorResponse "No transcription available for this episode"
// @Failure      500 {object} types.ErrorResponse "Database or service error"
// @Router       /api/v1/episodes/{id}/transcribe [get]
func GetTranscription(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || episodeID < 0 {
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Invalid Podcast Index Episode ID",
			})
			return
		}

		// Check if TranscriptionService is available
		if deps.TranscriptionService == nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Transcription service not available",
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Get transcription from database
		transcriptionModel, err := deps.TranscriptionService.GetTranscription(ctx, episodeID)
		if err != nil {
			// Check if it's a not found error
			if err.Error() == "transcription not found" {
				c.JSON(http.StatusNotFound, types.ErrorResponse{
					Status:  types.StatusError,
					Message: "Transcription not found for episode",
				})
				return
			}

			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Failed to retrieve transcription",
				Details: err.Error(),
			})
			return
		}

		// Convert to response format
		transcriptionData := &types.TranscriptionData{
			EpisodeID: episodeID,
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
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Invalid Podcast Index Episode ID",
			})
			return
		}

		// Check if TranscriptionService is available
		if deps.TranscriptionService == nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Transcription service not available",
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Check if transcription exists
		transcriptionModel, err := deps.TranscriptionService.GetTranscription(ctx, episodeID)
		if err == nil && transcriptionModel != nil {
			c.JSON(http.StatusOK, types.JobStatusResponse{
				EpisodeID: episodeID,
				Status:    "completed",
				Progress:  100,
				Message:   "Transcription ready",
			})
			return
		}

		// Check if there's a job in progress
		if deps.JobService != nil {
			job, jobErr := deps.JobService.GetJobForTranscription(ctx, int64(episodeID))
			if jobErr == nil && job != nil {
				response := types.JobStatusResponse{
					EpisodeID: episodeID,
					JobID:     job.ID,
					Status:    string(job.Status),
					Progress:  job.Progress,
					Message:   "Transcription generation in progress",
				}

				if job.Status == models.JobStatusFailed {
					response.Message = "Transcription generation failed"
					response.Error = job.Error
				}

				c.JSON(http.StatusOK, response)
				return
			}
		}

		c.JSON(http.StatusNotFound, types.JobStatusResponse{
			EpisodeID: episodeID,
			Status:    "not_found",
			Progress:  0,
			Message:   "Transcription not available",
		})
	}
}
