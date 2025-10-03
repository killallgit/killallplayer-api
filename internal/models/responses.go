package models

import "github.com/killallgit/player-api/internal/services/podcastindex"

// EpisodeResponse represents the episodes response for consistent formatting
// Used by the random episodes endpoint
type EpisodeResponse struct {
	Status      string                 `json:"status"`
	Results     []podcastindex.Episode `json:"results"`
	TotalCount  int                    `json:"totalCount"`
	Max         string                 `json:"max,omitempty"`    // Max results parameter used (string to match PodcastIndex API)
	Lang        string                 `json:"lang,omitempty"`   // Language parameter used
	NotCat      []string               `json:"notcat,omitempty"` // Excluded categories
	Description string                 `json:"description,omitempty"`
}
