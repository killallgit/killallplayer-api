package download

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DownloadOptions configures the download behavior
type DownloadOptions struct {
	TempDir       string        // Directory for temporary files
	MaxSize       int64         // Maximum file size in bytes (0 = no limit)
	Timeout       time.Duration // Download timeout
	ProgressFunc  ProgressFunc  // Optional progress callback
	UserAgent     string        // User agent string
	ValidateAudio bool          // Validate content-type is audio
}

// ProgressFunc is called during download to report progress
type ProgressFunc func(downloaded, total int64)

// DefaultOptions returns default download options
func DefaultOptions() DownloadOptions {
	return DownloadOptions{
		TempDir:       "/tmp",
		MaxSize:       500 * 1024 * 1024, // 500MB default max
		Timeout:       5 * time.Minute,
		UserAgent:     "PodcastPlayerAPI/1.0",
		ValidateAudio: true,
	}
}

// DownloadResult contains information about a successful download
type DownloadResult struct {
	FilePath      string    // Path to downloaded file
	ContentType   string    // Content-Type from response
	ContentLength int64     // Size in bytes
	ETag          string    // ETag header if present
	LastModified  time.Time // Last-Modified header if present
}

// Downloader handles downloading audio files to temporary storage
type Downloader struct {
	client  *http.Client
	options DownloadOptions
}

// NewDownloader creates a new downloader with the given options
func NewDownloader(options DownloadOptions) *Downloader {
	return &Downloader{
		client: &http.Client{
			Timeout: options.Timeout,
			Transport: &http.Transport{
				MaxIdleConns:        10,
				IdleConnTimeout:     30 * time.Second,
				DisableCompression:  true, // Don't compress audio
				TLSHandshakeTimeout: 10 * time.Second,
			},
		},
		options: options,
	}
}

// DownloadToTemp downloads a URL to a temporary file
func (d *Downloader) DownloadToTemp(ctx context.Context, url string, episodeID uint) (*DownloadResult, error) {
	log.Printf("[DEBUG] Starting download from %s for episode %d", url, episodeID)

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("User-Agent", d.options.UserAgent)
	req.Header.Set("Accept", "audio/*,*/*")

	// Execute request
	resp, err := d.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	// Validate content type if required
	contentType := resp.Header.Get("Content-Type")
	if d.options.ValidateAudio && !isAudioContentType(contentType) {
		return nil, fmt.Errorf("invalid content type: %s", contentType)
	}

	// Check content length
	contentLength := resp.ContentLength
	if d.options.MaxSize > 0 && contentLength > d.options.MaxSize {
		return nil, fmt.Errorf("file too large: %d bytes (max %d)", contentLength, d.options.MaxSize)
	}

	// Create temp file
	tempFile, err := d.createTempFile(episodeID, url)
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Download to file
	written, err := d.downloadToFile(resp.Body, tempFile, contentLength)
	tempPath := tempFile.Name()
	tempFile.Close()

	if err != nil {
		os.Remove(tempPath)
		return nil, fmt.Errorf("failed to download: %w", err)
	}

	log.Printf("[DEBUG] Downloaded %d bytes to %s", written, tempPath)

	// Parse headers
	result := &DownloadResult{
		FilePath:      tempPath,
		ContentType:   contentType,
		ContentLength: written,
		ETag:          resp.Header.Get("ETag"),
	}

	// Parse Last-Modified header
	if lastMod := resp.Header.Get("Last-Modified"); lastMod != "" {
		if t, err := http.ParseTime(lastMod); err == nil {
			result.LastModified = t
		}
	}

	return result, nil
}

// createTempFile creates a temporary file for the download
func (d *Downloader) createTempFile(episodeID uint, url string) (*os.File, error) {
	// Extract file extension from URL if possible
	ext := ".mp3" // default
	if parts := strings.Split(url, "."); len(parts) > 1 {
		lastPart := parts[len(parts)-1]
		// Remove query params if present
		if idx := strings.Index(lastPart, "?"); idx > 0 {
			lastPart = lastPart[:idx]
		}
		if isValidAudioExtension(lastPart) {
			ext = "." + lastPart
		}
	}

	// Create temp file with pattern: episode_<id>_<timestamp>.<ext>
	pattern := fmt.Sprintf("episode_%d_*%s", episodeID, ext)
	return os.CreateTemp(d.options.TempDir, pattern)
}

// downloadToFile downloads response body to file with optional progress tracking
func (d *Downloader) downloadToFile(src io.Reader, dst *os.File, totalSize int64) (int64, error) {
	// Create progress reader if callback provided
	reader := src
	if d.options.ProgressFunc != nil && totalSize > 0 {
		reader = &progressReader{
			reader:   src,
			total:    totalSize,
			callback: d.options.ProgressFunc,
		}
	}

	// Copy with size limit if configured
	if d.options.MaxSize > 0 {
		reader = &io.LimitedReader{
			R: reader,
			N: d.options.MaxSize,
		}
	}

	// Copy to file
	return io.Copy(dst, reader)
}

// CleanupTempFile removes a temporary file
func CleanupTempFile(path string) error {
	if path == "" {
		return nil
	}

	log.Printf("[DEBUG] Cleaning up temp file: %s", path)
	return os.Remove(path)
}

// CleanupOldTempFiles removes temp files older than the specified duration
func CleanupOldTempFiles(tempDir string, maxAge time.Duration) error {
	pattern := filepath.Join(tempDir, "episode_*")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-maxAge)
	var removed int

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		if info.ModTime().Before(cutoff) {
			if err := os.Remove(file); err == nil {
				removed++
			}
		}
	}

	if removed > 0 {
		log.Printf("[DEBUG] Cleaned up %d old temp files", removed)
	}

	return nil
}

// isAudioContentType checks if content type is audio
func isAudioContentType(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.HasPrefix(contentType, "audio/") ||
		contentType == "application/octet-stream" // Some servers use this for audio
}

// isValidAudioExtension checks if extension is valid for audio files
func isValidAudioExtension(ext string) bool {
	ext = strings.ToLower(ext)
	validExts := []string{"mp3", "m4a", "aac", "ogg", "wav", "flac", "opus", "webm"}
	for _, valid := range validExts {
		if ext == valid {
			return true
		}
	}
	return false
}

// progressReader wraps a reader to report progress
type progressReader struct {
	reader     io.Reader
	total      int64
	downloaded int64
	callback   ProgressFunc
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	if n > 0 {
		pr.downloaded += int64(n)
		if pr.callback != nil {
			pr.callback(pr.downloaded, pr.total)
		}
	}
	return n, err
}
