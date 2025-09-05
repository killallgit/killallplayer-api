package models

import (
	"time"

	"gorm.io/gorm"
)

// Transcription represents a text transcription of an episode
type Transcription struct {
	ID        uint           `gorm:"primarykey" json:"id"`
	EpisodeID uint           `gorm:"uniqueIndex" json:"episode_id"`
	Text      string         `gorm:"type:text" json:"text"`
	Language  string         `json:"language"`
	Model     string         `json:"model"`
	Duration  float64        `json:"duration"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Transcription
func (Transcription) TableName() string {
	return "transcriptions"
}
