package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// JobStatus represents the status of a job in the queue
type JobStatus string

const (
	JobStatusPending    JobStatus = "pending"
	JobStatusProcessing JobStatus = "processing"
	JobStatusCompleted  JobStatus = "completed"
	JobStatusFailed     JobStatus = "failed"
	JobStatusCancelled  JobStatus = "cancelled"
)

// JobType represents the type of job to be processed
type JobType string

const (
	JobTypeWaveformGeneration JobType = "waveform_generation"
	JobTypeTranscription      JobType = "transcription"
	JobTypePodcastSync        JobType = "podcast_sync"
)

// Job represents a background job in the queue
type Job struct {
	gorm.Model
	Type        JobType    `json:"type" gorm:"not null;index:idx_jobs_type_status"`
	Status      JobStatus  `json:"status" gorm:"default:'pending';index:idx_jobs_status_priority"`
	Payload     JobPayload `json:"payload" gorm:"type:json"`
	Priority    int        `json:"priority" gorm:"default:0;index:idx_jobs_status_priority"`
	MaxRetries  int        `json:"max_retries" gorm:"default:3"`
	RetryCount  int        `json:"retry_count" gorm:"default:0"`
	Progress    int        `json:"progress" gorm:"default:0"` // 0-100
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	Error       string     `json:"error,omitempty"`
	Result      JobResult  `json:"result,omitempty" gorm:"type:json"`
	WorkerID    string     `json:"worker_id,omitempty"` // ID of the worker processing this job

	// Metadata
	CreatedBy string `json:"created_by,omitempty"` // Optional user/system identifier
}

// JobPayload represents the input data for a job
type JobPayload map[string]interface{}

// Value implements driver.Valuer interface for JobPayload
func (p JobPayload) Value() (driver.Value, error) {
	if p == nil {
		return nil, nil
	}
	return json.Marshal(p)
}

// Scan implements sql.Scanner interface for JobPayload
func (p *JobPayload) Scan(value interface{}) error {
	if value == nil {
		*p = make(JobPayload)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, p)
}

// JobResult represents the output data from a completed job
type JobResult map[string]interface{}

// Value implements driver.Valuer interface for JobResult
func (r JobResult) Value() (driver.Value, error) {
	if r == nil {
		return nil, nil
	}
	return json.Marshal(r)
}

// Scan implements sql.Scanner interface for JobResult
func (r *JobResult) Scan(value interface{}) error {
	if value == nil {
		*r = make(JobResult)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}

	return json.Unmarshal(bytes, r)
}

// Helper methods

// IsRetryable returns true if the job can be retried
func (j *Job) IsRetryable() bool {
	return j.Status == JobStatusFailed && j.RetryCount < j.MaxRetries
}

// CanProcess returns true if the job is ready to be processed
func (j *Job) CanProcess() bool {
	return j.Status == JobStatusPending
}

// IsTerminal returns true if the job is in a terminal state
func (j *Job) IsTerminal() bool {
	return j.Status == JobStatusCompleted ||
		j.Status == JobStatusCancelled ||
		(j.Status == JobStatusFailed && !j.IsRetryable())
}

// GetPayloadValue safely retrieves a value from the payload
func (j *Job) GetPayloadValue(key string) (interface{}, bool) {
	if j.Payload == nil {
		return nil, false
	}
	val, ok := j.Payload[key]
	return val, ok
}

// GetPayloadString safely retrieves a string value from the payload
func (j *Job) GetPayloadString(key string) (string, bool) {
	val, ok := j.GetPayloadValue(key)
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetPayloadInt safely retrieves an int value from the payload
func (j *Job) GetPayloadInt(key string) (int, bool) {
	val, ok := j.GetPayloadValue(key)
	if !ok {
		return 0, false
	}

	// Handle both int and float64 (JSON numbers are decoded as float64)
	switch v := val.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	case int64:
		return int(v), true
	default:
		return 0, false
	}
}

// SetResult sets a result value
func (j *Job) SetResult(key string, value interface{}) {
	if j.Result == nil {
		j.Result = make(JobResult)
	}
	j.Result[key] = value
}

// TableName specifies the table name for GORM
func (Job) TableName() string {
	return "jobs"
}
