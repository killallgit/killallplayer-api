package episodes

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/killallgit/player-api/internal/models"
)

// Service implements the EpisodeService interface with business logic
type Service struct {
	fetcher           EpisodeFetcher
	repository        EpisodeRepository
	cache             EpisodeCache
	keyGen            CacheKeyGenerator
	maxConcurrentSync int
	syncTimeout       time.Duration
}

// ServiceOption is a functional option for configuring the service
type ServiceOption func(*Service)

// WithMaxConcurrentSync sets the maximum concurrent sync operations
func WithMaxConcurrentSync(max int) ServiceOption {
	return func(s *Service) {
		if max > 0 {
			s.maxConcurrentSync = max
		}
	}
}

// WithSyncTimeout sets the timeout for sync operations
func WithSyncTimeout(timeout time.Duration) ServiceOption {
	return func(s *Service) {
		if timeout > 0 {
			s.syncTimeout = timeout
		}
	}
}

// NewService creates a new episode service with optional configuration
func NewService(fetcher EpisodeFetcher, repository EpisodeRepository, cache EpisodeCache, opts ...ServiceOption) *Service {
	s := &Service{
		fetcher:           fetcher,
		repository:        repository,
		cache:             cache,
		keyGen:            NewKeyGenerator("episode"),
		maxConcurrentSync: DefaultMaxConcurrentSyncs,
		syncTimeout:       DefaultSyncTimeout,
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// FetchAndSyncEpisodes fetches episodes from external API and syncs to database
func (s *Service) FetchAndSyncEpisodes(ctx context.Context, podcastID int64, limit int) (*PodcastIndexResponse, error) {
	// Try to fetch from external API first
	response, err := s.fetcher.GetEpisodesByPodcastID(ctx, podcastID, limit)
	if err != nil {
		return nil, fmt.Errorf("fetching episodes from API: %w", err)
	}

	// Sync to database in background with detached context but respecting cancellation
	go func() {
		// Recover from any panics in the goroutine
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] Panic in background sync for podcast %d: %v", podcastID, r)
			}
		}()

		// Create a new context that inherits values but not cancellation from parent
		// This allows the sync to continue even if the HTTP request is cancelled
		syncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.syncTimeout)
		defer cancel()

		synced, err := s.SyncEpisodesToDatabase(syncCtx, response.Items, uint(podcastID))
		if err != nil {
			log.Printf("[ERROR] Failed to sync episodes for podcast %d: %v", podcastID, err)
		} else {
			log.Printf("[DEBUG] Successfully synced %d episodes for podcast %d", synced, podcastID)
		}
	}()

	return response, nil
}

// SyncEpisodesToDatabase syncs episodes from external API to database
func (s *Service) SyncEpisodesToDatabase(ctx context.Context, episodes []PodcastIndexEpisode, podcastID uint) (int, error) {
	var (
		successCount int
		failureCount int
		errors       []error
		mu           sync.Mutex
		wg           sync.WaitGroup
	)

	// Use a semaphore to limit concurrent operations
	sem := make(chan struct{}, s.maxConcurrentSync)

	transformer := NewTransformer()

	for _, piEpisode := range episodes {
		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(pie PodcastIndexEpisode) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore
			
			// Recover from any panics in the goroutine
			defer func() {
				if r := recover(); r != nil {
					mu.Lock()
					failureCount++
					errors = append(errors, fmt.Errorf("panic processing episode %s: %v", pie.Title, r))
					mu.Unlock()
					log.Printf("[ERROR] Panic processing episode %s: %v", pie.Title, r)
				}
			}()

			episode := transformer.PodcastIndexToModel(pie, podcastID)

			// Check if episode exists
			existing, err := s.repository.GetEpisodeByGUID(ctx, episode.GUID)
			if err == nil && existing != nil {
				// Preserve playback state
				episode.ID = existing.ID
				episode.Played = existing.Played
				episode.Position = existing.Position
				episode.CreatedAt = existing.CreatedAt

				err = s.repository.UpdateEpisode(ctx, episode)
			} else if IsNotFound(err) {
				// Create new episode
				err = s.repository.CreateEpisode(ctx, episode)
			}

			mu.Lock()
			if err != nil {
				failureCount++
				errors = append(errors, err)
			} else {
				successCount++
				// Invalidate cache for this podcast
				s.cache.InvalidatePattern(s.keyGen.PodcastPattern(podcastID))
			}
			mu.Unlock()
		}(piEpisode)
	}

	wg.Wait()

	if failureCount > 0 {
		return successCount, NewSyncError(successCount, failureCount, errors)
	}

	return successCount, nil
}

// GetEpisodeByID retrieves an episode by ID with caching
func (s *Service) GetEpisodeByID(ctx context.Context, id uint) (*models.Episode, error) {
	key := s.keyGen.EpisodeByID(id)

	// Check cache first
	if episode, found := s.cache.GetEpisode(key); found {
		return episode, nil
	}

	// Fetch from repository
	episode, err := s.repository.GetEpisodeByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Cache the result
	s.cache.SetEpisode(key, episode)
	return episode, nil
}

// GetEpisodeByGUID retrieves an episode by GUID with caching
func (s *Service) GetEpisodeByGUID(ctx context.Context, guid string) (*models.Episode, error) {
	key := s.keyGen.EpisodeByGUID(guid)

	// Check cache first
	if episode, found := s.cache.GetEpisode(key); found {
		return episode, nil
	}

	// Fetch from repository
	episode, err := s.repository.GetEpisodeByGUID(ctx, guid)
	if err != nil {
		return nil, err
	}

	// Cache the result
	s.cache.SetEpisode(key, episode)
	return episode, nil
}

// GetEpisodeByPodcastIndexID retrieves an episode by Podcast Index ID with caching
func (s *Service) GetEpisodeByPodcastIndexID(ctx context.Context, podcastIndexID int64) (*models.Episode, error) {
	// For now, try to get from repository
	// TODO: Add caching with a key generator for Podcast Index IDs
	episode, err := s.repository.GetEpisodeByPodcastIndexID(ctx, podcastIndexID)
	if err != nil {
		if IsNotFound(err) {
			// Episode not in database - could fetch from Podcast Index API here
			// For now, return not found
			return nil, err
		}
		return nil, err
	}
	
	return episode, nil
}

// GetEpisodesByPodcastID retrieves episodes for a podcast with caching
func (s *Service) GetEpisodesByPodcastID(ctx context.Context, podcastID uint, page, limit int) ([]models.Episode, int64, error) {
	key := s.keyGen.EpisodesByPodcast(podcastID, page, limit)

	// Check cache first
	if episodes, total, found := s.cache.GetEpisodeList(key); found {
		return episodes, total, nil
	}

	// Fetch from repository
	episodes, total, err := s.repository.GetEpisodesByPodcastID(ctx, podcastID, page, limit)
	if err != nil {
		return nil, 0, err
	}

	// Cache the result
	s.cache.SetEpisodeList(key, episodes, total)
	return episodes, total, nil
}

// GetRecentEpisodes retrieves recent episodes with caching
func (s *Service) GetRecentEpisodes(ctx context.Context, limit int) ([]models.Episode, error) {
	key := s.keyGen.RecentEpisodes(limit)

	// Check cache first
	if episodes, _, found := s.cache.GetEpisodeList(key); found {
		return episodes, nil
	}

	// Fetch from repository
	episodes, err := s.repository.GetRecentEpisodes(ctx, limit)
	if err != nil {
		return nil, err
	}

	// Cache the result
	s.cache.SetEpisodeList(key, episodes, int64(len(episodes)))
	return episodes, nil
}

// UpdatePlaybackState updates the playback state of an episode
func (s *Service) UpdatePlaybackState(ctx context.Context, id uint, position int, played bool) error {
	// Update position
	if err := s.repository.UpdatePlaybackPosition(ctx, id, position); err != nil {
		return fmt.Errorf("updating playback position: %w", err)
	}

	// Update played status
	if err := s.repository.MarkEpisodeAsPlayed(ctx, id, played); err != nil {
		return fmt.Errorf("updating played status: %w", err)
	}

	// Invalidate cache for this episode
	s.cache.Invalidate(s.keyGen.EpisodeByID(id))

	return nil
}

// UpdatePlaybackStateByPodcastIndexID updates the playback state of an episode using Podcast Index ID
func (s *Service) UpdatePlaybackStateByPodcastIndexID(ctx context.Context, podcastIndexID int64, position int, played bool) error {
	// First get the episode to find the internal ID
	episode, err := s.repository.GetEpisodeByPodcastIndexID(ctx, podcastIndexID)
	if err != nil {
		return fmt.Errorf("finding episode by podcast index id: %w", err)
	}

	// Update position
	if err := s.repository.UpdatePlaybackPosition(ctx, episode.ID, position); err != nil {
		return fmt.Errorf("updating playback position: %w", err)
	}

	// Update played status
	if err := s.repository.MarkEpisodeAsPlayed(ctx, episode.ID, played); err != nil {
		return fmt.Errorf("updating played status: %w", err)
	}

	// Invalidate cache for this episode
	s.cache.Invalidate(s.keyGen.EpisodeByID(episode.ID))
	// TODO: Add cache invalidation for Podcast Index ID key when implemented

	return nil
}

// Constants for default configuration
const (
	DefaultSyncTimeout        = 30 * time.Second
	DefaultMaxConcurrentSyncs = 5 // default max concurrent sync operations
)
