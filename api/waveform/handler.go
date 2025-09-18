package waveform

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
	"github.com/killallgit/player-api/internal/services/waveforms"
)

// GetWaveform returns waveform data with status for an episode
// @Summary      Get waveform data and status for an episode
// @Description Retrieve generated waveform data and status for a specific episode. If waveform doesn't exist, it will be queued for generation. Failed jobs are retried with exponential backoff.
// @Tags         waveform
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode ID (Podcast Index ID)"
// @Success      200 {object} types.WaveformResponse "Waveform data retrieved successfully"
// @Success      202 {object} types.WaveformResponse "Waveform generation in progress or queued"
// @Failure      400 {object} types.ErrorResponse "Invalid episode ID"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Failure      503 {object} types.WaveformResponse "Waveform generation failed, retry pending"
// @Router       /api/v1/episodes/{id}/waveform [get]
func GetWaveform(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID (this is the Podcast Index ID from the URL)
		podcastIndexID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || podcastIndexID < 0 {
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Invalid episode ID",
			})
			return
		}

		// Check if WaveformService is available
		if deps.WaveformService == nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Waveform service not available",
			})
			return
		}

		// Check if EpisodeService is available
		if deps.EpisodeService == nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Episode service not available",
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
							c.JSON(http.StatusAccepted, types.WaveformResponse{
								BaseResponse: types.BaseResponse{
									Status:  types.StatusProcessing,
									Message: "Waveform generation in progress",
								},
								Waveform: &types.Waveform{
									ID:        strconv.FormatInt(podcastIndexID, 10),
									EpisodeID: podcastIndexID,
									Status:    types.StatusProcessing,
								},
							})
							return
						case models.JobStatusFailed:
							// Failed job exists - worker will retry it automatically
							// Don't create a new job, just report the current status
							c.JSON(http.StatusAccepted, types.WaveformResponse{
								BaseResponse: types.BaseResponse{
									Status: types.StatusProcessing,
									Message: fmt.Sprintf("Waveform generation failed, retry %d/%d pending",
										existingJob.RetryCount, existingJob.MaxRetries),
								},
								Waveform: &types.Waveform{
									ID:        strconv.FormatInt(podcastIndexID, 10),
									EpisodeID: podcastIndexID,
									Status:    types.StatusProcessing,
								},
							})
							return
						case models.JobStatusCompleted:
							// Job completed but waveform not found? Try to check again
							// This might happen if the job just completed
							waveformModel, err = deps.WaveformService.GetWaveform(ctx, uint(podcastIndexID))
							if err == nil {
								// Found it! Continue to return the waveform
								goto returnWaveform
							}
							c.JSON(http.StatusInternalServerError, types.ErrorResponse{
								Status:  types.StatusError,
								Message: "Waveform processing completed but data not found",
							})
							return
						case models.JobStatusPermanentlyFailed:
							// Job permanently failed - don't retry, return error status
							log.Printf("Previous waveform job %d permanently failed for episode %d, not allowing new jobs",
								existingJob.ID, podcastIndexID)

							// Return an error indicating permanent failure
							errorMsg := "Waveform generation permanently failed"
							if existingJob.Error != "" {
								errorMsg = fmt.Sprintf("Waveform generation permanently failed: %s", existingJob.Error)
							}

							c.JSON(http.StatusServiceUnavailable, types.ErrorResponse{
								Status:  types.StatusError,
								Message: errorMsg,
							})
							return
						default:
							// Unknown status - log and return error
							log.Printf("Unknown job status %s for job %d", existingJob.Status, existingJob.ID)
							c.JSON(http.StatusInternalServerError, types.ErrorResponse{
								Status:  types.StatusError,
								Message: "Unknown job status",
							})
							return
						}
					}

					// No existing job or permanently failed job - check duration before creating job

					// Get episode details to check duration BEFORE creating a job
					episode, err := deps.EpisodeService.GetEpisodeByPodcastIndexID(ctx, podcastIndexID)
					if err != nil {
						log.Printf("Failed to get episode %d for duration check: %v", podcastIndexID, err)
						c.JSON(http.StatusInternalServerError, types.ErrorResponse{
							Status:  types.StatusError,
							Message: "Failed to get episode details",
						})
						return
					}

					// Check if episode duration exceeds the processing limit BEFORE creating a job
					if episode.Duration != nil {
						const maxDurationSeconds = 7200 // 2 hours limit (matches DefaultProcessingOptions)
						episodeDuration := *episode.Duration

						if episodeDuration > maxDurationSeconds {
							log.Printf("Episode %d duration %d seconds exceeds limit %d seconds, rejecting waveform request",
								podcastIndexID, episodeDuration, maxDurationSeconds)

							c.JSON(http.StatusServiceUnavailable, types.ErrorResponse{
								Status: types.StatusError,
								Message: fmt.Sprintf("Episode duration (%.1f hours) exceeds maximum limit (%.1f hours)",
									float64(episodeDuration)/3600, float64(maxDurationSeconds)/3600),
							})
							return
						}
					}

					// Duration is acceptable - clean up any old permanently failed jobs and create a new one

					// If there was a permanently failed job, delete it from the database to clean up
					if existingJob != nil && existingJob.Status == models.JobStatusPermanentlyFailed {
						log.Printf("Deleting permanently failed job %d for episode %d", existingJob.ID, podcastIndexID)
						// Note: We'll add this functionality to the job service later
					}

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

				c.JSON(http.StatusAccepted, types.WaveformResponse{
					BaseResponse: types.BaseResponse{
						Status:  types.StatusQueued,
						Message: "Waveform generation has been queued",
					},
					Waveform: &types.Waveform{
						ID:        strconv.FormatInt(podcastIndexID, 10),
						EpisodeID: podcastIndexID,
						Status:    types.StatusQueued,
					},
				})
				return
			}

			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Failed to retrieve waveform",
			})
			return
		}

	returnWaveform:
		// Decode peaks data
		peaks, err := waveformModel.Peaks()
		if err != nil {
			c.JSON(http.StatusInternalServerError, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Failed to decode waveform data",
			})
			return
		}

		// Convert to response format (use Podcast Index ID in response for consistency)
		c.JSON(http.StatusOK, types.WaveformResponse{
			BaseResponse: types.BaseResponse{
				Status:  types.StatusOK,
				Message: "Waveform retrieved successfully",
			},
			Waveform: &types.Waveform{
				ID:         strconv.FormatInt(podcastIndexID, 10),
				EpisodeID:  podcastIndexID,
				Data:       peaks,
				Duration:   waveformModel.Duration,
				SampleRate: waveformModel.SampleRate,
				Status:     types.StatusOK,
			},
		})
	}
}
