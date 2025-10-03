package types

import (
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/services/audiocache"
	"github.com/killallgit/player-api/internal/services/auth"
	"github.com/killallgit/player-api/internal/services/clips"
	episodeanalysis "github.com/killallgit/player-api/internal/services/episode_analysis"
	"github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/itunes"
	"github.com/killallgit/player-api/internal/services/jobs"
	"github.com/killallgit/player-api/internal/services/podcasts"
	"github.com/killallgit/player-api/internal/services/transcription"
	"github.com/killallgit/player-api/internal/services/waveforms"
	"github.com/killallgit/player-api/internal/services/workers"
)

// Dependencies holds all the dependencies needed by handlers
type Dependencies struct {
	DB                     *database.DB
	PodcastService         podcasts.PodcastService
	EpisodeService         episodes.EpisodeService
	EpisodeTransformer     episodes.EpisodeTransformer
	WaveformService        waveforms.WaveformService
	TranscriptionService   transcription.TranscriptionService
	AudioCacheService      audiocache.Service
	ClipService            clips.Service // New clip service for ML training data
	EpisodeAnalysisService episodeanalysis.Service
	JobService             jobs.Service
	WorkerPool             *workers.WorkerPool
	PodcastClient          PodcastClient
	ITunesClient           *itunes.Client
	AuthService            *auth.Service
}
