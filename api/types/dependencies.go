package types

import (
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/jobs"
	"github.com/killallgit/player-api/internal/services/waveforms"
	"github.com/killallgit/player-api/internal/services/workers"
)

// Dependencies holds all the dependencies needed by handlers
type Dependencies struct {
	DB                 *database.DB
	EpisodeService     episodes.EpisodeService
	EpisodeTransformer episodes.EpisodeTransformer
	WaveformService    waveforms.WaveformService
	JobService         jobs.Service
	WorkerPool         *workers.WorkerPool
	PodcastClient      PodcastClient
}
