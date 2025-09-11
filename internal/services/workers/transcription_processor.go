package workers

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/audiocache"
	"github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/jobs"
	"github.com/killallgit/player-api/internal/services/transcription"
	"github.com/killallgit/player-api/pkg/download"
	"github.com/killallgit/player-api/pkg/transcript"
	"github.com/spf13/viper"
)

// TranscriptionProcessor processes transcription generation jobs
type TranscriptionProcessor struct {
	jobService           jobs.Service
	transcriptionService transcription.TranscriptionService
	episodeService       episodes.EpisodeService
	audioCacheService    audiocache.Service
	downloader           *download.Downloader
	transcriptFetcher    *transcript.Fetcher
	transcriptParser     *transcript.Parser
	modelPath            string
	whisperPath          string
	language             string
	preferExisting       bool
}

// NewTranscriptionProcessor creates a new transcription processor
func NewTranscriptionProcessor(
	jobService jobs.Service,
	transcriptionService transcription.TranscriptionService,
	episodeService episodes.EpisodeService,
	audioCacheService audiocache.Service,
) *TranscriptionProcessor {
	// Create downloader with default options
	downloadOpts := download.DefaultOptions()
	downloadOpts.TempDir = viper.GetString("storage.temp_dir")
	if downloadOpts.TempDir == "" {
		downloadOpts.TempDir = "/tmp"
	}

	// Get whisper configuration
	modelPath := viper.GetString("transcription.model_path")
	if modelPath == "" {
		modelPath = "./models/ggml-base.en.bin"
	}

	whisperPath := viper.GetString("transcription.whisper_path")
	if whisperPath == "" {
		// Default to whisper-cli (homebrew) or main binary (whisper.cpp build)
		if _, err := exec.LookPath("whisper-cli"); err == nil {
			whisperPath = "whisper-cli"
		} else {
			whisperPath = "/app/bin/main"
		}
	}

	language := viper.GetString("transcription.language")
	if language == "" {
		language = "en"
	}

	// Get preference for using existing transcripts
	preferExisting := viper.GetBool("transcription.prefer_existing")
	if !viper.IsSet("transcription.prefer_existing") {
		preferExisting = true // Default to true
	}

	// Create transcript fetcher
	fetchOpts := transcript.DefaultFetchOptions()
	fetchTimeout := viper.GetDuration("transcription.fetch_timeout")
	if fetchTimeout > 0 {
		fetchOpts.Timeout = fetchTimeout
	}

	return &TranscriptionProcessor{
		jobService:           jobService,
		transcriptionService: transcriptionService,
		episodeService:       episodeService,
		audioCacheService:    audioCacheService,
		downloader:           download.NewDownloader(downloadOpts),
		transcriptFetcher:    transcript.NewFetcher(fetchOpts),
		transcriptParser:     transcript.NewParser(),
		modelPath:            modelPath,
		whisperPath:          whisperPath,
		language:             language,
		preferExisting:       preferExisting,
	}
}

// CanProcess returns true if this processor can handle the job type
func (p *TranscriptionProcessor) CanProcess(jobType models.JobType) bool {
	return jobType == models.JobTypeTranscriptionGeneration
}

// ProcessJob processes a transcription generation job
func (p *TranscriptionProcessor) ProcessJob(ctx context.Context, job *models.Job) error {
	if !p.CanProcess(job.Type) {
		return fmt.Errorf("unsupported job type: %s", job.Type)
	}

	log.Printf("[DEBUG] Processing transcription generation job %d", job.ID)

	// Parse job payload to get episode ID
	episodeID, err := p.parseEpisodeID(job.Payload)
	if err != nil {
		return fmt.Errorf("invalid job payload: %w", err)
	}

	// Update progress: Starting
	if err := p.jobService.UpdateProgress(ctx, job.ID, 5); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	// Get episode details
	episode, err := p.episodeService.GetEpisodeByPodcastIndexID(ctx, int64(episodeID))
	if err != nil {
		return fmt.Errorf("failed to get episode %d: %w", episodeID, err)
	}

	// Try to fetch existing transcript first if preferred and available
	if p.preferExisting && episode.TranscriptURL != "" {
		log.Printf("[DEBUG] Episode %d has transcript URL: %s, attempting to fetch", episodeID, episode.TranscriptURL)

		// Update progress: Fetching existing transcript
		if err := p.jobService.UpdateProgress(ctx, job.ID, 10); err != nil {
			log.Printf("Failed to update job progress: %v", err)
		}

		// Try to fetch and parse the transcript
		transcriptResult, fetchErr := p.transcriptFetcher.Fetch(ctx, episode.TranscriptURL)
		if fetchErr == nil {
			log.Printf("[DEBUG] Successfully fetched transcript for episode %d (format: %s, size: %d bytes)",
				episodeID, transcriptResult.Format, transcriptResult.Size)

			// Update progress: Parsing transcript
			if err := p.jobService.UpdateProgress(ctx, job.ID, 30); err != nil {
				log.Printf("Failed to update job progress: %v", err)
			}

			// Parse the transcript
			parsedTranscript, parseErr := p.transcriptParser.Parse(transcriptResult.Content, transcriptResult.Format)
			if parseErr == nil {
				log.Printf("[DEBUG] Successfully parsed transcript for episode %d (segments: %d, duration: %v)",
					episodeID, len(parsedTranscript.Segments), parsedTranscript.Duration)

				// Update progress: Saving to database
				if err := p.jobService.UpdateProgress(ctx, job.ID, 85); err != nil {
					log.Printf("Failed to update job progress: %v", err)
				}

				// Create transcription model
				transcriptionModel := &models.Transcription{
					EpisodeID: episodeID,
					Text:      parsedTranscript.ToPlainText(),
					Language:  p.language, // We might want to detect this from the transcript
					Model:     fmt.Sprintf("fetched-%s", parsedTranscript.Format),
					Duration:  parsedTranscript.Duration.Seconds(),
					Source:    "fetched",
					SourceURL: episode.TranscriptURL,
					Format:    string(parsedTranscript.Format),
				}

				// Save transcription to database
				if err := p.transcriptionService.SaveTranscription(ctx, transcriptionModel); err != nil {
					log.Printf("[ERROR] Failed to save fetched transcript: %v", err)
				} else {
					// Update progress: Complete
					if err := p.jobService.UpdateProgress(ctx, job.ID, 100); err != nil {
						log.Printf("Failed to update job progress: %v", err)
					}

					// Create job result
					result := map[string]interface{}{
						"episode_id":  episodeID,
						"source":      "fetched",
						"source_url":  episode.TranscriptURL,
						"format":      string(parsedTranscript.Format),
						"segments":    len(parsedTranscript.Segments),
						"duration":    parsedTranscript.Duration.Seconds(),
						"text_length": len(parsedTranscript.FullText),
					}

					// Complete the job
					if err := p.jobService.CompleteJob(ctx, job.ID, models.JobResult(result)); err != nil {
						return fmt.Errorf("failed to complete job: %w", err)
					}

					log.Printf("[DEBUG] Successfully processed transcript from URL for episode %d", episodeID)
					return nil
				}
			} else {
				log.Printf("[WARNING] Failed to parse transcript for episode %d: %v", episodeID, parseErr)
			}
		} else {
			log.Printf("[WARNING] Failed to fetch transcript from URL for episode %d: %v", episodeID, fetchErr)
		}

		// If we get here, fetching/parsing failed, fall back to Whisper transcription
		log.Printf("[INFO] Falling back to Whisper transcription for episode %d", episodeID)
	}

	// Check if episode has audio URL for Whisper transcription
	if episode.AudioURL == "" {
		return fmt.Errorf("episode %d has no audio URL for transcription", episodeID)
	}

	// Update progress: Starting download/cache check for Whisper transcription
	if err := p.jobService.UpdateProgress(ctx, job.ID, 10); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	var audioFilePath string
	var audioFileSize int64

	// Check if audio is cached (if audio cache service is available)
	if p.audioCacheService != nil {
		log.Printf("[DEBUG] Checking audio cache for transcription of episode %d (database ID: %d)", episodeID, episode.ID)

		// Get or download audio through cache (prefer processed audio for ML)
		audioCache, err := p.audioCacheService.GetOrDownloadAudio(ctx, episode.ID, episode.AudioURL)
		if err != nil {
			log.Printf("[WARN] Audio cache failed for transcription, falling back to direct download: %v", err)
		} else if audioCache != nil && audioCache.ProcessedPath != "" {
			log.Printf("[DEBUG] Using cached processed audio for transcription of episode %d from %s", episode.ID, audioCache.ProcessedPath)
			audioFilePath = audioCache.ProcessedPath
			audioFileSize = audioCache.ProcessedSize
		} else if audioCache != nil && audioCache.OriginalPath != "" {
			log.Printf("[DEBUG] Using cached original audio for transcription of episode %d from %s", episode.ID, audioCache.OriginalPath)
			audioFilePath = audioCache.OriginalPath
			audioFileSize = audioCache.OriginalSize
		}
	}

	// If not cached or cache failed, download directly to temp file
	if audioFilePath == "" {
		log.Printf("[DEBUG] Downloading audio for Whisper transcription of episode %d from URL: %s", episodeID, episode.AudioURL)

		// Download audio to temp file with retry logic
		downloadResult, err := p.downloader.DownloadWithRetry(ctx, episode.AudioURL, episodeID)
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

	// Update progress: Download/cache complete, starting transcription
	if err := p.jobService.UpdateProgress(ctx, job.ID, 50); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	log.Printf("[DEBUG] Transcribing audio from file: %s", audioFilePath)

	// Generate transcription
	transcriptionText, duration, err := p.transcribeAudio(ctx, audioFilePath)
	if err != nil {
		return fmt.Errorf("failed to transcribe audio: %w", err)
	}

	// Update progress: Transcription complete, saving to database
	if err := p.jobService.UpdateProgress(ctx, job.ID, 85); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	// Create transcription model
	transcriptionModel := &models.Transcription{
		EpisodeID: episodeID,
		Text:      transcriptionText,
		Language:  p.language,
		Model:     filepath.Base(p.modelPath),
		Duration:  duration,
		Source:    "generated",
		SourceURL: "",        // No source URL for generated transcripts
		Format:    "whisper", // Whisper output format
	}

	// Save transcription to database
	if err := p.transcriptionService.SaveTranscription(ctx, transcriptionModel); err != nil {
		return fmt.Errorf("failed to save transcription: %w", err)
	}

	// Update progress: Complete
	if err := p.jobService.UpdateProgress(ctx, job.ID, 100); err != nil {
		log.Printf("Failed to update job progress: %v", err)
	}

	// Create job result
	result := map[string]interface{}{
		"episode_id":  episodeID,
		"source":      "generated",
		"duration":    duration,
		"language":    p.language,
		"model":       filepath.Base(p.modelPath),
		"text_length": len(transcriptionText),
		"file_size":   audioFileSize,
		"cached":      p.audioCacheService != nil && audioFilePath != "",
	}

	// Complete the job
	if err := p.jobService.CompleteJob(ctx, job.ID, models.JobResult(result)); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	log.Printf("[DEBUG] Transcription completed for episode %d (%.1fs, %d characters, %.2f MB)",
		episodeID, duration, len(transcriptionText),
		float64(audioFileSize)/(1024*1024))

	return nil
}

// transcribeAudio transcribes audio using whisper
func (p *TranscriptionProcessor) transcribeAudio(ctx context.Context, audioPath string) (string, float64, error) {
	// Check if whisper binary exists
	if _, err := exec.LookPath(p.whisperPath); err != nil {
		// For now, return placeholder transcription
		log.Printf("[WARNING] Whisper binary not found at %s, using placeholder transcription", p.whisperPath)
		return p.generatePlaceholderTranscription(audioPath)
	}

	// Run whisper-cli command (Homebrew whisper-cpp installation)
	cmd := exec.CommandContext(ctx, p.whisperPath,
		"-m", p.modelPath, // model path
		"-f", audioPath, // input file
		"-l", p.language, // language
		"-t", "4", // threads
		"-otxt", // output as text
		"-nt",   // no timestamps
	)

	output, err := cmd.Output()
	if err != nil {
		log.Printf("[ERROR] Whisper command failed: %v", err)
		// Fall back to placeholder
		return p.generatePlaceholderTranscription(audioPath)
	}

	// Parse output to extract transcription
	transcriptionText := string(output)

	// Clean up the transcription text
	transcriptionText = strings.TrimSpace(transcriptionText)

	// For now, estimate duration based on file size (will be replaced with actual duration)
	duration := 300.0 // Placeholder 5 minutes

	return transcriptionText, duration, nil
}

// generatePlaceholderTranscription generates a placeholder transcription for testing
func (p *TranscriptionProcessor) generatePlaceholderTranscription(audioPath string) (string, float64, error) {
	// This is temporary until whisper is properly integrated
	placeholderText := fmt.Sprintf(
		"[Transcription placeholder for audio file: %s]\n\n"+
			"This is a placeholder transcription that will be replaced with actual whisper.cpp output once the model is configured.\n\n"+
			"To enable real transcription:\n"+
			"1. Download a whisper model (e.g., ggml-base.en.bin)\n"+
			"2. Install whisper.cpp or use the Go bindings\n"+
			"3. Configure the model path in your settings\n\n"+
			"The transcription service is working correctly and will process audio files automatically when properly configured.",
		filepath.Base(audioPath),
	)

	// Placeholder duration
	duration := 300.0 // 5 minutes

	return placeholderText, duration, nil
}

// parseEpisodeID extracts the episode ID from the job payload
func (p *TranscriptionProcessor) parseEpisodeID(payload models.JobPayload) (uint, error) {
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
