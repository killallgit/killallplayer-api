package workers

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/audiocache"
	"github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/jobs"
	"github.com/killallgit/player-api/internal/services/waveforms"
	"github.com/killallgit/player-api/pkg/download"
	"github.com/killallgit/player-api/pkg/ffmpeg"
)

// EnhancedWaveformProcessor processes waveform generation jobs with temp file download
type EnhancedWaveformProcessor struct {
	jobService        jobs.Service
	waveformService   waveforms.WaveformService
	episodeService    episodes.EpisodeService
	audioCacheService audiocache.Service
	ffmpeg            *ffmpeg.FFmpeg
	downloader        *download.Downloader
	options           ffmpeg.ProcessingOptions
}

// NewEnhancedWaveformProcessor creates a new enhanced waveform processor
func NewEnhancedWaveformProcessor(
	jobService jobs.Service,
	waveformService waveforms.WaveformService,
	episodeService episodes.EpisodeService,
	audioCacheService audiocache.Service,
	ffmpegInstance *ffmpeg.FFmpeg,
	options ffmpeg.ProcessingOptions,
) *EnhancedWaveformProcessor {
	// Create downloader with default options
	downloadOpts := download.DefaultOptions()
	downloadOpts.TempDir = options.TempDir

	// Add progress callback that updates job progress
	var currentJobID uint
	downloadOpts.ProgressFunc = func(downloaded, total int64) {
		if currentJobID > 0 && total > 0 {
			// Map download progress to 10-50% of job progress
			progress := int(10 + (40 * downloaded / total))
			if err := jobService.UpdateProgress(context.Background(), currentJobID, progress); err != nil {
				log.Printf("Failed to update download progress: %v", err)
			}
		}
	}

	return &EnhancedWaveformProcessor{
		jobService:        jobService,
		waveformService:   waveformService,
		episodeService:    episodeService,
		audioCacheService: audioCacheService,
		ffmpeg:            ffmpegInstance,
		downloader:        download.NewDownloader(downloadOpts),
		options:           options,
	}
}

// CanProcess returns true if this processor can handle the job type
func (p *EnhancedWaveformProcessor) CanProcess(jobType models.JobType) bool {
	return jobType == models.JobTypeWaveformGeneration
}

// ProcessJob processes a waveform generation job with temp file download
func (p *EnhancedWaveformProcessor) ProcessJob(ctx context.Context, job *models.Job) error {
	if !p.CanProcess(job.Type) {
		return fmt.Errorf("unsupported job type: %s", job.Type)
	}

	log.Printf("[DEBUG] Processing waveform generation job %d", job.ID)

	// Parse job payload to get episode ID (this is the Podcast Index ID)
	podcastIndexID, err := p.parseEpisodeID(job.Payload)
	if err != nil {
		return fmt.Errorf("invalid job payload: %w", err)
	}

	// Update progress: Starting
	if err := p.jobService.UpdateProgress(ctx, job.ID, 5); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	// Get episode details using Podcast Index ID
	episode, err := p.episodeService.GetEpisodeByPodcastIndexID(ctx, int64(podcastIndexID))
	if err != nil {
		return fmt.Errorf("failed to get episode %d: %w", podcastIndexID, err)
	}

	// Check if waveform already exists for this episode
	existingWaveform, err := p.waveformService.GetWaveform(ctx, uint(podcastIndexID))
	if err == nil && existingWaveform != nil {
		log.Printf("[DEBUG] Waveform already exists for Podcast Index Episode %d, skipping generation", podcastIndexID)

		// Complete the job immediately since waveform exists
		result := map[string]interface{}{
			"episode_id": podcastIndexID,
			"status":     "already_exists",
			"message":    "Waveform already exists for this episode",
		}

		// Update progress to 100%
		if err := p.jobService.UpdateProgress(ctx, job.ID, 100); err != nil {
			log.Printf("Failed to update job progress: %v", err)
		}

		// Complete the job
		if err := p.jobService.CompleteJob(ctx, job.ID, models.JobResult(result)); err != nil {
			return fmt.Errorf("failed to complete job: %w", err)
		}

		return nil
	}

	// Check if episode has audio URL
	if episode.AudioURL == "" {
		return fmt.Errorf("episode %d has no audio URL", podcastIndexID)
	}

	// Update progress: Starting download/cache check
	if err := p.jobService.UpdateProgress(ctx, job.ID, 10); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	var audioFilePath string
	var audioFileSize int64

	// Check if audio is cached (if audio cache service is available)
	if p.audioCacheService != nil {
		log.Printf("[DEBUG] Checking audio cache for episode %d (database ID: %d)", podcastIndexID, episode.ID)

		// Get or download audio through cache
		audioCache, err := p.audioCacheService.GetOrDownloadAudio(ctx, episode.ID, episode.AudioURL)
		if err != nil {
			log.Printf("[WARN] Audio cache failed, falling back to direct download: %v", err)
		} else if audioCache != nil && audioCache.OriginalPath != "" {
			log.Printf("[DEBUG] Using cached audio for episode %d from %s", episode.ID, audioCache.OriginalPath)
			audioFilePath = audioCache.OriginalPath
			audioFileSize = audioCache.OriginalSize
		}
	}

	// If not cached or cache failed, download directly to temp file
	if audioFilePath == "" {
		log.Printf("[DEBUG] Downloading audio for episode %d (database ID: %d) from URL: %s", podcastIndexID, episode.ID, episode.AudioURL)

		// Download audio to temp file (use Podcast Index ID for logging)
		downloadResult, err := p.downloader.DownloadToTemp(ctx, episode.AudioURL, podcastIndexID)
		if err != nil {
			return fmt.Errorf("failed to download audio: %w", err)
		}

		// Ensure temp file cleanup
		defer func() {
			if err := download.CleanupTempFile(downloadResult.FilePath); err != nil {
				log.Printf("[ERROR] Failed to cleanup temp file %s: %v", downloadResult.FilePath, err)
			}
		}()

		audioFilePath = downloadResult.FilePath
		audioFileSize = downloadResult.ContentLength

		log.Printf("[DEBUG] Downloaded audio to %s (%.2f MB)", downloadResult.FilePath,
			float64(downloadResult.ContentLength)/(1024*1024))
	}

	// Update progress: Download/cache complete, starting processing
	if err := p.jobService.UpdateProgress(ctx, job.ID, 50); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	log.Printf("[DEBUG] Processing waveform from file: %s", audioFilePath)

	// Generate waveform from audio file
	waveformData, err := p.ffmpeg.GenerateWaveform(ctx, audioFilePath, p.options)
	if err != nil {
		return fmt.Errorf("failed to generate waveform: %w", err)
	}

	// Update progress: Processing complete, saving to database
	if err := p.jobService.UpdateProgress(ctx, job.ID, 85); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	// Create waveform model - Use Podcast Index Episode ID for API consistency
	waveformModel := &models.Waveform{
		EpisodeID:  uint(podcastIndexID), // Use Podcast Index Episode ID, not database ID
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

	// Create job result with additional info
	result := map[string]interface{}{
		"episode_id":  podcastIndexID,
		"duration":    waveformData.Duration,
		"resolution":  waveformData.Resolution,
		"sample_rate": waveformData.SampleRate,
		"peaks_count": len(waveformData.Peaks),
		"file_size":   audioFileSize,
		"cached":      p.audioCacheService != nil && audioFilePath != "",
	}

	// Complete the job
	if err := p.jobService.CompleteJob(ctx, job.ID, models.JobResult(result)); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	log.Printf("[DEBUG] Waveform generation completed for Podcast Index Episode %d (%.1fs, %d peaks, %.2f MB)",
		podcastIndexID, waveformData.Duration, len(waveformData.Peaks),
		float64(audioFileSize)/(1024*1024))

	return nil
}

// parseEpisodeID extracts the episode ID from the job payload
func (p *EnhancedWaveformProcessor) parseEpisodeID(payload models.JobPayload) (uint, error) {
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
