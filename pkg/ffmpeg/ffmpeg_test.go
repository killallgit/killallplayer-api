package ffmpeg

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	ffmpeg := New("ffmpeg", "ffprobe", 30*time.Second)
	if ffmpeg.ffmpegPath != "ffmpeg" {
		t.Errorf("Expected ffmpegPath to be 'ffmpeg', got %s", ffmpeg.ffmpegPath)
	}
	if ffmpeg.ffprobePath != "ffprobe" {
		t.Errorf("Expected ffprobePath to be 'ffprobe', got %s", ffmpeg.ffprobePath)
	}
	if ffmpeg.timeout != 30*time.Second {
		t.Errorf("Expected timeout to be 30s, got %v", ffmpeg.timeout)
	}
}

func TestDefaultProcessingOptions(t *testing.T) {
	opts := DefaultProcessingOptions()
	if opts.WaveformResolution != 1000 {
		t.Errorf("Expected WaveformResolution to be 1000, got %d", opts.WaveformResolution)
	}
	if opts.MaxDuration != 2*time.Hour {
		t.Errorf("Expected MaxDuration to be 2h, got %v", opts.MaxDuration)
	}
	if opts.TempDir != "/tmp" {
		t.Errorf("Expected TempDir to be '/tmp', got %s", opts.TempDir)
	}
}

func TestAbs(t *testing.T) {
	tests := []struct {
		input    float32
		expected float32
	}{
		{1.5, 1.5},
		{-1.5, 1.5},
		{0, 0},
		{-0.001, 0.001},
	}

	for _, test := range tests {
		result := abs(test.input)
		if result != test.expected {
			t.Errorf("abs(%f) = %f, expected %f", test.input, result, test.expected)
		}
	}
}

// Integration test - only runs if ffmpeg/ffprobe are available
func TestValidateBinaries(t *testing.T) {
	ffmpeg := New("ffmpeg", "ffprobe", 30*time.Second)

	// This test will pass if ffmpeg/ffprobe are installed, skip otherwise
	err := ffmpeg.ValidateBinaries()
	if err != nil {
		t.Skipf("FFmpeg binaries not available: %v", err)
	}
}

// Test metadata extraction with real audio file
func TestGetMetadataWithRealAudio(t *testing.T) {
	ffmpeg := New("ffmpeg", "ffprobe", 30*time.Second)

	// Skip if binaries not available
	if err := ffmpeg.ValidateBinaries(); err != nil {
		t.Skipf("FFmpeg binaries not available: %v", err)
	}

	// Test with 5-second clip
	testFile := filepath.Join("..", "..", "data", "tests", "clips", "test-5s.mp3")
	ctx := context.Background()

	metadata, err := ffmpeg.GetMetadata(ctx, testFile)
	if err != nil {
		t.Fatalf("Failed to get metadata: %v", err)
	}

	// Validate basic metadata
	if metadata.Duration <= 0 {
		t.Errorf("Expected positive duration, got %f", metadata.Duration)
	}
	if metadata.Duration < 4 || metadata.Duration > 6 {
		t.Errorf("Expected duration around 5 seconds, got %f", metadata.Duration)
	}
	if metadata.Format == "" {
		t.Errorf("Expected format to be set, got empty string")
	}
	if metadata.SampleRate <= 0 {
		t.Errorf("Expected positive sample rate, got %d", metadata.SampleRate)
	}

	t.Logf("Metadata: Duration=%.2fs, Format=%s, SampleRate=%d, Channels=%d, Bitrate=%d",
		metadata.Duration, metadata.Format, metadata.SampleRate, metadata.Channels, metadata.Bitrate)
}

// Test waveform generation with real audio file
func TestGenerateWaveformWithRealAudio(t *testing.T) {
	ffmpeg := New("ffmpeg", "ffprobe", 30*time.Second)

	// Skip if binaries not available
	if err := ffmpeg.ValidateBinaries(); err != nil {
		t.Skipf("FFmpeg binaries not available: %v", err)
	}

	testFile := filepath.Join("..", "..", "data", "tests", "clips", "test-5s.mp3")
	ctx := context.Background()

	// Test with small resolution for quick test
	opts := ProcessingOptions{
		WaveformResolution: 100,
		MaxDuration:        1 * time.Minute,
		TempDir:            "/tmp",
	}

	waveform, err := ffmpeg.GenerateWaveform(ctx, testFile, opts)
	if err != nil {
		t.Fatalf("Failed to generate waveform: %v", err)
	}

	// Validate waveform data
	if len(waveform.Peaks) == 0 {
		t.Fatalf("Expected peaks data, got empty slice")
	}
	if len(waveform.Peaks) != waveform.Resolution {
		t.Errorf("Expected %d peaks, got %d", waveform.Resolution, len(waveform.Peaks))
	}
	if waveform.Duration <= 0 {
		t.Errorf("Expected positive duration, got %f", waveform.Duration)
	}
	if waveform.SampleRate <= 0 {
		t.Errorf("Expected positive sample rate, got %d", waveform.SampleRate)
	}

	// Validate peak values are in expected range [0.0, 1.0]
	for i, peak := range waveform.Peaks {
		if peak < 0 || peak > 1 {
			t.Errorf("Peak %d out of range [0,1]: %f", i, peak)
		}
	}

	t.Logf("Waveform: %d peaks, Duration=%.2fs, SampleRate=%d",
		len(waveform.Peaks), waveform.Duration, waveform.SampleRate)
}

// Test waveform generation with different resolutions
func TestGenerateWaveformResolutions(t *testing.T) {
	ffmpeg := New("ffmpeg", "ffprobe", 30*time.Second)

	// Skip if binaries not available
	if err := ffmpeg.ValidateBinaries(); err != nil {
		t.Skipf("FFmpeg binaries not available: %v", err)
	}

	testFile := filepath.Join("..", "..", "data", "tests", "clips", "test-5s.mp3")
	ctx := context.Background()

	resolutions := []int{50, 100, 200}

	for _, resolution := range resolutions {
		t.Run(fmt.Sprintf("resolution_%d", resolution), func(t *testing.T) {
			opts := ProcessingOptions{
				WaveformResolution: resolution,
				MaxDuration:        1 * time.Minute,
				TempDir:            "/tmp",
			}

			waveform, err := ffmpeg.GenerateWaveform(ctx, testFile, opts)
			if err != nil {
				t.Fatalf("Failed to generate waveform with resolution %d: %v", resolution, err)
			}

			if len(waveform.Peaks) != resolution {
				t.Errorf("Expected %d peaks for resolution %d, got %d",
					resolution, resolution, len(waveform.Peaks))
			}

			// Should have some non-zero peaks for audio with content
			nonZeroPeaks := 0
			for _, peak := range waveform.Peaks {
				if peak > 0 {
					nonZeroPeaks++
				}
			}

			if nonZeroPeaks == 0 {
				t.Errorf("Expected some non-zero peaks, got all zeros")
			}
		})
	}
}

// Test audio file validation
func TestValidateAudioFile(t *testing.T) {
	ffmpeg := New("ffmpeg", "ffprobe", 30*time.Second)

	// Skip if binaries not available
	if err := ffmpeg.ValidateBinaries(); err != nil {
		t.Skipf("FFmpeg binaries not available: %v", err)
	}

	testFile := filepath.Join("..", "..", "data", "tests", "clips", "test-5s.mp3")
	ctx := context.Background()

	err := ffmpeg.ValidateAudioFile(ctx, testFile)
	if err != nil {
		t.Errorf("Expected valid audio file, got error: %v", err)
	}
}

// Test waveform generation with 30-second test clip
func TestGenerateWaveformWith30sClip(t *testing.T) {
	ffmpeg := New("ffmpeg", "ffprobe", 30*time.Second)

	// Skip if binaries not available
	if err := ffmpeg.ValidateBinaries(); err != nil {
		t.Skipf("FFmpeg binaries not available: %v", err)
	}

	testFile := filepath.Join("..", "..", "data", "tests", "clips", "test-30s.mp3")
	ctx := context.Background()

	// Test with medium resolution for 30s clip
	opts := ProcessingOptions{
		WaveformResolution: 300, // 10 peaks per second for 30s = good detail
		MaxDuration:        1 * time.Minute,
		TempDir:            "/tmp",
	}

	waveform, err := ffmpeg.GenerateWaveform(ctx, testFile, opts)
	if err != nil {
		t.Fatalf("Failed to generate waveform from 30s clip: %v", err)
	}

	// Validate waveform data
	if len(waveform.Peaks) == 0 {
		t.Fatalf("Expected peaks data, got empty slice")
	}
	if len(waveform.Peaks) != waveform.Resolution {
		t.Errorf("Expected %d peaks, got %d", waveform.Resolution, len(waveform.Peaks))
	}
	if waveform.Duration <= 0 {
		t.Errorf("Expected positive duration, got %f", waveform.Duration)
	}
	// Should be around 30 seconds
	if waveform.Duration < 25 || waveform.Duration > 35 {
		t.Errorf("Expected duration around 30 seconds, got %f", waveform.Duration)
	}
	if waveform.SampleRate <= 0 {
		t.Errorf("Expected positive sample rate, got %d", waveform.SampleRate)
	}

	// Validate peak values are in expected range [0.0, 1.0]
	for i, peak := range waveform.Peaks {
		if peak < 0 || peak > 1 {
			t.Errorf("Peak %d out of range [0,1]: %f", i, peak)
		}
	}

	// Should have some variation in peaks for real audio
	nonZeroPeaks := 0
	for _, peak := range waveform.Peaks {
		if peak > 0.01 { // Small threshold to account for quiet sections
			nonZeroPeaks++
		}
	}

	if nonZeroPeaks == 0 {
		t.Errorf("Expected some non-zero peaks, got all near-zero values")
	}

	t.Logf("30s Waveform: %d peaks, Duration=%.2fs, SampleRate=%d, NonZeroPeaks=%d",
		len(waveform.Peaks), waveform.Duration, waveform.SampleRate, nonZeroPeaks)
}

// Test error handling for non-existent file
func TestGetMetadataFileNotFound(t *testing.T) {
	ffmpeg := New("ffmpeg", "ffprobe", 30*time.Second)

	// Skip if binaries not available
	if err := ffmpeg.ValidateBinaries(); err != nil {
		t.Skipf("FFmpeg binaries not available: %v", err)
	}

	ctx := context.Background()

	_, err := ffmpeg.GetMetadata(ctx, "/nonexistent/file.mp3")
	if err == nil {
		t.Errorf("Expected error for non-existent file, got nil")
	}

	// Should be a ProcessingError
	var procErr *ProcessingError
	if !errors.As(err, &procErr) {
		t.Errorf("Expected ProcessingError, got %T", err)
	}
}
