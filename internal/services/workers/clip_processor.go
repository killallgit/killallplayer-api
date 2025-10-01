package workers

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/clips"
	"github.com/killallgit/player-api/internal/services/jobs"
	"gorm.io/gorm"
)

// ClipExtractionProcessor processes clip extraction jobs
type ClipExtractionProcessor struct {
	jobService jobs.Service
	db         *gorm.DB
	extractor  clips.AudioExtractor
	storage    clips.ClipStorage
}

// NewClipExtractionProcessor creates a new clip extraction processor
func NewClipExtractionProcessor(
	jobService jobs.Service,
	db *gorm.DB,
	extractor clips.AudioExtractor,
	storage clips.ClipStorage,
) *ClipExtractionProcessor {
	return &ClipExtractionProcessor{
		jobService: jobService,
		db:         db,
		extractor:  extractor,
		storage:    storage,
	}
}

// CanProcess returns true if this processor can handle the job type
func (p *ClipExtractionProcessor) CanProcess(jobType models.JobType) bool {
	return jobType == models.JobTypeClipExtraction
}

// ProcessJob processes a clip extraction job
func (p *ClipExtractionProcessor) ProcessJob(ctx context.Context, job *models.Job) error {
	if !p.CanProcess(job.Type) {
		return fmt.Errorf("unsupported job type: %s", job.Type)
	}

	log.Printf("[DEBUG] Processing clip extraction job %d", job.ID)

	// Parse job payload to get clip UUID
	clipUUID, err := p.parseClipUUID(job.Payload)
	if err != nil {
		return models.NewSystemError(
			"invalid_payload",
			"Invalid job payload",
			fmt.Sprintf("Failed to parse clip UUID: %v", err),
			err,
		)
	}

	// Update progress: Starting
	if err := p.jobService.UpdateProgress(ctx, job.ID, 5); err != nil {
		log.Printf("[WARN] Failed to update job progress: %v", err)
	}

	// Get clip from database
	var clip models.Clip
	if err := p.db.Where("uuid = ?", clipUUID).First(&clip).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return models.NewNotFoundError(
				"clip_not_found",
				fmt.Sprintf("Clip %s not found", clipUUID),
				"The clip record was not found in the database",
				err,
			)
		}
		return models.NewSystemError(
			"database_error",
			"Failed to fetch clip from database",
			err.Error(),
			err,
		)
	}

	// Update clip status to processing
	if err := p.db.Model(&clip).Updates(map[string]interface{}{
		"status":     "processing",
		"updated_at": time.Now(),
	}).Error; err != nil {
		log.Printf("[WARN] Failed to update clip status to processing: %v", err)
	}

	// Update progress: Downloading/extracting
	if err := p.jobService.UpdateProgress(ctx, job.ID, 10); err != nil {
		log.Printf("[WARN] Failed to update job progress: %v", err)
	}

	// Set timeout for extraction
	extractCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Use temp directory for extraction
	tempPath := fmt.Sprintf("/tmp/%s", clip.ClipFilename)

	// Extract the clip to temp location
	log.Printf("[DEBUG] Extracting clip %s from %s (%.2fs - %.2fs)",
		clipUUID, clip.SourceEpisodeURL, clip.OriginalStartTime, clip.OriginalEndTime)

	result, err := p.extractor.ExtractClip(extractCtx, clips.ExtractParams{
		SourceURL:  clip.SourceEpisodeURL,
		StartTime:  clip.OriginalStartTime,
		EndTime:    clip.OriginalEndTime,
		OutputPath: tempPath,
	})

	if err != nil {
		// Classify the error
		errMsg := err.Error()
		log.Printf("[ERROR] Clip extraction failed for %s: %v", clipUUID, err)

		// Update clip status to failed
		p.db.Model(&clip).Updates(map[string]interface{}{
			"status":        "failed",
			"error_message": errMsg,
			"updated_at":    time.Now(),
		})

		// Return classified error for proper retry handling
		return p.classifyExtractionError(err, clipUUID)
	}

	// Ensure temp file cleanup
	defer func() {
		if err := os.Remove(result.FilePath); err != nil {
			log.Printf("[WARN] Failed to cleanup temp file %s: %v", result.FilePath, err)
		}
	}()

	// Update progress: Saving to storage
	if err := p.jobService.UpdateProgress(ctx, job.ID, 50); err != nil {
		log.Printf("[WARN] Failed to update job progress: %v", err)
	}

	// Open the extracted file for storage
	file, err := os.Open(result.FilePath)
	if err != nil {
		errMsg := fmt.Sprintf("failed to open extracted file: %v", err)
		p.db.Model(&clip).Updates(map[string]interface{}{
			"status":        "failed",
			"error_message": errMsg,
			"updated_at":    time.Now(),
		})
		return models.NewSystemError(
			"file_open_error",
			"Failed to open extracted audio file",
			err.Error(),
			err,
		)
	}
	defer file.Close()

	// Save to storage (uses label-based directory structure)
	log.Printf("[DEBUG] Saving clip %s to storage (label: %s)", clipUUID, clip.Label)
	if err := p.storage.SaveClip(ctx, clip.Label, clip.ClipFilename, file); err != nil {
		errMsg := fmt.Sprintf("failed to save clip: %v", err)
		p.db.Model(&clip).Updates(map[string]interface{}{
			"status":        "failed",
			"error_message": errMsg,
			"updated_at":    time.Now(),
		})
		return models.NewSystemError(
			"storage_error",
			"Failed to save clip to storage",
			err.Error(),
			err,
		)
	}

	// Update progress: Updating database
	if err := p.jobService.UpdateProgress(ctx, job.ID, 85); err != nil {
		log.Printf("[WARN] Failed to update job progress: %v", err)
	}

	// Update clip record with success
	if err := p.db.Model(&clip).Updates(map[string]interface{}{
		"status":          "ready",
		"clip_duration":   result.Duration,
		"clip_size_bytes": result.SizeBytes,
		"error_message":   nil,
		"updated_at":      time.Now(),
	}).Error; err != nil {
		log.Printf("[ERROR] Failed to update clip record: %v", err)
		return models.NewSystemError(
			"database_update_error",
			"Failed to update clip record",
			err.Error(),
			err,
		)
	}

	// Update progress: Complete
	if err := p.jobService.UpdateProgress(ctx, job.ID, 100); err != nil {
		log.Printf("[WARN] Failed to update job progress: %v", err)
	}

	// Create job result
	jobResult := map[string]interface{}{
		"clip_uuid":      clipUUID,
		"label":          clip.Label,
		"duration":       result.Duration,
		"size_bytes":     result.SizeBytes,
		"sample_rate":    result.SampleRate,
		"channels":       result.Channels,
		"source_url":     clip.SourceEpisodeURL,
		"storage_path":   p.storage.GetClipPath(clip.Label, clip.ClipFilename),
		"original_range": fmt.Sprintf("%.2f-%.2f", clip.OriginalStartTime, clip.OriginalEndTime),
	}

	// Complete the job
	if err := p.jobService.CompleteJob(ctx, job.ID, models.JobResult(jobResult)); err != nil {
		return fmt.Errorf("failed to complete job: %w", err)
	}

	log.Printf("[INFO] Clip extraction completed for %s (%.2fs, %d bytes, stored in %s/%s)",
		clipUUID, result.Duration, result.SizeBytes, clip.Label, clip.ClipFilename)

	// TODO: Optionally enqueue autolabel job here
	// if viper.GetBool("autolabel.enabled") && viper.GetBool("autolabel.on_create") {
	//     p.jobService.EnqueueJob(ctx, models.JobTypeAutoLabel, models.JobPayload{"clip_uuid": clipUUID})
	// }

	return nil
}

// parseClipUUID extracts the clip UUID from the job payload
func (p *ClipExtractionProcessor) parseClipUUID(payload models.JobPayload) (string, error) {
	clipUUIDValue, exists := payload["clip_uuid"]
	if !exists {
		return "", fmt.Errorf("clip_uuid not found in payload")
	}

	clipUUID, ok := clipUUIDValue.(string)
	if !ok {
		return "", fmt.Errorf("clip_uuid is not a string: %T", clipUUIDValue)
	}

	if clipUUID == "" {
		return "", fmt.Errorf("clip_uuid is empty")
	}

	return clipUUID, nil
}

// classifyExtractionError classifies extraction errors for proper retry handling
func (p *ClipExtractionProcessor) classifyExtractionError(err error, clipUUID string) error {
	errMsg := err.Error()

	// Download errors (network, 403, 404, etc.)
	if containsAny(errMsg, []string{"download", "http", "403", "404", "timeout", "connection"}) {
		return models.NewDownloadError(
			"download_failed",
			fmt.Sprintf("Failed to download source audio for clip %s", clipUUID),
			errMsg,
			err,
		)
	}

	// FFmpeg processing errors
	if containsAny(errMsg, []string{"ffmpeg", "extract", "convert", "codec", "invalid data"}) {
		return models.NewProcessingError(
			"extraction_failed",
			fmt.Sprintf("Failed to extract clip %s", clipUUID),
			errMsg,
			err,
		)
	}

	// Default to system error
	return models.NewSystemError(
		"unknown_error",
		fmt.Sprintf("Clip extraction failed for %s", clipUUID),
		errMsg,
		err,
	)
}

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrs []string) bool {
	for _, substr := range substrs {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}
