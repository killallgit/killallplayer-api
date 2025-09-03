package episodes

import "time"

// EpisodeMetadata represents metadata about an episode file (audio/video)
type EpisodeMetadata struct {
	URL          string
	ContentType  string
	Size         int64
	LastModified time.Time
	FileName     string
}

// PodcastIndexEpisode represents an episode in the exact format returned by Podcast Index API
type PodcastIndexEpisode struct {
	ID                  int64               `json:"id" example:"123456789"`
	Title               string              `json:"title" example:"Episode 42: The Answer to Everything"`
	Link                string              `json:"link,omitempty" example:"https://example.com/episode/42"`
	Description         string              `json:"description" example:"In this episode, we explore the meaning of life, the universe, and everything."`
	GUID                string              `json:"guid" example:"episode-42-guid-string"`
	DatePublished       int64               `json:"datePublished" example:"1704063600"`
	DatePublishedPretty string              `json:"datePublishedPretty,omitempty" example:"2024-01-01 00:00:00"`
	DateCrawled         int64               `json:"dateCrawled,omitempty" example:"1704067200"`
	EnclosureURL        string              `json:"enclosureUrl" example:"https://example.com/audio/episode42.mp3"`
	EnclosureType       string              `json:"enclosureType,omitempty" example:"audio/mpeg"`
	EnclosureLength     int64               `json:"enclosureLength,omitempty" example:"52428800"`
	Duration            *int                `json:"duration" example:"3600"`
	Explicit            int                 `json:"explicit,omitempty" example:"0"`
	Episode             *int                `json:"episode,omitempty" example:"42"`
	EpisodeType         string              `json:"episodeType,omitempty" example:"full"`
	Season              *int                `json:"season,omitempty" example:"2"`
	Image               string              `json:"image,omitempty" example:"https://example.com/episode42-cover.jpg"`
	FeedItunesID        *int64              `json:"feedItunesId,omitempty" example:"987654321"`
	FeedURL             string              `json:"feedUrl,omitempty" example:"https://example.com/rss.xml"`
	FeedImage           string              `json:"feedImage,omitempty" example:"https://example.com/podcast-cover.jpg"`
	FeedID              int64               `json:"feedId" example:"123456"`
	FeedTitle           string              `json:"feedTitle,omitempty" example:"The Tech Show"`
	PodcastGUID         string              `json:"podcastGuid,omitempty" example:"podcast-guid-string"`
	FeedLanguage        string              `json:"feedLanguage,omitempty" example:"en"`
	FeedDead            int                 `json:"feedDead,omitempty" example:"0"`
	FeedDuplicateOf     *int64              `json:"feedDuplicateOf,omitempty"`
	ChaptersURL         string              `json:"chaptersUrl,omitempty" example:"https://example.com/chapters/episode42.json"`
	TranscriptURL       string              `json:"transcriptUrl,omitempty" example:"https://example.com/transcripts/episode42.txt"`
	Transcripts         []Transcript        `json:"transcripts,omitempty"`
	Soundbite           *Soundbite          `json:"soundbite,omitempty"`
	Soundbites          []Soundbite         `json:"soundbites,omitempty"`
	Persons             []Person            `json:"persons,omitempty"`
	SocialInteract      []SocialInteraction `json:"socialInteract,omitempty"`
	Value               *Value              `json:"value,omitempty"`
}

// PodcastIndexResponse represents the standard response wrapper from Podcast Index API
type PodcastIndexResponse struct {
	Status      string                `json:"status" example:"true"`        // "true" or "false"
	Items       []PodcastIndexEpisode `json:"items"`
	LiveItems   []PodcastIndexEpisode `json:"liveItems,omitempty"`
	Count       int                   `json:"count" example:"10"`
	Query       interface{}           `json:"query,omitempty"`
	Description string                `json:"description" example:"Found 10 episodes"`
}

// PodcastIndexErrorResponse represents an error response from Podcast Index API
type PodcastIndexErrorResponse struct {
	Status      string `json:"status"` // "false"
	Description string `json:"description"`
}

// Transcript represents a transcript object in Podcast Index format
type Transcript struct {
	URL      string `json:"url"`
	Type     string `json:"type"`
	Language string `json:"language,omitempty"`
	Rel      string `json:"rel,omitempty"`
}

// Soundbite represents a soundbite in Podcast Index format
type Soundbite struct {
	StartTime float64 `json:"startTime"`
	Duration  float64 `json:"duration"`
	Title     string  `json:"title,omitempty"`
}

// Person represents a person associated with an episode
type Person struct {
	Name  string `json:"name"`
	Role  string `json:"role,omitempty"`
	Group string `json:"group,omitempty"`
	Href  string `json:"href,omitempty"`
	Img   string `json:"img,omitempty"`
}

// SocialInteraction represents social interaction data
type SocialInteraction struct {
	URL        string `json:"url"`
	Protocol   string `json:"protocol,omitempty"`
	Platform   string `json:"platform,omitempty"`
	AccountID  string `json:"accountId,omitempty"`
	AccountURL string `json:"accountUrl,omitempty"`
	Priority   int    `json:"priority,omitempty"`
}

// Value represents value 4 value information
type Value struct {
	Model        ValueModel         `json:"model"`
	Destinations []ValueDestination `json:"destinations"`
}

// ValueModel represents the value model type
type ValueModel struct {
	Type      string `json:"type"`
	Method    string `json:"method"`
	Suggested string `json:"suggested,omitempty"`
}

// ValueDestination represents a value destination
type ValueDestination struct {
	Name        string `json:"name,omitempty"`
	Address     string `json:"address"`
	Type        string `json:"type"`
	Split       int    `json:"split"`
	Fee         bool   `json:"fee,omitempty"`
	CustomKey   string `json:"customKey,omitempty"`
	CustomValue string `json:"customValue,omitempty"`
}

// EpisodeByGUIDResponse represents the response for single episode by GUID
type EpisodeByGUIDResponse struct {
	Status      string               `json:"status"`
	Episode     *PodcastIndexEpisode `json:"episode"`
	Description string               `json:"description"`
}

// LiveEpisode represents a live episode with additional live item data
type LiveEpisode struct {
	PodcastIndexEpisode
	LiveItem LiveItem `json:"liveItem,omitempty"`
}

// LiveItem represents live streaming information
type LiveItem struct {
	Status      string `json:"status"`
	Start       string `json:"start"`
	End         string `json:"end,omitempty"`
	Chat        string `json:"chat,omitempty"`
	ContentLink string `json:"contentLink,omitempty"`
}
