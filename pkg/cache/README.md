# Cache Package

This package provides caching utilities for the Podcast Player API.

## Features
- In-memory caching with TTL
- Cache key generation
- Cache invalidation
- Metrics collection
- Thread-safe operations

## Structure
- `cache.go` - Main cache interface and implementation
- `memory.go` - In-memory cache implementation
- `keys.go` - Cache key generation utilities
- `metrics.go` - Cache hit/miss metrics

## Cache Layers
1. In-memory cache for hot data
2. File-based cache for larger data (waveforms, etc.)
3. API response caching

## Usage
```go
cache := cache.NewMemoryCache(defaultTTL)
cache.Set("key", value, ttl)
value, found := cache.Get("key")
```