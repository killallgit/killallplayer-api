package episodes

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// PodcastIndexHTTPClient implements the PodcastIndexClient interface
type PodcastIndexHTTPClient struct {
	apiKey    string
	apiSecret string
	baseURL   string
	client    *http.Client
}

// NewPodcastIndexClient creates a new Podcast Index HTTP client
func NewPodcastIndexClient(apiKey, apiSecret, baseURL string, timeout time.Duration) PodcastIndexClient {
	return &PodcastIndexHTTPClient{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   baseURL,
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

// Get makes a GET request to the Podcast Index API
func (c *PodcastIndexHTTPClient) Get(ctx context.Context, endpoint string, params map[string]string) ([]byte, error) {
	// Build URL
	fullURL := fmt.Sprintf("%s%s", c.baseURL, endpoint)
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Add query parameters
	if len(params) > 0 {
		q := req.URL.Query()
		for key, value := range params {
			q.Add(key, value)
		}
		req.URL.RawQuery = q.Encode()
	}

	// Add authentication headers
	c.addAuthHeaders(req)

	// Execute request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, NewAPIError(endpoint, resp.StatusCode, string(body))
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	return body, nil
}

// Head makes a HEAD request to get headers
func (c *PodcastIndexHTTPClient) Head(ctx context.Context, url string) (*HTTPHeaders, error) {
	req, err := http.NewRequestWithContext(ctx, "HEAD", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating HEAD request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing HEAD request: %w", err)
	}
	defer resp.Body.Close()

	headers := &HTTPHeaders{
		ContentType: resp.Header.Get("Content-Type"),
	}

	// Parse Content-Length
	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
			headers.ContentLength = size
		}
	}

	// Parse Last-Modified
	if lastModified := resp.Header.Get("Last-Modified"); lastModified != "" {
		if t, err := time.Parse(http.TimeFormat, lastModified); err == nil {
			headers.LastModified = t
		}
	}

	return headers, nil
}

// addAuthHeaders adds Podcast Index authentication headers
func (c *PodcastIndexHTTPClient) addAuthHeaders(req *http.Request) {
	apiTime := strconv.FormatInt(time.Now().Unix(), 10)
	authString := c.apiKey + c.apiSecret + apiTime
	hash := sha1.Sum([]byte(authString))
	hashString := fmt.Sprintf("%x", hash)

	req.Header.Set("X-Auth-Date", apiTime)
	req.Header.Set("X-Auth-Key", c.apiKey)
	req.Header.Set("Authorization", hashString)
	req.Header.Set("User-Agent", "PodcastPlayerAPI/1.0")
}

// SimplifiedFetcher uses the PodcastIndexClient for cleaner separation of concerns
type SimplifiedFetcher struct {
	client PodcastIndexClient
}

// NewSimplifiedFetcher creates a new fetcher with dependency injection
func NewSimplifiedFetcher(client PodcastIndexClient) EpisodeFetcher {
	return &SimplifiedFetcher{
		client: client,
	}
}

// GetEpisodesByPodcastID fetches episodes using the client
func (f *SimplifiedFetcher) GetEpisodesByPodcastID(ctx context.Context, podcastID int64, limit int) (*PodcastIndexResponse, error) {
	params := map[string]string{
		"id": strconv.FormatInt(podcastID, 10),
	}
	if limit > 0 {
		params["max"] = strconv.Itoa(limit)
	}

	data, err := f.client.Get(ctx, "/episodes/byfeedid", params)
	if err != nil {
		return nil, err
	}

	var response PodcastIndexResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if response.Status != "true" {
		return nil, fmt.Errorf("API error: %s", response.Description)
	}

	return &response, nil
}

// GetEpisodeByGUID fetches a single episode by GUID
func (f *SimplifiedFetcher) GetEpisodeByGUID(ctx context.Context, guid string) (*EpisodeByGUIDResponse, error) {
	params := map[string]string{
		"guid": guid,
	}

	data, err := f.client.Get(ctx, "/episodes/byguid", params)
	if err != nil {
		return nil, err
	}

	var response EpisodeByGUIDResponse
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if response.Status != "true" {
		return nil, fmt.Errorf("API error: %s", response.Description)
	}

	return &response, nil
}

// GetEpisodeMetadata fetches metadata for an episode URL
func (f *SimplifiedFetcher) GetEpisodeMetadata(ctx context.Context, episodeURL string) (*EpisodeMetadata, error) {
	parsedURL, err := url.Parse(episodeURL)
	if err != nil {
		return nil, NewValidationError("episodeURL", "invalid URL format")
	}

	headers, err := f.client.Head(ctx, episodeURL)
	if err != nil {
		return nil, err
	}

	metadata := &EpisodeMetadata{
		URL:          episodeURL,
		ContentType:  headers.ContentType,
		Size:         headers.ContentLength,
		LastModified: headers.LastModified,
		FileName:     parsedURL.Path[len(parsedURL.Path)-1:],
	}

	return metadata, nil
}
