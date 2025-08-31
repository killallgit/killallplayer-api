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
