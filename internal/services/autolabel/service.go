package autolabel

import (
	"context"
	"fmt"

	"github.com/killallgit/player-api/internal/models"
	"gorm.io/gorm"
)

// Service defines the interface for autolabeling clips
type Service interface {
	// AutoLabelClip analyzes a clip and assigns a label based on audio characteristics
	AutoLabelClip(ctx context.Context, clipPath string) (*AutoLabelResult, error)

	// UpdateClipWithAutoLabel updates a clip in the database with autolabel metadata
	UpdateClipWithAutoLabel(ctx context.Context, clipUUID string, result *AutoLabelResult) error
}

// AutoLabelResult contains the results of autolabeling
type AutoLabelResult struct {
	Label      string   // Assigned label (e.g., "music", "speech", "silence", "advertisement")
	Confidence float64  // Confidence score 0.0-1.0
	Method     string   // Detection method used (e.g., "peak_detection")
	Metadata   Metadata // Additional detection metadata
}

// Metadata contains detailed detection information
type Metadata struct {
	MeanVolume      float64 `json:"mean_volume"`      // Mean volume in dB
	MaxVolume       float64 `json:"max_volume"`       // Max volume in dB
	PeakCount       int     `json:"peak_count"`       // Number of audio peaks detected
	SilenceDuration float64 `json:"silence_duration"` // Duration of silence in seconds
}

// ServiceImpl implements the Service interface
type ServiceImpl struct {
	db           *gorm.DB
	peakDetector PeakDetector
}

// NewService creates a new autolabel service
func NewService(db *gorm.DB, peakDetector PeakDetector) Service {
	return &ServiceImpl{
		db:           db,
		peakDetector: peakDetector,
	}
}

// AutoLabelClip analyzes a clip and assigns a label
func (s *ServiceImpl) AutoLabelClip(ctx context.Context, clipPath string) (*AutoLabelResult, error) {
	// Use peak detector to analyze audio
	volumeStats, err := s.peakDetector.DetectPeaks(ctx, clipPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect peaks: %w", err)
	}

	// Apply labeling heuristics based on volume statistics
	result := s.classifyAudio(volumeStats)

	return result, nil
}

// UpdateClipWithAutoLabel updates a clip with autolabel metadata
func (s *ServiceImpl) UpdateClipWithAutoLabel(ctx context.Context, clipUUID string, result *AutoLabelResult) error {
	var clip models.Clip

	// Find clip by UUID
	if err := s.db.Where("uuid = ?", clipUUID).First(&clip).Error; err != nil {
		return fmt.Errorf("failed to find clip: %w", err)
	}

	// Update autolabel fields
	clip.AutoLabeled = true
	clip.LabelConfidence = &result.Confidence
	clip.LabelMethod = result.Method
	clip.Label = result.Label

	// Save to database
	if err := s.db.Save(&clip).Error; err != nil {
		return fmt.Errorf("failed to update clip: %w", err)
	}

	return nil
}

// classifyAudio applies heuristics to classify audio based on volume statistics
func (s *ServiceImpl) classifyAudio(stats *VolumeStats) *AutoLabelResult {
	result := &AutoLabelResult{
		Method: "peak_detection",
		Metadata: Metadata{
			MeanVolume:      stats.MeanVolume,
			MaxVolume:       stats.MaxVolume,
			PeakCount:       stats.PeakCount,
			SilenceDuration: stats.SilenceDuration,
		},
	}

	// Heuristic 1: Silence detection
	// If most of the clip is silence (low mean volume or high silence duration)
	if stats.MeanVolume < -40.0 || stats.SilenceDuration > 4.0 {
		result.Label = "silence"
		result.Confidence = 0.85
		return result
	}

	// Heuristic 2: Music detection
	// Music typically has consistent volume with moderate peaks
	if stats.PeakCount >= 5 && stats.MeanVolume > -20.0 {
		result.Label = "music"
		result.Confidence = 0.75
		return result
	}

	// Heuristic 3: Advertisement detection
	// Ads often have high, consistent volume (loud)
	if stats.MeanVolume > -10.0 && stats.MaxVolume > -5.0 {
		result.Label = "advertisement"
		result.Confidence = 0.70
		return result
	}

	// Default: Speech
	// Normal speech patterns
	result.Label = "speech"
	result.Confidence = 0.65

	return result
}
