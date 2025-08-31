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
	stopCh   chan struct{}
	stopped  bool
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
		stopCh:   make(chan struct{}),
		stopped:  false,
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

	for {
		select {
		case <-ticker.C:
			c.mu.Lock()
			now := time.Now()
			for key, entry := range c.episodes {
				if entry.expiresAt.Before(now) {
					delete(c.episodes, key)
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

// Stop gracefully stops the cache cleanup goroutine
func (c *Cache) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.stopped {
		close(c.stopCh)
		c.stopped = true
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
