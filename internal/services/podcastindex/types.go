package podcastindex

import "time"

// SearchRequest represents a search request to the Podcast Index API
type SearchRequest struct {
	Query string `json:"q"`
	Max   int    `json:"max,omitempty"`
}

// SearchResponse represents the response from Podcast Index search API
type SearchResponse struct {
	Status      string    `json:"status"`
	Feeds       []Podcast `json:"feeds"`
	Count       int       `json:"count"`
	Query       string    `json:"query"`
	Description string    `json:"description"`
}

// Podcast represents a podcast from the Podcast Index API
type Podcast struct {
	ID               int               `json:"id"`
	Title            string            `json:"title"`
	URL              string            `json:"url"`
	OriginalURL      string            `json:"originalUrl"`
	Link             string            `json:"link"`
	Description      string            `json:"description"`
	Author           string            `json:"author"`
	OwnerName        string            `json:"ownerName"`
	Image            string            `json:"image"`
	Artwork          string            `json:"artwork"`
	LastUpdateTime   int64             `json:"lastUpdateTime"`
	LastCrawlTime    int64             `json:"lastCrawlTime"`
	LastParseTime    int64             `json:"lastParseTime"`
	LastGoodHTTPCode int               `json:"lastGoodHttpStatusCode"`
	Language         string            `json:"language"`
	Categories       map[string]string `json:"categories"`
	Locked           int               `json:"locked"`
	ImageURLHash     int               `json:"imageUrlHash"`
	EpisodeCount     int               `json:"episodeCount"`
	ITunesID         int               `json:"itunesId"`
	CreatedOn        time.Time         `json:"createdOn"`
}

// Episode represents an episode from the Podcast Index API
type Episode struct {
	ID                  int64  `json:"id"`
	Title               string `json:"title"`
	Link                string `json:"link"`
	Description         string `json:"description"`
	GUID                string `json:"guid"`
	DatePublished       int64  `json:"datePublished"`
	DatePublishedPretty string `json:"datePublishedPretty"`
	DateCrawled         int64  `json:"dateCrawled"`
	EnclosureURL        string `json:"enclosureUrl"`
	EnclosureType       string `json:"enclosureType"`
	EnclosureLength     int    `json:"enclosureLength"`
	Duration            int    `json:"duration"`
	Explicit            int    `json:"explicit"`
	Episode             int    `json:"episode"`
	EpisodeType         string `json:"episodeType"`
	Season              int    `json:"season"`
	Image               string `json:"image"`
	FeedItunesId        int    `json:"feedItunesId"`
	FeedImage           string `json:"feedImage"`
	FeedId              int    `json:"feedId"`
	FeedLanguage        string `json:"feedLanguage"`
	FeedDead            int    `json:"feedDead"`
	FeedDuplicateOf     int    `json:"feedDuplicateOf"`
	ChaptersURL         string `json:"chaptersUrl"`
	TranscriptURL       string `json:"transcriptUrl"`
}

// EpisodesResponse represents the response from episodes API
type EpisodesResponse struct {
	Status      string    `json:"status"`
	Items       []Episode `json:"items"`
	Count       int       `json:"count"`
	Max         string    `json:"max"`
	Description string    `json:"description"`
}

// EpisodeByGUIDResponse represents the response from episode by GUID API
type EpisodeByGUIDResponse struct {
	Status      string  `json:"status"`
	Episode     Episode `json:"episode"`
	Description string  `json:"description"`
}
