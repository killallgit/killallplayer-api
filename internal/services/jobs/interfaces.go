package jobs

import (
	"context"

	"github.com/killallgit/player-api/internal/models"
)

// Service defines the business logic interface for job operations
type Service interface {
	// Enqueue operations
	EnqueueJob(ctx context.Context, jobType models.JobType, payload models.JobPayload, opts ...JobOption) (*models.Job, error)
	EnqueueUniqueJob(ctx context.Context, jobType models.JobType, payload models.JobPayload, uniqueKey string, opts ...JobOption) (*models.Job, error)

	// Status and retrieval
	GetJob(ctx context.Context, jobID uint) (*models.Job, error)
	GetJobStatus(ctx context.Context, jobID uint) (models.JobStatus, error)
	GetJobForWaveform(ctx context.Context, episodeID uint) (*models.Job, error)
	GetJobForTranscription(ctx context.Context, episodeID uint) (*models.Job, error)

	// Worker operations (used by worker pool)
	ClaimNextJob(ctx context.Context, workerID string, jobTypes []models.JobType) (*models.Job, error)
	UpdateProgress(ctx context.Context, jobID uint, progress int) error
	CompleteJob(ctx context.Context, jobID uint, result models.JobResult) error
	FailJob(ctx context.Context, jobID uint, err error) error
	ReleaseJob(ctx context.Context, jobID uint) error

	// Maintenance
	CleanupOldJobs(ctx context.Context, retentionDays int) (int64, error)
}

// JobOption is a functional option for configuring jobs
type JobOption func(*jobConfig)

// jobConfig holds configuration for a job
type jobConfig struct {
	Priority   int
	MaxRetries int
	CreatedBy  string
}

// WithPriority sets the priority of a job (higher = more priority)
func WithPriority(priority int) JobOption {
	return func(cfg *jobConfig) {
		cfg.Priority = priority
	}
}

// WithMaxRetries sets the maximum number of retries for a job
func WithMaxRetries(retries int) JobOption {
	return func(cfg *jobConfig) {
		cfg.MaxRetries = retries
	}
}

// WithCreatedBy sets who created the job
func WithCreatedBy(createdBy string) JobOption {
	return func(cfg *jobConfig) {
		cfg.CreatedBy = createdBy
	}
}
