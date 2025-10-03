package podcasts

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/podcastindex"
	"gorm.io/datatypes"
)

type Service struct {
	repository   PodcastRepository
	piClient     *podcastindex.Client
	refreshAfter time.Duration // How old before auto-refresh
}

func NewService(repository PodcastRepository, piClient *podcastindex.Client) PodcastService {
	return &Service{
		repository:   repository,
		piClient:     piClient,
		refreshAfter: 24 * time.Hour, // Refresh if older than 24 hours
	}
}

// GetPodcastByPodcastIndexID - DB-first lookup with automatic API fallback
func (s *Service) GetPodcastByPodcastIndexID(ctx context.Context, piID int64) (*models.Podcast, error) {
	// Check database first
	podcast, err := s.repository.GetPodcastByPodcastIndexID(ctx, piID)
	if err == nil {
		// Found in DB - check if needs refresh
		if s.ShouldRefresh(podcast) && s.piClient != nil {
			log.Printf("[INFO] Podcast %d is stale, refreshing in background", piID)
			// Async refresh - don't block the request
			go func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("[ERROR] Panic refreshing podcast %d: %v", piID, r)
					}
				}()
				if _, refreshErr := s.RefreshPodcast(context.Background(), piID); refreshErr != nil {
					log.Printf("[WARN] Failed to refresh podcast %d: %v", piID, refreshErr)
				}
			}()
		}

		// Update last fetched timestamp
		_ = s.repository.UpdateLastFetched(ctx, podcast.ID)
		_ = s.repository.IncrementFetchCount(ctx, podcast.ID)

		return podcast, nil
	}

	// Not in DB - fetch from API and store
	log.Printf("[INFO] Podcast %d not in DB, fetching from API", piID)
	return s.FetchAndStorePodcast(ctx, piID)
}

// FetchAndStorePodcast fetches podcast from Podcast Index API and stores in DB
func (s *Service) FetchAndStorePodcast(ctx context.Context, piID int64) (*models.Podcast, error) {
	if s.piClient == nil {
		return nil, fmt.Errorf("podcast index client not available")
	}

	// Fetch from Podcast Index API
	response, err := s.piClient.GetPodcastByID(ctx, piID)
	if err != nil {
		return nil, fmt.Errorf("fetching from Podcast Index API: %w", err)
	}

	if response.Status != "true" || response.Feed.ID == 0 {
		return nil, fmt.Errorf("podcast not found in Podcast Index: status=%s", response.Status)
	}

	// Transform API response to our model
	podcast, err := s.transformFromPodcastIndex(&response.Feed)
	if err != nil {
		return nil, fmt.Errorf("transforming podcast data: %w", err)
	}

	// Set tracking fields
	now := time.Now()
	podcast.LastFetchedAt = &now
	podcast.FetchCount = 1

	// Store in database
	if err := s.repository.UpsertPodcast(ctx, podcast); err != nil {
		return nil, fmt.Errorf("storing podcast in database: %w", err)
	}

	log.Printf("[INFO] Stored podcast %d: %s", piID, podcast.Title)

	return podcast, nil
}

// RefreshPodcast refreshes podcast data from Podcast Index API
func (s *Service) RefreshPodcast(ctx context.Context, piID int64) (*models.Podcast, error) {
	log.Printf("[INFO] Refreshing podcast %d from API", piID)

	// Get existing podcast to preserve ID and CreatedAt
	existing, err := s.repository.GetPodcastByPodcastIndexID(ctx, piID)
	if err != nil {
		// If not in DB, just fetch and store
		return s.FetchAndStorePodcast(ctx, piID)
	}

	// Fetch fresh data from API
	response, err := s.piClient.GetPodcastByID(ctx, piID)
	if err != nil {
		return nil, fmt.Errorf("fetching from Podcast Index API: %w", err)
	}

	// Transform to our model
	podcast, err := s.transformFromPodcastIndex(&response.Feed)
	if err != nil {
		return nil, fmt.Errorf("transforming podcast data: %w", err)
	}

	// Preserve database ID and timestamps
	podcast.ID = existing.ID
	podcast.CreatedAt = existing.CreatedAt

	// Update tracking fields
	now := time.Now()
	podcast.LastFetchedAt = &now
	podcast.FetchCount = existing.FetchCount + 1

	// Update in database
	if err := s.repository.UpdatePodcast(ctx, podcast); err != nil {
		return nil, fmt.Errorf("updating podcast in database: %w", err)
	}

	log.Printf("[INFO] Refreshed podcast %d: %s", piID, podcast.Title)

	return podcast, nil
}

// ShouldRefresh checks if podcast data needs refreshing
func (s *Service) ShouldRefresh(podcast *models.Podcast) bool {
	if podcast.LastFetchedAt == nil {
		return true
	}

	return time.Since(*podcast.LastFetchedAt) > s.refreshAfter
}

// GetByID gets podcast by database ID
func (s *Service) GetByID(ctx context.Context, id uint) (*models.Podcast, error) {
	return s.repository.GetPodcastByID(ctx, id)
}

// UpdatePodcastMetrics updates podcast metrics like episode count
func (s *Service) UpdatePodcastMetrics(ctx context.Context, podcastID uint, episodeCount int) error {
	podcast, err := s.repository.GetPodcastByID(ctx, podcastID)
	if err != nil {
		return err
	}

	podcast.EpisodeCount = episodeCount
	return s.repository.UpdatePodcast(ctx, podcast)
}

// transformFromPodcastIndex transforms Podcast Index API response to our model
func (s *Service) transformFromPodcastIndex(piFeed *podcastindex.Podcast) (*models.Podcast, error) {
	// Convert categories map to JSON
	categoriesJSON, err := json.Marshal(piFeed.Categories)
	if err != nil {
		log.Printf("[WARN] Failed to marshal categories for podcast %d: %v", piFeed.ID, err)
		categoriesJSON = []byte("{}")
	}

	// Convert timestamps
	var lastUpdateTime, lastCrawlTime, lastParseTime *time.Time
	if piFeed.LastUpdateTime > 0 {
		t := time.Unix(piFeed.LastUpdateTime, 0)
		lastUpdateTime = &t
	}
	if piFeed.LastCrawlTime > 0 {
		t := time.Unix(piFeed.LastCrawlTime, 0)
		lastCrawlTime = &t
	}
	if piFeed.LastParseTime > 0 {
		t := time.Unix(piFeed.LastParseTime, 0)
		lastParseTime = &t
	}

	// Handle iTunes ID (0 means no iTunes ID)
	var itunesID *int64
	if piFeed.ITunesID > 0 {
		id := int64(piFeed.ITunesID)
		itunesID = &id
	}

	podcast := &models.Podcast{
		PodcastIndexID:   int64(piFeed.ID),
		Title:            piFeed.Title,
		Author:           piFeed.Author,
		OwnerName:        piFeed.OwnerName,
		Description:      piFeed.Description,
		FeedURL:          piFeed.URL,
		OriginalURL:      piFeed.OriginalURL,
		Link:             piFeed.Link,
		Image:            piFeed.Image,
		Artwork:          piFeed.Artwork,
		ITunesID:         itunesID,
		Language:         piFeed.Language,
		Categories:       datatypes.JSON(categoriesJSON),
		EpisodeCount:     piFeed.EpisodeCount,
		LastUpdateTime:   lastUpdateTime,
		LastCrawlTime:    lastCrawlTime,
		LastParseTime:    lastParseTime,
		LastGoodHTTPCode: piFeed.LastGoodHTTPCode,
		ImageURLHash:     int64(piFeed.ImageURLHash),
		Locked:           piFeed.Locked,
	}

	return podcast, nil
}
