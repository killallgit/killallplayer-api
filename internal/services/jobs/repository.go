package jobs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Repository errors
var (
	ErrJobNotFound       = errors.New("job not found")
	ErrNoJobsAvailable   = errors.New("no jobs available")
	ErrJobAlreadyClaimed = errors.New("job already claimed")
)

// Repository defines the interface for job persistence
type Repository interface {
	// Create operations
	CreateJob(ctx context.Context, job *models.Job) error

	// Read operations
	GetJob(ctx context.Context, id uint) (*models.Job, error)
	GetJobByTypeAndPayload(ctx context.Context, jobType models.JobType, key, value string) (*models.Job, error)
	GetPendingJobs(ctx context.Context, limit int) ([]*models.Job, error)
	GetJobsByStatus(ctx context.Context, status models.JobStatus, limit int) ([]*models.Job, error)

	// Update operations
	ClaimNextJob(ctx context.Context, workerID string, jobTypes []models.JobType) (*models.Job, error)
	UpdateJobProgress(ctx context.Context, jobID uint, progress int) error
	UpdateJobStatus(ctx context.Context, jobID uint, status models.JobStatus) error
	CompleteJob(ctx context.Context, jobID uint, result models.JobResult) error
	FailJob(ctx context.Context, jobID uint, errorMsg string) error
	FailJobWithDetails(ctx context.Context, jobID uint, errorType models.JobErrorType, errorCode, errorMsg, errorDetails string) error
	ReleaseJob(ctx context.Context, jobID uint) error

	// Delete operations
	DeleteOldJobs(ctx context.Context, olderThan time.Time) (int64, error)
}

// repository implements Repository interface
type repository struct {
	db *gorm.DB
}

// NewRepository creates a new job repository
func NewRepository(db *gorm.DB) Repository {
	return &repository{
		db: db,
	}
}

// CreateJob creates a new job
func (r *repository) CreateJob(ctx context.Context, job *models.Job) error {
	return r.db.WithContext(ctx).Create(job).Error
}

// GetJob retrieves a job by ID
func (r *repository) GetJob(ctx context.Context, id uint) (*models.Job, error) {
	var job models.Job
	err := r.db.WithContext(ctx).First(&job, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("getting job: %w", err)
	}
	return &job, nil
}

// GetJobByTypeAndPayload finds a job by type and a specific payload value
func (r *repository) GetJobByTypeAndPayload(ctx context.Context, jobType models.JobType, key, value string) (*models.Job, error) {
	var job models.Job

	// Use JSON extract for SQLite
	query := r.db.WithContext(ctx).
		Where("type = ?", jobType).
		Where("json_extract(payload, ?) = ?", "$."+key, value).
		First(&job)

	if err := query.Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("getting job by type and payload: %w", err)
	}

	return &job, nil
}

// GetPendingJobs retrieves pending jobs up to the limit
func (r *repository) GetPendingJobs(ctx context.Context, limit int) ([]*models.Job, error) {
	var jobs []*models.Job
	err := r.db.WithContext(ctx).
		Where("status = ?", models.JobStatusPending).
		Order("priority DESC, created_at ASC").
		Limit(limit).
		Find(&jobs).Error
	return jobs, err
}

// GetJobsByStatus retrieves jobs by status
func (r *repository) GetJobsByStatus(ctx context.Context, status models.JobStatus, limit int) ([]*models.Job, error) {
	var jobs []*models.Job
	query := r.db.WithContext(ctx).
		Where("status = ?", status).
		Order("created_at DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&jobs).Error
	return jobs, err
}

// ClaimNextJob atomically claims the next available job for a worker
func (r *repository) ClaimNextJob(ctx context.Context, workerID string, jobTypes []models.JobType) (*models.Job, error) {
	var job models.Job

	// Start a transaction for atomic claim
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Find and lock the next available job
		// Exclude permanently failed jobs from being claimed
		query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("status IN ?", []models.JobStatus{models.JobStatusPending, models.JobStatusFailed}).
			Where("status != ?", models.JobStatusPermanentlyFailed).
			Where("(status = ? OR (status = ? AND retry_count < max_retries))",
				models.JobStatusPending, models.JobStatusFailed)

		// Filter by job types if specified
		if len(jobTypes) > 0 {
			query = query.Where("type IN ?", jobTypes)
		}

		// Order by priority and creation time
		err := query.Order("priority DESC, created_at ASC").
			First(&job).Error

		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNoJobsAvailable
			}
			return fmt.Errorf("finding job to claim: %w", err)
		}

		// Update job status and worker
		now := time.Now()
		updates := map[string]interface{}{
			"status":     models.JobStatusProcessing,
			"worker_id":  workerID,
			"started_at": &now,
		}

		// Increment retry count if this is a retry
		if job.Status == models.JobStatusFailed {
			updates["retry_count"] = job.RetryCount + 1
		}

		if err := tx.Model(&job).Updates(updates).Error; err != nil {
			return fmt.Errorf("updating claimed job: %w", err)
		}

		// Update the job object with the new values
		job.Status = models.JobStatusProcessing
		job.WorkerID = workerID
		job.StartedAt = &now
		if job.Status == models.JobStatusFailed {
			job.RetryCount++
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &job, nil
}

// UpdateJobProgress updates the progress of a job
func (r *repository) UpdateJobProgress(ctx context.Context, jobID uint, progress int) error {
	// Ensure progress is within bounds
	if progress < 0 {
		progress = 0
	} else if progress > 100 {
		progress = 100
	}

	result := r.db.WithContext(ctx).
		Model(&models.Job{}).
		Where("id = ? AND status = ?", jobID, models.JobStatusProcessing).
		Update("progress", progress)

	if result.Error != nil {
		return fmt.Errorf("updating job progress: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrJobNotFound
	}

	return nil
}

// UpdateJobStatus updates the status of a job
func (r *repository) UpdateJobStatus(ctx context.Context, jobID uint, status models.JobStatus) error {
	result := r.db.WithContext(ctx).
		Model(&models.Job{}).
		Where("id = ?", jobID).
		Update("status", status)

	if result.Error != nil {
		return fmt.Errorf("updating job status: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrJobNotFound
	}

	return nil
}

// CompleteJob marks a job as completed with a result
func (r *repository) CompleteJob(ctx context.Context, jobID uint, result models.JobResult) error {
	now := time.Now()
	updates := map[string]interface{}{
		"status":       models.JobStatusCompleted,
		"progress":     100,
		"completed_at": &now,
		"result":       result,
	}

	res := r.db.WithContext(ctx).
		Model(&models.Job{}).
		Where("id = ?", jobID).
		Updates(updates)

	if res.Error != nil {
		return fmt.Errorf("completing job: %w", res.Error)
	}

	if res.RowsAffected == 0 {
		return ErrJobNotFound
	}

	return nil
}

// FailJob marks a job as failed with an error message
func (r *repository) FailJob(ctx context.Context, jobID uint, errorMsg string) error {
	return r.FailJobWithDetails(ctx, jobID, models.ErrorTypeSystem, "", errorMsg, "")
}

// FailJobWithDetails marks a job as failed with detailed error information
func (r *repository) FailJobWithDetails(ctx context.Context, jobID uint, errorType models.JobErrorType, errorCode, errorMsg, errorDetails string) error {
	now := time.Now()

	// Get current job state
	var job models.Job
	if err := r.db.WithContext(ctx).First(&job, jobID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrJobNotFound
		}
		return fmt.Errorf("finding job to fail: %w", err)
	}

	// Calculate new retry count
	newRetryCount := job.RetryCount + 1

	// Determine if job should be permanently failed
	var status models.JobStatus
	if newRetryCount >= job.MaxRetries {
		status = models.JobStatusPermanentlyFailed
	} else {
		status = models.JobStatusFailed
	}

	updates := map[string]interface{}{
		"status":         status,
		"error":          errorMsg,
		"error_type":     string(errorType),
		"error_code":     errorCode,
		"error_details":  errorDetails,
		"last_failed_at": &now,
		"retry_count":    newRetryCount,
		"worker_id":      "", // Clear worker ID
	}

	// Only set completed_at for permanently failed jobs
	if status == models.JobStatusPermanentlyFailed {
		updates["completed_at"] = &now
	}

	if err := r.db.WithContext(ctx).
		Model(&models.Job{}).
		Where("id = ?", jobID).
		Updates(updates).Error; err != nil {
		return fmt.Errorf("failing job: %w", err)
	}

	return nil
}

// ReleaseJob releases a job back to pending status (e.g., if worker crashes)
func (r *repository) ReleaseJob(ctx context.Context, jobID uint) error {
	updates := map[string]interface{}{
		"status":     models.JobStatusPending,
		"worker_id":  "",
		"started_at": nil,
		"progress":   0,
	}

	result := r.db.WithContext(ctx).
		Model(&models.Job{}).
		Where("id = ? AND status = ?", jobID, models.JobStatusProcessing).
		Updates(updates)

	if result.Error != nil {
		return fmt.Errorf("releasing job: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return ErrJobNotFound
	}

	return nil
}

// DeleteOldJobs deletes jobs older than the specified time
func (r *repository) DeleteOldJobs(ctx context.Context, olderThan time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", olderThan).
		Where("status IN ?", []models.JobStatus{
			models.JobStatusCompleted,
			models.JobStatusFailed,
			models.JobStatusCancelled,
		}).
		Delete(&models.Job{})

	if result.Error != nil {
		return 0, fmt.Errorf("deleting old jobs: %w", result.Error)
	}

	return result.RowsAffected, nil
}
