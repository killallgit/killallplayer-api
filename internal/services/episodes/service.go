package episodes

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/podcasts"
)

// Service implements the EpisodeService interface with business logic
type Service struct {
	fetcher           EpisodeFetcher
	repository        EpisodeRepository
	cache             EpisodeCache
	podcastService    podcasts.PodcastService
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
func NewService(fetcher EpisodeFetcher, repository EpisodeRepository, cache EpisodeCache, podcastService podcasts.PodcastService, opts ...ServiceOption) *Service {
	s := &Service{
		fetcher:           fetcher,
		repository:        repository,
		cache:             cache,
		podcastService:    podcastService,
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
func (s *Service) FetchAndSyncEpisodes(ctx context.Context, podcastIndexID int64, limit int) (*PodcastIndexResponse, error) {
	// Check if fetcher is available
	if s.fetcher == nil {
		return nil, fmt.Errorf("podcast API client not available - check Podcast Index API credentials")
	}

	// STEP 1: Ensure podcast exists in DB (will fetch from API if needed)
	if s.podcastService != nil {
		podcast, err := s.podcastService.GetPodcastByPodcastIndexID(ctx, podcastIndexID)
		if err != nil {
			return nil, fmt.Errorf("ensuring podcast exists: %w", err)
		}
		log.Printf("[DEBUG] Podcast %d exists in DB (ID=%d): %s", podcastIndexID, podcast.ID, podcast.Title)
	}

	// STEP 2: Fetch episodes from external API
	response, err := s.fetcher.GetEpisodesByPodcastID(ctx, podcastIndexID, limit)
	if err != nil {
		return nil, fmt.Errorf("fetching episodes from API: %w", err)
	}

	// STEP 3: Sync to database in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] Panic in background sync for podcast %d: %v", podcastIndexID, r)
			}
		}()

		syncCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.syncTimeout)
		defer cancel()

		// Get podcast record (should be in DB now)
		var podcastDBID uint
		if s.podcastService != nil {
			podcast, err := s.podcastService.GetPodcastByPodcastIndexID(syncCtx, podcastIndexID)
			if err != nil {
				log.Printf("[ERROR] Failed to get podcast for sync: %v", err)
				return
			}
			podcastDBID = podcast.ID
		}

		synced, err := s.SyncEpisodesToDatabase(syncCtx, response.Items, podcastDBID, podcastIndexID)
		if err != nil {
			log.Printf("[ERROR] Failed to sync episodes for podcast %d: %v", podcastIndexID, err)
		} else {
			log.Printf("[INFO] Successfully synced %d episodes for podcast %d", synced, podcastIndexID)

			// Update podcast episode count
			if s.podcastService != nil && podcastDBID > 0 {
				_ = s.podcastService.UpdatePodcastMetrics(syncCtx, podcastDBID, synced)
			}
		}
	}()

	return response, nil
}

// SyncEpisodesToDatabase syncs episodes from external API to database
func (s *Service) SyncEpisodesToDatabase(ctx context.Context, episodes []PodcastIndexEpisode, podcastID uint, podcastIndexFeedID int64) (int, error) {
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
			// Set the Podcast Index Feed ID for fast queries
			episode.PodcastIndexFeedID = podcastIndexFeedID

			// Check if episode exists
			existing, err := s.repository.GetEpisodeByGUID(ctx, episode.GUID)
			if err == nil && existing != nil {
				// Preserve existing data
				episode.ID = existing.ID
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
	key := s.keyGen.EpisodeByPodcastIndexID(podcastIndexID)

	// Check cache first
	if episode, found := s.cache.GetEpisode(key); found {
		log.Printf("[DEBUG] Service.GetEpisodeByPodcastIndexID: Cache HIT for PodcastIndexID=%d, returning episode with ID=%d",
			podcastIndexID, episode.ID)
		return episode, nil
	}

	// Fetch from repository
	log.Printf("[DEBUG] Service.GetEpisodeByPodcastIndexID: Cache MISS for PodcastIndexID=%d, fetching from repository", podcastIndexID)
	episode, err := s.repository.GetEpisodeByPodcastIndexID(ctx, podcastIndexID)
	if err != nil {
		if IsNotFound(err) {
			// Episode not in database - try fetching from Podcast Index API
			log.Printf("[DEBUG] Service.GetEpisodeByPodcastIndexID: Episode %d not in database, attempting to fetch from Podcast Index API", podcastIndexID)

			// Check if fetcher is available
			if s.fetcher == nil {
				log.Printf("[ERROR] Service.GetEpisodeByPodcastIndexID: Podcast API client not available")
				return nil, err // Return original not found error
			}

			// Fetch single episode from Podcast Index API
			apiEpisode, fetchErr := s.fetcher.GetEpisodeByID(ctx, podcastIndexID)
			if fetchErr != nil {
				// Check if it's a "not found" error from the API
				if strings.Contains(fetchErr.Error(), "not found") || strings.Contains(fetchErr.Error(), "404") {
					log.Printf("[ERROR] Service.GetEpisodeByPodcastIndexID: Episode %d does not exist in Podcast Index API", podcastIndexID)
					// Return a permanent error indicating the episode doesn't exist
					return nil, fmt.Errorf("episode %d does not exist in Podcast Index", podcastIndexID)
				}
				log.Printf("[ERROR] Service.GetEpisodeByPodcastIndexID: Failed to fetch episode %d from API: %v", podcastIndexID, fetchErr)
				return nil, err // Return original not found error if API fetch fails
			}

			// We have the episode from the API, now sync it to the database
			// We need a podcast ID - use the podcast service to resolve it
			var podcastID uint
			var feedID int64
			if apiEpisode.FeedID > 0 {
				feedID = apiEpisode.FeedID
				// Ensure the podcast exists in our database
				if s.podcastService != nil {
					podcast, err := s.podcastService.GetPodcastByPodcastIndexID(ctx, feedID)
					if err != nil {
						log.Printf("[ERROR] Service.GetEpisodeByPodcastIndexID: Failed to get/create podcast %d: %v", feedID, err)
						return nil, fmt.Errorf("failed to resolve podcast for episode: %w", err)
					}
					podcastID = podcast.ID
					log.Printf("[DEBUG] Service.GetEpisodeByPodcastIndexID: Resolved podcast %d to DB ID %d", feedID, podcastID)
				} else {
					log.Printf("[WARN] Service.GetEpisodeByPodcastIndexID: Podcast service not available, using FeedID as podcast ID")
					podcastID = uint(feedID)
				}
			} else {
				log.Printf("[ERROR] Service.GetEpisodeByPodcastIndexID: Episode %d has no FeedID, cannot associate with podcast", podcastIndexID)
				return nil, fmt.Errorf("episode %d has no feed ID", podcastIndexID)
			}

			// Sync this single episode to the database
			syncedCount, syncErr := s.SyncEpisodesToDatabase(ctx, []PodcastIndexEpisode{*apiEpisode}, podcastID, feedID)
			if syncErr != nil {
				log.Printf("[ERROR] Service.GetEpisodeByPodcastIndexID: Failed to sync episode %d to database: %v", podcastIndexID, syncErr)
				return nil, fmt.Errorf("failed to sync episode from API: %w", syncErr)
			}

			if syncedCount == 0 {
				log.Printf("[ERROR] Service.GetEpisodeByPodcastIndexID: Episode %d sync returned 0 synced episodes", podcastIndexID)
				return nil, fmt.Errorf("failed to sync episode from API")
			}

			// Now try to fetch from repository again
			episode, err = s.repository.GetEpisodeByPodcastIndexID(ctx, podcastIndexID)
			if err != nil {
				log.Printf("[ERROR] Service.GetEpisodeByPodcastIndexID: Failed to retrieve episode %d after sync: %v", podcastIndexID, err)
				return nil, fmt.Errorf("episode synced but not retrievable: %w", err)
			}

			log.Printf("[INFO] Service.GetEpisodeByPodcastIndexID: Successfully fetched and synced episode %d from Podcast Index API", podcastIndexID)
		} else {
			// Some other database error
			return nil, err
		}
	}

	log.Printf("[DEBUG] Service.GetEpisodeByPodcastIndexID: Repository returned episode with ID=%d for PodcastIndexID=%d",
		episode.ID, podcastIndexID)

	// Cache the result
	s.cache.SetEpisode(key, episode)
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

// GetEpisodesByPodcastIndexFeedID retrieves episodes for a podcast by Podcast Index feed ID
func (s *Service) GetEpisodesByPodcastIndexFeedID(ctx context.Context, feedID int64, page, limit int) ([]models.Episode, int64, error) {
	// First check if we have episodes in the database
	episodes, total, err := s.repository.GetEpisodesByPodcastIndexFeedID(ctx, feedID, page, limit)

	// If we found episodes, return them
	if err == nil && len(episodes) > 0 {
		return episodes, total, nil
	}

	// No episodes in DB - try to fetch and sync from API
	log.Printf("[INFO] No episodes found for feed %d, fetching from Podcast Index API", feedID)

	// Fetch from API (this will also ensure podcast exists and sync episodes)
	_, fetchErr := s.FetchAndSyncEpisodes(ctx, feedID, 100) // Fetch up to 100 episodes
	if fetchErr != nil {
		// If API fetch fails, return the original error or empty result
		if err != nil {
			return nil, 0, err
		}
		return nil, 0, fetchErr
	}

	// Try again from repository after sync
	episodes, total, err = s.repository.GetEpisodesByPodcastIndexFeedID(ctx, feedID, page, limit)
	if err != nil {
		return nil, 0, err
	}

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

// Constants for default configuration
const (
	DefaultSyncTimeout        = 30 * time.Second
	DefaultMaxConcurrentSyncs = 5 // default max concurrent sync operations
)
