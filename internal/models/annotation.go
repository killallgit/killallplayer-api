package models

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Annotation represents a labeled time segment of an episode for ML training
type Annotation struct {
	gorm.Model
	UUID      string  `json:"uuid" gorm:"uniqueIndex"`
	EpisodeID uint    `json:"episode_id" gorm:"not null;index"`
	Label     string  `json:"label" gorm:"not null"`
	StartTime float64 `json:"start_time" gorm:"not null"` // Time in seconds
	EndTime   float64 `json:"end_time" gorm:"not null"`   // Time in seconds

	// Clip extraction fields
	ClipPath   string `json:"clip_path" gorm:""`                  // Path to extracted audio clip
	ClipStatus string `json:"clip_status" gorm:"default:pending"` // pending|processing|ready|failed
	ClipSize   int64  `json:"clip_size" gorm:"default:0"`         // File size in bytes

	// Relationship
	Episode Episode `json:"episode,omitempty" gorm:"foreignKey:EpisodeID"`
}

// BeforeCreate generates a UUID before creating a new annotation
func (a *Annotation) BeforeCreate(tx *gorm.DB) error {
	if a.UUID == "" {
		a.UUID = uuid.New().String()
	}
	return nil
}

// TableName returns the table name for the Annotation model
func (Annotation) TableName() string {
	return "annotations"
}
