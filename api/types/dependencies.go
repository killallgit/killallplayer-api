package types

import (
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/services/episodes"
)

// Dependencies holds all the dependencies needed by handlers
type Dependencies struct {
	DB                 *database.DB
	EpisodeService     episodes.EpisodeService
	EpisodeTransformer episodes.EpisodeTransformer
	PodcastClient      PodcastClient
}
