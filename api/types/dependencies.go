package types

import (
	"github.com/killallgit/player-api/internal/database"
	"github.com/killallgit/player-api/internal/services/annotations"
	"github.com/killallgit/player-api/internal/services/audiocache"
	"github.com/killallgit/player-api/internal/services/episodes"
	"github.com/killallgit/player-api/internal/services/itunes"
	"github.com/killallgit/player-api/internal/services/jobs"
	"github.com/killallgit/player-api/internal/services/transcription"
	"github.com/killallgit/player-api/internal/services/waveforms"
	"github.com/killallgit/player-api/internal/services/workers"
)

// Dependencies holds all the dependencies needed by handlers
type Dependencies struct {
	DB                   *database.DB
	EpisodeService       episodes.EpisodeService
	EpisodeTransformer   episodes.EpisodeTransformer
	WaveformService      waveforms.WaveformService
	TranscriptionService transcription.TranscriptionService
	AnnotationService    annotations.Service
	AudioCacheService    audiocache.Service
	JobService           jobs.Service
	WorkerPool           *workers.WorkerPool
	PodcastClient        PodcastClient
	ITunesClient         *itunes.Client
}
