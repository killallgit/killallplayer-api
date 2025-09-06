package transcript

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// FetchOptions configures transcript fetching behavior
type FetchOptions struct {
	Timeout   time.Duration
	UserAgent string
	MaxSize   int64 // Maximum transcript size in bytes
}

// DefaultFetchOptions returns default fetch options
func DefaultFetchOptions() FetchOptions {
	return FetchOptions{
		Timeout:   30 * time.Second,
		UserAgent: "PodcastPlayerAPI/1.0",
		MaxSize:   10 * 1024 * 1024, // 10MB max for transcripts
	}
}

// Fetcher handles downloading transcripts from URLs
type Fetcher struct {
	client  *http.Client
	options FetchOptions
}

// NewFetcher creates a new transcript fetcher
func NewFetcher(options FetchOptions) *Fetcher {
	return &Fetcher{
		client: &http.Client{
			Timeout: options.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        5,
				IdleConnTimeout:     30 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
		options: options,
	}
}

// TranscriptResult contains the fetched transcript and metadata
type TranscriptResult struct {
	Content     string
	Format      TranscriptFormat
	ContentType string
	Size        int64
}

// Fetch downloads a transcript from the given URL
func (f *Fetcher) Fetch(ctx context.Context, url string) (*TranscriptResult, error) {
	if url == "" {
		return nil, fmt.Errorf("empty transcript URL")
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", f.options.UserAgent)
	req.Header.Set("Accept", "text/vtt,text/plain,application/x-subrip,application/json,*/*")

	// Execute request
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch transcript: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// Check content length
	contentLength := resp.ContentLength
	if contentLength > f.options.MaxSize {
		return nil, fmt.Errorf("transcript too large: %d bytes (max: %d)", contentLength, f.options.MaxSize)
	}

	// Read body with size limit
	limitedReader := io.LimitReader(resp.Body, f.options.MaxSize)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, fmt.Errorf("failed to read transcript: %w", err)
	}

	content := string(body)
	contentType := resp.Header.Get("Content-Type")

	// Detect format from content type and URL
	format := detectFormat(url, contentType, content)

	return &TranscriptResult{
		Content:     content,
		Format:      format,
		ContentType: contentType,
		Size:        int64(len(body)),
	}, nil
}

// detectFormat determines the transcript format from URL, content type, and content
func detectFormat(url, contentType, content string) TranscriptFormat {
	// Check URL extension
	urlLower := strings.ToLower(url)
	if strings.HasSuffix(urlLower, ".vtt") {
		return FormatVTT
	}
	if strings.HasSuffix(urlLower, ".srt") {
		return FormatSRT
	}
	if strings.HasSuffix(urlLower, ".json") {
		return FormatJSON
	}
	if strings.HasSuffix(urlLower, ".txt") {
		return FormatText
	}

	// Check content type
	contentTypeLower := strings.ToLower(contentType)
	if strings.Contains(contentTypeLower, "vtt") {
		return FormatVTT
	}
	if strings.Contains(contentTypeLower, "subrip") || strings.Contains(contentTypeLower, "srt") {
		return FormatSRT
	}
	if strings.Contains(contentTypeLower, "json") {
		return FormatJSON
	}

	// Check content for format markers
	contentStart := content
	if len(contentStart) > 100 {
		contentStart = content[:100]
	}

	if strings.HasPrefix(strings.TrimSpace(contentStart), "WEBVTT") {
		return FormatVTT
	}
	if strings.Contains(contentStart, "-->") {
		// Could be VTT or SRT, check for WEBVTT header
		if strings.Contains(content[:min(1000, len(content))], "WEBVTT") {
			return FormatVTT
		}
		return FormatSRT
	}
	if strings.HasPrefix(strings.TrimSpace(contentStart), "{") || strings.HasPrefix(strings.TrimSpace(contentStart), "[") {
		return FormatJSON
	}

	// Default to plain text
	return FormatText
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
