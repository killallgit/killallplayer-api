package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// Podcast represents a podcast feed with full Podcast Index metadata
type Podcast struct {
	gorm.Model

	// Primary identifier from Podcast Index
	PodcastIndexID int64 `json:"podcast_index_id" gorm:"uniqueIndex;not null;index"`

	// Core metadata
	Title       string `json:"title" gorm:"not null;index"`
	Author      string `json:"author" gorm:"index"`
	OwnerName   string `json:"owner_name"`
	Description string `json:"description" gorm:"type:text"`

	// URLs
	FeedURL     string `json:"feed_url" gorm:"uniqueIndex;not null"`
	OriginalURL string `json:"original_url"`
	Link        string `json:"link"`
	Image       string `json:"image"`
	Artwork     string `json:"artwork"`

	// Identifiers & Discovery
	ITunesID *int64 `json:"itunes_id" gorm:"index"`
	Language string `json:"language" gorm:"index"`

	// Categories stored as JSON map
	Categories datatypes.JSON `json:"categories" gorm:"type:json"`

	// Metrics
	EpisodeCount int `json:"episode_count" gorm:"default:0"`

	// Podcast Index metadata
	LastUpdateTime   *time.Time `json:"last_update_time"`
	LastCrawlTime    *time.Time `json:"last_crawl_time"`
	LastParseTime    *time.Time `json:"last_parse_time"`
	LastGoodHTTPCode int        `json:"last_good_http_code" gorm:"default:0"`
	ImageURLHash     int64      `json:"image_url_hash" gorm:"default:0"`

	// Status flags
	Locked      int    `json:"locked" gorm:"default:0"`
	Dead        int    `json:"dead" gorm:"default:0"`
	DuplicateOf *int64 `json:"duplicate_of"`

	// Local tracking metadata
	LastFetchedAt *time.Time `json:"last_fetched_at" gorm:"index"`
	FetchCount    int        `json:"fetch_count" gorm:"default:0"`

	// Relationships
	Episodes      []Episode      `json:"episodes,omitempty" gorm:"foreignKey:PodcastID;constraint:OnDelete:CASCADE"`
	Subscriptions []Subscription `json:"-" gorm:"foreignKey:PodcastID;constraint:OnDelete:CASCADE"`
}

// Episode represents a podcast episode with all Podcast Index fields
type Episode struct {
	gorm.Model

	// Foreign key to Podcast table (proper relationship)
	PodcastID uint     `json:"podcast_id" gorm:"not null;index;constraint:OnDelete:CASCADE"`
	Podcast   *Podcast `json:"podcast,omitempty" gorm:"foreignKey:PodcastID"`

	// Podcast Index identifiers
	PodcastIndexID     int64 `json:"podcast_index_id" gorm:"uniqueIndex;not null;index"`
	PodcastIndexFeedID int64 `json:"podcast_index_feed_id" gorm:"not null;index"` // For fast queries by feed

	// Core fields
	Title       string `json:"title" gorm:"not null"`
	Description string `json:"description" gorm:"type:text"`
	Link        string `json:"link"`
	GUID        string `json:"guid" gorm:"uniqueIndex;not null"`

	// Media information
	AudioURL        string `json:"audio_url" gorm:"not null"`
	EnclosureType   string `json:"enclosure_type"`
	EnclosureLength int64  `json:"enclosure_length"`
	Duration        *int   `json:"duration"` // Duration in seconds, nullable

	// Timestamps
	PublishedAt time.Time `json:"published_at" gorm:"index"`
	DateCrawled time.Time `json:"date_crawled"`

	// Episode metadata
	EpisodeNumber *int   `json:"episode_number"`
	Season        *int   `json:"season"`
	EpisodeType   string `json:"episode_type"` // full, trailer, bonus
	Explicit      int    `json:"explicit"`     // 0=not explicit, 1=explicit
	Image         string `json:"image"`

	// Feed metadata (denormalized for performance - synced from Podcast)
	FeedTitle    string `json:"feed_title"`
	FeedImage    string `json:"feed_image"`
	FeedLanguage string `json:"feed_language"`
	FeedItunesID *int64 `json:"feed_itunes_id"`

	// Podcast 2.0 features
	ChaptersURL   string `json:"chapters_url"`
	TranscriptURL string `json:"transcript_url"`

	// Relationships (all use Podcast Index IDs for consistency)
	Waveform      *Waveform      `json:"waveform,omitempty" gorm:"foreignKey:PodcastIndexEpisodeID;references:PodcastIndexID"`
	AudioCache    *AudioCache    `json:"audio_cache,omitempty" gorm:"foreignKey:PodcastIndexEpisodeID;references:PodcastIndexID"`
	Clips         []Clip         `json:"clips,omitempty" gorm:"foreignKey:PodcastIndexEpisodeID;references:PodcastIndexID"`
	Transcription *Transcription `json:"transcription,omitempty" gorm:"foreignKey:PodcastIndexEpisodeID;references:PodcastIndexID"`
}

// Subscription represents a user's subscription to a podcast
// Note: UserID now references Supabase user UUID
type Subscription struct {
	gorm.Model
	UserID    string  `json:"user_id" gorm:"not null;size:36;index"` // Supabase UUID
	PodcastID uint    `json:"podcast_id" gorm:"not null"`
	Podcast   Podcast `json:"podcast,omitempty" gorm:"foreignKey:PodcastID"`
}
