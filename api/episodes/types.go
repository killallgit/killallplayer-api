package episodes

import (
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/services/episodes"
)

// WaveformStatus represents the status of waveform processing
type WaveformStatus struct {
	Status   string              `json:"status" example:"completed"`       // completed|processing|pending|failed
	Message  string              `json:"message" example:"Waveform ready"` // Human-readable status message
	Progress int                 `json:"progress,omitempty" example:"75"`  // 0-100 for processing states
	Data     *types.WaveformData `json:"data,omitempty"`                   // Only present when status="completed"
}

// EpisodeResponse represents a single episode with embedded waveform and transcription status
// This is our API's standardized episode response format for single episode GET requests
// Note: Annotations are already included in PodcastIndexEpisode
type EpisodeResponse struct {
	episodes.PodcastIndexEpisode
	Waveform      *WaveformStatus      `json:"waveform,omitempty"`      // Waveform processing status and data
	Transcription *TranscriptionStatus `json:"transcription,omitempty"` // Transcription processing status and data
}

// Status constants for waveform processing
const (
	WaveformStatusCompleted  = "completed"
	WaveformStatusProcessing = "processing"
	WaveformStatusPending    = "pending"
	WaveformStatusFailed     = "failed"
)

// Status messages for human-readable display
var WaveformStatusMessages = map[string]string{
	WaveformStatusCompleted:  "Waveform ready",
	WaveformStatusProcessing: "Processing waveform...",
	WaveformStatusPending:    "Waveform generation pending...",
	WaveformStatusFailed:     "Waveform generation failed",
}

// TranscriptionStatus represents the status of transcription processing
type TranscriptionStatus struct {
	Status   string                   `json:"status" example:"completed"`            // completed|processing|pending|failed
	Message  string                   `json:"message" example:"Transcription ready"` // Human-readable status message
	Progress int                      `json:"progress,omitempty" example:"75"`       // 0-100 for processing states
	Data     *types.TranscriptionData `json:"data,omitempty"`                        // Only present when status="completed"
}

// Status constants for transcription processing
const (
	TranscriptionStatusCompleted  = "completed"
	TranscriptionStatusProcessing = "processing"
	TranscriptionStatusPending    = "pending"
	TranscriptionStatusFailed     = "failed"
)

// Status messages for transcription
var TranscriptionStatusMessages = map[string]string{
	TranscriptionStatusCompleted:  "Transcription ready",
	TranscriptionStatusProcessing: "Processing transcription...",
	TranscriptionStatusPending:    "Transcription generation pending...",
	TranscriptionStatusFailed:     "Transcription generation failed",
}

// EpisodeByGUIDResponse represents our API's response wrapper for single episode by GUID
type EpisodeByGUIDResponse struct {
	Status      string           `json:"status" example:"true"`
	Episode     *EpisodeResponse `json:"episode,omitempty"`
	Description string           `json:"description" example:"Episode found"`
}
