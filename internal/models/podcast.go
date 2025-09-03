package models

// PodcastSearchResult represents a podcast in search results from the API
type PodcastSearchResult struct {
	ID          string `json:"id" example:"123456"`
	Title       string `json:"title" example:"The Tech Show"`
	Author      string `json:"author" example:"John Smith"`
	Description string `json:"description" example:"A weekly show about technology and innovation"`
	Image       string `json:"image" example:"https://example.com/podcast-image.jpg"`
	URL         string `json:"url" example:"https://example.com/rss.xml"`
}
