package models

// SearchRequest represents the incoming search request
type SearchRequest struct {
	Query    string `json:"query" validate:"required,min=1,max=200" example:"technology podcasts"`
	Limit    int    `json:"limit,omitempty" validate:"min=1,max=100" example:"10"`
	FullText bool   `json:"fullText,omitempty" example:"false"` // Return full descriptions instead of truncated
}

// SearchResponse represents the search response
type SearchResponse struct {
	Podcasts []PodcastSearchResult `json:"podcasts"`
}
