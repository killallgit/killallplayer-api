package episodes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/killallgit/player-api/internal/models"
)

type Cache struct {
	episodes map[string]*cacheEntry
	mu       sync.RWMutex
	ttl      time.Duration
}

type cacheEntry struct {
	episode   *models.Episode
	episodes  []models.Episode
	total     int64
	expiresAt time.Time
}

func NewCache(ttl time.Duration) *Cache {
	cache := &Cache{
		episodes: make(map[string]*cacheEntry),
		ttl:      ttl,
	}
	
	go cache.cleanupExpired()
	
	return cache
}

func (c *Cache) GetEpisode(key string) (*models.Episode, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.episodes[key]
	if !exists || entry.expiresAt.Before(time.Now()) {
		return nil, false
	}
	
	return entry.episode, true
}

func (c *Cache) SetEpisode(key string, episode *models.Episode) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.episodes[key] = &cacheEntry{
		episode:   episode,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *Cache) GetEpisodeList(key string) ([]models.Episode, int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.episodes[key]
	if !exists || entry.expiresAt.Before(time.Now()) {
		return nil, 0, false
	}
	
	return entry.episodes, entry.total, true
}

func (c *Cache) SetEpisodeList(key string, episodes []models.Episode, total int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.episodes[key] = &cacheEntry{
		episodes:  episodes,
		total:     total,
		expiresAt: time.Now().Add(c.ttl),
	}
}

func (c *Cache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	delete(c.episodes, key)
}

func (c *Cache) InvalidatePattern(pattern string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	for key := range c.episodes {
		if matched, _ := matchPattern(pattern, key); matched {
			delete(c.episodes, key)
		}
	}
}

func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.episodes = make(map[string]*cacheEntry)
}

func (c *Cache) cleanupExpired() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		for key, entry := range c.episodes {
			if entry.expiresAt.Before(now) {
				delete(c.episodes, key)
			}
		}
		c.mu.Unlock()
	}
}

func (c *Cache) WarmCache(ctx context.Context, popularEpisodes []models.Episode) {
	for _, episode := range popularEpisodes {
		key := fmt.Sprintf("episode:id:%d", episode.ID)
		c.SetEpisode(key, &episode)
		
		if episode.GUID != "" {
			guidKey := fmt.Sprintf("episode:guid:%s", episode.GUID)
			c.SetEpisode(guidKey, &episode)
		}
	}
}

func matchPattern(pattern, str string) (bool, error) {
	if pattern == "*" {
		return true, nil
	}
	
	if len(pattern) > 0 && pattern[len(pattern)-1] == '*' {
		prefix := pattern[:len(pattern)-1]
		return len(str) >= len(prefix) && str[:len(prefix)] == prefix, nil
	}
	
	return pattern == str, nil
}

type CachedRepository struct {
	repo  *Repository
	cache *Cache
}

func NewCachedRepository(repo *Repository, cache *Cache) *CachedRepository {
	return &CachedRepository{
		repo:  repo,
		cache: cache,
	}
}

func (cr *CachedRepository) GetEpisodeByID(ctx context.Context, id uint) (*models.Episode, error) {
	key := fmt.Sprintf("episode:id:%d", id)
	
	if episode, found := cr.cache.GetEpisode(key); found {
		return episode, nil
	}
	
	episode, err := cr.repo.GetEpisodeByID(ctx, id)
	if err != nil {
		return nil, err
	}
	
	cr.cache.SetEpisode(key, episode)
	return episode, nil
}

func (cr *CachedRepository) GetEpisodeByGUID(ctx context.Context, guid string) (*models.Episode, error) {
	key := fmt.Sprintf("episode:guid:%s", guid)
	
	if episode, found := cr.cache.GetEpisode(key); found {
		return episode, nil
	}
	
	episode, err := cr.repo.GetEpisodeByGUID(ctx, guid)
	if err != nil {
		return nil, err
	}
	
	cr.cache.SetEpisode(key, episode)
	return episode, nil
}

func (cr *CachedRepository) GetEpisodesByPodcastID(ctx context.Context, podcastID uint, page, limit int) ([]models.Episode, int64, error) {
	key := fmt.Sprintf("episodes:podcast:%d:page:%d:limit:%d", podcastID, page, limit)
	
	if episodes, total, found := cr.cache.GetEpisodeList(key); found {
		return episodes, total, nil
	}
	
	episodes, total, err := cr.repo.GetEpisodesByPodcastID(ctx, podcastID, page, limit)
	if err != nil {
		return nil, 0, err
	}
	
	cr.cache.SetEpisodeList(key, episodes, total)
	return episodes, total, nil
}

func (cr *CachedRepository) CreateEpisode(ctx context.Context, episode *models.Episode) error {
	err := cr.repo.CreateEpisode(ctx, episode)
	if err != nil {
		return err
	}
	
	cr.cache.InvalidatePattern(fmt.Sprintf("episodes:podcast:%d:*", episode.PodcastID))
	return nil
}

func (cr *CachedRepository) UpdateEpisode(ctx context.Context, episode *models.Episode) error {
	err := cr.repo.UpdateEpisode(ctx, episode)
	if err != nil {
		return err
	}
	
	cr.cache.Invalidate(fmt.Sprintf("episode:id:%d", episode.ID))
	cr.cache.Invalidate(fmt.Sprintf("episode:guid:%s", episode.GUID))
	cr.cache.InvalidatePattern(fmt.Sprintf("episodes:podcast:%d:*", episode.PodcastID))
	return nil
}

func (cr *CachedRepository) DeleteEpisode(ctx context.Context, id uint) error {
	episode, err := cr.repo.GetEpisodeByID(ctx, id)
	if err != nil {
		return err
	}
	
	err = cr.repo.DeleteEpisode(ctx, id)
	if err != nil {
		return err
	}
	
	cr.cache.Invalidate(fmt.Sprintf("episode:id:%d", id))
	cr.cache.Invalidate(fmt.Sprintf("episode:guid:%s", episode.GUID))
	cr.cache.InvalidatePattern(fmt.Sprintf("episodes:podcast:%d:*", episode.PodcastID))
	return nil
}

func (cr *CachedRepository) GetRecentEpisodes(ctx context.Context, limit int) ([]models.Episode, error) {
	key := fmt.Sprintf("episodes:recent:limit:%d", limit)
	
	if episodes, _, found := cr.cache.GetEpisodeList(key); found {
		return episodes, nil
	}
	
	episodes, err := cr.repo.GetRecentEpisodes(ctx, limit)
	if err != nil {
		return nil, err
	}
	
	cr.cache.SetEpisodeList(key, episodes, int64(len(episodes)))
	return episodes, nil
}

func (cr *CachedRepository) MarkEpisodeAsPlayed(ctx context.Context, id uint, played bool) error {
	err := cr.repo.MarkEpisodeAsPlayed(ctx, id, played)
	if err != nil {
		return err
	}
	
	cr.cache.Invalidate(fmt.Sprintf("episode:id:%d", id))
	return nil
}

func (cr *CachedRepository) UpdatePlaybackPosition(ctx context.Context, id uint, position int) error {
	err := cr.repo.UpdatePlaybackPosition(ctx, id, position)
	if err != nil {
		return err
	}
	
	cr.cache.Invalidate(fmt.Sprintf("episode:id:%d", id))
	return nil
}

func (cr *CachedRepository) UpsertEpisode(ctx context.Context, episode *models.Episode) error {
	err := cr.repo.UpsertEpisode(ctx, episode)
	if err != nil {
		return err
	}
	
	cr.cache.Invalidate(fmt.Sprintf("episode:id:%d", episode.ID))
	cr.cache.Invalidate(fmt.Sprintf("episode:guid:%s", episode.GUID))
	cr.cache.InvalidatePattern(fmt.Sprintf("episodes:podcast:%d:*", episode.PodcastID))
	return nil
}