package workers

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/pkg/ffmpeg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnhancedWaveformProcessor_CanProcess tests job type filtering (basic functionality test)
func TestEnhancedWaveformProcessor_CanProcess(t *testing.T) {
	// Create an enhanced processor for testing
	processor := &EnhancedWaveformProcessor{}

	// Should accept waveform jobs
	assert.True(t, processor.CanProcess(models.JobTypeWaveformGeneration))

	// Should reject other job types
	assert.False(t, processor.CanProcess(models.JobTypeTranscription))
	assert.False(t, processor.CanProcess(models.JobTypePodcastSync))
	assert.False(t, processor.CanProcess("unknown_type"))
}

// TestEnhancedWaveformProcessor_ParseEpisodeID tests job payload parsing
func TestEnhancedWaveformProcessor_ParseEpisodeID(t *testing.T) {
	processor := &EnhancedWaveformProcessor{}

	tests := []struct {
		name     string
		payload  models.JobPayload
		expected uint
		hasError bool
	}{
		{
			name:     "valid episode_id as float64",
			payload:  models.JobPayload{"episode_id": float64(123)},
			expected: 123,
			hasError: false,
		},
		{
			name:     "valid episode_id as int",
			payload:  models.JobPayload{"episode_id": 456},
			expected: 456,
			hasError: false,
		},
		{
			name:     "missing episode_id",
			payload:  models.JobPayload{"other_field": "value"},
			expected: 0,
			hasError: true,
		},
		{
			name:     "invalid episode_id type",
			payload:  models.JobPayload{"episode_id": "not_a_number"},
			expected: 0,
			hasError: true,
		},
		{
			name:     "zero episode_id",
			payload:  models.JobPayload{"episode_id": 0},
			expected: 0,
			hasError: false, // Zero ID is actually valid
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := processor.parseEpisodeID(tt.payload)

			if tt.hasError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestFFmpegIntegrationWithWorkerSystem tests that FFmpeg can process our test audio
// This validates the FFmpeg→waveform generation pipeline that the worker would use
func TestFFmpegIntegrationWithWorkerSystem(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping FFmpeg integration test in short mode")
	}
	// Setup FFmpeg - skip if not available
	ffmpegInstance := ffmpeg.New("ffmpeg", "ffprobe", 30*time.Second)
	if err := ffmpegInstance.ValidateBinaries(); err != nil {
		t.Skipf("FFmpeg binaries not available: %v", err)
	}

	// Test with our test audio files
	testCases := []struct {
		name     string
		filename string
		minPeaks int // Minimum expected non-zero peaks
	}{
		{
			name:     "5-second test clip",
			filename: "test-5s.mp3",
			minPeaks: 50, // Expect at least 50 non-zero peaks for 5s audio
		},
		{
			name:     "30-second test clip",
			filename: "test-30s.mp3",
			minPeaks: 250, // Expect at least 250 non-zero peaks for 30s audio
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Setup file path
			testFile := filepath.Join("..", "..", "..", "data", "tests", "clips", tc.filename)

			// Use processing options similar to what the worker would use
			opts := ffmpeg.ProcessingOptions{
				WaveformResolution: 300, // Good resolution for testing
				MaxDuration:        1 * time.Minute,
				TempDir:            "/tmp",
			}

			// Generate waveform (this is what the worker system does internally)
			ctx := context.Background()
			waveform, err := ffmpegInstance.GenerateWaveform(ctx, testFile, opts)

			require.NoError(t, err, "Waveform generation should succeed")
			require.NotNil(t, waveform, "Waveform should not be nil")

			// Validate waveform properties
			assert.Greater(t, len(waveform.Peaks), 0, "Should have peaks")
			assert.Equal(t, opts.WaveformResolution, len(waveform.Peaks), "Should have requested number of peaks")
			assert.Greater(t, waveform.Duration, 0.0, "Duration should be positive")
			assert.Greater(t, waveform.SampleRate, 0, "Sample rate should be positive")

			// Count non-zero peaks to ensure we have actual audio content
			nonZeroPeaks := 0
			for _, peak := range waveform.Peaks {
				if peak > 0.01 { // Small threshold for noise
					nonZeroPeaks++
				}
				// Validate peak is in valid range
				assert.GreaterOrEqual(t, peak, float32(0.0), "Peak should be >= 0")
				assert.LessOrEqual(t, peak, float32(1.0), "Peak should be <= 1")
			}

			assert.GreaterOrEqual(t, nonZeroPeaks, tc.minPeaks,
				"Should have sufficient non-zero peaks indicating real audio content")

			t.Logf("Successfully processed %s: %d peaks, %.2fs duration, %d non-zero peaks",
				tc.filename, len(waveform.Peaks), waveform.Duration, nonZeroPeaks)
		})
	}
}
