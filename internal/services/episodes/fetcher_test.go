package episodes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/killallgit/player-api/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetcher_GetEpisodesByPodcastID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/1.0/episodes/byfeedid", r.URL.Path)
		assert.Equal(t, "123", r.URL.Query().Get("id"))
		assert.Equal(t, "10", r.URL.Query().Get("max"))
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": "true",
			"items": [
				{
					"id": 1,
					"title": "Episode 1",
					"description": "First episode",
					"enclosureUrl": "https://example.com/episode1.mp3",
					"duration": 3600,
					"datePublished": 1609459200,
					"guid": "guid-1"
				},
				{
					"id": 2,
					"title": "Episode 2",
					"description": "Second episode",
					"enclosureUrl": "https://example.com/episode2.mp3",
					"duration": 1800,
					"datePublished": 1609545600,
					"guid": "guid-2"
				}
			]
		}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		PodcastIndex: config.PodcastIndexConfig{
			APIKey:    "test-key",
			APISecret: "test-secret",
			BaseURL:    server.URL + "/api/1.0",
			Timeout:   5 * time.Second,
		},
	}

	fetcher := NewFetcher(cfg)
	episodes, err := fetcher.GetEpisodesByPodcastID(context.Background(), 123, 10)

	require.NoError(t, err)
	assert.Len(t, episodes, 2)

	assert.Equal(t, "Episode 1", episodes[0].Title)
	assert.Equal(t, "First episode", episodes[0].Description)
	assert.Equal(t, "https://example.com/episode1.mp3", episodes[0].AudioURL)
	assert.Equal(t, 3600, episodes[0].Duration)
	assert.Equal(t, "guid-1", episodes[0].GUID)
	assert.Equal(t, time.Unix(1609459200, 0), episodes[0].PublishedAt)

	assert.Equal(t, "Episode 2", episodes[1].Title)
	assert.Equal(t, "https://example.com/episode2.mp3", episodes[1].AudioURL)
}

func TestFetcher_GetEpisodeByGUID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/1.0/episodes/byguid", r.URL.Path)
		assert.Equal(t, "test-guid", r.URL.Query().Get("guid"))
		
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": "true",
			"episode": {
				"id": 1,
				"title": "Test Episode",
				"description": "Test description",
				"enclosureUrl": "https://example.com/test.mp3",
				"duration": 2400,
				"datePublished": 1609459200,
				"guid": "test-guid"
			}
		}`))
	}))
	defer server.Close()

	cfg := &config.Config{
		PodcastIndex: config.PodcastIndexConfig{
			APIKey:    "test-key",
			APISecret: "test-secret",
			BaseURL:    server.URL + "/api/1.0",
			Timeout:   5 * time.Second,
		},
	}

	fetcher := NewFetcher(cfg)
	episode, err := fetcher.GetEpisodeByGUID(context.Background(), "test-guid")

	require.NoError(t, err)
	require.NotNil(t, episode)

	assert.Equal(t, "Test Episode", episode.Title)
	assert.Equal(t, "Test description", episode.Description)
	assert.Equal(t, "https://example.com/test.mp3", episode.AudioURL)
	assert.Equal(t, 2400, episode.Duration)
	assert.Equal(t, "test-guid", episode.GUID)
	assert.Equal(t, time.Unix(1609459200, 0), episode.PublishedAt)
}

func TestFetcher_GetEpisodeMetadata(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "HEAD", r.Method)
		
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Length", "5242880")
		w.Header().Set("Last-Modified", "Mon, 02 Jan 2006 15:04:05 GMT")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &config.Config{
		PodcastIndex: config.PodcastIndexConfig{
			APIKey:    "test-key",
			APISecret: "test-secret",
			BaseURL:    "https://api.podcastindex.org/api/1.0",
			Timeout:   5 * time.Second,
		},
	}

	fetcher := NewFetcher(cfg)
	metadata, err := fetcher.GetEpisodeMetadata(context.Background(), server.URL+"/episode.mp3")

	require.NoError(t, err)
	require.NotNil(t, metadata)

	assert.Equal(t, server.URL+"/episode.mp3", metadata.URL)
	assert.Equal(t, "audio/mpeg", metadata.ContentType)
	assert.Equal(t, int64(5242880), metadata.Size)
	assert.Equal(t, "Mon, 02 Jan 2006 15:04:05 GMT", metadata.LastModified.Format(http.TimeFormat))
}

func TestFetcher_ErrorHandling(t *testing.T) {
	t.Run("API returns error status", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"status": "false",
				"description": "Invalid API key"
			}`))
		}))
		defer server.Close()

		cfg := &config.Config{
			PodcastIndex: config.PodcastIndexConfig{
				APIKey:    "invalid-key",
				APISecret: "invalid-secret",
				BaseURL:    server.URL + "/api/1.0",
				Timeout:   5 * time.Second,
			},
		}

		fetcher := NewFetcher(cfg)
		_, err := fetcher.GetEpisodesByPodcastID(context.Background(), 123, 10)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "Invalid API key")
	})

	t.Run("HTTP error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Internal Server Error"))
		}))
		defer server.Close()

		cfg := &config.Config{
			PodcastIndex: config.PodcastIndexConfig{
				APIKey:    "test-key",
				APISecret: "test-secret",
				BaseURL:    server.URL + "/api/1.0",
				Timeout:   5 * time.Second,
			},
		}

		fetcher := NewFetcher(cfg)
		_, err := fetcher.GetEpisodesByPodcastID(context.Background(), 123, 10)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "API returned status 500")
	})
}