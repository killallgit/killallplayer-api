package models

import (
	"gorm.io/gorm"
)

// Annotation represents a labeled time segment of an episode for ML training
type Annotation struct {
	gorm.Model
	EpisodeID uint    `json:"episode_id" gorm:"not null;index"`
	Label     string  `json:"label" gorm:"not null"`
	StartTime float64 `json:"start_time" gorm:"not null"` // Time in seconds
	EndTime   float64 `json:"end_time" gorm:"not null"`   // Time in seconds

	// Relationship
	Episode Episode `json:"episode,omitempty" gorm:"foreignKey:EpisodeID"`
}

// TableName returns the table name for the Annotation model
func (Annotation) TableName() string {
	return "annotations"
}
