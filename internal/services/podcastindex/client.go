package podcastindex

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
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

// makeAPIRequest is a helper method to reduce code duplication for API requests
func (c *Client) makeAPIRequest(ctx context.Context, endpoint string, result interface{}) error {
	fullURL := fmt.Sprintf("%s/%s", c.baseURL, endpoint)

	// Create a clean context that inherits deadlines but not values/metadata
	// This prevents auth middleware headers from propagating to external API calls
	cleanCtx := context.Background()
	if deadline, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		cleanCtx, cancel = context.WithDeadline(cleanCtx, deadline)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(cleanCtx, "GET", fullURL, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	signRequest(req, c.apiKey, c.apiSecret, c.userAgent)

	// Debug log the headers being sent (only Authorization for security)
	if authHeader := req.Header.Get("Authorization"); authHeader != "" {
		// Only log first 10 chars of auth header for security
		if len(authHeader) > 10 {
			log.Printf("[DEBUG] Sending Authorization header to Podcast Index: %.10s...", authHeader)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[ERROR] Podcast Index API returned status %d for %s", resp.StatusCode, fullURL)
		return fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}

	// Check for API error status if the result has Status field
	if statusObj, ok := result.(interface{ GetStatus() string }); ok {
		if statusObj.GetStatus() != "true" {
			if descObj, ok := result.(interface{ GetDescription() string }); ok {
				return fmt.Errorf("API error: %s", descObj.GetDescription())
			}
			return fmt.Errorf("API returned error status")
		}
	}

	return nil
}

// Search searches for podcasts by term
func (c *Client) Search(ctx context.Context, query string, limit int, fullText bool, val string, apOnly bool, clean bool) (*SearchResponse, error) {
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
	params := url.Values{}
	params.Set("q", query)
	params.Set("max", fmt.Sprintf("%d", limit))
	if fullText {
		params.Set("fulltext", "true")
	}
	if val != "" {
		params.Set("val", val)
	}
	if apOnly {
		params.Set("aponly", "true")
	}
	if clean {
		params.Set("clean", "true")
	}

	endpoint := fmt.Sprintf("%s/search/byterm?%s", c.baseURL, params.Encode())

	// Create a clean context that inherits deadlines but not values/metadata
	// This prevents auth middleware headers from propagating to external API calls
	cleanCtx := context.Background()
	if deadline, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		cleanCtx, cancel = context.WithDeadline(cleanCtx, deadline)
		defer cancel()
	}

	// Create request with clean context
	req, err := http.NewRequestWithContext(cleanCtx, "GET", endpoint, nil)
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
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(searchResp.Feeds) > 0 {
		fmt.Printf("DEBUG: Found %d podcasts\n", len(searchResp.Feeds))
	}

	// Check API status
	if searchResp.Status != "true" {
		return nil, fmt.Errorf("API returned error status: %s", searchResp.Description)
	}

	return &searchResp, nil
}

// GetTrending fetches trending podcasts from Podcast Index with optional filters
func (c *Client) GetTrending(ctx context.Context, max, since int, categories []string, lang string, fullText bool) (*SearchResponse, error) {
	// Default and max limit
	if max <= 0 {
		max = 10
	}
	if max > 100 {
		max = 100
	}

	// Build URL with query parameters
	params := url.Values{}
	params.Set("max", fmt.Sprintf("%d", max))

	if since > 0 {
		params.Set("since", fmt.Sprintf("%d", since))
	}

	if len(categories) > 0 {
		// Join categories with comma for the API
		params.Set("cat", strings.Join(categories, ","))
	}

	if lang != "" {
		params.Set("lang", lang)
	} else {
		params.Set("lang", "en") // Default to English
	}

	if fullText {
		params.Set("fulltext", "true")
	}

	endpoint := fmt.Sprintf("%s/podcasts/trending?%s", c.baseURL, params.Encode())

	// Create a clean context that inherits deadlines but not values/metadata
	// This prevents auth middleware headers from propagating to external API calls
	cleanCtx := context.Background()
	if deadline, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		cleanCtx, cancel = context.WithDeadline(cleanCtx, deadline)
		defer cancel()
	}

	// Create request with clean context
	req, err := http.NewRequestWithContext(cleanCtx, "GET", endpoint, nil)
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
	params.Set("id", fmt.Sprintf("%d", podcastID)) // API expects "id" not "feedId"
	if limit > 0 {
		params.Set("max", fmt.Sprintf("%d", limit))
	}

	endpoint := fmt.Sprintf("episodes/byfeedid?%s", params.Encode())

	// Debug log the endpoint
	log.Printf("[DEBUG] Calling Podcast Index API: %s/%s", c.baseURL, endpoint)

	var episodesResp EpisodesResponse
	if err := c.makeAPIRequest(ctx, endpoint, &episodesResp); err != nil {
		return nil, err
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

	// Create a clean context that inherits deadlines but not values/metadata
	// This prevents auth middleware headers from propagating to external API calls
	cleanCtx := context.Background()
	if deadline, ok := ctx.Deadline(); ok {
		var cancel context.CancelFunc
		cleanCtx, cancel = context.WithDeadline(cleanCtx, deadline)
		defer cancel()
	}

	// Create request with clean context
	req, err := http.NewRequestWithContext(cleanCtx, "GET", endpoint, nil)
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

	endpoint := fmt.Sprintf("episodes/byfeedurl?%s", params.Encode())

	var episodesResp EpisodesResponse
	if err := c.makeAPIRequest(ctx, endpoint, &episodesResp); err != nil {
		return nil, err
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

	endpoint := fmt.Sprintf("episodes/byitunesid?%s", params.Encode())

	var episodesResp EpisodesResponse
	if err := c.makeAPIRequest(ctx, endpoint, &episodesResp); err != nil {
		return nil, err
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

	endpoint := fmt.Sprintf("recent/episodes?%s", params.Encode())

	var episodesResp EpisodesResponse
	if err := c.makeAPIRequest(ctx, endpoint, &episodesResp); err != nil {
		return nil, err
	}

	if episodesResp.Status != "true" {
		return nil, fmt.Errorf("API error: %s", episodesResp.Description)
	}

	return &episodesResp, nil
}

// GetRandomEpisodes fetches random podcast episodes from the Podcast Index API
func (c *Client) GetRandomEpisodes(ctx context.Context, max int, lang string, notCategories []string) (*EpisodesResponse, error) {
	if max <= 0 {
		max = 10
	}
	if max > 100 {
		max = 100
	}

	if lang == "" {
		lang = "en"
	}

	params := url.Values{}
	params.Set("max", fmt.Sprintf("%d", max))
	params.Set("lang", lang)

	if len(notCategories) > 0 {
		params.Set("notcat", strings.Join(notCategories, ","))
	}

	endpoint := fmt.Sprintf("%s/episodes/random?%s", c.baseURL, params.Encode())

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

	// The random endpoint returns "episodes" instead of "items" like other endpoints
	// We need to handle this inconsistency
	var randomResp struct {
		Status      string    `json:"status"`
		Episodes    []Episode `json:"episodes"` // Note: different field name
		Count       int       `json:"count"`
		Max         string    `json:"max"`
		Description string    `json:"description"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&randomResp); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if randomResp.Status != "true" {
		return nil, fmt.Errorf("API error: %s", randomResp.Description)
	}

	// Map to our standard response structure
	episodesResp := &EpisodesResponse{
		Status:      randomResp.Status,
		Items:       randomResp.Episodes, // Map episodes to items
		Count:       randomResp.Count,
		Max:         randomResp.Max,
		Description: randomResp.Description,
	}

	return episodesResp, nil
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

	endpoint := fmt.Sprintf("recent/feeds?%s", params.Encode())

	var feedsResp RecentFeedsResponse
	if err := c.makeAPIRequest(ctx, endpoint, &feedsResp); err != nil {
		return nil, err
	}

	if feedsResp.Status != "true" {
		return nil, fmt.Errorf("API error: %s", feedsResp.Description)
	}

	return &feedsResp, nil
}

// GetEpisodeByID fetches a single episode by its Podcast Index ID
func (c *Client) GetEpisodeByID(ctx context.Context, episodeID int64) (*Episode, error) {
	if episodeID <= 0 {
		return nil, fmt.Errorf("invalid episode ID: %d", episodeID)
	}

	params := url.Values{}
	params.Set("id", fmt.Sprintf("%d", episodeID))

	endpoint := fmt.Sprintf("episodes/byid?%s", params.Encode())

	// Create a struct matching the API response structure for a single episode
	var episodeResp struct {
		Status      string  `json:"status"`
		ID          int64   `json:"id"`
		Episode     Episode `json:"episode"`
		Description string  `json:"description"`
	}

	if err := c.makeAPIRequest(ctx, endpoint, &episodeResp); err != nil {
		// Check if it's a 404 error
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {
			return nil, fmt.Errorf("episode not found: ID %d", episodeID)
		}
		return nil, fmt.Errorf("fetching episode by ID: %w", err)
	}

	if episodeResp.Status != "true" {
		// Handle case where episode doesn't exist
		if strings.Contains(strings.ToLower(episodeResp.Description), "not found") ||
			strings.Contains(strings.ToLower(episodeResp.Description), "no episode") {
			return nil, fmt.Errorf("episode not found: ID %d", episodeID)
		}
		return nil, fmt.Errorf("API error: %s", episodeResp.Description)
	}

	// Some responses might have the episode data at the top level or nested
	if episodeResp.Episode.ID != 0 {
		return &episodeResp.Episode, nil
	}

	// If the data is at the top level, construct the episode
	if episodeResp.ID != 0 {
		episode := Episode{ID: episodeResp.ID}
		// The API might return the full episode data at the top level
		// For now, return what we have
		return &episode, nil
	}

	return nil, fmt.Errorf("unexpected response structure for episode ID %d", episodeID)
}

// GetPodcastByID fetches a single podcast by its Podcast Index ID
func (c *Client) GetPodcastByID(ctx context.Context, podcastID int64) (*PodcastByIDResponse, error) {
	if podcastID <= 0 {
		return nil, fmt.Errorf("invalid podcast ID: %d", podcastID)
	}

	params := url.Values{}
	params.Set("id", fmt.Sprintf("%d", podcastID))

	endpoint := fmt.Sprintf("podcasts/byfeedid?%s", params.Encode())

	var podcastResp PodcastByIDResponse
	if err := c.makeAPIRequest(ctx, endpoint, &podcastResp); err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {
			return nil, fmt.Errorf("podcast not found: ID %d", podcastID)
		}
		return nil, fmt.Errorf("fetching podcast by ID: %w", err)
	}

	if podcastResp.Status != "true" {
		if strings.Contains(strings.ToLower(podcastResp.Description), "not found") {
			return nil, fmt.Errorf("podcast not found: ID %d", podcastID)
		}
		return nil, fmt.Errorf("API error: %s", podcastResp.Description)
	}

	return &podcastResp, nil
}
