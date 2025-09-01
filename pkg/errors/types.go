package errors

import (
	"fmt"
	"net/http"
)

// ErrorCode represents a structured error code
type ErrorCode string

const (
	// Configuration errors
	ErrCodeConfigNotFound   ErrorCode = "CONFIG_NOT_FOUND"
	ErrCodeConfigInvalid    ErrorCode = "CONFIG_INVALID"
	ErrCodeConfigRequired   ErrorCode = "CONFIG_REQUIRED"

	// Database errors
	ErrCodeDatabaseConnection ErrorCode = "DATABASE_CONNECTION"
	ErrCodeDatabaseQuery      ErrorCode = "DATABASE_QUERY"
	ErrCodeDatabaseMigration  ErrorCode = "DATABASE_MIGRATION"

	// Resource errors
	ErrCodeNotFound      ErrorCode = "NOT_FOUND"
	ErrCodeAlreadyExists ErrorCode = "ALREADY_EXISTS"
	ErrCodeConflict      ErrorCode = "CONFLICT"

	// Validation errors
	ErrCodeValidation   ErrorCode = "VALIDATION"
	ErrCodeInvalidInput ErrorCode = "INVALID_INPUT"
	ErrCodeMissingField ErrorCode = "MISSING_FIELD"

	// External service errors
	ErrCodeExternalService ErrorCode = "EXTERNAL_SERVICE"
	ErrCodeAPITimeout      ErrorCode = "API_TIMEOUT"
	ErrCodeAPIRateLimit    ErrorCode = "API_RATE_LIMIT"

	// Internal errors
	ErrCodeInternal        ErrorCode = "INTERNAL"
	ErrCodeServiceDown     ErrorCode = "SERVICE_DOWN"
	ErrCodeResourceExhaust ErrorCode = "RESOURCE_EXHAUSTED"

	// Authentication/Authorization errors
	ErrCodeUnauthorized ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden    ErrorCode = "FORBIDDEN"
)

// AppError represents a structured application error
type AppError struct {
	Code     ErrorCode              `json:"code"`
	Message  string                 `json:"message"`
	Details  map[string]interface{} `json:"details,omitempty"`
	Cause    error                  `json:"-"`
	HTTPCode int                    `json:"-"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the underlying cause
func (e *AppError) Unwrap() error {
	return e.Cause
}

// WithDetail adds a detail to the error
func (e *AppError) WithDetail(key string, value interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithCause sets the underlying cause
func (e *AppError) WithCause(cause error) *AppError {
	e.Cause = cause
	return e
}

// GetHTTPCode returns the appropriate HTTP status code
func (e *AppError) GetHTTPCode() int {
	if e.HTTPCode != 0 {
		return e.HTTPCode
	}
	return getDefaultHTTPCode(e.Code)
}

// New creates a new AppError
func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:     code,
		Message:  message,
		HTTPCode: getDefaultHTTPCode(code),
	}
}

// Newf creates a new AppError with formatted message
func Newf(code ErrorCode, format string, args ...interface{}) *AppError {
	return &AppError{
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
		HTTPCode: getDefaultHTTPCode(code),
	}
}

// Wrap wraps an existing error with an AppError
func Wrap(cause error, code ErrorCode, message string) *AppError {
	return &AppError{
		Code:     code,
		Message:  message,
		Cause:    cause,
		HTTPCode: getDefaultHTTPCode(code),
	}
}

// Wrapf wraps an existing error with a formatted message
func Wrapf(cause error, code ErrorCode, format string, args ...interface{}) *AppError {
	return &AppError{
		Code:     code,
		Message:  fmt.Sprintf(format, args...),
		Cause:    cause,
		HTTPCode: getDefaultHTTPCode(code),
	}
}

// getDefaultHTTPCode returns the default HTTP status code for an error code
func getDefaultHTTPCode(code ErrorCode) int {
	switch code {
	case ErrCodeNotFound:
		return http.StatusNotFound
	case ErrCodeAlreadyExists, ErrCodeConflict:
		return http.StatusConflict
	case ErrCodeValidation, ErrCodeInvalidInput, ErrCodeMissingField:
		return http.StatusBadRequest
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden:
		return http.StatusForbidden
	case ErrCodeAPIRateLimit:
		return http.StatusTooManyRequests
	case ErrCodeAPITimeout:
		return http.StatusRequestTimeout
	case ErrCodeServiceDown:
		return http.StatusServiceUnavailable
	case ErrCodeResourceExhaust:
		return http.StatusInsufficientStorage
	case ErrCodeExternalService:
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// Common error constructors

// NotFound creates a not found error
func NotFound(resource string, id interface{}) *AppError {
	return New(ErrCodeNotFound, fmt.Sprintf("%s not found", resource)).
		WithDetail("resource", resource).
		WithDetail("id", id)
}

// AlreadyExists creates an already exists error
func AlreadyExists(resource string, id interface{}) *AppError {
	return New(ErrCodeAlreadyExists, fmt.Sprintf("%s already exists", resource)).
		WithDetail("resource", resource).
		WithDetail("id", id)
}

// ValidationError creates a validation error
func ValidationError(field string, reason string) *AppError {
	return New(ErrCodeValidation, fmt.Sprintf("validation failed for field '%s': %s", field, reason)).
		WithDetail("field", field).
		WithDetail("reason", reason)
}

// MissingFieldError creates a missing field error
func MissingFieldError(field string) *AppError {
	return New(ErrCodeMissingField, fmt.Sprintf("required field '%s' is missing", field)).
		WithDetail("field", field)
}

// DatabaseError creates a database error
func DatabaseError(operation string, cause error) *AppError {
	return Wrap(cause, ErrCodeDatabaseQuery, fmt.Sprintf("database %s failed", operation)).
		WithDetail("operation", operation)
}

// ExternalServiceError creates an external service error
func ExternalServiceError(service string, cause error) *AppError {
	return Wrap(cause, ErrCodeExternalService, fmt.Sprintf("external service '%s' error", service)).
		WithDetail("service", service)
}

// ConfigError creates a configuration error
func ConfigError(key string, reason string) *AppError {
	return New(ErrCodeConfigInvalid, fmt.Sprintf("configuration error for '%s': %s", key, reason)).
		WithDetail("key", key).
		WithDetail("reason", reason)
}

// TimeoutError creates a timeout error
func TimeoutError(operation string, timeout string) *AppError {
	return New(ErrCodeAPITimeout, fmt.Sprintf("operation '%s' timed out after %s", operation, timeout)).
		WithDetail("operation", operation).
		WithDetail("timeout", timeout)
}

// RateLimitError creates a rate limit error
func RateLimitError(resource string, limit string) *AppError {
	return New(ErrCodeAPIRateLimit, fmt.Sprintf("rate limit exceeded for '%s': %s", resource, limit)).
		WithDetail("resource", resource).
		WithDetail("limit", limit)
}

// Is checks if an error is of a specific type
func Is(err error, code ErrorCode) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code == code
	}
	return false
}

// GetCode extracts the error code from an error
func GetCode(err error) ErrorCode {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Code
	}
	return ErrCodeInternal
}

// GetHTTPCode extracts the HTTP status code from an error
func GetHTTPCode(err error) int {
	if appErr, ok := err.(*AppError); ok {
		return appErr.GetHTTPCode()
	}
	return http.StatusInternalServerError
}