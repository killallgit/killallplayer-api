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
func GetWaveform(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || episodeID < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode ID"})
			return
		}

		// Check if WaveformService is available
		if deps.WaveformService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Waveform service not available",
				"episode_id": episodeID,
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Get waveform from database
		waveformModel, err := deps.WaveformService.GetWaveform(ctx, uint(episodeID))
		if err != nil {
			if errors.Is(err, waveforms.ErrWaveformNotFound) {
				// Check if there's already a job for this episode
				if deps.JobService != nil {
					existingJob, jobErr := deps.JobService.GetJobForWaveform(ctx, uint(episodeID))
					if jobErr == nil && existingJob != nil {
						// Job already exists, return status based on job state
						switch existingJob.Status {
						case models.JobStatusPending, models.JobStatusProcessing:
							c.JSON(http.StatusAccepted, gin.H{
								"message":    "Waveform generation in progress",
								"episode_id": episodeID,
								"job_id":     existingJob.ID,
								"status":     string(existingJob.Status),
								"progress":   existingJob.Progress,
							})
							return
						case models.JobStatusFailed:
							// Job failed, create a new one
							break
						case models.JobStatusCompleted:
							// Job completed but waveform not found? This shouldn't happen, but handle gracefully
							c.JSON(http.StatusInternalServerError, gin.H{
								"error":      "Waveform processing completed but data not found",
								"episode_id": episodeID,
							})
							return
						}
					}

					// No existing job or job failed, create a new waveform generation job
					payload := models.JobPayload{
						"episode_id": episodeID,
					}

					job, jobErr := deps.JobService.EnqueueJob(ctx, models.JobTypeWaveformGeneration, payload)
					if jobErr != nil {
						log.Printf("Failed to enqueue waveform job for episode %d: %v", episodeID, jobErr)
					} else {
						log.Printf("Enqueued waveform generation job %d for episode %d", job.ID, episodeID)
					}
				}

				c.JSON(http.StatusNotFound, gin.H{
					"error":      "Waveform not found for episode",
					"episode_id": episodeID,
					"message":    "Waveform generation has been queued",
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to retrieve waveform",
				"episode_id": episodeID,
			})
			return
		}

		// Decode peaks data
		peaks, err := waveformModel.Peaks()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to decode waveform data",
				"episode_id": episodeID,
			})
			return
		}

		// Convert to response format
		waveformData := &WaveformData{
			EpisodeID:  episodeID,
			Peaks:      peaks,
			Duration:   waveformModel.Duration,
			Resolution: waveformModel.Resolution,
			SampleRate: waveformModel.SampleRate,
			Cached:     true, // Always true since it's from database
		}

		c.JSON(http.StatusOK, waveformData)
	}
}

// GetWaveformStatus returns the processing status of a waveform
func GetWaveformStatus(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || episodeID < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode ID"})
			return
		}

		// Check if WaveformService is available
		if deps.WaveformService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Waveform service not available",
				"episode_id": episodeID,
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Check if waveform exists
		exists, err := deps.WaveformService.WaveformExists(ctx, uint(episodeID))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to check waveform status",
				"episode_id": episodeID,
			})
			return
		}

		if exists {
			status := gin.H{
				"episode_id": episodeID,
				"status":     "completed",
				"progress":   100,
				"message":    "Waveform ready",
			}
			c.JSON(http.StatusOK, status)
		} else {
			// Check if there's a job in progress
			if deps.JobService != nil {
				job, jobErr := deps.JobService.GetJobForWaveform(ctx, uint(episodeID))
				if jobErr == nil && job != nil {
					status := gin.H{
						"episode_id": episodeID,
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
				"episode_id": episodeID,
				"status":     "not_found",
				"progress":   0,
				"message":    "Waveform not available",
			}
			c.JSON(http.StatusNotFound, status)
		}
	}
}
