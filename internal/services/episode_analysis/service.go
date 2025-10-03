package episodeanalysis

import (
	"context"
	"fmt"
	"log"

	"github.com/killallgit/player-api/internal/services/audiocache"
	"github.com/killallgit/player-api/internal/services/clips"
	"github.com/killallgit/player-api/internal/services/episodes"
)

// Service analyzes episodes for volume anomalies and creates clips
type Service interface {
	// AnalyzeAndCreateClips finds volume spikes in an episode and auto-creates clips
	// Returns list of created clip UUIDs
	AnalyzeAndCreateClips(ctx context.Context, episodeID int64) ([]string, error)
}

type serviceImpl struct {
	audioCache     audiocache.Service
	clipService    clips.Service
	episodeService episodes.EpisodeService
	analyzer       *VolumeAnalyzer
}

// NewService creates a new episode analysis service
func NewService(
	audioCache audiocache.Service,
	clipService clips.Service,
	episodeService episodes.EpisodeService,
) Service {
	return &serviceImpl{
		audioCache:     audioCache,
		clipService:    clipService,
		episodeService: episodeService,
		analyzer:       NewVolumeAnalyzer(),
	}
}

// AnalyzeAndCreateClips is the main entry point for episode analysis
func (s *serviceImpl) AnalyzeAndCreateClips(ctx context.Context, episodeID int64) ([]string, error) {
	log.Printf("[INFO] Starting volume spike analysis for episode %d", episodeID)

	// 1. Fetch episode details to get audio URL
	episode, err := s.episodeService.GetEpisodeByPodcastIndexID(ctx, episodeID)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch episode: %w", err)
	}

	if episode.AudioURL == "" {
		return nil, fmt.Errorf("episode has no audio URL")
	}

	log.Printf("[INFO] Episode: %s (duration: %v seconds)", episode.Title, episode.Duration)

	// 2. Get or download audio (uses cache if available)
	audioCache, err := s.audioCache.GetOrDownloadAudio(ctx, episodeID, episode.AudioURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get audio: %w", err)
	}

	log.Printf("[INFO] Using cached audio: %s (%.2f seconds)", audioCache.ProcessedPath, audioCache.DurationSeconds)

	// 3. Analyze audio for volume spikes
	spikes, err := s.analyzer.FindSpikes(ctx, audioCache.ProcessedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze audio: %w", err)
	}

	log.Printf("[INFO] Detected %d volume spikes", len(spikes))

	if len(spikes) == 0 {
		log.Printf("[INFO] No volume spikes found in episode %d", episodeID)
		return []string{}, nil
	}

	// 4. Create clips from detected spikes
	var clipUUIDs []string

	for i, spike := range spikes {
		log.Printf("[INFO] Creating clip %d/%d: %.2fs-%.2fs (peak: %.2f dB)",
			i+1, len(spikes), spike.StartTime, spike.EndTime, spike.PeakDB)

		// Create clip with special label for auto-detected spikes
		// Not approved - user must review and approve before extraction
		clip, err := s.clipService.CreateClip(ctx, clips.CreateClipParams{
			PodcastIndexEpisodeID: episodeID,
			OriginalStartTime:     spike.StartTime,
			OriginalEndTime:       spike.EndTime,
			Label:                 "volume_spike", // Special label for auto-detected
			Approved:              false,          // Needs review before extraction
		})

		if err != nil {
			log.Printf("[WARN] Failed to create clip for spike %d: %v", i+1, err)
			continue
		}

		clipUUIDs = append(clipUUIDs, clip.UUID)
		log.Printf("[INFO] Created clip %s for spike at %.2fs-%.2fs", clip.UUID, spike.StartTime, spike.EndTime)
	}

	log.Printf("[INFO] Successfully created %d clips from %d detected spikes", len(clipUUIDs), len(spikes))

	return clipUUIDs, nil
}
