package models

import (
	"time"

	"gorm.io/gorm"
)

// Podcast represents a podcast feed
type Podcast struct {
	gorm.Model
	Title       string    `json:"title" gorm:"not null"`
	Author      string    `json:"author"`
	Description string    `json:"description"`
	FeedURL     string    `json:"feed_url" gorm:"uniqueIndex;not null"`
	ImageURL    string    `json:"image_url"`
	Language    string    `json:"language"`
	Category    string    `json:"category"`
	Episodes    []Episode `json:"episodes,omitempty" gorm:"foreignKey:PodcastID"`
}

// Episode represents a podcast episode with all Podcast Index fields
type Episode struct {
	gorm.Model
	// Core episode fields
	PodcastID      uint   `json:"podcast_id" gorm:"not null;index"`
	PodcastIndexID int64  `json:"podcast_index_id" gorm:"uniqueIndex"` // Podcast Index episode ID
	Title          string `json:"title" gorm:"not null"`
	Description    string `json:"description" gorm:"type:text"`
	Link           string `json:"link"`
	GUID           string `json:"guid" gorm:"uniqueIndex"`

	// Media information
	AudioURL        string `json:"audio_url" gorm:"not null;column:audio_url"`
	EnclosureType   string `json:"enclosure_type"`
	EnclosureLength int64  `json:"enclosure_length"`
	Duration        *int   `json:"duration"` // Duration in seconds, nullable

	// Timestamps
	PublishedAt time.Time `json:"published_at"`
	DateCrawled time.Time `json:"date_crawled"`

	// Episode metadata
	EpisodeNumber *int   `json:"episode_number"`
	Season        *int   `json:"season"`
	EpisodeType   string `json:"episode_type"` // full, trailer, bonus
	Explicit      int    `json:"explicit"`     // 0=not explicit, 1=explicit
	Image         string `json:"image"`

	// Feed metadata (denormalized for performance)
	FeedTitle    string `json:"feed_title"`
	FeedImage    string `json:"feed_image"`
	FeedLanguage string `json:"feed_language"`
	FeedItunesID *int64 `json:"feed_itunes_id"`

	// Podcast 2.0 features
	ChaptersURL   string `json:"chapters_url"`
	TranscriptURL string `json:"transcript_url"`

	// Playback state (user-specific, should be in separate table for multi-user)
	Played   bool `json:"played" gorm:"default:false"`
	Position int  `json:"position" gorm:"default:0"` // Current playback position in seconds

	// Waveform relationship (one-to-one)
	Waveform *Waveform `json:"waveform,omitempty" gorm:"foreignKey:EpisodeID"`
}

// User represents a user account
type User struct {
	gorm.Model
	Email         string         `json:"email" gorm:"uniqueIndex;not null"`
	Username      string         `json:"username" gorm:"uniqueIndex;not null"`
	PasswordHash  string         `json:"-" gorm:"not null"`
	IsActive      bool           `json:"is_active" gorm:"default:true"`
	Subscriptions []Subscription `json:"subscriptions,omitempty" gorm:"foreignKey:UserID"`
}

// Subscription represents a user's subscription to a podcast
type Subscription struct {
	gorm.Model
	UserID    uint    `json:"user_id" gorm:"not null"`
	PodcastID uint    `json:"podcast_id" gorm:"not null"`
	User      User    `json:"-" gorm:"foreignKey:UserID"`
	Podcast   Podcast `json:"podcast,omitempty" gorm:"foreignKey:PodcastID"`
}

// PlaybackState represents the playback state of an episode for a user
type PlaybackState struct {
	gorm.Model
	UserID    uint    `json:"user_id" gorm:"not null"`
	EpisodeID uint    `json:"episode_id" gorm:"not null"`
	Position  int     `json:"position"` // Current playback position in seconds
	Completed bool    `json:"completed" gorm:"default:false"`
	User      User    `json:"-" gorm:"foreignKey:UserID"`
	Episode   Episode `json:"episode,omitempty" gorm:"foreignKey:EpisodeID"`
}
