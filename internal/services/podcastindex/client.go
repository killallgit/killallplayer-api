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

	// Build URL with query parameters
	endpoint := fmt.Sprintf("%s/podcasts/trending?max=%d", c.baseURL, limit)

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
