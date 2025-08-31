package episodes

import (
	"errors"
	"fmt"
)

// Common errors
var (
	ErrEpisodeNotFound = errors.New("episode not found")
	ErrInvalidInput    = errors.New("invalid input")
	ErrCacheMiss       = errors.New("cache miss")
	ErrSyncFailed      = errors.New("sync failed")
)

// NotFoundError represents an error when a resource is not found
type NotFoundError struct {
	Resource string
	ID       interface{}
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("%s with identifier %v not found", e.Resource, e.ID)
}

func (e NotFoundError) Is(target error) bool {
	return target == ErrEpisodeNotFound
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on field %s: %s", e.Field, e.Message)
}

func (e ValidationError) Is(target error) bool {
	return target == ErrInvalidInput
}

// APIError represents an error from an external API
type APIError struct {
	StatusCode int
	Message    string
	Endpoint   string
}

func (e APIError) Error() string {
	return fmt.Sprintf("API error from %s (status %d): %s", e.Endpoint, e.StatusCode, e.Message)
}

// SyncError represents an error during synchronization
type SyncError struct {
	SuccessCount int
	FailureCount int
	Errors       []error
}

func (e SyncError) Error() string {
	return fmt.Sprintf("sync completed with %d successes and %d failures", e.SuccessCount, e.FailureCount)
}

func (e SyncError) Is(target error) bool {
	return target == ErrSyncFailed
}

// Helper functions for creating errors

// NewNotFoundError creates a new NotFoundError
func NewNotFoundError(resource string, id interface{}) error {
	return NotFoundError{
		Resource: resource,
		ID:       id,
	}
}

// NewValidationError creates a new ValidationError
func NewValidationError(field, message string) error {
	return ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewAPIError creates a new APIError
func NewAPIError(endpoint string, statusCode int, message string) error {
	return APIError{
		Endpoint:   endpoint,
		StatusCode: statusCode,
		Message:    message,
	}
}

// NewSyncError creates a new SyncError
func NewSyncError(successCount, failureCount int, errors []error) error {
	return SyncError{
		SuccessCount: successCount,
		FailureCount: failureCount,
		Errors:       errors,
	}
}

// IsNotFound checks if an error is a not found error
func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	var notFoundErr NotFoundError
	return errors.As(err, &notFoundErr) || errors.Is(err, ErrEpisodeNotFound)
}

