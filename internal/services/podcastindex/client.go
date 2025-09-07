package podcastindex

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

// Client handles communication with the Podcast Index API
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	apiSecret  string
	userAgent  string
}

// Config holds configuration for the Podcast Index client
type Config struct {
	APIKey    string
	APISecret string
	BaseURL   string
	UserAgent string
	Timeout   time.Duration
}

// NewClient creates a new Podcast Index API client
func NewClient(cfg Config) *Client {
	httpClient := &http.Client{
		Timeout: cfg.Timeout,
	}

	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.podcastindex.org/api/1.0"
	}

	if cfg.UserAgent == "" {
		cfg.UserAgent = "PodcastPlayerAPI/1.0"
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    cfg.BaseURL,
		apiKey:     cfg.APIKey,
		apiSecret:  cfg.APISecret,
		userAgent:  cfg.UserAgent,
	}
}

// Search searches for podcasts by term
func (c *Client) Search(ctx context.Context, query string, limit int) (*SearchResponse, error) {
	if query == "" {
		return nil, fmt.Errorf("search query cannot be empty")
	}

	// Default and max limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	// Build URL with query parameters
	endpoint := fmt.Sprintf("%s/search/byterm?q=%s&max=%d",
		c.baseURL,
		url.QueryEscape(query),
		limit)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Sign the request
	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		// Log headers for debugging
		fmt.Printf("Request headers: %v\n", req.Header)
		fmt.Printf("Response status: %d\n", resp.StatusCode)
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Decode response
	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Check API status
	if searchResp.Status != "true" {
		return nil, fmt.Errorf("API returned error status: %s", searchResp.Description)
	}

	return &searchResp, nil
}

// GetTrending fetches trending podcasts from Podcast Index
func (c *Client) GetTrending(limit int) (*SearchResponse, error) {
	// Default and max limit
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	// Build URL with query parameters - default to English language
	endpoint := fmt.Sprintf("%s/podcasts/trending?max=%d&lang=en", c.baseURL, limit)

	// Create request
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Sign the request
	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Decode response
	var trendingResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&trendingResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Check API status
	if trendingResp.Status != "true" {
		return nil, fmt.Errorf("API returned error status: %s", trendingResp.Description)
	}

	return &trendingResp, nil
}

// GetCategories retrieves all supported podcast categories
func (c *Client) GetCategories() (*CategoriesResponse, error) {
	// Build URL
	endpoint := fmt.Sprintf("%s/categories/list", c.baseURL)

	// Create request
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Sign the request
	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Decode response
	var categoriesResp CategoriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&categoriesResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Check API status
	if categoriesResp.Status != "true" {
		return nil, fmt.Errorf("API returned error status: %s", categoriesResp.Description)
	}

	return &categoriesResp, nil
}

// GetEpisodesByPodcastID fetches episodes for a specific podcast
func (c *Client) GetEpisodesByPodcastID(ctx context.Context, podcastID int64, limit int) (*EpisodesResponse, error) {
	// Build URL with query parameters
	params := url.Values{}
	params.Set("id", fmt.Sprintf("%d", podcastID))
	if limit > 0 {
		params.Set("max", fmt.Sprintf("%d", limit))
	}

	endpoint := fmt.Sprintf("%s/episodes/byfeedid?%s", c.baseURL, params.Encode())

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Sign the request
	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Decode response
	var episodesResp EpisodesResponse
	if err := json.NewDecoder(resp.Body).Decode(&episodesResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Check API status
	if episodesResp.Status != "true" {
		return nil, fmt.Errorf("API error: %s", episodesResp.Description)
	}

	return &episodesResp, nil
}

// GetEpisodeByGUID fetches a single episode by GUID
func (c *Client) GetEpisodeByGUID(ctx context.Context, guid string) (*EpisodeByGUIDResponse, error) {
	// Build URL with query parameters
	params := url.Values{}
	params.Set("guid", guid)

	endpoint := fmt.Sprintf("%s/episodes/byguid?%s", c.baseURL, params.Encode())

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Sign the request
	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	// Decode response
	var episodeResp EpisodeByGUIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&episodeResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	// Check API status
	if episodeResp.Status != "true" {
		return nil, fmt.Errorf("API error: %s", episodeResp.Description)
	}

	return &episodeResp, nil
}

// GetPodcastByFeedURL fetches podcast information by feed URL
func (c *Client) GetPodcastByFeedURL(ctx context.Context, feedURL string) (*PodcastResponse, error) {
	if feedURL == "" {
		return nil, fmt.Errorf("feed URL cannot be empty")
	}

	params := url.Values{}
	params.Set("url", feedURL)

	endpoint := fmt.Sprintf("%s/podcasts/byfeedurl?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var podcastResp PodcastResponse
	if err := json.NewDecoder(resp.Body).Decode(&podcastResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if podcastResp.Status != "true" {
		return nil, fmt.Errorf("API error: %s", podcastResp.Description)
	}

	return &podcastResp, nil
}

// GetPodcastByFeedID fetches podcast information by feed ID
func (c *Client) GetPodcastByFeedID(ctx context.Context, feedID int64) (*PodcastResponse, error) {
	params := url.Values{}
	params.Set("id", fmt.Sprintf("%d", feedID))

	endpoint := fmt.Sprintf("%s/podcasts/byfeedid?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var podcastResp PodcastResponse
	if err := json.NewDecoder(resp.Body).Decode(&podcastResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if podcastResp.Status != "true" {
		return nil, fmt.Errorf("API error: %s", podcastResp.Description)
	}

	return &podcastResp, nil
}

// GetPodcastByiTunesID fetches podcast information by iTunes ID
func (c *Client) GetPodcastByiTunesID(ctx context.Context, itunesID int64) (*PodcastResponse, error) {
	params := url.Values{}
	params.Set("id", fmt.Sprintf("%d", itunesID))

	endpoint := fmt.Sprintf("%s/podcasts/byitunesid?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var podcastResp PodcastResponse
	if err := json.NewDecoder(resp.Body).Decode(&podcastResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if podcastResp.Status != "true" {
		return nil, fmt.Errorf("API error: %s", podcastResp.Description)
	}

	return &podcastResp, nil
}

// GetEpisodesByFeedURL fetches episodes for a podcast by feed URL
func (c *Client) GetEpisodesByFeedURL(ctx context.Context, feedURL string, limit int) (*EpisodesResponse, error) {
	if feedURL == "" {
		return nil, fmt.Errorf("feed URL cannot be empty")
	}

	params := url.Values{}
	params.Set("url", feedURL)
	if limit > 0 {
		params.Set("max", fmt.Sprintf("%d", limit))
	}

	endpoint := fmt.Sprintf("%s/episodes/byfeedurl?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var episodesResp EpisodesResponse
	if err := json.NewDecoder(resp.Body).Decode(&episodesResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if episodesResp.Status != "true" {
		return nil, fmt.Errorf("API error: %s", episodesResp.Description)
	}

	return &episodesResp, nil
}

// GetEpisodesByiTunesID fetches episodes for a podcast by iTunes ID
func (c *Client) GetEpisodesByiTunesID(ctx context.Context, itunesID int64, limit int) (*EpisodesResponse, error) {
	params := url.Values{}
	params.Set("id", fmt.Sprintf("%d", itunesID))
	if limit > 0 {
		params.Set("max", fmt.Sprintf("%d", limit))
	}

	endpoint := fmt.Sprintf("%s/episodes/byitunesid?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var episodesResp EpisodesResponse
	if err := json.NewDecoder(resp.Body).Decode(&episodesResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if episodesResp.Status != "true" {
		return nil, fmt.Errorf("API error: %s", episodesResp.Description)
	}

	return &episodesResp, nil
}

// GetRecentEpisodes fetches the most recent episodes globally
func (c *Client) GetRecentEpisodes(ctx context.Context, limit int) (*EpisodesResponse, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	params := url.Values{}
	params.Set("max", fmt.Sprintf("%d", limit))

	endpoint := fmt.Sprintf("%s/recent/episodes?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var episodesResp EpisodesResponse
	if err := json.NewDecoder(resp.Body).Decode(&episodesResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if episodesResp.Status != "true" {
		return nil, fmt.Errorf("API error: %s", episodesResp.Description)
	}

	return &episodesResp, nil
}

// GetRecentFeeds fetches the most recently updated feeds
func (c *Client) GetRecentFeeds(ctx context.Context, limit int) (*RecentFeedsResponse, error) {
	if limit <= 0 {
		limit = 25
	}
	if limit > 100 {
		limit = 100
	}

	params := url.Values{}
	params.Set("max", fmt.Sprintf("%d", limit))

	endpoint := fmt.Sprintf("%s/recent/feeds?%s", c.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	var feedsResp RecentFeedsResponse
	if err := json.NewDecoder(resp.Body).Decode(&feedsResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if feedsResp.Status != "true" {
		return nil, fmt.Errorf("API error: %s", feedsResp.Description)
	}

	return &feedsResp, nil
}
