package workers

import (
	"context"
	"fmt"
	"log"
	"path/filepath"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/autolabel"
	"github.com/killallgit/player-api/internal/services/jobs"
	"gorm.io/gorm"
)

// AutoLabelProcessor processes autolabel jobs
type AutoLabelProcessor struct {
	jobService      jobs.Service
	db              *gorm.DB
	autolabelSvc    autolabel.Service
	clipStoragePath string // Base path for clip storage (e.g., "./clips")
}

// NewAutoLabelProcessor creates a new autolabel processor
func NewAutoLabelProcessor(
	jobService jobs.Service,
	db *gorm.DB,
	autolabelSvc autolabel.Service,
	clipStoragePath string,
) *AutoLabelProcessor {
	return &AutoLabelProcessor{
		jobService:      jobService,
		db:              db,
		autolabelSvc:    autolabelSvc,
		clipStoragePath: clipStoragePath,
	}
}

// CanProcess returns true if this processor can handle the job type
func (p *AutoLabelProcessor) CanProcess(jobType models.JobType) bool {
	return jobType == models.JobTypeAutoLabel
}

// ProcessJob processes an autolabel job
func (p *AutoLabelProcessor) ProcessJob(ctx context.Context, job *models.Job) error {
	if !p.CanProcess(job.Type) {
		return fmt.Errorf("unsupported job type: %s", job.Type)
	}

	log.Printf("[DEBUG] Processing autolabel job %d", job.ID)

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

	// Verify clip file exists
	clipPath := filepath.Join(p.clipStoragePath, clip.GetRelativePath())
	log.Printf("[DEBUG] Analyzing clip at: %s", clipPath)

	// Update progress: Analyzing audio
	if err := p.jobService.UpdateProgress(ctx, job.ID, 20); err != nil {
		log.Printf("[WARN] Failed to update job progress: %v", err)
	}

	// Run autolabel analysis
	result, err := p.autolabelSvc.AutoLabelClip(ctx, clipPath)
	if err != nil {
		return models.NewProcessingError(
			"autolabel_failed",
			"Failed to analyze clip audio",
			fmt.Sprintf("Error during autolabeling: %v", err),
			err,
		)
	}

	log.Printf("[DEBUG] Autolabel result for clip %s: label=%s, confidence=%.2f, method=%s",
		clipUUID, result.Label, result.Confidence, result.Method)

	// Update progress: Saving results
	if err := p.jobService.UpdateProgress(ctx, job.ID, 80); err != nil {
		log.Printf("[WARN] Failed to update job progress: %v", err)
	}

	// Update clip with autolabel metadata
	if err := p.autolabelSvc.UpdateClipWithAutoLabel(ctx, clipUUID, result); err != nil {
		return models.NewSystemError(
			"database_update_failed",
			"Failed to update clip with autolabel results",
			err.Error(),
			err,
		)
	}

	// Update progress: Complete
	if err := p.jobService.UpdateProgress(ctx, job.ID, 100); err != nil {
		log.Printf("[WARN] Failed to update job progress: %v", err)
	}

	log.Printf("[DEBUG] Successfully autolabeled clip %s", clipUUID)

	return nil
}

// parseClipUUID extracts the clip UUID from the job payload
func (p *AutoLabelProcessor) parseClipUUID(payload models.JobPayload) (string, error) {
	clipUUID, ok := payload["clip_uuid"]
	if !ok {
		return "", fmt.Errorf("missing clip_uuid in payload")
	}

	uuid, ok := clipUUID.(string)
	if !ok {
		return "", fmt.Errorf("clip_uuid is not a string: %T", clipUUID)
	}

	return uuid, nil
}
