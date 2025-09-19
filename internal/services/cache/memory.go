package cache

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// MemoryCache implements an in-memory cache
type MemoryCache struct {
	mu          sync.RWMutex
	items       map[string]*cacheItem
	maxSizeMB   int64
	currentSize int64
	stats       CacheStats
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

type cacheItem struct {
	value  []byte
	expiry time.Time
	size   int64
}

// NewMemoryCache creates a new in-memory cache
func NewMemoryCache(maxSizeMB int64) *MemoryCache {
	mc := &MemoryCache{
		items:     make(map[string]*cacheItem),
		maxSizeMB: maxSizeMB * 1024 * 1024, // Convert MB to bytes
		stopCh:    make(chan struct{}),
	}

	// Start cleanup goroutine
	mc.wg.Add(1)
	go mc.cleanupExpired()

	return mc
}

// Get retrieves a value from the cache
func (mc *MemoryCache) Get(ctx context.Context, key string) ([]byte, bool) {
	mc.mu.RLock()
	item, exists := mc.items[key]
	mc.mu.RUnlock()

	if !exists {
		atomic.AddInt64(&mc.stats.Misses, 1)
		return nil, false
	}

	// Check if expired
	if time.Now().After(item.expiry) {
		_ = mc.Delete(ctx, key)
		atomic.AddInt64(&mc.stats.Misses, 1)
		return nil, false
	}

	atomic.AddInt64(&mc.stats.Hits, 1)
	return item.value, true
}

// Set stores a value in the cache with a TTL
func (mc *MemoryCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if ttl <= 0 {
		ttl = 30 * time.Minute // Default TTL
	}

	size := int64(len(key) + len(value))

	// Check if we need to make room
	mc.makeRoom(size)

	item := &cacheItem{
		value:  value,
		expiry: time.Now().Add(ttl),
		size:   size,
	}

	mc.mu.Lock()
	// If replacing, adjust size
	if oldItem, exists := mc.items[key]; exists {
		atomic.AddInt64(&mc.currentSize, -(oldItem.size))
	}
	mc.items[key] = item
	atomic.AddInt64(&mc.currentSize, size)
	mc.mu.Unlock()

	atomic.AddInt64(&mc.stats.Sets, 1)
	return nil
}

// Delete removes a value from the cache
func (mc *MemoryCache) Delete(ctx context.Context, key string) error {
	mc.mu.Lock()
	if item, exists := mc.items[key]; exists {
		delete(mc.items, key)
		atomic.AddInt64(&mc.currentSize, -(item.size))
		atomic.AddInt64(&mc.stats.Deletes, 1)
	}
	mc.mu.Unlock()
	return nil
}

// Clear removes all values from the cache
func (mc *MemoryCache) Clear(ctx context.Context) error {
	mc.mu.Lock()
	mc.items = make(map[string]*cacheItem)
	atomic.StoreInt64(&mc.currentSize, 0)
	mc.mu.Unlock()
	return nil
}

// Has checks if a key exists in the cache
func (mc *MemoryCache) Has(ctx context.Context, key string) bool {
	mc.mu.RLock()
	item, exists := mc.items[key]
	mc.mu.RUnlock()

	if exists && time.Now().Before(item.expiry) {
		return true
	}
	return false
}

// Stats returns cache statistics
func (mc *MemoryCache) Stats() CacheStats {
	stats := mc.stats
	stats.Size = atomic.LoadInt64(&mc.currentSize)
	stats.MaxSize = mc.maxSizeMB
	return stats
}

// Stop gracefully shuts down the cache
func (mc *MemoryCache) Stop() {
	close(mc.stopCh)
	mc.wg.Wait()
}

// cleanupExpired removes expired items periodically
func (mc *MemoryCache) cleanupExpired() {
	defer mc.wg.Done()
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			mc.removeExpired()
		case <-mc.stopCh:
			return
		}
	}
}

// removeExpired removes all expired items
func (mc *MemoryCache) removeExpired() {
	now := time.Now()
	mc.mu.Lock()
	for key, item := range mc.items {
		if now.After(item.expiry) {
			delete(mc.items, key)
			atomic.AddInt64(&mc.currentSize, -(item.size))
			atomic.AddInt64(&mc.stats.Evictions, 1)
		}
	}
	mc.mu.Unlock()
}

// makeRoom ensures there's room for new data by evicting old items if necessary
func (mc *MemoryCache) makeRoom(sizeNeeded int64) {
	currentSize := atomic.LoadInt64(&mc.currentSize)
	if mc.maxSizeMB <= 0 || currentSize+sizeNeeded <= mc.maxSizeMB {
		return
	}

	// Simple eviction: remove expired items first, then oldest items
	mc.removeExpired()

	// If still not enough room, remove items until we have space
	currentSize = atomic.LoadInt64(&mc.currentSize)
	if currentSize+sizeNeeded > mc.maxSizeMB {
		mc.mu.Lock()
		targetSize := mc.maxSizeMB - sizeNeeded
		for key, item := range mc.items {
			if atomic.LoadInt64(&mc.currentSize) <= targetSize {
				break
			}
			delete(mc.items, key)
			atomic.AddInt64(&mc.currentSize, -(item.size))
			atomic.AddInt64(&mc.stats.Evictions, 1)
		}
		mc.mu.Unlock()
	}
}
