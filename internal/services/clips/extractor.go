package clips

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// AudioExtractor handles the extraction and processing of audio clips
type AudioExtractor interface {
	ExtractClip(ctx context.Context, params ExtractParams) (*ExtractResult, error)
}

// ExtractParams contains parameters for clip extraction
type ExtractParams struct {
	SourceURL  string  // URL of the source audio
	StartTime  float64 // Start time in seconds
	EndTime    float64 // End time in seconds
	OutputPath string  // Full path where clip should be saved
}

// ExtractResult contains the results of clip extraction
type ExtractResult struct {
	FilePath      string  // Path where clip was saved
	Duration      float64 // Actual duration of the clip
	SizeBytes     int64   // File size in bytes
	SampleRate    int     // Sample rate (should be 16000)
	Channels      int     // Number of channels (should be 1/mono)
	ProcessedPath string  // Path to processed file if different from original
}

// FFmpegExtractor implements AudioExtractor using FFmpeg
type FFmpegExtractor struct {
	ffmpegPath     string
	tempDir        string
	targetDuration float64 // Target duration in seconds (e.g., 15.0)
}

// NewFFmpegExtractor creates a new FFmpeg-based extractor
func NewFFmpegExtractor(tempDir string, targetDuration float64) (*FFmpegExtractor, error) {
	// Check if ffmpeg is available
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	// Ensure temp directory exists
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Default to 15 seconds if not specified
	if targetDuration <= 0 {
		targetDuration = 15.0
	}

	return &FFmpegExtractor{
		ffmpegPath:     ffmpegPath,
		tempDir:        tempDir,
		targetDuration: targetDuration,
	}, nil
}

// ExtractClip extracts and processes an audio clip
func (e *FFmpegExtractor) ExtractClip(ctx context.Context, params ExtractParams) (*ExtractResult, error) {
	// Calculate duration
	duration := params.EndTime - params.StartTime
	if duration <= 0 {
		return nil, fmt.Errorf("invalid time range: start=%f, end=%f", params.StartTime, params.EndTime)
	}

	// Download source audio to temp file if it's a URL
	var sourcePath string
	if strings.HasPrefix(params.SourceURL, "http://") || strings.HasPrefix(params.SourceURL, "https://") {
		tempFile, err := e.downloadToTemp(ctx, params.SourceURL)
		if err != nil {
			return nil, fmt.Errorf("failed to download source audio: %w", err)
		}
		defer os.Remove(tempFile) // Clean up temp file
		sourcePath = tempFile
	} else {
		sourcePath = params.SourceURL
	}

	// Extract the segment and convert to 16kHz mono WAV
	if err := e.extractAndConvert(ctx, sourcePath, params); err != nil {
		return nil, fmt.Errorf("failed to extract clip: %w", err)
	}

	// Apply padding or cropping to reach target duration
	processedPath, actualDuration, err := e.applyTargetDuration(ctx, params.OutputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to apply target duration: %w", err)
	}

	// Get file info
	fileInfo, err := os.Stat(processedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &ExtractResult{
		FilePath:      processedPath,
		Duration:      actualDuration,
		SizeBytes:     fileInfo.Size(),
		SampleRate:    16000, // We always convert to 16kHz
		Channels:      1,     // We always convert to mono
		ProcessedPath: processedPath,
	}, nil
}

// downloadToTemp downloads a URL to a temporary file
func (e *FFmpegExtractor) downloadToTemp(ctx context.Context, url string) (string, error) {
	// Create temp file
	tempFile := filepath.Join(e.tempDir, fmt.Sprintf("download_%d.tmp", time.Now().UnixNano()))

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 5 * time.Minute,
	}

	// Create request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	// Execute request
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Create output file
	out, err := os.Create(tempFile)
	if err != nil {
		return "", err
	}
	defer out.Close()

	// Copy data
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		os.Remove(tempFile)
		return "", err
	}

	return tempFile, nil
}

// extractAndConvert extracts a segment and converts to 16kHz mono WAV
func (e *FFmpegExtractor) extractAndConvert(ctx context.Context, sourcePath string, params ExtractParams) error {
	// Build FFmpeg command
	// -ss: seek to start time (before input for faster seeking)
	// -i: input file
	// -t: duration to extract
	// -ar: audio sample rate (16000 Hz for Whisper/Wav2Vec2)
	// -ac: audio channels (1 for mono)
	// -c:a: audio codec (pcm_s16le for WAV)
	// -f: force format to wav
	duration := params.EndTime - params.StartTime

	args := []string{
		"-ss", fmt.Sprintf("%.3f", params.StartTime), // Seek before input (faster)
		"-i", sourcePath, // Input file
		"-t", fmt.Sprintf("%.3f", duration), // Duration to extract
		"-ar", "16000", // 16kHz sample rate
		"-ac", "1", // Mono
		"-c:a", "pcm_s16le", // PCM 16-bit little-endian
		"-f", "wav", // Output format
		"-y",              // Overwrite output
		params.OutputPath, // Output file
	}

	cmd := exec.CommandContext(ctx, e.ffmpegPath, args...)

	// Capture stderr for debugging
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// applyTargetDuration applies padding or cropping to reach target duration
func (e *FFmpegExtractor) applyTargetDuration(ctx context.Context, inputPath string) (string, float64, error) {
	// Get current duration
	duration, err := e.getAudioDuration(ctx, inputPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to get duration: %w", err)
	}

	// If duration is already close to target (within 0.5 seconds), keep as is
	if duration >= e.targetDuration-0.5 && duration <= e.targetDuration+0.5 {
		return inputPath, duration, nil
	}

	// Create temp file for processed audio
	processedPath := strings.TrimSuffix(inputPath, ".wav") + "_processed.wav"

	if duration < e.targetDuration {
		// Pad with silence to reach target duration
		err = e.padWithSilence(ctx, inputPath, processedPath, e.targetDuration)
	} else {
		// Crop to target duration (take from center)
		startOffset := (duration - e.targetDuration) / 2
		err = e.cropAudio(ctx, inputPath, processedPath, startOffset, e.targetDuration)
	}

	if err != nil {
		return "", 0, err
	}

	// Replace original with processed
	if err := os.Rename(processedPath, inputPath); err != nil {
		return "", 0, fmt.Errorf("failed to replace file: %w", err)
	}

	return inputPath, e.targetDuration, nil
}

// getAudioDuration gets the duration of an audio file
func (e *FFmpegExtractor) getAudioDuration(ctx context.Context, filePath string) (float64, error) {
	args := []string{
		"-i", filePath,
		"-show_entries", "format=duration",
		"-v", "quiet",
		"-of", "csv=p=0",
	}

	// Use ffprobe for getting duration
	ffprobePath := strings.Replace(e.ffmpegPath, "ffmpeg", "ffprobe", 1)
	cmd := exec.CommandContext(ctx, ffprobePath, args...)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var duration float64
	if _, err := fmt.Sscanf(string(output), "%f", &duration); err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}

// padWithSilence pads audio with silence to reach target duration
func (e *FFmpegExtractor) padWithSilence(ctx context.Context, inputPath, outputPath string, targetDuration float64) error {
	// Use FFmpeg's apad filter to add silence
	args := []string{
		"-i", inputPath,
		"-af", fmt.Sprintf("apad=whole_dur=%.3f", targetDuration),
		"-ar", "16000", // Maintain sample rate
		"-ac", "1", // Maintain mono
		"-c:a", "pcm_s16le", // Maintain codec
		"-f", "wav",
		"-y",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, e.ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg pad failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// cropAudio crops audio to target duration
func (e *FFmpegExtractor) cropAudio(ctx context.Context, inputPath, outputPath string, startOffset, duration float64) error {
	args := []string{
		"-ss", fmt.Sprintf("%.3f", startOffset),
		"-i", inputPath,
		"-t", fmt.Sprintf("%.3f", duration),
		"-ar", "16000", // Maintain sample rate
		"-ac", "1", // Maintain mono
		"-c:a", "pcm_s16le", // Maintain codec
		"-f", "wav",
		"-y",
		outputPath,
	}

	cmd := exec.CommandContext(ctx, e.ffmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg crop failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}
