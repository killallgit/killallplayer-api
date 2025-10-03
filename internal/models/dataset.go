package models

import (
	"time"

	"gorm.io/gorm"
)

// Dataset represents a generated ML dataset
type Dataset struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Basic info
	Name        string `gorm:"not null;size:255" json:"name"`
	Description string `gorm:"type:text" json:"description"`
	Label       string `gorm:"not null;size:100" json:"label"` // e.g., "advertisement"

	// Format info
	Format      string `gorm:"not null;size:50" json:"format"`       // "jsonl" or "audiofolder"
	AudioFormat string `gorm:"not null;size:50" json:"audio_format"` // "original" or "processed"

	// Statistics
	TotalSamples    int     `json:"total_samples"`
	TotalDuration   float64 `json:"total_duration_seconds"`
	AverageDuration float64 `json:"average_duration_seconds"`
	TotalSize       int64   `json:"total_size_bytes"`

	// File paths
	DatasetPath  string `gorm:"not null;size:500" json:"dataset_path"` // Path to JSONL or directory
	MetadataPath string `gorm:"size:500" json:"metadata_path"`         // Path to metadata file

	// Generation info
	GenerationTimeMs int64  `json:"generation_time_ms"`                       // Generation time in milliseconds
	FiltersJSON      string `gorm:"type:text" json:"filters_json,omitempty"`  // JSON-encoded filters
	MetadataJSON     string `gorm:"type:text" json:"metadata_json,omitempty"` // JSON-encoded metadata
}

// TableName returns the table name for the Dataset model
func (Dataset) TableName() string {
	return "datasets"
}

// BeforeCreate hook to set timestamps and generate ID
func (d *Dataset) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	d.CreatedAt = now
	d.UpdatedAt = now

	// Generate ID if not set
	if d.ID == "" {
		d.ID = generateDatasetID()
	}

	return nil
}

// BeforeUpdate hook to update timestamp
func (d *Dataset) BeforeUpdate(tx *gorm.DB) error {
	d.UpdatedAt = time.Now()
	return nil
}

// generateDatasetID generates a unique dataset ID
func generateDatasetID() string {
	// Use timestamp + random suffix for unique ID
	timestamp := time.Now().Format("20060102-150405")
	return "ds-" + timestamp
}
