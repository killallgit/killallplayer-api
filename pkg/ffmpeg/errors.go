package ffmpeg

import (
	"errors"
	"fmt"
)

// Common errors
var (
	ErrFFmpegNotFound        = errors.New("ffmpeg binary not found")
	ErrFFprobeNotFound       = errors.New("ffprobe binary not found")
	ErrInvalidAudioFile      = errors.New("invalid or unsupported audio file")
	ErrAudioTooLong          = errors.New("audio file exceeds maximum duration")
	ErrProcessingTimeout     = errors.New("audio processing timeout")
	ErrInsufficientDiskSpace = errors.New("insufficient disk space for processing")
	ErrTempFileCreation      = errors.New("failed to create temporary file")
)

// ProcessingError represents an error during audio processing
type ProcessingError struct {
	Operation string // The operation that failed (e.g., "metadata_extraction", "waveform_generation")
	File      string // The file being processed
	Err       error  // The underlying error
	Stderr    string // stderr output from ffmpeg/ffprobe
}

func (e *ProcessingError) Error() string {
	if e.Stderr != "" {
		return fmt.Sprintf("ffmpeg %s failed for %s: %v (stderr: %s)", e.Operation, e.File, e.Err, e.Stderr)
	}
	return fmt.Sprintf("ffmpeg %s failed for %s: %v", e.Operation, e.File, e.Err)
}

func (e *ProcessingError) Unwrap() error {
	return e.Err
}

// NewProcessingError creates a new ProcessingError
func NewProcessingError(operation, file string, err error, stderr string) *ProcessingError {
	return &ProcessingError{
		Operation: operation,
		File:      file,
		Err:       err,
		Stderr:    stderr,
	}
}
