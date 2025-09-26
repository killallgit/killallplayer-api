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
// @Summary      Get audio waveform visualization data
// @Description  Retrieve waveform data for audio visualization of a podcast episode. Waveform data consists of
// @Description  amplitude values (0-1 range) sampled at regular intervals, suitable for rendering audio waveform
// @Description  visualizations. If waveform doesn't exist, it will be automatically queued for generation and the
// @Description  response will include status:"pending" or "processing". Generation typically takes 10-60 seconds
// @Description  depending on episode duration. Poll this endpoint until status:"ready" to get the final data.
// @Tags         waveform
// @Accept       json
// @Produce      json
// @Param        id path int64 true "Episode's Podcast Index ID" minimum(1)
// @Success      200 {object} types.WaveformResponse "Waveform ready with amplitude data array (status:ready)"
// @Success      202 {object} types.WaveformResponse "Generation in progress (status:processing or pending)"
// @Failure      400 {object} types.ErrorResponse "Invalid episode ID format"
// @Failure      500 {object} types.ErrorResponse "Waveform service error or database failure"
// @Failure      503 {object} types.WaveformResponse "Generation failed, automatic retry scheduled (status:failed)"
// @Router       /api/v1/episodes/{id}/waveform [get]
func GetWaveform(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")

		// Parse episode ID (this is the Podcast Index ID from the URL)
		podcastIndexID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || podcastIndexID < 0 {
			c.JSON(http.StatusBadRequest, types.ErrorResponse{
				Status:  types.StatusError,
				Message: "Invalid Podcast Index Episode ID",
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
		waveformModel, err := deps.WaveformService.GetWaveform(ctx, podcastIndexID)
		if err != nil {
			if errors.Is(err, waveforms.ErrWaveformNotFound) {
				// Check if there's already a job for this episode (using Podcast Index ID)
				if deps.JobService != nil {
					existingJob, jobErr := deps.JobService.GetJobForWaveform(ctx, podcastIndexID)
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
							waveformModel, err = deps.WaveformService.GetWaveform(ctx, podcastIndexID)
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
							// Job permanently failed - allow creating a new job after cleanup
							log.Printf("Previous waveform job %d permanently failed for episode %d, will clean up and retry",
								existingJob.ID, podcastIndexID)

							// Clean up the permanently failed job by deleting it
							if deps.JobService != nil {
								if err := deps.JobService.DeletePermanentlyFailedJob(c.Request.Context(), existingJob.ID); err != nil {
									log.Printf("Failed to delete permanently failed job %d: %v", existingJob.ID, err)
								} else {
									log.Printf("Successfully deleted permanently failed job %d for episode %d", existingJob.ID, podcastIndexID)
								}
							}

							// Continue to create a new job (don't return here)
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

					// No existing job or cleaned up permanently failed job - create a new one
					// Use atomic job creation to handle race conditions
					payload := models.JobPayload{
						"episode_id": podcastIndexID, // Use Podcast Index ID in the job payload
					}

					job, jobErr := deps.JobService.EnqueueUniqueJob(ctx, models.JobTypeWaveformGeneration, payload, "episode_id")
					if jobErr != nil {
						log.Printf("Failed to enqueue waveform job for episode %d: %v", podcastIndexID, jobErr)
					} else {
						if job.ID > 0 {
							// Check if this is a new job or existing job returned by EnqueueUniqueJob
							if job.Status == models.JobStatusPending {
								log.Printf("Enqueued waveform generation job %d for Podcast Index Episode %d", job.ID, podcastIndexID)
							} else {
								log.Printf("Existing waveform job %d found for episode %d (Status: %s)", job.ID, podcastIndexID, job.Status)
							}
						}
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
