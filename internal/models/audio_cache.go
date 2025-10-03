package models

import (
	"time"

	"gorm.io/gorm"
)

// AudioCache represents cached audio files for episodes
type AudioCache struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Episode reference - uses Podcast Index ID for consistency
	PodcastIndexEpisodeID int64 `gorm:"uniqueIndex;not null" json:"podcast_index_episode_id"`

	// Original audio info
	OriginalURL    string `gorm:"not null" json:"original_url"`
	OriginalSHA256 string `gorm:"size:64" json:"original_sha256"`
	OriginalPath   string `json:"original_path"`
	OriginalSize   int64  `json:"original_size"`

	// Processed audio (16kHz mono for ML)
	ProcessedPath   string `json:"processed_path"`
	ProcessedSHA256 string `gorm:"size:64" json:"processed_sha256"`
	ProcessedSize   int64  `json:"processed_size"`

	// Metadata
	DurationSeconds float64   `json:"duration_seconds"`
	SampleRate      int       `json:"sample_rate"`
	CachedAt        time.Time `json:"cached_at"`
	LastUsedAt      time.Time `json:"last_used_at"`

	// No direct relationship - we use Podcast Index ID for lookups
}

// TableName returns the table name for the AudioCache model
func (AudioCache) TableName() string {
	return "audio_cache"
}

// BeforeCreate hook to set timestamps
func (a *AudioCache) BeforeCreate(tx *gorm.DB) error {
	now := time.Now()
	a.CreatedAt = now
	a.UpdatedAt = now
	a.CachedAt = now
	a.LastUsedAt = now
	return nil
}

// BeforeUpdate hook to update timestamp
func (a *AudioCache) BeforeUpdate(tx *gorm.DB) error {
	a.UpdatedAt = time.Now()
	return nil
}

// UpdateLastUsed updates the last used timestamp
func (a *AudioCache) UpdateLastUsed(db *gorm.DB) error {
	a.LastUsedAt = time.Now()
	return db.Model(a).Update("last_used_at", a.LastUsedAt).Error
}
