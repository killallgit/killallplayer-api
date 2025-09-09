package episodes

import (
	"context"
	"log"

	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/models"
	internalEpisodes "github.com/killallgit/player-api/internal/services/episodes"
)

// EpisodeEnricher handles enriching episodes with additional data like waveforms
type EpisodeEnricher struct {
	deps *types.Dependencies
}

// NewEpisodeEnricher creates a new episode enricher
func NewEpisodeEnricher(deps *types.Dependencies) *EpisodeEnricher {
	return &EpisodeEnricher{deps: deps}
}

// EnrichSingleEpisodeWithWaveform adds waveform and transcription status to a single episode and triggers generation if needed
// This is only used for single episode GET requests, not for lists
func (e *EpisodeEnricher) EnrichSingleEpisodeWithWaveform(ctx context.Context, episode *internalEpisodes.PodcastIndexEpisode) *EnhancedEpisodeResponse {
	if episode == nil {
		return nil
	}

	enhanced := &EnhancedEpisodeResponse{
		PodcastIndexEpisode: *episode,
	}

	// Add waveform status if waveform service is available
	if e.deps.WaveformService != nil {
		enhanced.Waveform = e.getWaveformStatusForSingleEpisode(ctx, episode)
	}

	// Add transcription status if transcription service is available
	if e.deps.TranscriptionService != nil {
		enhanced.Transcription = e.getTranscriptionStatusForSingleEpisode(ctx, episode)
	}

	return enhanced
}

// getWaveformStatusForSingleEpisode retrieves waveform status and triggers generation if needed
func (e *EpisodeEnricher) getWaveformStatusForSingleEpisode(ctx context.Context, episode *internalEpisodes.PodcastIndexEpisode) *WaveformStatus {
	episodeID := episode.ID

	// Check if waveform exists
	waveform, err := e.deps.WaveformService.GetWaveform(ctx, uint(episodeID))
	if err == nil && waveform != nil {
		// Waveform exists, return it
		peaks, _ := waveform.Peaks()
		return &WaveformStatus{
			Status:  WaveformStatusCompleted,
			Message: WaveformStatusMessages[WaveformStatusCompleted],
			Data: &types.WaveformData{
				Peaks:      peaks,
				Duration:   waveform.Duration,
				Resolution: waveform.Resolution,
				SampleRate: waveform.SampleRate,
			},
		}
	}

	// Check if there's a job in progress
	if e.deps.JobService != nil {
		job, err := e.deps.JobService.GetJobForWaveform(ctx, uint(episodeID))
		if err == nil && job != nil {
			return e.mapJobToWaveformStatus(job)
		}

		// No job exists, trigger waveform generation for single episode request
		// We already have the episode data, so use it directly
		if episode.EnclosureURL != "" {
			// Try to enqueue a waveform generation job
			newJob, err := e.deps.JobService.EnqueueJob(ctx, models.JobTypeWaveformGeneration, map[string]interface{}{
				"episode_id": episodeID,
				"audio_url":  episode.EnclosureURL,
			})
			if err == nil {
				log.Printf("[DEBUG] Auto-triggered waveform generation for episode %d (job %d)", episodeID, newJob.ID)
				return &WaveformStatus{
					Status:   WaveformStatusProcessing,
					Message:  "Waveform generation started",
					Progress: 0,
				}
			} else {
				log.Printf("[ERROR] Failed to enqueue waveform job for episode %d: %v", episodeID, err)
			}
		}
	}

	// No waveform, no job, and couldn't trigger generation
	return nil
}

// getTranscriptionStatusForSingleEpisode retrieves transcription status and triggers generation if needed
func (e *EpisodeEnricher) getTranscriptionStatusForSingleEpisode(ctx context.Context, episode *internalEpisodes.PodcastIndexEpisode) *TranscriptionStatus {
	episodeID := episode.ID

	// Check if transcription exists
	transcription, err := e.deps.TranscriptionService.GetTranscription(ctx, uint(episodeID))
	if err == nil && transcription != nil {
		// Transcription exists, return it
		return &TranscriptionStatus{
			Status:  TranscriptionStatusCompleted,
			Message: TranscriptionStatusMessages[TranscriptionStatusCompleted],
			Data: &types.TranscriptionData{
				Text:     transcription.Text,
				Language: transcription.Language,
				Duration: transcription.Duration,
				Model:    transcription.Model,
			},
		}
	}

	// Check if there's a job in progress
	if e.deps.JobService != nil {
		job, err := e.deps.JobService.GetJobForTranscription(ctx, uint(episodeID))
		if err == nil && job != nil {
			return e.mapJobToTranscriptionStatus(job)
		}

		// No job exists, trigger transcription generation for single episode request
		// We already have the episode data, so use it directly
		if episode.EnclosureURL != "" {
			// Try to enqueue a transcription generation job
			newJob, err := e.deps.JobService.EnqueueJob(ctx, models.JobTypeTranscriptionGeneration, map[string]interface{}{
				"episode_id": episodeID,
				"audio_url":  episode.EnclosureURL,
			})
			if err == nil {
				log.Printf("[DEBUG] Auto-triggered transcription generation for episode %d (job %d)", episodeID, newJob.ID)
				return &TranscriptionStatus{
					Status:   TranscriptionStatusProcessing,
					Message:  "Transcription generation started",
					Progress: 0,
				}
			} else {
				log.Printf("[ERROR] Failed to enqueue transcription job for episode %d: %v", episodeID, err)
			}
		}
	}

	// No transcription, no job, and couldn't trigger generation
	return nil
}

// mapJobToWaveformStatus converts job status to waveform status
func (e *EpisodeEnricher) mapJobToWaveformStatus(job *models.Job) *WaveformStatus {
	switch job.Status {
	case models.JobStatusPending:
		return &WaveformStatus{
			Status:   WaveformStatusProcessing,
			Message:  "Waveform generation queued",
			Progress: 0,
		}
	case models.JobStatusProcessing:
		return &WaveformStatus{
			Status:   WaveformStatusProcessing,
			Message:  WaveformStatusMessages[WaveformStatusProcessing],
			Progress: job.Progress,
		}
	case models.JobStatusFailed:
		return &WaveformStatus{
			Status:  WaveformStatusFailed,
			Message: "Waveform generation failed",
		}
	case models.JobStatusCompleted:
		// Job completed but waveform not found - shouldn't happen but handle gracefully
		log.Printf("[WARNING] Job %d completed but waveform not found", job.ID)
		return &WaveformStatus{
			Status:  WaveformStatusFailed,
			Message: "Processing completed but waveform not found",
		}
	default:
		return nil
	}
}

// mapJobToTranscriptionStatus converts job status to transcription status
func (e *EpisodeEnricher) mapJobToTranscriptionStatus(job *models.Job) *TranscriptionStatus {
	switch job.Status {
	case models.JobStatusPending:
		return &TranscriptionStatus{
			Status:   TranscriptionStatusProcessing,
			Message:  "Transcription generation queued",
			Progress: 0,
		}
	case models.JobStatusProcessing:
		return &TranscriptionStatus{
			Status:   TranscriptionStatusProcessing,
			Message:  TranscriptionStatusMessages[TranscriptionStatusProcessing],
			Progress: job.Progress,
		}
	case models.JobStatusFailed:
		return &TranscriptionStatus{
			Status:  TranscriptionStatusFailed,
			Message: "Transcription generation failed",
		}
	case models.JobStatusCompleted:
		// Job completed but transcription not found - shouldn't happen but handle gracefully
		log.Printf("[WARNING] Job %d completed but transcription not found", job.ID)
		return &TranscriptionStatus{
			Status:  TranscriptionStatusFailed,
			Message: "Processing completed but transcription not found",
		}
	default:
		return nil
	}
}
