package waveform

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/waveforms"
)

// WaveformData represents the waveform peaks for an audio file
type WaveformData struct {
	EpisodeID  int64     `json:"episode_id"`
	Peaks      []float32 `json:"peaks"`
	Duration   float64   `json:"duration"`   // Duration in seconds
	Resolution int       `json:"resolution"` // Number of peaks
	SampleRate int       `json:"sample_rate,omitempty"`
	Cached     bool      `json:"cached"`
}

// GetWaveform returns waveform data for an episode
// GetWaveform returns waveform data for an episode
// @Summary Get waveform data for an episode
// @Description Retrieve generated waveform data for a specific episode. If waveform doesn't exist, it will be queued for generation. Failed jobs are retried with exponential backoff.
// @Tags Waveform
// @Accept json
// @Produce json
// @Param id path int true "Episode ID (Podcast Index ID)"
// @Success 200 {object} WaveformData "Waveform data retrieved successfully"
// @Success 202 {object} map[string]interface{} "Waveform generation in progress"
// @Failure 400 {object} map[string]interface{} "Invalid episode ID"
// @Failure 404 {object} map[string]interface{} "Episode or waveform not found"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Failure 503 {object} map[string]interface{} "Waveform generation failed, retry pending (includes retry_after in seconds)"
// @Router /api/v1/episodes/{id}/waveform [get]
func GetWaveform(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID (this is the Podcast Index ID from the URL)
		podcastIndexID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || podcastIndexID < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode ID"})
			return
		}

		// Check if WaveformService is available
		if deps.WaveformService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Waveform service not available",
				"episode_id": podcastIndexID,
			})
			return
		}

		// Check if EpisodeService is available
		if deps.EpisodeService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Episode service not available",
				"episode_id": podcastIndexID,
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Debug logging
		log.Printf("[DEBUG] GetWaveform: PodcastIndexID=%d", podcastIndexID)

		// Get waveform directly using Podcast Index Episode ID
		waveformModel, err := deps.WaveformService.GetWaveform(ctx, uint(podcastIndexID))
		if err != nil {
			if errors.Is(err, waveforms.ErrWaveformNotFound) {
				// Check if there's already a job for this episode (using Podcast Index ID)
				if deps.JobService != nil {
					existingJob, jobErr := deps.JobService.GetJobForWaveform(ctx, uint(podcastIndexID))
					if jobErr == nil && existingJob != nil {
						// Job already exists, return status based on job state
						switch existingJob.Status {
						case models.JobStatusPending, models.JobStatusProcessing:
							c.JSON(http.StatusAccepted, gin.H{
								"message":    "Waveform generation in progress",
								"episode_id": podcastIndexID,
								"job_id":     existingJob.ID,
								"status":     string(existingJob.Status),
								"progress":   existingJob.Progress,
							})
							return
						case models.JobStatusFailed:
							// Check if job can be retried
							minRetryDelay := 30 * time.Second // Minimum 30 seconds between retries
							if !existingJob.CanRetryNow(minRetryDelay) {
								remainingDelay := minRetryDelay*time.Duration(1<<uint(existingJob.RetryCount)) - time.Since(*existingJob.LastFailedAt)
								c.JSON(http.StatusServiceUnavailable, gin.H{
									"message":     "Waveform generation failed, retry pending",
									"episode_id":  podcastIndexID,
									"job_id":      existingJob.ID,
									"status":      string(existingJob.Status),
									"retry_count": existingJob.RetryCount,
									"max_retries": existingJob.MaxRetries,
									"retry_after": remainingDelay.Seconds(),
									"error":       existingJob.Error,
								})
								return
							}
							// Job can be retried, allow creating a new one
							log.Printf("Retrying failed waveform job %d for episode %d (attempt %d/%d)",
								existingJob.ID, podcastIndexID, existingJob.RetryCount+1, existingJob.MaxRetries)
						case models.JobStatusCompleted:
							// Job completed but waveform not found? Try to check again
							// This might happen if the job just completed
							waveformModel, err = deps.WaveformService.GetWaveform(ctx, uint(podcastIndexID))
							if err == nil {
								// Found it! Continue to return the waveform
								goto returnWaveform
							}
							c.JSON(http.StatusInternalServerError, gin.H{
								"error":      "Waveform processing completed but data not found",
								"episode_id": podcastIndexID,
							})
							return
						}
					}

					// No existing job or job failed, create a new waveform generation job
					payload := models.JobPayload{
						"episode_id": podcastIndexID, // Use Podcast Index ID in the job payload
					}

					job, jobErr := deps.JobService.EnqueueJob(ctx, models.JobTypeWaveformGeneration, payload)
					if jobErr != nil {
						log.Printf("Failed to enqueue waveform job for episode %d: %v", podcastIndexID, jobErr)
					} else {
						log.Printf("Enqueued waveform generation job %d for Podcast Index Episode %d", job.ID, podcastIndexID)
					}
				}

				c.JSON(http.StatusNotFound, gin.H{
					"error":      "Waveform not found for episode",
					"episode_id": podcastIndexID,
					"message":    "Waveform generation has been queued",
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to retrieve waveform",
				"episode_id": podcastIndexID,
			})
			return
		}

	returnWaveform:
		// Decode peaks data
		peaks, err := waveformModel.Peaks()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to decode waveform data",
				"episode_id": podcastIndexID,
			})
			return
		}

		// Convert to response format (use Podcast Index ID in response for consistency)
		waveformData := &WaveformData{
			EpisodeID:  podcastIndexID,
			Peaks:      peaks,
			Duration:   waveformModel.Duration,
			Resolution: waveformModel.Resolution,
			SampleRate: waveformModel.SampleRate,
			Cached:     true, // Always true since it's from database
		}

		c.JSON(http.StatusOK, waveformData)
	}
}

// TriggerWaveform manually triggers waveform generation for an episode
// @Summary Trigger waveform generation
// @Description Manually trigger waveform generation for a specific episode. Implements retry logic with exponential backoff (30s, 60s, 120s) and max 3 retries.
// @Tags Waveform
// @Accept json
// @Produce json
// @Param id path int true "Episode ID (Podcast Index ID)"
// @Success 200 {object} map[string]interface{} "Waveform already exists"
// @Success 202 {object} map[string]interface{} "Waveform generation triggered"
// @Failure 400 {object} map[string]interface{} "Invalid episode ID"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Failure 503 {object} map[string]interface{} "Previous generation failed, retry pending (includes retry_after, retry_count, max_retries)"
// @Router /api/v1/episodes/{id}/waveform [post]
func TriggerWaveform(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID (this is the Podcast Index ID from the URL)
		podcastIndexID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || podcastIndexID < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode ID"})
			return
		}

		// Check if WaveformService is available
		if deps.WaveformService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Waveform service not available",
				"episode_id": podcastIndexID,
			})
			return
		}

		// Check if JobService is available
		if deps.JobService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Job service not available",
				"episode_id": podcastIndexID,
			})
			return
		}

		// Check if EpisodeService is available
		if deps.EpisodeService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Episode service not available",
				"episode_id": podcastIndexID,
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Debug logging
		log.Printf("[DEBUG] TriggerWaveform: PodcastIndexID=%d", podcastIndexID)

		// Check if waveform already exists (using Podcast Index Episode ID)
		exists, err := deps.WaveformService.WaveformExists(ctx, uint(podcastIndexID))
		if err == nil && exists {
			c.JSON(http.StatusOK, gin.H{
				"message":    "Waveform already exists",
				"episode_id": podcastIndexID,
				"status":     "completed",
				"progress":   100,
			})
			return
		}

		// Check if there's already a job for this episode (using Podcast Index ID)
		existingJob, jobErr := deps.JobService.GetJobForWaveform(ctx, uint(podcastIndexID))
		if jobErr == nil && existingJob != nil {
			// Job already exists, return status based on job state
			switch existingJob.Status {
			case models.JobStatusPending, models.JobStatusProcessing:
				c.JSON(http.StatusAccepted, gin.H{
					"message":    "Waveform generation already in progress",
					"episode_id": podcastIndexID,
					"job_id":     existingJob.ID,
					"status":     string(existingJob.Status),
					"progress":   existingJob.Progress,
				})
				return
			case models.JobStatusCompleted:
				// Job completed but waveform not found? Try to return success anyway
				c.JSON(http.StatusOK, gin.H{
					"message":    "Waveform generation completed",
					"episode_id": podcastIndexID,
					"job_id":     existingJob.ID,
					"status":     "completed",
					"progress":   100,
				})
				return
			case models.JobStatusFailed:
				// Check if job can be retried
				minRetryDelay := 30 * time.Second
				if !existingJob.CanRetryNow(minRetryDelay) {
					remainingDelay := minRetryDelay*time.Duration(1<<uint(existingJob.RetryCount)) - time.Since(*existingJob.LastFailedAt)
					c.JSON(http.StatusServiceUnavailable, gin.H{
						"message":     "Waveform generation failed, retry pending",
						"episode_id":  podcastIndexID,
						"job_id":      existingJob.ID,
						"status":      string(existingJob.Status),
						"retry_count": existingJob.RetryCount,
						"max_retries": existingJob.MaxRetries,
						"retry_after": remainingDelay.Seconds(),
						"error":       existingJob.Error,
					})
					return
				}
				// Job can be retried, allow creating a new one
				log.Printf("Retrying failed waveform job %d for episode %d (attempt %d/%d)",
					existingJob.ID, podcastIndexID, existingJob.RetryCount+1, existingJob.MaxRetries)
			}
		}

		// Create a new waveform generation job (using Podcast Index ID)
		payload := models.JobPayload{
			"episode_id": podcastIndexID,
		}

		job, err := deps.JobService.EnqueueJob(ctx, models.JobTypeWaveformGeneration, payload)
		if err != nil {
			log.Printf("Failed to enqueue waveform job for episode %d: %v", podcastIndexID, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to trigger waveform generation",
				"episode_id": podcastIndexID,
			})
			return
		}

		log.Printf("Enqueued waveform generation job %d for Podcast Index Episode %d", job.ID, podcastIndexID)
		c.JSON(http.StatusAccepted, gin.H{
			"message":    "Waveform generation triggered",
			"episode_id": podcastIndexID,
			"job_id":     job.ID,
			"status":     string(job.Status),
			"progress":   job.Progress,
		})
	}
}

// GetWaveformStatus returns the processing status of a waveform
// GetWaveformStatus returns the processing status of a waveform
// @Summary Get waveform generation status
// @Description Check the status of waveform generation for a specific episode
// @Tags Waveform
// @Accept json
// @Produce json
// @Param id path int true "Episode ID (Podcast Index ID)"
// @Success 200 {object} map[string]interface{} "Status information"
// @Success 404 {object} map[string]interface{} "Waveform not found or episode not found"
// @Failure 400 {object} map[string]interface{} "Invalid episode ID"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Router /api/v1/episodes/{id}/waveform/status [get]
func GetWaveformStatus(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID (this is the Podcast Index ID from the URL)
		podcastIndexID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || podcastIndexID < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode ID"})
			return
		}

		// Check if WaveformService is available
		if deps.WaveformService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Waveform service not available",
				"episode_id": podcastIndexID,
			})
			return
		}

		// Check if EpisodeService is available
		if deps.EpisodeService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Episode service not available",
				"episode_id": podcastIndexID,
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Debug logging
		log.Printf("[DEBUG] GetWaveformStatus: PodcastIndexID=%d", podcastIndexID)

		// Check if waveform exists using Podcast Index Episode ID
		exists, err := deps.WaveformService.WaveformExists(ctx, uint(podcastIndexID))
		if err != nil {
			// Check if there's a job in progress
			if deps.JobService != nil {
				job, jobErr := deps.JobService.GetJobForWaveform(ctx, uint(podcastIndexID))
				if jobErr == nil && job != nil {
					status := gin.H{
						"episode_id": podcastIndexID,
						"job_id":     job.ID,
						"status":     string(job.Status),
						"progress":   job.Progress,
						"message":    "Waveform generation in progress",
					}

					if job.Status == models.JobStatusFailed {
						status["message"] = "Waveform generation failed"
						status["error"] = job.Error
					}

					c.JSON(http.StatusOK, status)
					return
				}
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to check waveform status",
				"episode_id": podcastIndexID,
			})
			return
		}

		if exists {
			status := gin.H{
				"episode_id": podcastIndexID,
				"status":     "completed",
				"progress":   100,
				"message":    "Waveform ready",
			}
			c.JSON(http.StatusOK, status)
		} else {
			// Check if there's a job in progress (using Podcast Index ID)
			if deps.JobService != nil {
				job, jobErr := deps.JobService.GetJobForWaveform(ctx, uint(podcastIndexID))
				if jobErr == nil && job != nil {
					status := gin.H{
						"episode_id": podcastIndexID,
						"job_id":     job.ID,
						"status":     string(job.Status),
						"progress":   job.Progress,
						"message":    "Waveform generation in progress",
					}

					if job.Status == models.JobStatusFailed {
						status["message"] = "Waveform generation failed"
						status["error"] = job.Error
					}

					c.JSON(http.StatusOK, status)
					return
				}
			}

			status := gin.H{
				"episode_id": podcastIndexID,
				"status":     "not_found",
				"progress":   0,
				"message":    "Waveform not available",
			}
			c.JSON(http.StatusNotFound, status)
		}
	}
}
