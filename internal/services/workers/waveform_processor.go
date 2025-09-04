package workers

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/jobs"
	"github.com/killallgit/player-api/internal/services/waveforms"
	"github.com/killallgit/player-api/pkg/ffmpeg"
)

// WaveformProcessor processes waveform generation jobs
type WaveformProcessor struct {
	jobService      jobs.Service
	waveformService waveforms.WaveformService
	episodeService  episodes.Service
	ffmpeg          *ffmpeg.FFmpeg
	options         ffmpeg.ProcessingOptions
}

// NewWaveformProcessor creates a new waveform processor
func NewWaveformProcessor(
	jobService jobs.Service,
	waveformService waveforms.WaveformService,
	episodeService episodes.Service,
	ffmpegInstance *ffmpeg.FFmpeg,
	options ffmpeg.ProcessingOptions,
) *WaveformProcessor {
	return &WaveformProcessor{
		jobService:      jobService,
		waveformService: waveformService,
		episodeService:  episodeService,
		ffmpeg:          ffmpegInstance,
		options:         options,
	}
}

// CanProcess returns true if this processor can handle the job type
func (p *WaveformProcessor) CanProcess(jobType models.JobType) bool {
	return jobType == models.JobTypeWaveformGeneration
}

// ProcessJob processes a waveform generation job
func (p *WaveformProcessor) ProcessJob(ctx context.Context, job *models.Job) error {
	if !p.CanProcess(job.Type) {
		return fmt.Errorf("unsupported job type: %s", job.Type)
	}

	log.Printf("Processing waveform generation job %d", job.ID)

	// Parse job payload to get episode ID
	episodeID, err := p.parseEpisodeID(job.Payload)
	if err != nil {
		return fmt.Errorf("invalid job payload: %w", err)
	}

	// Update progress: Starting
	if err := p.jobService.UpdateProgress(ctx, job.ID, 10); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	// Get episode details
	episode, err := p.episodeService.GetEpisodeByID(ctx, episodeID)
	if err != nil {
		return fmt.Errorf("failed to get episode %d: %w", episodeID, err)
	}

	// Check if episode has audio URL
	if episode.AudioURL == "" {
		return fmt.Errorf("episode %d has no audio URL", episodeID)
	}

	// Update progress: Fetching audio
	if err := p.jobService.UpdateProgress(ctx, job.ID, 25); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	log.Printf("Generating waveform for episode %d from URL: %s", episodeID, episode.AudioURL)

	// Generate waveform
	waveformData, err := p.ffmpeg.GenerateWaveform(ctx, episode.AudioURL, p.options)
	if err != nil {
		return fmt.Errorf("failed to generate waveform: %w", err)
	}

	// Update progress: Processing complete, saving to database
	if err := p.jobService.UpdateProgress(ctx, job.ID, 85); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	// Create waveform model
	waveformModel := &models.Waveform{
		EpisodeID:  episodeID,
		Duration:   waveformData.Duration,
		Resolution: waveformData.Resolution,
		SampleRate: waveformData.SampleRate,
	}

	// Set peaks data
	if err := waveformModel.SetPeaks(waveformData.Peaks); err != nil {
		return fmt.Errorf("failed to encode peaks data: %w", err)
	}

	// Save waveform to database
	if err := p.waveformService.SaveWaveform(ctx, waveformModel); err != nil {
		return fmt.Errorf("failed to save waveform: %w", err)
	}

	// Update progress: Complete
	if err := p.jobService.UpdateProgress(ctx, job.ID, 100); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	// Create job result
	result := map[string]interface{}{
		"episode_id":  episodeID,
		"duration":    waveformData.Duration,
		"resolution":  waveformData.Resolution,
		"sample_rate": waveformData.SampleRate,
		"peaks_count": len(waveformData.Peaks),
	}

	// Complete the job
	if err := p.jobService.CompleteJob(ctx, job.ID, models.JobResult(result)); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	log.Printf("Waveform generation completed for episode %d (%.1fs, %d peaks)",
		episodeID, waveformData.Duration, len(waveformData.Peaks))

	return nil
}

// parseEpisodeID extracts the episode ID from the job payload
func (p *WaveformProcessor) parseEpisodeID(payload models.JobPayload) (uint, error) {
	// JobPayload is already a map[string]interface{}, so use it directly
	data := map[string]interface{}(payload)

	// Extract episode_id
	episodeIDValue, exists := data["episode_id"]
	if !exists {
		return 0, fmt.Errorf("episode_id not found in payload")
	}

	// Handle different number types
	switch v := episodeIDValue.(type) {
	case float64:
		return uint(v), nil
	case int:
		return uint(v), nil
	case int64:
		return uint(v), nil
	case uint:
		return v, nil
	case string:
		// Try to parse as string number
		if id, err := strconv.ParseUint(v, 10, 32); err == nil {
			return uint(id), nil
		}
		return 0, fmt.Errorf("invalid episode_id string: %s", v)
	default:
		return 0, fmt.Errorf("invalid episode_id type: %T", v)
	}
}
