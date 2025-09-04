package episodes

import (
	"github.com/killallgit/player-api/internal/services/episodes"
)

// WaveformStatus represents the status of waveform processing
type WaveformStatus struct {
	Status   string        `json:"status" example:"ok"`              // ok|processing|downloading|error
	Message  string        `json:"message" example:"Waveform ready"` // Human-readable status message
	Progress int           `json:"progress,omitempty" example:"75"`  // 0-100 for processing/downloading states
	Data     *WaveformData `json:"data,omitempty"`                   // Only present when status="ok"
}

// WaveformData represents the waveform data when ready
type WaveformData struct {
	Peaks      []float32 `json:"peaks" example:"0.1,0.2,0.3"` // Waveform peak values
	Duration   float64   `json:"duration" example:"300.5"`    // Duration in seconds
	Resolution int       `json:"resolution" example:"1000"`   // Number of peaks
	SampleRate int       `json:"sample_rate" example:"44100"` // Sample rate in Hz
}

// EnhancedEpisodeResponse represents a single episode with embedded waveform status
// This is used only for single episode GET requests, not for lists/searches
type EnhancedEpisodeResponse struct {
	episodes.PodcastIndexEpisode
	Waveform *WaveformStatus `json:"waveform,omitempty"` // Waveform processing status and data
}

// Status constants for waveform processing
const (
	WaveformStatusOK          = "ok"
	WaveformStatusProcessing  = "processing"
	WaveformStatusDownloading = "downloading"
	WaveformStatusError       = "error"
)

// Status messages for human-readable display
var WaveformStatusMessages = map[string]string{
	WaveformStatusOK:          "Waveform ready",
	WaveformStatusProcessing:  "Processing waveform...",
	WaveformStatusDownloading: "Downloading audio...",
	WaveformStatusError:       "Waveform generation failed",
}

// EpisodeByGUIDEnhancedResponse represents a single episode response with waveform
type EpisodeByGUIDEnhancedResponse struct {
	Status      string                   `json:"status" example:"true"`
	Episode     *EnhancedEpisodeResponse `json:"episode,omitempty"`
	Description string                   `json:"description" example:"Episode found"`
}
