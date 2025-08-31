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
	"strings"
	"time"

	"github.com/killallgit/player-api/pkg/config"
)

type Fetcher struct {
	apiKey    string
	apiSecret string
	apiURL    string
	client    *http.Client
}

func NewFetcher(cfg *config.Config) *Fetcher {
	return &Fetcher{
		apiKey:    cfg.PodcastIndex.APIKey,
		apiSecret: cfg.PodcastIndex.APISecret,
		apiURL:    cfg.PodcastIndex.BaseURL,
		client: &http.Client{
			Timeout: cfg.PodcastIndex.Timeout,
		},
	}
}

func (f *Fetcher) GetEpisodesByPodcastID(ctx context.Context, podcastID int64, limit int) (*PodcastIndexResponse, error) {
	endpoint := fmt.Sprintf("%s/episodes/byfeedid", f.apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	q.Add("id", strconv.FormatInt(podcastID, 10))
	if limit > 0 {
		q.Add("max", strconv.Itoa(limit))
	}
	req.URL.RawQuery = q.Encode()

	f.setAuthHeaders(req)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result PodcastIndexResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if result.Status != "true" {
		return nil, fmt.Errorf("API error: %s", result.Description)
	}

	return &result, nil
}

func (f *Fetcher) GetEpisodeByGUID(ctx context.Context, guid string) (*EpisodeByGUIDResponse, error) {
	endpoint := fmt.Sprintf("%s/episodes/byguid", f.apiURL)

	req, err := http.NewRequestWithContext(ctx, "GET", endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	q := req.URL.Query()
	q.Add("guid", guid)
	req.URL.RawQuery = q.Encode()

	f.setAuthHeaders(req)

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result EpisodeByGUIDResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if result.Status != "true" {
		return nil, fmt.Errorf("API error: %s", result.Description)
	}

	return &result, nil
}

func (f *Fetcher) setAuthHeaders(req *http.Request) {
	apiTime := strconv.FormatInt(time.Now().Unix(), 10)
	authString := f.apiKey + f.apiSecret + apiTime
	hash := sha1.Sum([]byte(authString))
	hashString := fmt.Sprintf("%x", hash)

	req.Header.Set("X-Auth-Date", apiTime)
	req.Header.Set("X-Auth-Key", f.apiKey)
	req.Header.Set("Authorization", hashString)
	req.Header.Set("User-Agent", "PodcastPlayerAPI/1.0")
}

func (f *Fetcher) GetEpisodeMetadata(ctx context.Context, episodeURL string) (*EpisodeMetadata, error) {
	parsedURL, err := url.Parse(episodeURL)
	if err != nil {
		return nil, fmt.Errorf("parsing episode URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", episodeURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating HEAD request: %w", err)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing HEAD request: %w", err)
	}
	defer resp.Body.Close()

	// Extract filename from URL path, handling edge cases
	var fileName string
	if parsedURL.Path != "" {
		// Extract the last part of the path after the final slash
		parts := strings.Split(parsedURL.Path, "/")
		fileName = parts[len(parts)-1]
		// If empty or just a slash, use a default
		if fileName == "" {
			fileName = "episode"
		}
	} else {
		fileName = "episode"
	}

	metadata := &EpisodeMetadata{
		URL:         episodeURL,
		ContentType: resp.Header.Get("Content-Type"),
		FileName:    fileName,
	}

	if contentLength := resp.Header.Get("Content-Length"); contentLength != "" {
		if size, err := strconv.ParseInt(contentLength, 10, 64); err == nil {
			metadata.Size = size
		}
	}

	if lastModified := resp.Header.Get("Last-Modified"); lastModified != "" {
		if t, err := time.Parse(http.TimeFormat, lastModified); err == nil {
			metadata.LastModified = t
		}
	}

	return metadata, nil
}

type EpisodeMetadata struct {
	URL          string
	ContentType  string
	Size         int64
	LastModified time.Time
	FileName     string
}
