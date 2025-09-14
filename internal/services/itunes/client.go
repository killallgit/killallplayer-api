package itunes

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

var (
	// ErrRateLimited indicates the API rate limit was exceeded
	ErrRateLimited = errors.New("itunes api rate limit exceeded")

	// ErrNoResults indicates no results were found
	ErrNoResults = errors.New("no results found")

	// ErrInvalidResponse indicates the API returned an invalid response
	ErrInvalidResponse = errors.New("invalid response from itunes api")
)

// defaultUserAgents provides a pool of legitimate user agents to rotate through
var defaultUserAgents = []string{
	"PodcastPlayer/1.0 (compatible; +https://github.com/killallgit/killallplayer-api)",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.1 Safari/605.1.15",
	"curl/7.88.1",
	"Podcast/1.0 (https://podcastindex.org/)",
}

// Config holds configuration for the iTunes client
type Config struct {
	// Rate limiting
	RequestsPerMinute int           // Default: 250 (safe under 300 limit)
	BurstSize        int           // Default: 5

	// HTTP configuration
	Timeout          time.Duration // Default: 10s
	MaxRetries       int           // Default: 3
	RetryBackoff     time.Duration // Default: 1s

	// User agents
	UserAgents       []string      // Custom user agents, uses defaults if empty

	// Base URL (for testing)
	BaseURL          string        // Default: https://itunes.apple.com
}

// Client handles communication with the iTunes API
type Client struct {
	httpClient   *http.Client
	rateLimiter  *rate.Limiter
	userAgents   []string
	config       Config
	baseURL      string

	// Metrics
	metrics      *clientMetrics

	// State
	mu           sync.RWMutex
	userAgentIdx int32
}

// clientMetrics tracks client usage statistics
type clientMetrics struct {
	requests      atomic.Int64
	rateLimitHits atomic.Int64
	errors        atomic.Int64
	cacheHits     atomic.Int64
	cacheMisses   atomic.Int64
}

// NewClient creates a new iTunes API client
func NewClient(cfg Config) *Client {
	// Apply defaults
	if cfg.RequestsPerMinute == 0 {
		cfg.RequestsPerMinute = 250 // Conservative default (well under 300)
	}
	if cfg.BurstSize == 0 {
		cfg.BurstSize = 5
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 10 * time.Second
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryBackoff == 0 {
		cfg.RetryBackoff = time.Second
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://itunes.apple.com"
	}
	if len(cfg.UserAgents) == 0 {
		cfg.UserAgents = defaultUserAgents
	}

	// Create rate limiter
	limiter := rate.NewLimiter(
		rate.Every(time.Minute/time.Duration(cfg.RequestsPerMinute)),
		cfg.BurstSize,
	)

	return &Client{
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
		rateLimiter: limiter,
		userAgents:  cfg.UserAgents,
		config:      cfg,
		baseURL:     cfg.BaseURL,
		metrics:     &clientMetrics{},
	}
}

// LookupPodcast fetches podcast metadata by iTunes ID
func (c *Client) LookupPodcast(ctx context.Context, iTunesID int64) (*Podcast, error) {
	url := fmt.Sprintf("%s/lookup?id=%d", c.baseURL, iTunesID)

	resp, err := c.doRequestWithRetry(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("lookup podcast %d: %w", iTunesID, err)
	}

	if resp.ResultCount == 0 {
		return nil, ErrNoResults
	}

	// First result should be the podcast
	if len(resp.Results) == 0 {
		return nil, ErrInvalidResponse
	}

	return transformToPodcast(&resp.Results[0]), nil
}

// LookupPodcastWithEpisodes fetches podcast metadata and episodes by iTunes ID
func (c *Client) LookupPodcastWithEpisodes(ctx context.Context, iTunesID int64, limit int) (*PodcastWithEpisodes, error) {
	// Validate and cap limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200 // iTunes API max
	}

	url := fmt.Sprintf(
		"%s/lookup?id=%d&media=podcast&entity=podcastEpisode&limit=%d",
		c.baseURL, iTunesID, limit,
	)

	resp, err := c.doRequestWithRetry(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("lookup podcast with episodes %d: %w", iTunesID, err)
	}

	if resp.ResultCount == 0 {
		return nil, ErrNoResults
	}

	return transformToPodcastWithEpisodes(resp), nil
}

// Search searches for podcasts by term
func (c *Client) Search(ctx context.Context, term string, opts *SearchOptions) (*SearchResults, error) {
	if term == "" {
		return nil, errors.New("search term cannot be empty")
	}

	// Build query parameters
	params := url.Values{}
	params.Set("term", term)
	params.Set("media", "podcast")

	if opts != nil {
		if opts.Entity != "" {
			params.Set("entity", opts.Entity)
		}
		if opts.Country != "" {
			params.Set("country", opts.Country)
		}
		if opts.Limit > 0 {
			limit := opts.Limit
			if limit > 200 {
				limit = 200
			}
			params.Set("limit", fmt.Sprintf("%d", limit))
		}
		if opts.Language != "" {
			params.Set("lang", opts.Language)
		}
		if opts.Explicit != "" {
			params.Set("explicit", opts.Explicit)
		}
	}

	searchURL := fmt.Sprintf("%s/search?%s", c.baseURL, params.Encode())

	resp, err := c.doRequestWithRetry(ctx, searchURL)
	if err != nil {
		return nil, fmt.Errorf("search podcasts: %w", err)
	}

	return transformToSearchResults(term, resp), nil
}

// doRequestWithRetry performs an HTTP request with retry logic
func (c *Client) doRequestWithRetry(ctx context.Context, url string) (*iTunesResponse, error) {
	var lastErr error
	backoff := c.config.RetryBackoff

	for attempt := 0; attempt < c.config.MaxRetries; attempt++ {
		resp, err := c.doRequest(ctx, url)
		if err == nil {
			return resp, nil
		}

		// Check if it's a rate limit error
		if errors.Is(err, ErrRateLimited) {
			c.metrics.rateLimitHits.Add(1)

			// Wait with exponential backoff
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
				lastErr = err
				continue
			}
		}

		// Check if it's a temporary error
		if isTemporaryError(err) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff *= 2
				lastErr = err
				continue
			}
		}

		// Non-retryable error
		return nil, err
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// doRequest performs a single HTTP request
func (c *Client) doRequest(ctx context.Context, url string) (*iTunesResponse, error) {
	// Wait for rate limiter
	if err := c.rateLimiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter wait: %w", err)
	}

	// Track request
	c.metrics.requests.Add(1)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set headers to look legitimate
	req.Header.Set("User-Agent", c.getUserAgent())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.metrics.errors.Add(1)
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	// Handle rate limiting
	if resp.StatusCode == http.StatusTooManyRequests {
		c.metrics.rateLimitHits.Add(1)
		return nil, ErrRateLimited
	}

	// Check status code
	if resp.StatusCode != http.StatusOK {
		c.metrics.errors.Add(1)
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	// Handle gzip encoding
	var reader io.Reader = resp.Body
	if strings.Contains(resp.Header.Get("Content-Encoding"), "gzip") {
		gzReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			c.metrics.errors.Add(1)
			return nil, fmt.Errorf("create gzip reader: %w", err)
		}
		defer gzReader.Close()
		reader = gzReader
	}

	// Parse response
	var result iTunesResponse
	if err := json.NewDecoder(reader).Decode(&result); err != nil {
		c.metrics.errors.Add(1)
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}

// getUserAgent returns a user agent string, rotating through the pool
func (c *Client) getUserAgent() string {
	idx := atomic.AddInt32(&c.userAgentIdx, 1)
	return c.userAgents[int(idx)%len(c.userAgents)]
}

// GetMetrics returns current client metrics
func (c *Client) GetMetrics() map[string]int64 {
	return map[string]int64{
		"requests":        c.metrics.requests.Load(),
		"rate_limit_hits": c.metrics.rateLimitHits.Load(),
		"errors":          c.metrics.errors.Load(),
		"cache_hits":      c.metrics.cacheHits.Load(),
		"cache_misses":    c.metrics.cacheMisses.Load(),
	}
}

// isTemporaryError checks if an error is temporary and should be retried
func isTemporaryError(err error) bool {
	// Check for network errors
	if netErr, ok := err.(interface{ Temporary() bool }); ok {
		return netErr.Temporary()
	}

	// Check for timeout errors
	if netErr, ok := err.(interface{ Timeout() bool }); ok {
		return netErr.Timeout()
	}

	return false
}