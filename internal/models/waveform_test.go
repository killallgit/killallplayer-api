package models

import (
	"reflect"
	"testing"
)

func TestWaveform_SetAndGetPeaks(t *testing.T) {
	tests := []struct {
		name    string
		peaks   []float32
		wantErr bool
	}{
		{
			name:    "normal peaks",
			peaks:   []float32{0.1, 0.5, 0.8, 0.3, 0.9, 0.2},
			wantErr: false,
		},
		{
			name:    "empty peaks",
			peaks:   []float32{},
			wantErr: false,
		},
		{
			name:    "single peak",
			peaks:   []float32{1.0},
			wantErr: false,
		},
		{
			name:    "large peaks array",
			peaks:   make([]float32, 1000),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &Waveform{}

			// Test SetPeaks
			err := wf.SetPeaks(tt.peaks)
			if (err != nil) != tt.wantErr {
				t.Errorf("Waveform.SetPeaks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return // Don't test Peaks() if SetPeaks should have failed
			}

			// Test that Resolution is set correctly
			if wf.Resolution != len(tt.peaks) {
				t.Errorf("Waveform.Resolution = %v, want %v", wf.Resolution, len(tt.peaks))
			}

			// Test Peaks
			gotPeaks, err := wf.Peaks()
			if err != nil {
				t.Errorf("Waveform.Peaks() error = %v", err)
				return
			}

			if !reflect.DeepEqual(gotPeaks, tt.peaks) {
				t.Errorf("Waveform.Peaks() = %v, want %v", gotPeaks, tt.peaks)
			}
		})
	}
}

func TestWaveform_PeaksWithInvalidData(t *testing.T) {
	wf := &Waveform{
		PeaksData: []byte("invalid json data"),
	}

	_, err := wf.Peaks()
	if err == nil {
		t.Error("Waveform.Peaks() expected error with invalid JSON data, got nil")
	}
}

func TestWaveform_SetPeaksWithFloatEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		peaks []float32
	}{
		{
			name:  "zero values",
			peaks: []float32{0.0, 0.0, 0.0},
		},
		{
			name:  "negative values",
			peaks: []float32{-0.5, -1.0, -0.3},
		},
		{
			name:  "max float values",
			peaks: []float32{3.4028235e+38, -3.4028235e+38},
		},
		{
			name:  "very small values",
			peaks: []float32{1e-10, 1e-20, 1e-30},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &Waveform{}

			err := wf.SetPeaks(tt.peaks)
			if err != nil {
				t.Errorf("Waveform.SetPeaks() error = %v", err)
				return
			}

			gotPeaks, err := wf.Peaks()
			if err != nil {
				t.Errorf("Waveform.Peaks() error = %v", err)
				return
			}

			if !reflect.DeepEqual(gotPeaks, tt.peaks) {
				t.Errorf("Waveform.Peaks() = %v, want %v", gotPeaks, tt.peaks)
			}
		})
	}
}

func TestWaveform_ModelFields(t *testing.T) {
	wf := &Waveform{
		PodcastIndexEpisodeID: 123,
		Duration:              300.5,
		Resolution:            1000,
		SampleRate:            44100,
	}

	if wf.PodcastIndexEpisodeID != 123 {
		t.Errorf("Waveform.PodcastIndexEpisodeID = %v, want 123", wf.PodcastIndexEpisodeID)
	}

	if wf.Duration != 300.5 {
		t.Errorf("Waveform.Duration = %v, want 300.5", wf.Duration)
	}

	if wf.Resolution != 1000 {
		t.Errorf("Waveform.Resolution = %v, want 1000", wf.Resolution)
	}

	if wf.SampleRate != 44100 {
		t.Errorf("Waveform.SampleRate = %v, want 44100", wf.SampleRate)
	}
}
