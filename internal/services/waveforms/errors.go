package waveforms

import "errors"

var (
	// ErrWaveformNotFound is returned when a waveform is not found
	ErrWaveformNotFound = errors.New("waveform not found")
	
	// ErrInvalidEpisodeID is returned when an episode ID is invalid
	ErrInvalidEpisodeID = errors.New("invalid episode ID")
	
	// ErrInvalidPeaksData is returned when peaks data is invalid
	ErrInvalidPeaksData = errors.New("invalid peaks data")
)