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

// GetWaveform returns waveform data for an episode
// @Summary      Get waveform data for an episode
// @Description Retrieve generated waveform data for a specific episode. If waveform doesn't exist, it will be queued for generation. Failed jobs are retried with exponential backoff.
// @Tags         waveform
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode ID (Podcast Index ID)"
// @Success      200 {object} types.WaveformData "Waveform data retrieved successfully (status='completed')"
// @Success      202 {object} types.JobStatusResponse "Waveform generation in progress"
// @Failure      400 {object} types.ErrorResponse "Invalid episode ID"
// @Failure      404 {object} types.ErrorResponse "Episode or waveform not found"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Failure      503 {object} types.JobStatusResponse "Waveform generation failed, retry pending (includes retry_after in seconds)"
// @Router       /api/v1/episodes/{id}/waveform [get]
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
		waveformData := &types.WaveformData{
			EpisodeID:  podcastIndexID,
			Status:     "completed",
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
// @Summary      Trigger waveform generation
// @Description Manually trigger waveform generation for a specific episode. Implements retry logic with exponential backoff (30s, 60s, 120s) and max 3 retries. Use query parameter retry=true to manually retry failed/permanently failed jobs.
// @Tags         waveform
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode ID (Podcast Index ID)"
// @Param        retry query bool false "Force retry of failed or permanently failed job"
// @Success      200 {object} types.JobStatusResponse "Waveform already exists or retry successful"
// @Success      202 {object} types.JobStatusResponse "Waveform generation triggered"
// @Failure      400 {object} types.ErrorResponse "Invalid episode ID or retry parameter"
// @Failure      409 {object} types.ErrorResponse "Job cannot be retried (not in failed state)"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Failure      503 {object} types.JobStatusResponse "Previous generation failed, retry pending (includes retry_after, retry_count, max_retries)"
// @Router       /api/v1/episodes/{id}/waveform [post]
func TriggerWaveform(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID (this is the Podcast Index ID from the URL)
		podcastIndexID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || podcastIndexID < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode ID"})
			return
		}

		// Check for retry query parameter
		retryParam := c.Query("retry")
		forceRetry := retryParam == "true"

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
			// Handle force retry for failed/permanently failed jobs
			if forceRetry && (existingJob.Status == models.JobStatusFailed || existingJob.Status == models.JobStatusPermanentlyFailed) {
				// Use RetryFailedJob to manually retry the job
				retriedJob, retryErr := deps.JobService.RetryFailedJob(ctx, existingJob.ID)
				if retryErr != nil {
					log.Printf("Failed to retry job %d: %v", existingJob.ID, retryErr)
					c.JSON(http.StatusConflict, gin.H{
						"error":      "Cannot retry job",
						"episode_id": podcastIndexID,
						"job_id":     existingJob.ID,
						"details":    retryErr.Error(),
					})
					return
				}

				log.Printf("Manually retried job %d for episode %d (status was %s, now %s)",
					retriedJob.ID, podcastIndexID, existingJob.Status, retriedJob.Status)

				c.JSON(http.StatusAccepted, gin.H{
					"message":    "Waveform generation retry triggered",
					"episode_id": podcastIndexID,
					"job_id":     retriedJob.ID,
					"status":     string(retriedJob.Status),
					"progress":   retriedJob.Progress,
					"retried":    true,
				})
				return
			}

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
						"hint":        "Use retry=true parameter to force immediate retry",
					})
					return
				}
				// Job can be retried, allow creating a new one
				log.Printf("Retrying failed waveform job %d for episode %d (attempt %d/%d)",
					existingJob.ID, podcastIndexID, existingJob.RetryCount+1, existingJob.MaxRetries)
			case models.JobStatusPermanentlyFailed:
				// Permanently failed jobs can only be retried with force retry parameter
				c.JSON(http.StatusConflict, gin.H{
					"message":     "Waveform generation permanently failed",
					"episode_id":  podcastIndexID,
					"job_id":      existingJob.ID,
					"status":      string(existingJob.Status),
					"retry_count": existingJob.RetryCount,
					"max_retries": existingJob.MaxRetries,
					"error":       existingJob.Error,
					"error_type":  existingJob.ErrorType,
					"error_code":  existingJob.ErrorCode,
					"hint":        "Use retry=true parameter to force manual retry",
				})
				return
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
// @Summary      Get waveform generation status
// @Description Check the status of waveform generation for a specific episode
// @Tags         waveform
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode ID (Podcast Index ID)"
// @Success      200 {object} types.WaveformData "Status information"
// @Failure      400 {object} types.ErrorResponse "Invalid episode ID"
// @Failure      404 {object} types.JobStatusResponse "Waveform not found or episode not found"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes/{id}/waveform/status [get]
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
						status["error_type"] = job.ErrorType
						status["error_code"] = job.ErrorCode
						status["error_details"] = job.ErrorDetails
						status["retry_count"] = job.RetryCount
						status["max_retries"] = job.MaxRetries
					} else if job.Status == models.JobStatusPermanentlyFailed {
						status["message"] = "Waveform generation permanently failed"
						status["error"] = job.Error
						status["error_type"] = job.ErrorType
						status["error_code"] = job.ErrorCode
						status["error_details"] = job.ErrorDetails
						status["retry_count"] = job.RetryCount
						status["max_retries"] = job.MaxRetries
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
			// Get the actual waveform data to return full response
			waveformModel, err := deps.WaveformService.GetWaveform(ctx, uint(podcastIndexID))
			if err != nil {
				// Fallback to basic status response if we can't get the waveform data
				status := gin.H{
					"episode_id": podcastIndexID,
					"status":     "completed",
					"progress":   100,
					"message":    "Waveform ready",
				}
				c.JSON(http.StatusOK, status)
				return
			}

			// Decode peaks data
			peaks, err := waveformModel.Peaks()
			if err != nil {
				// Fallback to basic status response if we can't decode peaks
				status := gin.H{
					"episode_id": podcastIndexID,
					"status":     "completed",
					"progress":   100,
					"message":    "Waveform ready",
				}
				c.JSON(http.StatusOK, status)
				return
			}

			// Return full waveform data (same structure as GetWaveform endpoint)
			waveformData := &types.WaveformData{
				EpisodeID:  podcastIndexID,
				Status:     "completed",
				Peaks:      peaks,
				Duration:   waveformModel.Duration,
				Resolution: waveformModel.Resolution,
				SampleRate: waveformModel.SampleRate,
				Cached:     true, // Always true since it's from database
			}
			c.JSON(http.StatusOK, waveformData)
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
						status["error_type"] = job.ErrorType
						status["error_code"] = job.ErrorCode
						status["error_details"] = job.ErrorDetails
						status["retry_count"] = job.RetryCount
						status["max_retries"] = job.MaxRetries
					} else if job.Status == models.JobStatusPermanentlyFailed {
						status["message"] = "Waveform generation permanently failed"
						status["error"] = job.Error
						status["error_type"] = job.ErrorType
						status["error_code"] = job.ErrorCode
						status["error_details"] = job.ErrorDetails
						status["retry_count"] = job.RetryCount
						status["max_retries"] = job.MaxRetries
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
