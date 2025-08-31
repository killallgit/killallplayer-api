package podcastindex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	cfg := Config{
		APIKey:    "test-key",
		APISecret: "test-secret",
		BaseURL:   "https://api.example.com",
		UserAgent: "TestAgent/1.0",
		Timeout:   10 * time.Second,
	}

	client := NewClient(cfg)

	if client.apiKey != cfg.APIKey {
		t.Errorf("Expected apiKey %s, got %s", cfg.APIKey, client.apiKey)
	}
	if client.apiSecret != cfg.APISecret {
		t.Errorf("Expected apiSecret %s, got %s", cfg.APISecret, client.apiSecret)
	}
	if client.baseURL != cfg.BaseURL {
		t.Errorf("Expected baseURL %s, got %s", cfg.BaseURL, client.baseURL)
	}
	if client.userAgent != cfg.UserAgent {
		t.Errorf("Expected userAgent %s, got %s", cfg.UserAgent, client.userAgent)
	}
}

func TestNewClientDefaults(t *testing.T) {
	cfg := Config{
		APIKey:    "test-key",
		APISecret: "test-secret",
		Timeout:   10 * time.Second,
	}

	client := NewClient(cfg)

	expectedBaseURL := "https://api.podcastindex.org/api/1.0"
	if client.baseURL != expectedBaseURL {
		t.Errorf("Expected default baseURL %s, got %s", expectedBaseURL, client.baseURL)
	}

	expectedUserAgent := "PodcastPlayerAPI/1.0"
	if client.userAgent != expectedUserAgent {
		t.Errorf("Expected default userAgent %s, got %s", expectedUserAgent, client.userAgent)
	}
}

func TestSearch(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.URL.Path != "/api/1.0/search/byterm" {
			t.Errorf("Expected path /api/1.0/search/byterm, got %s", r.URL.Path)
		}

		// Verify auth headers
		if r.Header.Get("X-Auth-Key") == "" {
			t.Error("Missing X-Auth-Key header")
		}
		if r.Header.Get("X-Auth-Date") == "" {
			t.Error("Missing X-Auth-Date header")
		}
		if r.Header.Get("Authorization") == "" {
			t.Error("Missing Authorization header")
		}
		if r.Header.Get("User-Agent") == "" {
			t.Error("Missing User-Agent header")
		}

		// Send mock response
		response := `{
			"status": "true",
			"feeds": [
				{
					"id": 123,
					"title": "Test Podcast",
					"author": "Test Author",
					"description": "Test Description",
					"image": "https://example.com/image.jpg",
					"url": "https://example.com/feed.xml"
				}
			],
			"count": 1,
			"query": "test",
			"description": "Found matching feeds"
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
	defer server.Close()

	// Create client with test server URL
	cfg := Config{
		APIKey:    "test-key",
		APISecret: "test-secret",
		BaseURL:   server.URL + "/api/1.0",
		UserAgent: "TestAgent/1.0",
		Timeout:   10 * time.Second,
	}
	client := NewClient(cfg)

	// Test search
	ctx := context.Background()
	resp, err := client.Search(ctx, "test", 10)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if resp.Status != "true" {
		t.Errorf("Expected status 'true', got %s", resp.Status)
	}
	if len(resp.Feeds) != 1 {
		t.Errorf("Expected 1 feed, got %d", len(resp.Feeds))
	}
	if resp.Feeds[0].Title != "Test Podcast" {
		t.Errorf("Expected title 'Test Podcast', got %s", resp.Feeds[0].Title)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	cfg := Config{
		APIKey:    "test-key",
		APISecret: "test-secret",
		Timeout:   10 * time.Second,
	}
	client := NewClient(cfg)

	ctx := context.Background()
	_, err := client.Search(ctx, "", 10)
	if err == nil {
		t.Error("Expected error for empty query, got nil")
	}
}
