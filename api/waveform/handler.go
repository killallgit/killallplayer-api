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
							// Check if job can be retried
							minRetryDelay := 30 * time.Second // Minimum 30 seconds between retries
							if !existingJob.CanRetryNow(minRetryDelay) {
								c.JSON(http.StatusServiceUnavailable, types.WaveformResponse{
									BaseResponse: types.BaseResponse{
										Status:  types.StatusFailed,
										Message: "Waveform generation failed, retry pending",
									},
									Waveform: &types.Waveform{
										ID:        strconv.FormatInt(podcastIndexID, 10),
										EpisodeID: podcastIndexID,
										Status:    types.StatusFailed,
									},
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
							c.JSON(http.StatusInternalServerError, types.ErrorResponse{
								Status:  types.StatusError,
								Message: "Waveform processing completed but data not found",
							})
							return
						case models.JobStatusPermanentlyFailed:
							c.JSON(http.StatusServiceUnavailable, types.WaveformResponse{
								BaseResponse: types.BaseResponse{
									Status:  types.StatusFailed,
									Message: "Waveform generation permanently failed",
								},
								Waveform: &types.Waveform{
									ID:        strconv.FormatInt(podcastIndexID, 10),
									EpisodeID: podcastIndexID,
									Status:    types.StatusFailed,
								},
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
