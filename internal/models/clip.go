package models

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Clip status constants
const (
	ClipStatusPending = "pending" // Created, awaiting export
	ClipStatusReady   = "ready"   // Extracted (cached for future exports)
	ClipStatusFailed  = "failed"  // Extraction failed
)

// Clip represents an extracted audio segment with ML label for training
type Clip struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UUID      string    `json:"uuid" gorm:"uniqueIndex;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Episode reference (Podcast Index ID for consistency)
	PodcastIndexEpisodeID int64    `json:"podcast_index_episode_id" gorm:"not null;index"` // Required reference to episode
	Episode               *Episode `json:"episode,omitempty" gorm:"foreignKey:PodcastIndexEpisodeID;references:PodcastIndexID"`

	// Source information
	SourceEpisodeURL  string  `json:"source_episode_url" gorm:"not null;size:500"`
	OriginalStartTime float64 `json:"original_start_time" gorm:"not null"` // Time in seconds
	OriginalEndTime   float64 `json:"original_end_time" gorm:"not null"`   // Time in seconds

	// Flexible label - any string allowed for future extensibility
	Label string `json:"label" gorm:"not null;size:100;index"` // Index for fast filtering by label

	// Autolabel metadata (Phase 2)
	AutoLabeled     bool     `json:"auto_labeled" gorm:"default:false"`                   // Whether this clip was automatically labeled
	LabelConfidence *float64 `json:"label_confidence,omitempty" gorm:"type:decimal(5,4)"` // Confidence score 0.0-1.0 (nullable)
	LabelMethod     string   `json:"label_method" gorm:"size:50;default:manual"`          // How it was labeled: "manual", "peak_detection", etc.

	// Approval workflow (for review before extraction)
	Approved bool `json:"approved" gorm:"default:false;index"` // Whether clip is approved for extraction/dataset

	// Extracted clip information (optional - NULL for auto-detected clips without extraction)
	// ClipFilename is just the filename (e.g., "clip_abc123.wav")
	// The full path is constructed as: {storage_base}/{label}/{filename}
	ClipFilename  *string  `json:"clip_filename,omitempty" gorm:"size:255;uniqueIndex"` // NULL if not extracted
	ClipDuration  *float64 `json:"clip_duration,omitempty"`                             // NULL if not extracted
	ClipSizeBytes *int64   `json:"clip_size_bytes,omitempty"`                           // NULL if not extracted
	Extracted     bool     `json:"extracted" gorm:"default:false;index"`                // Whether audio has been extracted to file

	// Processing status
	Status string `json:"status" gorm:"default:processing;size:20"`

	// Optional error message if processing failed
	ErrorMessage string `json:"error_message,omitempty" gorm:"size:500"`
}

// BeforeCreate generates a UUID before creating a new clip
func (c *Clip) BeforeCreate(tx *gorm.DB) error {
	if c.UUID == "" {
		c.UUID = uuid.New().String()
	}
	if c.Status == "" {
		c.Status = ClipStatusPending
	}
	return nil
}

// TableName returns the table name for the Clip model
func (Clip) TableName() string {
	return "clips"
}

// GetOriginalDuration returns the original segment duration before processing
func (c *Clip) GetOriginalDuration() float64 {
	return c.OriginalEndTime - c.OriginalStartTime
}

// IsReady returns true if the clip is ready for use
func (c *Clip) IsReady() bool {
	return c.Status == "ready"
}

// IsFailed returns true if the clip processing failed
func (c *Clip) IsFailed() bool {
	return c.Status == "failed"
}

// GetRelativePath returns the relative path within the dataset structure
// e.g., "advertisement/clip_abc123.wav"
// Returns empty string if clip hasn't been extracted
func (c *Clip) GetRelativePath() string {
	if c.ClipFilename == nil {
		return ""
	}
	return fmt.Sprintf("%s/%s", c.Label, *c.ClipFilename)
}

// IsExtracted returns true if the clip has been extracted to an audio file
func (c *Clip) IsExtracted() bool {
	return c.Extracted && c.ClipFilename != nil
}

// ClipExport represents the clip data for dataset export
type ClipExport struct {
	FilePath          string   `json:"file_path"`                  // Relative path: "label/filename.wav"
	Label             string   `json:"label"`                      // ML training label
	AutoLabeled       bool     `json:"auto_labeled"`               // Whether this was auto-labeled
	LabelConfidence   *float64 `json:"label_confidence,omitempty"` // Confidence score if auto-labeled
	LabelMethod       string   `json:"label_method"`               // How it was labeled
	Duration          float64  `json:"duration"`                   // Clip duration in seconds
	SourceURL         string   `json:"source_url"`                 // Original episode URL
	OriginalStartTime float64  `json:"original_start_time"`        // Start time in original
	OriginalEndTime   float64  `json:"original_end_time"`          // End time in original
	UUID              string   `json:"uuid"`                       // Stable identifier
	CreatedAt         string   `json:"created_at"`                 // ISO 8601 timestamp
}

// ToExport converts a Clip to its export representation
// Only exports clips that have been extracted (have audio files)
func (c *Clip) ToExport() ClipExport {
	duration := 0.0
	if c.ClipDuration != nil {
		duration = *c.ClipDuration
	}
	return ClipExport{
		FilePath:          c.GetRelativePath(),
		Label:             c.Label,
		AutoLabeled:       c.AutoLabeled,
		LabelConfidence:   c.LabelConfidence,
		LabelMethod:       c.LabelMethod,
		Duration:          duration,
		SourceURL:         c.SourceEpisodeURL,
		OriginalStartTime: c.OriginalStartTime,
		OriginalEndTime:   c.OriginalEndTime,
		UUID:              c.UUID,
		CreatedAt:         c.CreatedAt.Format(time.RFC3339),
	}
}
