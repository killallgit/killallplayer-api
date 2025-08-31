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

// Episode represents a podcast episode
type Episode struct {
	gorm.Model
	PodcastID   uint      `json:"podcast_id" gorm:"not null"`
	Title       string    `json:"title" gorm:"not null"`
	Description string    `json:"description"`
	AudioURL    string    `json:"audio_url" gorm:"not null"`
	Duration    int       `json:"duration"` // Duration in seconds
	PublishedAt time.Time `json:"published_at"`
	GUID        string    `json:"guid" gorm:"uniqueIndex"`
	Played      bool      `json:"played" gorm:"default:false"`
	Position    int       `json:"position" gorm:"default:0"` // Current playback position in seconds
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
