package ffmpeg

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// FFmpeg wraps ffmpeg and ffprobe functionality
type FFmpeg struct {
	ffmpegPath  string
	ffprobePath string
	timeout     time.Duration
}

// New creates a new FFmpeg instance
func New(ffmpegPath, ffprobePath string, timeout time.Duration) *FFmpeg {
	return &FFmpeg{
		ffmpegPath:  ffmpegPath,
		ffprobePath: ffprobePath,
		timeout:     timeout,
	}
}

// ValidateBinaries checks if ffmpeg and ffprobe are available
func (f *FFmpeg) ValidateBinaries() error {
	// Check ffmpeg
	if _, err := exec.LookPath(f.ffmpegPath); err != nil {
		return fmt.Errorf("%w: %s", ErrFFmpegNotFound, f.ffmpegPath)
	}

	// Check ffprobe
	if _, err := exec.LookPath(f.ffprobePath); err != nil {
		return fmt.Errorf("%w: %s", ErrFFprobeNotFound, f.ffprobePath)
	}

	return nil
}

// GenerateWaveform generates waveform data from an audio file or URL
func (f *FFmpeg) GenerateWaveform(ctx context.Context, input string, options ProcessingOptions) (*WaveformData, error) {
	// Download file if input is a URL
	var inputFile string
	var cleanup func() error

	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		tempFile, cleanupFunc, err := f.downloadToTemp(ctx, input, options.TempDir)
		if err != nil {
			return nil, err
		}
		inputFile = tempFile
		cleanup = cleanupFunc
	} else {
		inputFile = input
		cleanup = func() error { return nil }
	}
	defer func() {
		if cleanupErr := cleanup(); cleanupErr != nil {
			// Log cleanup error but don't override the main error
			log.Printf("Failed to cleanup temporary file: %v", cleanupErr)
		}
	}()

	// Validate the audio file
	if err := f.ValidateAudioFile(ctx, inputFile); err != nil {
		return nil, err
	}

	// Get metadata for duration and sample rate
	metadata, err := f.GetMetadata(ctx, inputFile)
	if err != nil {
		return nil, err
	}

	// Check duration limits
	if options.MaxDuration > 0 && time.Duration(metadata.Duration)*time.Second > options.MaxDuration {
		return nil, fmt.Errorf("%w: duration %.1fs exceeds limit %.1fs",
			ErrAudioTooLong, metadata.Duration, options.MaxDuration.Seconds())
	}

	// Generate waveform peaks using FFmpeg
	peaks, err := f.extractWaveformPeaks(ctx, inputFile, options.WaveformResolution)
	if err != nil {
		return nil, err
	}

	return &WaveformData{
		Peaks:      peaks,
		Duration:   metadata.Duration,
		Resolution: len(peaks),
		SampleRate: metadata.SampleRate,
	}, nil
}

// extractWaveformPeaks extracts waveform peak data using FFmpeg's showwavespic filter
func (f *FFmpeg) extractWaveformPeaks(ctx context.Context, inputFile string, resolution int) ([]float32, error) {
	// Create a temporary output file for the raw audio data
	tempDir := filepath.Dir(inputFile)
	rawFile, err := os.CreateTemp(tempDir, "waveform_*.raw")
	if err != nil {
		return nil, NewProcessingError("temp_file_creation", inputFile, err, "")
	}
	rawPath := rawFile.Name()
	rawFile.Close()
	defer os.Remove(rawPath)

	// Convert to raw PCM data for analysis
	args := []string{
		"-i", inputFile,
		"-f", "f32le", // 32-bit float little-endian
		"-ac", "1", // Convert to mono
		"-ar", "44100", // Resample to 44.1kHz
		"-y", // Overwrite output
		rawPath,
	}

	cmd := exec.CommandContext(ctx, f.ffmpegPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, NewProcessingError("pcm_conversion", inputFile, err, stderr.String())
	}

	// Read and analyze the raw PCM data
	return f.analyzePCMData(rawPath, resolution)
}

// analyzePCMData reads raw PCM data and generates peak values
func (f *FFmpeg) analyzePCMData(rawPath string, resolution int) ([]float32, error) {
	file, err := os.Open(rawPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get file size to calculate total samples
	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}

	totalBytes := stat.Size()
	totalSamples := totalBytes / 4 // 4 bytes per float32 sample
	samplesPerPeak := totalSamples / int64(resolution)

	if samplesPerPeak < 1 {
		samplesPerPeak = 1
	}

	peaks := make([]float32, 0, resolution)
	buffer := make([]byte, 4*samplesPerPeak) // Buffer for samples
	var globalMaxPeak float32

	// First pass: find the global maximum to normalize
	tempPeaks := make([]float32, 0, resolution)
	for i := 0; i < resolution; i++ {
		// Read chunk of samples
		n, err := file.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Convert bytes to float32 samples and find peak
		var maxPeak float32
		for j := 0; j < n; j += 4 {
			if j+4 <= len(buffer) {
				// Convert little-endian bytes to float32
				sample := bytesToFloat32(buffer[j : j+4])
				if abs(sample) > abs(maxPeak) {
					maxPeak = sample
				}
			}
		}

		peakValue := abs(maxPeak)
		tempPeaks = append(tempPeaks, peakValue)
		if peakValue > globalMaxPeak {
			globalMaxPeak = peakValue
		}
	}

	// Second pass: normalize peaks to [0,1] range
	if globalMaxPeak > 0 {
		for _, peak := range tempPeaks {
			normalizedPeak := peak / globalMaxPeak
			peaks = append(peaks, normalizedPeak)
		}
	} else {
		// All silence - just copy the zeros
		peaks = tempPeaks
	}

	return peaks, nil
}

// downloadToTemp downloads a URL to a temporary file
func (f *FFmpeg) downloadToTemp(ctx context.Context, url, tempDir string) (string, func() error, error) {
	// Create temporary file
	tempFile, err := os.CreateTemp(tempDir, "audio_download_*")
	if err != nil {
		return "", nil, NewProcessingError("temp_file_creation", url, err, "")
	}

	cleanup := func() error {
		return os.Remove(tempFile.Name())
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			log.Printf("Failed to cleanup on error: %v", cleanupErr)
		}
		return "", nil, err
	}

	// Set user agent to avoid blocking
	req.Header.Set("User-Agent", "Podcast-Player-API/1.0")

	// Download file
	client := &http.Client{Timeout: f.timeout}
	resp, err := client.Do(req)
	if err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			log.Printf("Failed to cleanup on error: %v", cleanupErr)
		}
		return "", nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if cleanupErr := cleanup(); cleanupErr != nil {
			log.Printf("Failed to cleanup on error: %v", cleanupErr)
		}
		return "", nil, fmt.Errorf("failed to download audio: HTTP %d", resp.StatusCode)
	}

	// Copy response body to temp file
	_, err = io.Copy(tempFile, resp.Body)
	tempFile.Close()
	if err != nil {
		if cleanupErr := cleanup(); cleanupErr != nil {
			log.Printf("Failed to cleanup on error: %v", cleanupErr)
		}
		return "", nil, err
	}

	return tempFile.Name(), cleanup, nil
}

// Helper functions

// bytesToFloat32 converts 4 bytes to a float32 in little-endian format
func bytesToFloat32(b []byte) float32 {
	var f float32
	buf := bytes.NewReader(b)
	if err := binary.Read(buf, binary.LittleEndian, &f); err != nil {
		// If we can't read the bytes, return 0 (silence)
		return 0
	}
	return f
}

// abs returns the absolute value of a float32
func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
