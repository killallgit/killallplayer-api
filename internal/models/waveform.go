package models

import (
	"encoding/json"

	"gorm.io/gorm"
)

// Waveform represents audio waveform data for an episode
type Waveform struct {
	gorm.Model
	PodcastIndexEpisodeID int64   `json:"podcast_index_episode_id" gorm:"not null;uniqueIndex"`
	PeaksData             []byte  `json:"-" gorm:"type:blob;not null"`                // JSON-encoded []float32
	Duration              float64 `json:"duration" gorm:"not null"`                   // Duration in seconds
	Resolution            int     `json:"resolution" gorm:"not null"`                 // Number of peaks
	SampleRate            int     `json:"sample_rate,omitempty" gorm:"default:44100"` // Sample rate of original audio
}

// Peaks returns the decoded peaks data
func (w *Waveform) Peaks() ([]float32, error) {
	var peaks []float32
	if err := json.Unmarshal(w.PeaksData, &peaks); err != nil {
		return nil, err
	}
	return peaks, nil
}

// SetPeaks encodes and sets the peaks data
func (w *Waveform) SetPeaks(peaks []float32) error {
	data, err := json.Marshal(peaks)
	if err != nil {
		return err
	}
	w.PeaksData = data
	w.Resolution = len(peaks)
	return nil
}
