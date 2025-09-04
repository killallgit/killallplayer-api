package episodes

import (
	"context"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
)

// GetByIDEnhanced returns a single episode by Podcast Index ID with waveform status
// @Summary      Get episode by ID with waveform status
// @Description  Retrieve a single episode by its Podcast Index ID including waveform processing status
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        id path int true "Episode Podcast Index ID" minimum(1) example(123456789)
// @Success      200 {object} EnhancedEpisodeResponse "Episode details with waveform status"
// @Failure      400 {object} episodes.PodcastIndexErrorResponse "Bad request - invalid ID"
// @Failure      404 {object} episodes.PodcastIndexErrorResponse "Episode not found"
// @Failure      500 {object} episodes.PodcastIndexErrorResponse "Internal server error"
// @Router       /api/v1/episodes/{id}/enhanced [get]
func GetByIDEnhanced(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")
		log.Printf("[DEBUG] GetByIDEnhanced called with Podcast Index ID: %s", episodeIDStr)

		// Parse Podcast Index ID (int64)
		podcastIndexID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil {
			log.Printf("[ERROR] Invalid episode ID '%s': %v", episodeIDStr, err)
			c.JSON(http.StatusBadRequest, deps.EpisodeTransformer.CreateErrorResponse("Invalid episode ID"))
			return
		}

		// Fetch episode
		log.Printf("[DEBUG] Fetching episode with Podcast Index ID: %d", podcastIndexID)
		episode, err := deps.EpisodeService.GetEpisodeByPodcastIndexID(c.Request.Context(), podcastIndexID)
		if err != nil {
			if IsNotFound(err) {
				log.Printf("[WARN] Episode not found - Podcast Index ID: %d, Error: %v", podcastIndexID, err)
				c.JSON(http.StatusNotFound, deps.EpisodeTransformer.CreateErrorResponse("Episode not found"))
			} else {
				log.Printf("[ERROR] Failed to fetch episode with Podcast Index ID %d: %v", podcastIndexID, err)
				c.JSON(http.StatusInternalServerError, deps.EpisodeTransformer.CreateErrorResponse("Failed to fetch episode"))
			}
			return
		}

		log.Printf("[DEBUG] Episode found - ID: %d, Title: %s", episode.ID, episode.Title)

		// Create enhanced response with waveform status
		response := createEnhancedResponse(c.Request.Context(), deps, episode)
		c.JSON(http.StatusOK, response)
	}
}

// createEnhancedResponse creates an enhanced episode response with waveform status
func createEnhancedResponse(ctx context.Context, deps *types.Dependencies, episode *models.Episode) EnhancedEpisodeResponse {
	// Convert episode to Podcast Index format
	pieFormat := deps.EpisodeTransformer.ModelToPodcastIndex(episode)

	response := EnhancedEpisodeResponse{
		PodcastIndexEpisode: pieFormat,
	}

	// Check for waveform and add status
	if deps.WaveformService != nil {
		waveformStatus := getWaveformStatus(ctx, deps, episode)
		response.Waveform = waveformStatus
	}

	return response
}

// getWaveformStatus retrieves or triggers waveform processing
func getWaveformStatus(ctx context.Context, deps *types.Dependencies, episode *models.Episode) *WaveformStatus {
	// Check if waveform exists
	waveform, err := deps.WaveformService.GetWaveform(ctx, episode.ID)
	if err == nil && waveform != nil {
		// Waveform exists - return with data
		peaks, _ := waveform.Peaks()
		return &WaveformStatus{
			Status:  WaveformStatusOK,
			Message: WaveformStatusMessages[WaveformStatusOK],
			Data: &WaveformData{
				Peaks:      peaks,
				Duration:   waveform.Duration,
				Resolution: waveform.Resolution,
				SampleRate: waveform.SampleRate,
			},
		}
	}

	// Waveform doesn't exist - check for existing job
	if deps.JobService != nil {
		job, jobErr := deps.JobService.GetJobForWaveform(ctx, episode.ID)
		if jobErr == nil && job != nil {
			// Job exists - return status based on job state
			return mapJobToWaveformStatus(job)
		}

		// No job exists - auto-trigger new job if episode has audio URL
		if episode.AudioURL != "" {
			log.Printf("[DEBUG] Auto-triggering waveform generation for episode %d", episode.ID)
			payload := models.JobPayload{
				"episode_id": episode.PodcastIndexID,
			}

			newJob, err := deps.JobService.EnqueueJob(ctx, models.JobTypeWaveformGeneration, payload)
			if err != nil {
				log.Printf("[ERROR] Failed to enqueue waveform job for episode %d: %v", episode.ID, err)
				return &WaveformStatus{
					Status:  WaveformStatusError,
					Message: "Failed to start waveform generation",
				}
			}

			log.Printf("[DEBUG] Enqueued waveform job %d for episode %d", newJob.ID, episode.ID)
			return mapJobToWaveformStatus(newJob)
		}
	}

	// No waveform and can't generate
	if episode.AudioURL == "" {
		return &WaveformStatus{
			Status:  WaveformStatusError,
			Message: "No audio URL available",
		}
	}

	// Waveform service not available
	return &WaveformStatus{
		Status:  WaveformStatusError,
		Message: "Waveform service unavailable",
	}
}

// mapJobToWaveformStatus converts job status to waveform status
func mapJobToWaveformStatus(job *models.Job) *WaveformStatus {
	switch job.Status {
	case models.JobStatusPending:
		return &WaveformStatus{
			Status:   WaveformStatusDownloading,
			Message:  WaveformStatusMessages[WaveformStatusDownloading],
			Progress: 0,
		}
	case models.JobStatusProcessing:
		status := WaveformStatusProcessing
		// If progress is less than 50, we're still downloading
		if job.Progress < 50 {
			status = WaveformStatusDownloading
		}
		return &WaveformStatus{
			Status:   status,
			Message:  WaveformStatusMessages[status],
			Progress: job.Progress,
		}
	case models.JobStatusCompleted:
		// Job completed but waveform not found - shouldn't happen but handle gracefully
		return &WaveformStatus{
			Status:  WaveformStatusError,
			Message: "Processing completed but waveform not found",
		}
	case models.JobStatusFailed:
		return &WaveformStatus{
			Status:  WaveformStatusError,
			Message: job.Error,
		}
	default:
		return &WaveformStatus{
			Status:  WaveformStatusError,
			Message: "Unknown processing status",
		}
	}
}
