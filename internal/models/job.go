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
	JobStatusPending           JobStatus = "pending"
	JobStatusProcessing        JobStatus = "processing"
	JobStatusCompleted         JobStatus = "completed"
	JobStatusFailed            JobStatus = "failed"
	JobStatusPermanentlyFailed JobStatus = "permanently_failed"
	JobStatusCancelled         JobStatus = "cancelled"
)

// JobType represents the type of job to be processed
type JobType string

const (
	JobTypeWaveformGeneration      JobType = "waveform_generation"
	JobTypeTranscription           JobType = "transcription"
	JobTypeTranscriptionGeneration JobType = "transcription_generation"
	JobTypePodcastSync             JobType = "podcast_sync"
	JobTypeClipExtraction          JobType = "clip_extraction"
	JobTypeAutoLabel               JobType = "autolabel"
)

// JobErrorType represents the category of error that occurred
type JobErrorType string

const (
	ErrorTypeDownload   JobErrorType = "download"   // Audio file download failed
	ErrorTypeProcessing JobErrorType = "processing" // FFmpeg/audio processing failed
	ErrorTypeSystem     JobErrorType = "system"     // Database, worker, or other system error
	ErrorTypeNotFound   JobErrorType = "not_found"  // Resource permanently not found
)

// StructuredJobError represents a structured error with classification information
type StructuredJobError struct {
	Type     JobErrorType
	Code     string
	Message  string
	Details  string
	Original error
}

func (e *StructuredJobError) Error() string {
	return e.Message
}

// NewDownloadError creates a download-related structured error
func NewDownloadError(code, message, details string, originalErr error) *StructuredJobError {
	return &StructuredJobError{
		Type:     ErrorTypeDownload,
		Code:     code,
		Message:  message,
		Details:  details,
		Original: originalErr,
	}
}

// NewProcessingError creates a processing-related structured error
func NewProcessingError(code, message, details string, originalErr error) *StructuredJobError {
	return &StructuredJobError{
		Type:     ErrorTypeProcessing,
		Code:     code,
		Message:  message,
		Details:  details,
		Original: originalErr,
	}
}

// NewSystemError creates a system-related structured error
func NewSystemError(code, message, details string, originalErr error) *StructuredJobError {
	return &StructuredJobError{
		Type:     ErrorTypeSystem,
		Code:     code,
		Message:  message,
		Details:  details,
		Original: originalErr,
	}
}

// NewNotFoundError creates a not-found error that should result in permanent failure
func NewNotFoundError(code, message, details string, originalErr error) *StructuredJobError {
	return &StructuredJobError{
		Type:     ErrorTypeNotFound,
		Code:     code,
		Message:  message,
		Details:  details,
		Original: originalErr,
	}
}

// Job represents a background job in the queue
type Job struct {
	gorm.Model
	Type         JobType    `json:"type" gorm:"not null;index:idx_jobs_type_status"`
	Status       JobStatus  `json:"status" gorm:"default:'pending';index:idx_jobs_status_priority"`
	Payload      JobPayload `json:"payload" gorm:"type:json"`
	Priority     int        `json:"priority" gorm:"default:0;index:idx_jobs_status_priority"`
	MaxRetries   int        `json:"max_retries" gorm:"default:3"`
	RetryCount   int        `json:"retry_count" gorm:"default:0"`
	Progress     int        `json:"progress" gorm:"default:0"` // 0-100
	StartedAt    *time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at"`
	LastFailedAt *time.Time `json:"last_failed_at"`
	Error        string     `json:"error,omitempty"`
	Result       JobResult  `json:"result,omitempty" gorm:"type:json"`
	WorkerID     string     `json:"worker_id,omitempty"` // ID of the worker processing this job

	// Error classification fields
	ErrorType    string `json:"error_type,omitempty"`    // "download", "processing", "system"
	ErrorCode    string `json:"error_code,omitempty"`    // "403", "ffmpeg_timeout", etc.
	ErrorDetails string `json:"error_details,omitempty"` // Technical details for debugging

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

// CanRetryNow returns true if the job can be retried now (considering retry delay)
func (j *Job) CanRetryNow(minDelay time.Duration) bool {
	if !j.IsRetryable() {
		return false
	}

	// If never failed, can retry immediately
	if j.LastFailedAt == nil {
		return true
	}

	// Check if enough time has passed since last failure
	// Use exponential backoff: minDelay * 2^(retryCount)
	backoffDelay := minDelay * time.Duration(1<<uint(j.RetryCount))
	return time.Since(*j.LastFailedAt) >= backoffDelay
}

// CanProcess returns true if the job is ready to be processed
func (j *Job) CanProcess() bool {
	return j.Status == JobStatusPending
}

// IsTerminal returns true if the job is in a terminal state
func (j *Job) IsTerminal() bool {
	return j.Status == JobStatusCompleted ||
		j.Status == JobStatusCancelled ||
		j.Status == JobStatusPermanentlyFailed ||
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

// IsPermanentlyFailed returns true if the job has permanently failed
func (j *Job) IsPermanentlyFailed() bool {
	return j.Status == JobStatusPermanentlyFailed
}

// CanBeRetriedManually returns true if the job can be manually retried
func (j *Job) CanBeRetriedManually() bool {
	return j.Status == JobStatusFailed || j.Status == JobStatusPermanentlyFailed
}

// SetErrorDetails sets error classification information
func (j *Job) SetErrorDetails(errorType JobErrorType, errorCode, errorMsg, errorDetails string) {
	j.ErrorType = string(errorType)
	j.ErrorCode = errorCode
	j.Error = errorMsg
	j.ErrorDetails = errorDetails
}

// TableName specifies the table name for GORM
func (Job) TableName() string {
	return "jobs"
}
