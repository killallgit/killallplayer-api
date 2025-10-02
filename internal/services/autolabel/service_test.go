package autolabel

import (
	"context"
	"os"
	"testing"

	"github.com/killallgit/player-api/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestPeakDetectorWithRealAudio(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping FFmpeg integration test in short mode")
	}

	// Use test audio file from testdata directory
	audioPath := "testdata/test-5s.mp3"
	if _, err := os.Stat(audioPath); err != nil {
		t.Fatalf("Test audio file not found at %s: %v", audioPath, err)
	}

	t.Logf("Testing with audio file: %s", audioPath)

	// Create peak detector
	detector := NewFFmpegPeakDetector("")

	// Detect peaks
	ctx := context.Background()
	stats, err := detector.DetectPeaks(ctx, audioPath)
	require.NoError(t, err, "Peak detection should succeed")
	require.NotNil(t, stats, "Stats should not be nil")

	// Validate stats
	t.Logf("Volume stats - Mean: %.2f dB, Max: %.2f dB, Peaks: %d, Silence: %.2fs",
		stats.MeanVolume, stats.MaxVolume, stats.PeakCount, stats.SilenceDuration)

	// Basic sanity checks
	// Volume in dB is typically negative (0 dB is max digital level)
	assert.LessOrEqual(t, stats.MeanVolume, 0.0, "Mean volume should be <= 0 dB")
	assert.LessOrEqual(t, stats.MaxVolume, 0.0, "Max volume should be <= 0 dB")
	assert.LessOrEqual(t, stats.MeanVolume, stats.MaxVolume, "Mean should be <= Max")
	assert.GreaterOrEqual(t, stats.PeakCount, 0, "Peak count should be non-negative")
}

func TestAutoLabelService(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping autolabel service test in short mode")
	}

	// Use test audio file from testdata directory
	audioPath := "testdata/test-5s.mp3"
	if _, err := os.Stat(audioPath); err != nil {
		t.Fatalf("Test audio file not found at %s: %v", audioPath, err)
	}

	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Auto-migrate
	err = db.AutoMigrate(&models.Clip{})
	require.NoError(t, err)

	// Create autolabel service
	detector := NewFFmpegPeakDetector("")
	service := NewService(db, detector)

	// Test AutoLabelClip
	ctx := context.Background()
	result, err := service.AutoLabelClip(ctx, audioPath)
	require.NoError(t, err, "AutoLabelClip should succeed")
	require.NotNil(t, result, "Result should not be nil")

	// Validate result
	t.Logf("Autolabel result - Label: %s, Confidence: %.2f, Method: %s",
		result.Label, result.Confidence, result.Method)

	assert.NotEmpty(t, result.Label, "Label should not be empty")
	assert.Greater(t, result.Confidence, 0.0, "Confidence should be > 0")
	assert.LessOrEqual(t, result.Confidence, 1.0, "Confidence should be <= 1")
	assert.Equal(t, "peak_detection", result.Method, "Method should be peak_detection")
	assert.Contains(t, []string{"speech", "music", "advertisement", "silence"}, result.Label,
		"Label should be one of the expected types")

	// Test UpdateClipWithAutoLabel
	filename := "test.wav"
	duration := 5.0
	sizeBytes := int64(12345)
	clip := &models.Clip{
		UUID:              "test-uuid-123",
		SourceEpisodeURL:  "https://example.com/test.mp3",
		OriginalStartTime: 0,
		OriginalEndTime:   5,
		Label:             "unlabeled",
		ClipFilename:      &filename,
		ClipDuration:      &duration,
		ClipSizeBytes:     &sizeBytes,
		Status:            "ready",
		Extracted:         true,
	}
	err = db.Create(clip).Error
	require.NoError(t, err)

	err = service.UpdateClipWithAutoLabel(ctx, clip.UUID, result)
	require.NoError(t, err, "UpdateClipWithAutoLabel should succeed")

	// Verify the clip was updated
	var updatedClip models.Clip
	err = db.Where("uuid = ?", clip.UUID).First(&updatedClip).Error
	require.NoError(t, err)

	assert.True(t, updatedClip.AutoLabeled, "AutoLabeled should be true")
	assert.NotNil(t, updatedClip.LabelConfidence, "LabelConfidence should not be nil")
	assert.Equal(t, result.Confidence, *updatedClip.LabelConfidence, "Confidence should match")
	assert.Equal(t, result.Method, updatedClip.LabelMethod, "Method should match")
	assert.Equal(t, result.Label, updatedClip.Label, "Label should match")

	t.Logf("Successfully updated clip with autolabel data")
}

func TestClassifyAudio(t *testing.T) {
	// Create service with mock detector (doesn't matter for this test)
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	service := NewService(db, &MockPeakDetector{})
	impl := service.(*ServiceImpl)

	tests := []struct {
		name     string
		stats    *VolumeStats
		expected string
	}{
		{
			name: "Silence - low mean volume",
			stats: &VolumeStats{
				MeanVolume:      -50.0,
				MaxVolume:       -30.0,
				PeakCount:       1,
				SilenceDuration: 1.0,
			},
			expected: "silence",
		},
		{
			name: "Silence - high silence duration",
			stats: &VolumeStats{
				MeanVolume:      -25.0,
				MaxVolume:       -15.0,
				PeakCount:       3,
				SilenceDuration: 5.0,
			},
			expected: "silence",
		},
		{
			name: "Music - multiple peaks and moderate volume",
			stats: &VolumeStats{
				MeanVolume:      -18.0,
				MaxVolume:       -8.0,
				PeakCount:       8,
				SilenceDuration: 0.5,
			},
			expected: "music",
		},
		{
			name: "Advertisement - high volume",
			stats: &VolumeStats{
				MeanVolume:      -8.0,
				MaxVolume:       -3.0,
				PeakCount:       4,
				SilenceDuration: 0.2,
			},
			expected: "advertisement",
		},
		{
			name: "Speech - normal patterns",
			stats: &VolumeStats{
				MeanVolume:      -25.0,
				MaxVolume:       -12.0,
				PeakCount:       3,
				SilenceDuration: 1.0,
			},
			expected: "speech",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := impl.classifyAudio(tt.stats)
			assert.Equal(t, tt.expected, result.Label, "Label should match expected")
			assert.Greater(t, result.Confidence, 0.0, "Confidence should be > 0")
			assert.Equal(t, "peak_detection", result.Method, "Method should be peak_detection")
		})
	}
}

// TestMain is no longer needed - testdata is relative to test file
