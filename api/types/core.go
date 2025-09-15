package types

import "time"

// Core data types used across API responses

// Podcast represents a simplified podcast with essential fields
type Podcast struct {
	ID           int64    `json:"id"` // Podcast Index ID
	Title        string   `json:"title"`
	Author       string   `json:"author"`
	Description  string   `json:"description"`
	Link         string   `json:"link,omitempty"` // Podcast website URL
	Image        string   `json:"image"`
	FeedURL      string   `json:"feedUrl"`
	ITunesID     int64    `json:"itunesId,omitempty"`
	Language     string   `json:"language,omitempty"`
	Categories   []string `json:"categories,omitempty"`
	EpisodeCount int      `json:"episodeCount,omitempty"`
	LastUpdated  int64    `json:"lastUpdated,omitempty"` // Unix timestamp
}

// Episode represents a simplified episode with essential fields
type Episode struct {
	ID            int64  `json:"id"`        // Podcast Index Episode ID
	PodcastID     int64  `json:"podcastId"` // Podcast Index Podcast ID
	Title         string `json:"title"`
	Description   string `json:"description"`
	Link          string `json:"link,omitempty"` // Episode webpage URL
	AudioURL      string `json:"audioUrl"`
	Duration      int    `json:"duration,omitempty"` // Seconds
	PublishedAt   int64  `json:"publishedAt"`        // Unix timestamp
	Image         string `json:"image,omitempty"`
	TranscriptURL string `json:"transcriptUrl,omitempty"`
	ChaptersURL   string `json:"chaptersUrl,omitempty"`
	Episode       int    `json:"episode,omitempty"` // Episode number
	Season        int    `json:"season,omitempty"`  // Season number
}

// Waveform represents audio waveform data
type Waveform struct {
	ID         string    `json:"id"`
	EpisodeID  int64     `json:"episodeId"`
	Data       []float32 `json:"data"`
	Duration   float64   `json:"duration"` // Total duration in seconds
	SampleRate int       `json:"sampleRate"`
	Status     string    `json:"status"`
}

// Transcription represents episode transcription data
type Transcription struct {
	ID        string `json:"id"`
	EpisodeID int64  `json:"episodeId"`
	Text      string `json:"text"`
	Format    string `json:"format"` // "vtt", "srt", "txt", "json"
	Language  string `json:"language"`
	Status    string `json:"status"`
}

// Annotation represents audio segment annotation
type Annotation struct {
	ID        string  `json:"id"`
	EpisodeID int64   `json:"episodeId"`
	StartTime float64 `json:"startTime"` // Seconds
	EndTime   float64 `json:"endTime"`   // Seconds
	Label     string  `json:"label"`
	Text      string  `json:"text,omitempty"`
}

// Category represents a podcast category
type Category struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ReviewData contains aggregated review information
type ReviewData struct {
	TotalCount         int            `json:"totalCount"`
	AverageRating      float64        `json:"averageRating"`
	RatingDistribution map[string]int `json:"ratingDistribution,omitempty"`
	RecentReviews      []Review       `json:"recentReviews,omitempty"`
	MostHelpful        []Review       `json:"mostHelpful,omitempty"`
}

// Review represents a single review
type Review struct {
	ID        string    `json:"id,omitempty"`
	Author    string    `json:"author"`
	Rating    int       `json:"rating"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	VoteCount int       `json:"voteCount,omitempty"`
	VoteSum   int       `json:"voteSum,omitempty"`
	UpdatedAt time.Time `json:"updatedAt,omitempty"`
}
