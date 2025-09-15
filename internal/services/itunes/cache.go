package itunes

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Cache defines the interface for caching iTunes API responses
type Cache interface {
	Get(key string) ([]byte, bool)
	Set(key string, value []byte, ttl time.Duration)
	Delete(key string)
	Clear()
}

// MemoryCache implements an in-memory cache with TTL support
type MemoryCache struct {
	mu       sync.RWMutex
	items    map[string]*cacheItem
	stopChan chan struct{}
}

type cacheItem struct {
	data      []byte
	expiresAt time.Time
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache() *MemoryCache {
	cache := &MemoryCache{
		items:    make(map[string]*cacheItem),
		stopChan: make(chan struct{}),
	}

	// Start cleanup goroutine
	go cache.cleanup()

	return cache
}

// Get retrieves a value from the cache
func (c *MemoryCache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, exists := c.items[key]
	if !exists {
		return nil, false
	}

	// Check if item has expired
	if time.Now().After(item.expiresAt) {
		return nil, false
	}

	return item.data, true
}

// Set stores a value in the cache with a TTL
func (c *MemoryCache) Set(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items[key] = &cacheItem{
		data:      value,
		expiresAt: time.Now().Add(ttl),
	}
}

// Delete removes a value from the cache
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.items, key)
}

// Clear removes all items from the cache
func (c *MemoryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.items = make(map[string]*cacheItem)
}

// Stop stops the cleanup goroutine
func (c *MemoryCache) Stop() {
	close(c.stopChan)
}

// cleanup periodically removes expired items from the cache
func (c *MemoryCache) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.removeExpired()
		case <-c.stopChan:
			return
		}
	}
}

// removeExpired removes all expired items from the cache
func (c *MemoryCache) removeExpired() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	for key, item := range c.items {
		if now.After(item.expiresAt) {
			delete(c.items, key)
		}
	}
}

// CachedClient wraps a Client with caching functionality
type CachedClient struct {
	*Client
	cache    Cache
	cacheTTL time.Duration
}

// NewCachedClient creates a new iTunes client with caching
func NewCachedClient(cfg Config, cache Cache, cacheTTL time.Duration) *CachedClient {
	if cache == nil {
		cache = NewMemoryCache()
	}

	if cacheTTL == 0 {
		cacheTTL = 10 * time.Minute
	}

	return &CachedClient{
		Client:   NewClient(cfg),
		cache:    cache,
		cacheTTL: cacheTTL,
	}
}

// LookupPodcast fetches podcast metadata with caching
func (c *CachedClient) LookupPodcast(ctx context.Context, iTunesID int64) (*Podcast, error) {
	cacheKey := fmt.Sprintf("podcast:%d", iTunesID)

	// Check cache
	if data, found := c.cache.Get(cacheKey); found {
		c.metrics.cacheHits.Add(1)

		var podcast Podcast
		if err := json.Unmarshal(data, &podcast); err == nil {
			return &podcast, nil
		}
	}

	c.metrics.cacheMisses.Add(1)

	// Fetch from API
	podcast, err := c.Client.LookupPodcast(ctx, iTunesID)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if data, err := json.Marshal(podcast); err == nil {
		c.cache.Set(cacheKey, data, c.cacheTTL)
	}

	return podcast, nil
}

// LookupPodcastWithEpisodes fetches podcast and episodes with caching
func (c *CachedClient) LookupPodcastWithEpisodes(ctx context.Context, iTunesID int64, limit int) (*PodcastWithEpisodes, error) {
	cacheKey := fmt.Sprintf("podcast_episodes:%d:%d", iTunesID, limit)

	// Check cache
	if data, found := c.cache.Get(cacheKey); found {
		c.metrics.cacheHits.Add(1)

		var result PodcastWithEpisodes
		if err := json.Unmarshal(data, &result); err == nil {
			return &result, nil
		}
	}

	c.metrics.cacheMisses.Add(1)

	// Fetch from API
	result, err := c.Client.LookupPodcastWithEpisodes(ctx, iTunesID, limit)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if data, err := json.Marshal(result); err == nil {
		c.cache.Set(cacheKey, data, c.cacheTTL)
	}

	return result, nil
}

// Search searches for podcasts with caching
func (c *CachedClient) Search(ctx context.Context, term string, opts *SearchOptions) (*SearchResults, error) {
	// Create cache key from search parameters
	cacheKey := fmt.Sprintf("search:%s", term)
	if opts != nil {
		if opts.Country != "" {
			cacheKey += ":" + opts.Country
		}
		if opts.Limit > 0 {
			cacheKey += fmt.Sprintf(":limit%d", opts.Limit)
		}
	}

	// Check cache
	if data, found := c.cache.Get(cacheKey); found {
		c.metrics.cacheHits.Add(1)

		var results SearchResults
		if err := json.Unmarshal(data, &results); err == nil {
			return &results, nil
		}
	}

	c.metrics.cacheMisses.Add(1)

	// Fetch from API
	results, err := c.Client.Search(ctx, term, opts)
	if err != nil {
		return nil, err
	}

	// Cache the result
	if data, err := json.Marshal(results); err == nil {
		c.cache.Set(cacheKey, data, c.cacheTTL)
	}

	return results, nil
}

// ClearCache clears all cached items
func (c *CachedClient) ClearCache() {
	c.cache.Clear()
}
