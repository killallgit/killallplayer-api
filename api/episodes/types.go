package episodes

import (
	"github.com/killallgit/player-api/internal/services/episodes"
)

// EpisodeByGUIDResponse represents our API's response wrapper for single episode by GUID
type EpisodeByGUIDResponse struct {
	Status      string                        `json:"status" example:"true"`
	Episode     *episodes.PodcastIndexEpisode `json:"episode,omitempty"`
	Description string                        `json:"description" example:"Episode found"`
}
