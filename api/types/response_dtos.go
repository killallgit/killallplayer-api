package types

// WaveformData represents waveform data in API responses
// This is the consolidated version replacing duplicates in api/episodes and api/waveform packages
type WaveformData struct {
	EpisodeID  int64     `json:"episode_id,omitempty"`                  // Episode ID (optional for some responses)
	Status     string    `json:"status,omitempty"`                      // Status of the waveform (completed, processing, etc.) - optional for simple responses
	Peaks      []float32 `json:"peaks" example:"0.1,0.2,0.3"`           // Waveform peak values
	Duration   float64   `json:"duration" example:"300.5"`              // Duration in seconds
	Resolution int       `json:"resolution" example:"1000"`             // Number of peaks
	SampleRate int       `json:"sample_rate,omitempty" example:"44100"` // Sample rate in Hz - optional
	Cached     bool      `json:"cached,omitempty"`                      // Whether data is cached - optional for some responses
}

// TranscriptionData represents transcription data in API responses
// This is the consolidated version replacing duplicates in api/episodes and api/transcription packages
type TranscriptionData struct {
	EpisodeID uint    `json:"episode_id,omitempty"`                        // Episode ID (optional for some responses)
	Text      string  `json:"text" example:"This is the transcription..."` // Full transcription text
	Language  string  `json:"language" example:"en"`                       // Detected or specified language
	Duration  float64 `json:"duration" example:"300.5"`                    // Duration in seconds
	Model     string  `json:"model" example:"ggml-base.en.bin"`            // Model used for transcription
	Source    string  `json:"source,omitempty"`                            // "fetched" or "generated" - optional for some responses
	Cached    bool    `json:"cached,omitempty"`                            // Whether data is cached - optional for some responses
}

// JobStatusResponse represents job status information
type JobStatusResponse struct {
	EpisodeID    int64   `json:"episode_id"`              // Episode ID
	JobID        uint    `json:"job_id,omitempty"`        // Job ID (optional)
	Status       string  `json:"status"`                  // Status: pending, processing, completed, failed, permanently_failed, not_found
	Progress     int     `json:"progress"`                // Progress 0-100
	Message      string  `json:"message"`                 // Human-readable message
	Error        string  `json:"error,omitempty"`         // Error message (only for failed status)
	ErrorType    string  `json:"error_type,omitempty"`    // Error type: "download", "processing", "system" (only for failed jobs)
	ErrorCode    string  `json:"error_code,omitempty"`    // Specific error code like "403", "timeout", "corrupt_file" (only for failed jobs)
	ErrorDetails string  `json:"error_details,omitempty"` // Technical error details for debugging (only for failed jobs)
	RetryCount   int     `json:"retry_count,omitempty"`   // Number of retries attempted (only for failed jobs)
	MaxRetries   int     `json:"max_retries,omitempty"`   // Maximum retry attempts (only for failed jobs)
	RetryAfter   float64 `json:"retry_after,omitempty"`   // Seconds until retry (only for failed jobs)
	Retried      bool    `json:"retried,omitempty"`       // True if this was a manual retry (only when applicable)
	Hint         string  `json:"hint,omitempty"`          // Helpful hint for the client (e.g., "Use retry=true parameter")
}
