package episodes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/killallgit/player-api/internal/services/podcastindex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPodcastIndexAdapter_GetEpisodesByPodcastID(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/api/1.0/episodes/byfeedid", r.URL.Path)
		assert.Equal(t, "123", r.URL.Query().Get("id"))
		assert.Equal(t, "10", r.URL.Query().Get("max"))

		// Verify auth headers
		assert.NotEmpty(t, r.Header.Get("X-Auth-Date"))
		assert.NotEmpty(t, r.Header.Get("X-Auth-Key"))
		assert.NotEmpty(t, r.Header.Get("Authorization"))
		assert.Equal(t, "TestAgent/1.0", r.Header.Get("User-Agent"))

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": "true",
			"count": 1,
			"items": [{
				"id": 1,
				"title": "Test Episode",
				"link": "https://example.com/episode1",
				"description": "Test description",
				"guid": "test-guid-1",
				"datePublished": 1234567890,
				"datePublishedPretty": "January 15, 2009",
				"dateCrawled": 1234567890,
				"enclosureUrl": "https://example.com/episode1.mp3",
				"enclosureType": "audio/mpeg",
				"enclosureLength": 12345678,
				"duration": 3600,
				"explicit": 0,
				"episode": 1,
				"episodeType": "full",
				"season": 1,
				"image": "https://example.com/image.jpg",
				"feedItunesId": 123456,
				"feedImage": "https://example.com/feed.jpg",
				"feedId": 123,
				"feedLanguage": "en",
				"feedDead": 0,
				"feedDuplicateOf": 0,
				"chaptersUrl": "",
				"transcriptUrl": ""
			}]
		}`))
	}))
	defer server.Close()

	// Create client and adapter
	client := podcastindex.NewClient(podcastindex.Config{
		APIKey:    "test-key",
		APISecret: "test-secret",
		BaseURL:   server.URL + "/api/1.0",
		UserAgent: "TestAgent/1.0",
		Timeout:   10 * time.Second,
	})
	adapter := NewPodcastIndexAdapter(client)

	// Test GetEpisodesByPodcastID
	response, err := adapter.GetEpisodesByPodcastID(context.Background(), 123, 10)

	// Verify response
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "true", response.Status)
	assert.Equal(t, 1, response.Count)
	assert.Len(t, response.Items, 1)

	episode := response.Items[0]
	assert.Equal(t, int64(1), episode.ID)
	assert.Equal(t, "Test Episode", episode.Title)
	assert.Equal(t, "test-guid-1", episode.GUID)
	assert.Equal(t, "https://example.com/episode1.mp3", episode.EnclosureURL)
	assert.Equal(t, int64(123), episode.FeedID)
}

func TestPodcastIndexAdapter_GetEpisodeByGUID(t *testing.T) {
	// Setup test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		assert.Equal(t, "/api/1.0/episodes/byguid", r.URL.Path)
		assert.Equal(t, "test-guid", r.URL.Query().Get("guid"))

		// Return mock response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"status": "true",
			"episode": {
				"id": 1,
				"title": "Test Episode",
				"link": "https://example.com/episode1",
				"description": "Test description",
				"guid": "test-guid",
				"datePublished": 1234567890,
				"datePublishedPretty": "January 15, 2009",
				"dateCrawled": 1234567890,
				"enclosureUrl": "https://example.com/episode1.mp3",
				"enclosureType": "audio/mpeg",
				"enclosureLength": 12345678,
				"duration": 3600,
				"explicit": 0,
				"episode": 1,
				"episodeType": "full",
				"season": 1,
				"image": "https://example.com/image.jpg",
				"feedItunesId": 123456,
				"feedImage": "https://example.com/feed.jpg",
				"feedId": 123,
				"feedLanguage": "en",
				"feedDead": 0,
				"feedDuplicateOf": 0,
				"chaptersUrl": "",
				"transcriptUrl": ""
			},
			"description": "Found matching episode"
		}`))
	}))
	defer server.Close()

	// Create client and adapter
	client := podcastindex.NewClient(podcastindex.Config{
		APIKey:    "test-key",
		APISecret: "test-secret",
		BaseURL:   server.URL + "/api/1.0",
		UserAgent: "TestAgent/1.0",
		Timeout:   10 * time.Second,
	})
	adapter := NewPodcastIndexAdapter(client)

	// Test GetEpisodeByGUID
	response, err := adapter.GetEpisodeByGUID(context.Background(), "test-guid")

	// Verify response
	require.NoError(t, err)
	assert.NotNil(t, response)
	assert.Equal(t, "true", response.Status)
	assert.NotNil(t, response.Episode)
	assert.Equal(t, "test-guid", response.Episode.GUID)
	assert.Equal(t, "Test Episode", response.Episode.Title)
}

func TestPodcastIndexAdapter_GetEpisodeMetadata(t *testing.T) {
	// Setup test server for episode metadata
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify HEAD request
		assert.Equal(t, "HEAD", r.Method)
		assert.Equal(t, "/episode.mp3", r.URL.Path)

		// Set headers
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Length", "12345678")
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create client and adapter
	client := podcastindex.NewClient(podcastindex.Config{
		APIKey:    "test-key",
		APISecret: "test-secret",
		BaseURL:   "https://api.podcastindex.org/api/1.0",
		UserAgent: "TestAgent/1.0",
		Timeout:   10 * time.Second,
	})
	adapter := NewPodcastIndexAdapter(client)

	// Test GetEpisodeMetadata
	metadata, err := adapter.GetEpisodeMetadata(context.Background(), server.URL+"/episode.mp3")

	// Verify metadata
	require.NoError(t, err)
	assert.NotNil(t, metadata)
	assert.Equal(t, server.URL+"/episode.mp3", metadata.URL)
	assert.Equal(t, "audio/mpeg", metadata.ContentType)
	assert.Equal(t, int64(12345678), metadata.Size)
	assert.Equal(t, "episode.mp3", metadata.FileName)
	assert.WithinDuration(t, time.Now(), metadata.LastModified, 2*time.Second)
}

func TestPodcastIndexAdapter_GetEpisodeMetadata_InvalidURL(t *testing.T) {
	// Create client and adapter
	client := podcastindex.NewClient(podcastindex.Config{
		APIKey:    "test-key",
		APISecret: "test-secret",
		BaseURL:   "https://api.podcastindex.org/api/1.0",
		UserAgent: "TestAgent/1.0",
		Timeout:   10 * time.Second,
	})
	adapter := NewPodcastIndexAdapter(client)

	// Test with invalid URL
	metadata, err := adapter.GetEpisodeMetadata(context.Background(), "://invalid-url")

	// Verify error
	assert.Error(t, err)
	assert.Nil(t, metadata)
	assert.Contains(t, err.Error(), "invalid URL format")
}

func TestPodcastIndexAdapter_ErrorResponses(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       string
		statusCode     int
		responseBody   string
		expectedError  string
		testFunc       func(*PodcastIndexAdapter) error
	}{
		{
			name:       "API returns error status",
			endpoint:   "/api/1.0/episodes/byfeedid",
			statusCode: http.StatusOK,
			responseBody: `{
				"status": "false",
				"description": "API key not valid"
			}`,
			expectedError: "API error",
			testFunc: func(adapter *PodcastIndexAdapter) error {
				_, err := adapter.GetEpisodesByPodcastID(context.Background(), 123, 10)
				return err
			},
		},
		{
			name:          "API returns 500 error",
			endpoint:      "/api/1.0/episodes/byguid",
			statusCode:    http.StatusInternalServerError,
			responseBody:  `{"error": "internal server error"}`,
			expectedError: "API returned status 500",
			testFunc: func(adapter *PodcastIndexAdapter) error {
				_, err := adapter.GetEpisodeByGUID(context.Background(), "test-guid")
				return err
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.endpoint, r.URL.Path)
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}))
			defer server.Close()

			// Create client and adapter
			client := podcastindex.NewClient(podcastindex.Config{
				APIKey:    "test-key",
				APISecret: "test-secret",
				BaseURL:   server.URL + "/api/1.0",
				UserAgent: "TestAgent/1.0",
				Timeout:   10 * time.Second,
			})
			adapter := NewPodcastIndexAdapter(client)

			// Test the function
			err := tt.testFunc(adapter.(*PodcastIndexAdapter))

			// Verify error
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

func TestPodcastIndexAdapter_ConvertEpisode(t *testing.T) {
	// Test the episode conversion logic
	adapter := &PodcastIndexAdapter{}

	// Create a sample podcastindex.Episode
	podcastEpisode := podcastindex.Episode{
		ID:                  1,
		Title:               "Test Episode",
		Link:                "https://example.com/episode",
		Description:         "Test description",
		GUID:                "test-guid",
		DatePublished:       1234567890,
		DatePublishedPretty: "January 15, 2009",
		DateCrawled:         1234567890,
		EnclosureURL:        "https://example.com/episode.mp3",
		EnclosureType:       "audio/mpeg",
		EnclosureLength:     12345678,
		Duration:            3600,
		Explicit:            0,
		Episode:             1,
		EpisodeType:         "full",
		Season:              1,
		Image:               "https://example.com/image.jpg",
		FeedItunesId:        123456,
		FeedImage:           "https://example.com/feed.jpg",
		FeedId:              123,
		FeedLanguage:        "en",
		FeedDead:            0,
		FeedDuplicateOf:     0,
		ChaptersURL:         "https://example.com/chapters.json",
		TranscriptURL:       "https://example.com/transcript.txt",
	}

	// Convert the episode
	internalEpisode := adapter.convertEpisode(podcastEpisode)

	// Verify conversion
	assert.Equal(t, int64(1), internalEpisode.ID)
	assert.Equal(t, "Test Episode", internalEpisode.Title)
	assert.Equal(t, "test-guid", internalEpisode.GUID)
	assert.Equal(t, "https://example.com/episode.mp3", internalEpisode.EnclosureURL)
	assert.Equal(t, int64(12345678), internalEpisode.EnclosureLength)
	assert.NotNil(t, internalEpisode.Duration)
	assert.Equal(t, 3600, *internalEpisode.Duration)
	assert.NotNil(t, internalEpisode.Episode)
	assert.Equal(t, 1, *internalEpisode.Episode)
	assert.NotNil(t, internalEpisode.Season)
	assert.Equal(t, 1, *internalEpisode.Season)
	assert.Equal(t, int64(123), internalEpisode.FeedID)
	assert.NotNil(t, internalEpisode.FeedItunesID)
	assert.Equal(t, int64(123456), *internalEpisode.FeedItunesID)
}

func TestPodcastIndexAdapter_ConvertEpisodeWithZeroValues(t *testing.T) {
	// Test conversion with zero values (should not set pointers)
	adapter := &PodcastIndexAdapter{}

	podcastEpisode := podcastindex.Episode{
		ID:              1,
		Title:           "Test Episode",
		GUID:            "test-guid",
		EnclosureURL:    "https://example.com/episode.mp3",
		FeedId:          123,
		Duration:        0, // Zero value
		Episode:         0, // Zero value
		Season:          0, // Zero value
		FeedItunesId:    0, // Zero value
		FeedDuplicateOf: 0, // Zero value
	}

	// Convert the episode
	internalEpisode := adapter.convertEpisode(podcastEpisode)

	// Verify zero values are nil
	assert.Nil(t, internalEpisode.Duration)
	assert.Nil(t, internalEpisode.Episode)
	assert.Nil(t, internalEpisode.Season)
	assert.Nil(t, internalEpisode.FeedItunesID)
	assert.Nil(t, internalEpisode.FeedDuplicateOf)
}