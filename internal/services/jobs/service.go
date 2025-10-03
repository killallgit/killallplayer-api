package jobs

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/killallgit/player-api/internal/models"
)

const (
	DefaultMaxRetries = 3
	DefaultPriority   = 0
)

type service struct {
	repo Repository
}

func NewService(repo Repository) Service {
	return &service{
		repo: repo,
	}
}

func (s *service) EnqueueJob(ctx context.Context, jobType models.JobType, payload models.JobPayload, opts ...JobOption) (*models.Job, error) {
	cfg := &jobConfig{
		Priority:   DefaultPriority,
		MaxRetries: DefaultMaxRetries,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	job := &models.Job{
		Type:       jobType,
		Status:     models.JobStatusPending,
		Payload:    payload,
		Priority:   cfg.Priority,
		MaxRetries: cfg.MaxRetries,
		CreatedBy:  cfg.CreatedBy,
	}

	if err := s.repo.CreateJob(ctx, job); err != nil {
		return nil, fmt.Errorf("creating job: %w", err)
	}

	log.Printf("[DEBUG] Enqueued %s job ID %d with priority %d", jobType, job.ID, job.Priority)

	return job, nil
}

func (s *service) EnqueueUniqueJob(ctx context.Context, jobType models.JobType, payload models.JobPayload, uniqueKey string, opts ...JobOption) (*models.Job, error) {
	uniqueValue, ok := payload[uniqueKey]
	if !ok {
		return nil, fmt.Errorf("unique key %s not found in payload", uniqueKey)
	}

	existingJob, err := s.repo.GetJobByTypeAndPayload(ctx, jobType, uniqueKey, fmt.Sprintf("%v", uniqueValue))
	if err == nil && existingJob != nil {
		if !existingJob.IsTerminal() {
			log.Printf("[DEBUG] Job already exists for %s with %s=%v (ID: %d, Status: %s)",
				jobType, uniqueKey, uniqueValue, existingJob.ID, existingJob.Status)
			return existingJob, nil
		}
	}

	return s.EnqueueJob(ctx, jobType, payload, opts...)
}

func (s *service) GetJob(ctx context.Context, jobID uint) (*models.Job, error) {
	job, err := s.repo.GetJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("getting job: %w", err)
	}
	return job, nil
}

func (s *service) GetJobStatus(ctx context.Context, jobID uint) (models.JobStatus, error) {
	job, err := s.GetJob(ctx, jobID)
	if err != nil {
		return "", err
	}
	return job.Status, nil
}

func (s *service) GetJobForWaveform(ctx context.Context, podcastIndexEpisodeID int64) (*models.Job, error) {
	job, err := s.repo.GetJobByTypeAndPayload(ctx, models.JobTypeWaveformGeneration, "episode_id", fmt.Sprintf("%d", podcastIndexEpisodeID))
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("getting job for waveform: %w", err)
	}
	return job, nil
}

func (s *service) GetJobForTranscription(ctx context.Context, podcastIndexEpisodeID int64) (*models.Job, error) {
	job, err := s.repo.GetJobByTypeAndPayload(ctx, models.JobTypeTranscriptionGeneration, "episode_id", fmt.Sprintf("%d", podcastIndexEpisodeID))
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("getting job for transcription: %w", err)
	}
	return job, nil
}

func (s *service) ClaimNextJob(ctx context.Context, workerID string, jobTypes []models.JobType) (*models.Job, error) {
	job, err := s.repo.ClaimNextJob(ctx, workerID, jobTypes)
	if err != nil {
		if errors.Is(err, ErrNoJobsAvailable) {
			return nil, err
		}
		return nil, fmt.Errorf("claiming job: %w", err)
	}

	log.Printf("[DEBUG] Worker %s claimed %s job ID %d", workerID, job.Type, job.ID)

	return job, nil
}

func (s *service) UpdateProgress(ctx context.Context, jobID uint, progress int) error {
	if err := s.repo.UpdateJobProgress(ctx, jobID, progress); err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return err
		}
		return fmt.Errorf("updating job progress: %w", err)
	}

	if progress%10 == 0 || progress == 100 {
		log.Printf("[DEBUG] Job %d progress: %d%%", jobID, progress)
	}

	return nil
}

func (s *service) CompleteJob(ctx context.Context, jobID uint, result models.JobResult) error {
	if err := s.repo.CompleteJob(ctx, jobID, result); err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return err
		}
		return fmt.Errorf("completing job: %w", err)
	}

	log.Printf("[DEBUG] Job %d completed successfully", jobID)

	return nil
}

func (s *service) FailJob(ctx context.Context, jobID uint, err error) error {
	errorMsg := err.Error()

	if err := s.repo.FailJob(ctx, jobID, errorMsg); err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return err
		}
		return fmt.Errorf("failing job: %w", err)
	}

	// Check if job is retryable
	job, _ := s.repo.GetJob(ctx, jobID)
	if job != nil && job.IsRetryable() {
		log.Printf("[ERROR] Job %d failed (retry %d/%d): %s", jobID, job.RetryCount, job.MaxRetries, errorMsg)
	} else {
		log.Printf("[ERROR] Job %d failed permanently: %s", jobID, errorMsg)
	}

	return nil
}

func (s *service) ReleaseJob(ctx context.Context, jobID uint) error {
	if err := s.repo.ReleaseJob(ctx, jobID); err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return err
		}
		return fmt.Errorf("releasing job: %w", err)
	}

	log.Printf("[DEBUG] Job %d released back to pending", jobID)

	return nil
}

func (s *service) FailJobWithDetails(ctx context.Context, jobID uint, errorType models.JobErrorType, errorCode, errorMsg, errorDetails string) error {
	if err := s.repo.FailJobWithDetails(ctx, jobID, errorType, errorCode, errorMsg, errorDetails); err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return err
		}
		return fmt.Errorf("failing job with details: %w", err)
	}

	// Check if job is retryable
	job, _ := s.repo.GetJob(ctx, jobID)
	if job != nil && job.IsRetryable() {
		log.Printf("[ERROR] Job %d failed with %s error '%s' (retry %d/%d): %s",
			jobID, errorType, errorCode, job.RetryCount, job.MaxRetries, errorMsg)
	} else {
		log.Printf("[ERROR] Job %d failed permanently with %s error '%s': %s",
			jobID, errorType, errorCode, errorMsg)
	}

	return nil
}

func (s *service) RetryFailedJob(ctx context.Context, jobID uint) (*models.Job, error) {
	job, err := s.repo.GetJob(ctx, jobID)
	if err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("getting job for retry: %w", err)
	}

	// Only allow retry for failed or permanently failed jobs
	if job.Status != models.JobStatusFailed && job.Status != models.JobStatusPermanentlyFailed {
		return nil, fmt.Errorf("job %d cannot be retried: status is %s (only 'failed' or 'permanently_failed' jobs can be retried)",
			jobID, job.Status)
	}

	// Reset the job to pending status for retry
	if err := s.repo.ReleaseJob(ctx, jobID); err != nil {
		if errors.Is(err, ErrJobNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("resetting job for retry: %w", err)
	}

	// Get the updated job
	updatedJob, err := s.repo.GetJob(ctx, jobID)
	if err != nil {
		return nil, fmt.Errorf("getting updated job after retry: %w", err)
	}

	log.Printf("[DEBUG] Job %d manually retried (was %s, now %s)", jobID, job.Status, updatedJob.Status)

	return updatedJob, nil
}

func (s *service) CleanupOldJobs(ctx context.Context, retentionDays int) (int64, error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("retention days must be positive")
	}

	cutoffTime := time.Now().AddDate(0, 0, -retentionDays)

	deleted, err := s.repo.DeleteOldJobs(ctx, cutoffTime)
	if err != nil {
		return 0, fmt.Errorf("cleaning up old jobs: %w", err)
	}

	if deleted > 0 {
		log.Printf("[DEBUG] Deleted %d old jobs (older than %d days)", deleted, retentionDays)
	}

	return deleted, nil
}

func (s *service) DeletePermanentlyFailedJob(ctx context.Context, jobID uint) error {
	return s.repo.DeletePermanentlyFailedJob(ctx, jobID)
}
