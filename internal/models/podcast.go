package models

// PodcastSearchResult represents a podcast in search results from the API
type PodcastSearchResult struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	Description string `json:"description"`
	Image       string `json:"image"`
	URL         string `json:"url"`
}
