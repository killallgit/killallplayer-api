package ffmpeg

import "time"

// AudioMetadata represents metadata extracted from an audio file
type AudioMetadata struct {
	Duration   float64 `json:"duration"`    // Duration in seconds
	SampleRate int     `json:"sample_rate"` // Sample rate in Hz
	Channels   int     `json:"channels"`    // Number of audio channels
	Bitrate    int     `json:"bitrate"`     // Bitrate in bits per second
	Format     string  `json:"format"`      // Container format (mp3, m4a, etc.)
	Codec      string  `json:"codec"`       // Audio codec
	Size       int64   `json:"size"`        // File size in bytes
	Title      string  `json:"title"`       // Title metadata
	Artist     string  `json:"artist"`      // Artist metadata
	Album      string  `json:"album"`       // Album metadata
	Year       string  `json:"year"`        // Year metadata
}

// WaveformData represents audio waveform peak data
type WaveformData struct {
	Peaks      []float32 `json:"peaks"`       // Peak values (0.0 - 1.0)
	Duration   float64   `json:"duration"`    // Duration in seconds
	Resolution int       `json:"resolution"`  // Number of peaks
	SampleRate int       `json:"sample_rate"` // Original sample rate
}

// ProcessingOptions defines options for audio processing
type ProcessingOptions struct {
	WaveformResolution int           `json:"waveform_resolution"` // Number of peaks to generate
	MaxDuration        time.Duration `json:"max_duration"`        // Maximum duration to process
	TempDir            string        `json:"temp_dir"`            // Directory for temporary files
}

// DefaultProcessingOptions returns sensible defaults for audio processing
func DefaultProcessingOptions() ProcessingOptions {
	return ProcessingOptions{
		WaveformResolution: 1000,
		MaxDuration:        2 * time.Hour, // 2 hours max
		TempDir:            "/tmp",
	}
}
