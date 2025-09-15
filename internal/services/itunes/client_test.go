package itunes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClient_LookupPodcast(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request
		if r.URL.Path != "/lookup" {
			t.Errorf("Expected path /lookup, got %s", r.URL.Path)
		}

		if r.URL.Query().Get("id") != "1469663053" {
			t.Errorf("Expected id=1469663053, got %s", r.URL.Query().Get("id"))
		}

		// Return mock response
		response := `{
			"resultCount": 1,
			"results": [{
				"wrapperType": "track",
				"kind": "podcast",
				"collectionId": 1469663053,
				"trackId": 1469663053,
				"artistName": "W. Curtis Preston (Mr. Backup)",
				"collectionName": "The Backup Wrap-Up",
				"trackName": "The Backup Wrap-Up",
				"collectionViewUrl": "https://podcasts.apple.com/us/podcast/the-backup-wrap-up/id1469663053?uo=4",
				"feedUrl": "https://feeds.captivate.fm/backupwrapup/",
				"artworkUrl600": "https://example.com/artwork.jpg",
				"trackCount": 317,
				"releaseDate": "2025-09-01T11:00:00Z",
				"primaryGenreName": "Technology",
				"country": "USA"
			}]
		}`

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client with test server URL
	client := NewClient(Config{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	// Test lookup
	ctx := context.Background()
	podcast, err := client.LookupPodcast(ctx, 1469663053)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if podcast == nil {
		t.Fatal("Expected podcast, got nil")
	}

	// Verify podcast data
	if podcast.ID != 1469663053 {
		t.Errorf("Expected ID 1469663053, got %d", podcast.ID)
	}

	if podcast.Title != "The Backup Wrap-Up" {
		t.Errorf("Expected title 'The Backup Wrap-Up', got %s", podcast.Title)
	}

	if podcast.Author != "W. Curtis Preston (Mr. Backup)" {
		t.Errorf("Expected author 'W. Curtis Preston (Mr. Backup)', got %s", podcast.Author)
	}

	if podcast.FeedURL != "https://feeds.captivate.fm/backupwrapup/" {
		t.Errorf("Expected feed URL, got %s", podcast.FeedURL)
	}
}

func TestClient_LookupPodcastWithEpisodes(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check request
		if r.URL.Query().Get("entity") != "podcastEpisode" {
			t.Errorf("Expected entity=podcastEpisode, got %s", r.URL.Query().Get("entity"))
		}

		// Return mock response with podcast and episode
		response := `{
			"resultCount": 2,
			"results": [{
				"wrapperType": "track",
				"kind": "podcast",
				"collectionId": 1469663053,
				"trackId": 1469663053,
				"artistName": "W. Curtis Preston",
				"collectionName": "The Backup Wrap-Up",
				"feedUrl": "https://feeds.captivate.fm/backupwrapup/",
				"artworkUrl600": "https://example.com/artwork.jpg"
			}, {
				"kind": "podcast-episode",
				"trackId": 1000725508157,
				"trackName": "Test Episode",
				"collectionId": 1469663053,
				"description": "Test episode description",
				"episodeUrl": "https://example.com/episode.mp3",
				"trackTimeMillis": 2508000,
				"releaseDate": "2025-09-08T11:00:00Z",
				"episodeGuid": "test-guid-123"
			}]
		}`

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client
	client := NewClient(Config{
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	// Test lookup
	ctx := context.Background()
	result, err := client.LookupPodcastWithEpisodes(ctx, 1469663053, 10)

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result == nil {
		t.Fatal("Expected result, got nil")
	}

	if result.Podcast == nil {
		t.Fatal("Expected podcast, got nil")
	}

	if result.Podcast.ID != 1469663053 {
		t.Errorf("Expected podcast ID 1469663053, got %d", result.Podcast.ID)
	}

	if len(result.Episodes) != 1 {
		t.Errorf("Expected 1 episode, got %d", len(result.Episodes))
	}

	if result.Episodes[0].Title != "Test Episode" {
		t.Errorf("Expected episode title 'Test Episode', got %s", result.Episodes[0].Title)
	}
}

func TestClient_RateLimiting(t *testing.T) {
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		response := `{"resultCount": 0, "results": []}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client with very low rate limit
	client := NewClient(Config{
		BaseURL:           server.URL,
		RequestsPerMinute: 60, // 1 per second
		BurstSize:         1,
		Timeout:           5 * time.Second,
	})

	ctx := context.Background()
	start := time.Now()

	// Make 3 rapid requests
	for i := 0; i < 3; i++ {
		_, _ = client.LookupPodcast(ctx, int64(i))
	}

	elapsed := time.Since(start)

	// Should take at least 2 seconds due to rate limiting (3 requests at 1/sec)
	if elapsed < 2*time.Second {
		t.Errorf("Expected rate limiting to slow requests, took %v", elapsed)
	}

	if requestCount != 3 {
		t.Errorf("Expected 3 requests, got %d", requestCount)
	}
}

func TestClient_RetryOnError(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++

		// Fail first 2 attempts
		if attemptCount < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		// Succeed on third attempt
		response := `{
			"resultCount": 1,
			"results": [{
				"kind": "podcast",
				"collectionId": 123,
				"collectionName": "Test Podcast"
			}]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client with fast retry
	client := NewClient(Config{
		BaseURL:      server.URL,
		MaxRetries:   3,
		RetryBackoff: 10 * time.Millisecond,
		Timeout:      5 * time.Second,
	})

	ctx := context.Background()
	podcast, err := client.LookupPodcast(ctx, 123)

	if err != nil {
		t.Fatalf("Expected successful retry, got error: %v", err)
	}

	if podcast == nil {
		t.Fatal("Expected podcast after retry, got nil")
	}

	if attemptCount != 3 {
		t.Errorf("Expected 3 attempts (2 failures + 1 success), got %d", attemptCount)
	}
}

func TestMemoryCache(t *testing.T) {
	cache := NewMemoryCache()
	defer cache.Stop()

	// Test Set and Get
	key := "test-key"
	value := []byte("test-value")
	cache.Set(key, value, 1*time.Second)

	retrieved, found := cache.Get(key)
	if !found {
		t.Error("Expected to find cached value")
	}

	if string(retrieved) != string(value) {
		t.Errorf("Expected %s, got %s", value, retrieved)
	}

	// Test expiration
	time.Sleep(1100 * time.Millisecond)

	_, found = cache.Get(key)
	if found {
		t.Error("Expected cached value to be expired")
	}

	// Test Delete
	cache.Set(key, value, 1*time.Hour)
	cache.Delete(key)

	_, found = cache.Get(key)
	if found {
		t.Error("Expected value to be deleted")
	}

	// Test Clear
	cache.Set("key1", []byte("value1"), 1*time.Hour)
	cache.Set("key2", []byte("value2"), 1*time.Hour)
	cache.Clear()

	_, found1 := cache.Get("key1")
	_, found2 := cache.Get("key2")

	if found1 || found2 {
		t.Error("Expected cache to be cleared")
	}
}

func TestCachedClient(t *testing.T) {
	hitCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hitCount++
		response := `{
			"resultCount": 1,
			"results": [{
				"kind": "podcast",
				"collectionId": 456,
				"collectionName": "Cached Podcast"
			}]
		}`
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	// Create cached client
	cache := NewMemoryCache()
	defer cache.Stop()

	client := NewCachedClient(
		Config{
			BaseURL: server.URL,
			Timeout: 5 * time.Second,
		},
		cache,
		1*time.Hour,
	)

	ctx := context.Background()

	// First request should hit the server
	podcast1, err := client.LookupPodcast(ctx, 456)
	if err != nil {
		t.Fatalf("First request failed: %v", err)
	}

	// Second request should use cache
	podcast2, err := client.LookupPodcast(ctx, 456)
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}

	// Verify both requests returned same data
	if podcast1.ID != podcast2.ID {
		t.Error("Expected same podcast from cache")
	}

	// Verify server was only hit once
	if hitCount != 1 {
		t.Errorf("Expected 1 server hit, got %d", hitCount)
	}

	// Verify cache metrics
	metrics := client.GetMetrics()
	if metrics["cache_hits"] != 1 {
		t.Errorf("Expected 1 cache hit, got %d", metrics["cache_hits"])
	}
	if metrics["cache_misses"] != 1 {
		t.Errorf("Expected 1 cache miss, got %d", metrics["cache_misses"])
	}
}
