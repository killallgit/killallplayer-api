package autolabel

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// PeakDetector analyzes audio files to detect volume characteristics
type PeakDetector interface {
	// DetectPeaks analyzes an audio file and returns volume statistics
	DetectPeaks(ctx context.Context, audioPath string) (*VolumeStats, error)
}

// VolumeStats contains volume analysis results
type VolumeStats struct {
	MeanVolume      float64 // Mean volume in dB
	MaxVolume       float64 // Maximum volume in dB
	PeakCount       int     // Number of significant peaks detected
	SilenceDuration float64 // Duration of silence in seconds
}

// FFmpegPeakDetector implements PeakDetector using FFmpeg
type FFmpegPeakDetector struct {
	ffmpegPath string
}

// NewFFmpegPeakDetector creates a new FFmpeg-based peak detector
func NewFFmpegPeakDetector(ffmpegPath string) PeakDetector {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg" // Use system PATH
	}
	return &FFmpegPeakDetector{
		ffmpegPath: ffmpegPath,
	}
}

// DetectPeaks uses FFmpeg's volumedetect and silencedetect filters
func (d *FFmpegPeakDetector) DetectPeaks(ctx context.Context, audioPath string) (*VolumeStats, error) {
	// Run FFmpeg with volumedetect filter
	// Command: ffmpeg -i input.wav -af volumedetect -f null -
	cmd := exec.CommandContext(ctx, d.ffmpegPath,
		"-i", audioPath,
		"-af", "volumedetect,silencedetect=n=-50dB:d=0.5",
		"-f", "null",
		"-",
	)

	// FFmpeg writes volumedetect output to stderr
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg volumedetect failed: %w, output: %s", err, string(output))
	}

	// Parse the output to extract volume statistics
	stats, err := d.parseVolumeOutput(string(output))
	if err != nil {
		return nil, fmt.Errorf("failed to parse volume output: %w", err)
	}

	return stats, nil
}

// parseVolumeOutput extracts volume statistics from FFmpeg output
func (d *FFmpegPeakDetector) parseVolumeOutput(output string) (*VolumeStats, error) {
	stats := &VolumeStats{
		PeakCount:       0,
		SilenceDuration: 0.0,
	}

	// Regex patterns for volumedetect output
	// Example: [Parsed_volumedetect_0 @ 0x...] mean_volume: -20.5 dB
	// Example: [Parsed_volumedetect_0 @ 0x...] max_volume: -5.2 dB
	meanVolumeRegex := regexp.MustCompile(`mean_volume:\s*([-\d.]+)\s*dB`)
	maxVolumeRegex := regexp.MustCompile(`max_volume:\s*([-\d.]+)\s*dB`)

	// Regex for silencedetect
	// Example: [silencedetect @ 0x...] silence_duration: 2.5
	silenceDurationRegex := regexp.MustCompile(`silence_duration:\s*([\d.]+)`)

	// Extract mean volume
	if matches := meanVolumeRegex.FindStringSubmatch(output); len(matches) > 1 {
		if val, err := strconv.ParseFloat(matches[1], 64); err == nil {
			stats.MeanVolume = val
		}
	}

	// Extract max volume
	if matches := maxVolumeRegex.FindStringSubmatch(output); len(matches) > 1 {
		if val, err := strconv.ParseFloat(matches[1], 64); err == nil {
			stats.MaxVolume = val
		}
	}

	// Extract all silence durations and sum them
	silenceMatches := silenceDurationRegex.FindAllStringSubmatch(output, -1)
	totalSilence := 0.0
	for _, match := range silenceMatches {
		if len(match) > 1 {
			if val, err := strconv.ParseFloat(match[1], 64); err == nil {
				totalSilence += val
			}
		}
	}
	stats.SilenceDuration = totalSilence

	// Count peaks (simplified heuristic based on volume range)
	// A peak is roughly estimated by the difference between max and mean
	// This is a placeholder - more sophisticated peak detection could be added
	volumeRange := stats.MaxVolume - stats.MeanVolume
	if volumeRange > 20.0 {
		stats.PeakCount = int(volumeRange / 4.0) // Rough estimate
	} else {
		stats.PeakCount = 1
	}

	// Validate that we got some data
	if stats.MeanVolume == 0 && stats.MaxVolume == 0 {
		return nil, fmt.Errorf("no volume data found in ffmpeg output")
	}

	return stats, nil
}

// MockPeakDetector is a mock implementation for testing
type MockPeakDetector struct {
	Stats *VolumeStats
	Err   error
}

// DetectPeaks returns the mock stats
func (m *MockPeakDetector) DetectPeaks(ctx context.Context, audioPath string) (*VolumeStats, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if m.Stats == nil {
		// Return default stats if none provided
		return &VolumeStats{
			MeanVolume:      -20.0,
			MaxVolume:       -5.0,
			PeakCount:       8,
			SilenceDuration: 0.5,
		}, nil
	}
	return m.Stats, nil
}

// ValidateFFmpegAvailable checks if FFmpeg is available on the system
func ValidateFFmpegAvailable(ffmpegPath string) error {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}

	cmd := exec.Command(ffmpegPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg not available: %w", err)
	}

	// Check that it's actually FFmpeg
	if !strings.Contains(string(output), "ffmpeg version") {
		return fmt.Errorf("invalid ffmpeg binary")
	}

	return nil
}
