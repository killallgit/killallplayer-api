package models

import (
	"time"

	"gorm.io/gorm"
)

// Transcription represents a text transcription of an episode
type Transcription struct {
	ID uint `gorm:"primarykey" json:"id"`

	// Episode reference (using Podcast Index ID for consistency)
	PodcastIndexEpisodeID int64    `gorm:"uniqueIndex;not null" json:"podcast_index_episode_id"`
	Episode               *Episode `json:"episode,omitempty" gorm:"foreignKey:PodcastIndexEpisodeID;references:PodcastIndexID"`

	Text      string         `gorm:"type:text" json:"text"`
	Language  string         `json:"language"`
	Model     string         `json:"model"`
	Duration  float64        `json:"duration"`
	Source    string         `json:"source"`     // "fetched" or "generated"
	SourceURL string         `json:"source_url"` // Original transcript URL if fetched
	Format    string         `json:"format"`     // Original format (vtt, srt, json, text)
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName specifies the table name for Transcription
func (Transcription) TableName() string {
	return "transcriptions"
}
