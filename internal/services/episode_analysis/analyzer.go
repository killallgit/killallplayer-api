package episodeanalysis

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// VolumeSpike represents a detected volume anomaly
type VolumeSpike struct {
	StartTime float64 // Start time in seconds
	EndTime   float64 // End time in seconds
	PeakDB    float64 // Peak volume in dB
}

// VolumeAnalyzer scans audio files for volume spikes
type VolumeAnalyzer struct {
	thresholdDB float64 // dB above baseline to consider a spike
	minDuration float64 // Minimum spike duration in seconds
	segmentSize float64 // Segment size for analysis in seconds
}

// NewVolumeAnalyzer creates a new analyzer with default settings
func NewVolumeAnalyzer() *VolumeAnalyzer {
	return &VolumeAnalyzer{
		thresholdDB: 20.0, // 20dB above baseline - catches exceptional spikes only
		minDuration: 5.0,  // 5 second minimum
		segmentSize: 5.0,  // Analyze in 5-second chunks
	}
}

// FindSpikes analyzes an audio file and returns detected volume spikes
func (a *VolumeAnalyzer) FindSpikes(ctx context.Context, audioPath string) ([]VolumeSpike, error) {
	log.Printf("[DEBUG] Analyzing audio file for volume spikes: %s", audioPath)

	// First, get the duration of the audio file
	duration, err := a.getAudioDuration(ctx, audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio duration: %w", err)
	}

	log.Printf("[DEBUG] Audio duration: %.2f seconds, analyzing in %.2f second segments", duration, a.segmentSize)

	// Analyze audio in segments
	segments, err := a.analyzeSegments(ctx, audioPath, duration)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze segments: %w", err)
	}

	// Calculate baseline volume (median)
	baseline := a.calculateBaseline(segments)
	log.Printf("[DEBUG] Baseline volume: %.2f dB, threshold: %.2f dB", baseline, baseline+a.thresholdDB)

	// Find segments that exceed threshold
	spikes := a.detectSpikes(segments, baseline)

	// Merge adjacent spike segments
	merged := a.mergeAdjacentSpikes(spikes)

	// Filter by minimum duration
	filtered := a.filterByDuration(merged)

	log.Printf("[DEBUG] Found %d volume spikes (baseline: %.2f dB, threshold: +%.2f dB)", len(filtered), baseline, a.thresholdDB)

	return filtered, nil
}

// segmentVolume represents volume stats for a time segment
type segmentVolume struct {
	startTime  float64
	endTime    float64
	meanVolume float64
	maxVolume  float64
}

// analyzeSegments analyzes audio in time segments
func (a *VolumeAnalyzer) analyzeSegments(ctx context.Context, audioPath string, duration float64) ([]segmentVolume, error) {
	var segments []segmentVolume

	// Analyze entire file at once with volumedetect
	// This is simpler than chunking for the initial version
	meanVol, maxVol, err := a.getVolumeStats(ctx, audioPath)
	if err != nil {
		return nil, err
	}

	// For now, create segments by analyzing the whole file
	// and assuming relatively consistent volume distribution
	// In a more sophisticated version, we'd analyze chunks
	numSegments := int(duration/a.segmentSize) + 1

	for i := 0; i < numSegments; i++ {
		startTime := float64(i) * a.segmentSize
		endTime := startTime + a.segmentSize
		if endTime > duration {
			endTime = duration
		}

		// Analyze this specific segment
		segMean, segMax, err := a.getSegmentVolume(ctx, audioPath, startTime, endTime)
		if err != nil {
			log.Printf("[WARN] Failed to analyze segment %.2f-%.2f: %v", startTime, endTime, err)
			// Use overall stats as fallback
			segMean = meanVol
			segMax = maxVol
		}

		segments = append(segments, segmentVolume{
			startTime:  startTime,
			endTime:    endTime,
			meanVolume: segMean,
			maxVolume:  segMax,
		})
	}

	return segments, nil
}

// getSegmentVolume analyzes a specific time segment
func (a *VolumeAnalyzer) getSegmentVolume(ctx context.Context, audioPath string, startTime, endTime float64) (float64, float64, error) {
	duration := endTime - startTime

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", fmt.Sprintf("%.2f", startTime),
		"-t", fmt.Sprintf("%.2f", duration),
		"-i", audioPath,
		"-af", "volumedetect",
		"-f", "null",
		"-",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("ffmpeg failed: %w", err)
	}

	return a.parseVolumeOutput(string(output))
}

// getVolumeStats gets overall volume statistics for the entire file
func (a *VolumeAnalyzer) getVolumeStats(ctx context.Context, audioPath string) (float64, float64, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", audioPath,
		"-af", "volumedetect",
		"-f", "null",
		"-",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return 0, 0, fmt.Errorf("ffmpeg failed: %w", err)
	}

	return a.parseVolumeOutput(string(output))
}

// parseVolumeOutput extracts volume statistics from FFmpeg output
func (a *VolumeAnalyzer) parseVolumeOutput(output string) (float64, float64, error) {
	meanVolumeRegex := regexp.MustCompile(`mean_volume:\s*([-\d.]+)\s*dB`)
	maxVolumeRegex := regexp.MustCompile(`max_volume:\s*([-\d.]+)\s*dB`)

	var meanVolume, maxVolume float64

	if matches := meanVolumeRegex.FindStringSubmatch(output); len(matches) > 1 {
		if val, err := strconv.ParseFloat(matches[1], 64); err == nil {
			meanVolume = val
		}
	}

	if matches := maxVolumeRegex.FindStringSubmatch(output); len(matches) > 1 {
		if val, err := strconv.ParseFloat(matches[1], 64); err == nil {
			maxVolume = val
		}
	}

	if meanVolume == 0 && maxVolume == 0 {
		return 0, 0, fmt.Errorf("no volume data found in ffmpeg output")
	}

	return meanVolume, maxVolume, nil
}

// getAudioDuration gets the duration of an audio file
func (a *VolumeAnalyzer) getAudioDuration(ctx context.Context, audioPath string) (float64, error) {
	cmd := exec.CommandContext(ctx, "ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		audioPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}

// calculateBaseline calculates the baseline volume (median)
func (a *VolumeAnalyzer) calculateBaseline(segments []segmentVolume) float64 {
	if len(segments) == 0 {
		return -20.0 // Default baseline
	}

	// Calculate median of mean volumes
	var volumes []float64
	for _, seg := range segments {
		volumes = append(volumes, seg.meanVolume)
	}

	// Simple median calculation
	if len(volumes) == 0 {
		return -20.0
	}

	sum := 0.0
	for _, v := range volumes {
		sum += v
	}

	return sum / float64(len(volumes))
}

// detectSpikes finds segments that exceed the threshold
func (a *VolumeAnalyzer) detectSpikes(segments []segmentVolume, baseline float64) []VolumeSpike {
	var spikes []VolumeSpike
	threshold := baseline + a.thresholdDB

	for _, seg := range segments {
		// Check if this segment exceeds threshold
		if seg.meanVolume > threshold || seg.maxVolume > threshold+5.0 {
			spikes = append(spikes, VolumeSpike{
				StartTime: seg.startTime,
				EndTime:   seg.endTime,
				PeakDB:    seg.maxVolume,
			})
		}
	}

	return spikes
}

// mergeAdjacentSpikes merges spike segments that are adjacent or overlapping
func (a *VolumeAnalyzer) mergeAdjacentSpikes(spikes []VolumeSpike) []VolumeSpike {
	if len(spikes) == 0 {
		return spikes
	}

	var merged []VolumeSpike
	current := spikes[0]

	for i := 1; i < len(spikes); i++ {
		next := spikes[i]

		// If next spike starts within 1 second of current ending, merge them
		if next.StartTime-current.EndTime <= 1.0 {
			current.EndTime = next.EndTime
			if next.PeakDB > current.PeakDB {
				current.PeakDB = next.PeakDB
			}
		} else {
			merged = append(merged, current)
			current = next
		}
	}

	merged = append(merged, current)
	return merged
}

// filterByDuration removes spikes shorter than minimum duration
func (a *VolumeAnalyzer) filterByDuration(spikes []VolumeSpike) []VolumeSpike {
	var filtered []VolumeSpike

	for _, spike := range spikes {
		duration := spike.EndTime - spike.StartTime
		if duration >= a.minDuration {
			filtered = append(filtered, spike)
		}
	}

	return filtered
}
