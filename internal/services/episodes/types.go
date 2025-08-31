package episodes

// PodcastIndexEpisode represents an episode in the exact format returned by Podcast Index API
type PodcastIndexEpisode struct {
	ID                  int64                  `json:"id"`
	Title               string                 `json:"title"`
	Link                string                 `json:"link,omitempty"`
	Description         string                 `json:"description"`
	GUID                string                 `json:"guid"`
	DatePublished       int64                  `json:"datePublished"`
	DatePublishedPretty string                 `json:"datePublishedPretty,omitempty"`
	DateCrawled         int64                  `json:"dateCrawled,omitempty"`
	EnclosureURL        string                 `json:"enclosureUrl"`
	EnclosureType       string                 `json:"enclosureType,omitempty"`
	EnclosureLength     int64                  `json:"enclosureLength,omitempty"`
	Duration            *int                   `json:"duration"`
	Explicit            int                    `json:"explicit,omitempty"`
	Episode             *int                   `json:"episode,omitempty"`
	EpisodeType         string                 `json:"episodeType,omitempty"`
	Season              *int                   `json:"season,omitempty"`
	Image               string                 `json:"image,omitempty"`
	FeedItunesID        *int64                 `json:"feedItunesId,omitempty"`
	FeedURL             string                 `json:"feedUrl,omitempty"`
	FeedImage           string                 `json:"feedImage,omitempty"`
	FeedID              int64                  `json:"feedId"`
	FeedTitle           string                 `json:"feedTitle,omitempty"`
	PodcastGUID         string                 `json:"podcastGuid,omitempty"`
	FeedLanguage        string                 `json:"feedLanguage,omitempty"`
	FeedDead            int                    `json:"feedDead,omitempty"`
	FeedDuplicateOf     *int64                 `json:"feedDuplicateOf,omitempty"`
	ChaptersURL         string                 `json:"chaptersUrl,omitempty"`
	TranscriptURL       string                 `json:"transcriptUrl,omitempty"`
	Transcripts         []Transcript           `json:"transcripts,omitempty"`
	Soundbite           *Soundbite             `json:"soundbite,omitempty"`
	Soundbites          []Soundbite            `json:"soundbites,omitempty"`
	Persons             []Person               `json:"persons,omitempty"`
	SocialInteract      []SocialInteraction    `json:"socialInteract,omitempty"`
	Value               *Value                 `json:"value,omitempty"`
}

// PodcastIndexResponse represents the standard response wrapper from Podcast Index API
type PodcastIndexResponse struct {
	Status      string                `json:"status"` // "true" or "false"
	Items       []PodcastIndexEpisode `json:"items"`
	LiveItems   []PodcastIndexEpisode `json:"liveItems,omitempty"`
	Count       int                   `json:"count"`
	Query       interface{}           `json:"query,omitempty"`
	Description string                `json:"description"`
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
	URL      string `json:"url"`
	Protocol string `json:"protocol,omitempty"`
	Platform string `json:"platform,omitempty"`
	AccountID string `json:"accountId,omitempty"`
	AccountURL string `json:"accountUrl,omitempty"`
	Priority  int    `json:"priority,omitempty"`
}

// Value represents value 4 value information
type Value struct {
	Model       ValueModel        `json:"model"`
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
	Name    string  `json:"name,omitempty"`
	Address string  `json:"address"`
	Type    string  `json:"type"`
	Split   int     `json:"split"`
	Fee     bool    `json:"fee,omitempty"`
	CustomKey string `json:"customKey,omitempty"`
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
	Status    string  `json:"status"`
	Start     string  `json:"start"`
	End       string  `json:"end,omitempty"`
	Chat      string  `json:"chat,omitempty"`
	ContentLink string `json:"contentLink,omitempty"`
}