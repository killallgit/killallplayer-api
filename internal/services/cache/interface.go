package cache

import (
	"context"
	"time"
)

// Cache defines the interface for cache implementations
type Cache interface {
	// Get retrieves a value from the cache
	Get(ctx context.Context, key string) ([]byte, bool)

	// Set stores a value in the cache with a TTL
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a value from the cache
	Delete(ctx context.Context, key string) error

	// Clear removes all values from the cache
	Clear(ctx context.Context) error

	// Has checks if a key exists in the cache
	Has(ctx context.Context, key string) bool
}

// CacheStats provides statistics about cache usage
type CacheStats struct {
	Hits      int64
	Misses    int64
	Sets      int64
	Deletes   int64
	Evictions int64
	Size      int64
	MaxSize   int64
}

// StatsProvider interface for caches that provide statistics
type StatsProvider interface {
	Stats() CacheStats
}
