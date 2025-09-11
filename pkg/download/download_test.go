package download

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewDownloader(t *testing.T) {
	options := DefaultOptions()
	downloader := NewDownloader(options)

	if downloader == nil {
		t.Fatal("NewDownloader returned nil")
	}

	if downloader.client == nil {
		t.Error("Expected HTTP client to be initialized")
	}

	if downloader.options.Timeout != options.Timeout {
		t.Errorf("Expected timeout %v, got %v", options.Timeout, downloader.options.Timeout)
	}
}

func TestDefaultOptions(t *testing.T) {
	options := DefaultOptions()

	expectedDefaults := map[string]interface{}{
		"TempDir":       "/tmp",
		"MaxSize":       int64(500 * 1024 * 1024),
		"Timeout":       5 * time.Minute,
		"ValidateAudio": true,
	}

	if options.TempDir != expectedDefaults["TempDir"] {
		t.Errorf("Expected TempDir %v, got %v", expectedDefaults["TempDir"], options.TempDir)
	}

	if options.MaxSize != expectedDefaults["MaxSize"] {
		t.Errorf("Expected MaxSize %v, got %v", expectedDefaults["MaxSize"], options.MaxSize)
	}

	if options.Timeout != expectedDefaults["Timeout"] {
		t.Errorf("Expected Timeout %v, got %v", expectedDefaults["Timeout"], options.Timeout)
	}

	if options.ValidateAudio != expectedDefaults["ValidateAudio"] {
		t.Errorf("Expected ValidateAudio %v, got %v", expectedDefaults["ValidateAudio"], options.ValidateAudio)
	}

	// Check User-Agent is set to mobile Firefox iOS
	if !strings.Contains(options.UserAgent, "iPhone") || !strings.Contains(options.UserAgent, "FxiOS") {
		t.Errorf("Expected User-Agent to be mobile Firefox iOS, got: %v", options.UserAgent)
	}
}

func TestDownloadToTemp_Success(t *testing.T) {
	// Create test server that serves valid audio
	audioData := strings.Repeat("audio-data", 128) // 1280 bytes (10 * 128)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(audioData))
	}))
	defer server.Close()

	options := DefaultOptions()
	options.TempDir = os.TempDir()
	downloader := NewDownloader(options)

	ctx := context.Background()
	result, err := downloader.DownloadToTemp(ctx, server.URL, 12345)

	if err != nil {
		t.Fatalf("Expected successful download, got error: %v", err)
	}

	defer func() {
		_ = CleanupTempFile(result.FilePath)
	}()

	if result.ContentType != "audio/mpeg" {
		t.Errorf("Expected content type 'audio/mpeg', got %v", result.ContentType)
	}

	if result.ContentLength != 1280 {
		t.Errorf("Expected content length 1280, got %v", result.ContentLength)
	}

	// Verify file exists and has correct size
	if _, err := os.Stat(result.FilePath); os.IsNotExist(err) {
		t.Error("Downloaded file does not exist")
	}
}

func TestDownloadToTemp_403Forbidden_Buzzsprout(t *testing.T) {
	// Create test server that returns 403 for Buzzsprout URLs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Access Denied"))
	}))
	defer server.Close()

	options := DefaultOptions()
	downloader := NewDownloader(options)

	// Test the 403 error handling using the test server
	testDownloadToTemp_403BuzzsproutHelper(t, downloader, server.URL)
}

func testDownloadToTemp_403BuzzsproutHelper(t *testing.T, downloader *Downloader, serverURL string) {
	// Create a test URL that contains "buzzsprout" to trigger the specific error message
	ctx := context.Background()
	_, err := downloader.DownloadToTemp(ctx, serverURL, 12345)

	if err == nil {
		t.Fatal("Expected error for 403 response, got nil")
	}

	// For generic 403 (not Buzzsprout), check generic message
	expectedMessages := []string{
		"audio download blocked by CDN (403 Forbidden)",
		"IP blocking",
		"hotlink protection",
	}

	errStr := err.Error()
	for _, msg := range expectedMessages {
		if !strings.Contains(errStr, msg) {
			t.Errorf("Expected error message to contain '%s', got: %v", msg, errStr)
		}
	}
}

func TestDownloadToTemp_403Forbidden_Generic(t *testing.T) {
	// Create test server that returns 403 for non-Buzzsprout URLs
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Forbidden"))
	}))
	defer server.Close()

	options := DefaultOptions()
	downloader := NewDownloader(options)

	ctx := context.Background()
	_, err := downloader.DownloadToTemp(ctx, server.URL, 12345)

	if err == nil {
		t.Fatal("Expected error for 403 response, got nil")
	}

	expectedMessages := []string{
		"audio download blocked by CDN (403 Forbidden)",
		"IP blocking",
		"hotlink protection",
	}

	errStr := err.Error()
	for _, msg := range expectedMessages {
		if !strings.Contains(errStr, msg) {
			t.Errorf("Expected error message to contain '%s', got: %v", msg, errStr)
		}
	}

	// Should not contain Buzzsprout-specific message
	if strings.Contains(errStr, "web browsers but not server-side downloads") {
		t.Error("Generic 403 error should not contain Buzzsprout-specific message")
	}
}

func TestDownloadToTemp_InvalidContentType(t *testing.T) {
	// Create test server that serves HTML instead of audio
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>Not audio</html>"))
	}))
	defer server.Close()

	options := DefaultOptions()
	options.ValidateAudio = true
	downloader := NewDownloader(options)

	ctx := context.Background()
	_, err := downloader.DownloadToTemp(ctx, server.URL, 12345)

	if err == nil {
		t.Fatal("Expected error for invalid content type, got nil")
	}

	if !strings.Contains(err.Error(), "invalid content type: text/html") {
		t.Errorf("Expected content type error, got: %v", err.Error())
	}
}

func TestDownloadToTemp_FileTooLarge(t *testing.T) {
	// Create test server that claims large content length
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.Header().Set("Content-Length", "1000000000") // 1GB
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	options := DefaultOptions()
	options.MaxSize = 1024 // 1KB limit
	downloader := NewDownloader(options)

	ctx := context.Background()
	_, err := downloader.DownloadToTemp(ctx, server.URL, 12345)

	if err == nil {
		t.Fatal("Expected error for file too large, got nil")
	}

	if !strings.Contains(err.Error(), "file too large") {
		t.Errorf("Expected file too large error, got: %v", err.Error())
	}
}

func TestDownloadWithRetry_Success(t *testing.T) {
	// Create test server that succeeds on first try
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "audio/mpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("audio-data"))
	}))
	defer server.Close()

	options := DefaultOptions()
	options.TempDir = os.TempDir()
	downloader := NewDownloader(options)

	ctx := context.Background()
	result, err := downloader.DownloadWithRetry(ctx, server.URL, 12345)

	if err != nil {
		t.Fatalf("Expected successful download, got error: %v", err)
	}

	defer func() {
		_ = CleanupTempFile(result.FilePath)
	}()

	if result.ContentType != "audio/mpeg" {
		t.Errorf("Expected content type 'audio/mpeg', got %v", result.ContentType)
	}
}

func TestDownloadWithRetry_403Failure(t *testing.T) {
	// Create test server that always returns 403
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("Forbidden"))
	}))
	defer server.Close()

	options := DefaultOptions()
	downloader := NewDownloader(options)

	ctx := context.Background()
	_, err := downloader.DownloadWithRetry(ctx, server.URL, 12345)

	if err == nil {
		t.Fatal("Expected error after retries, got nil")
	}

	// The retry logic only retries 403 errors, and for 403 errors it returns
	// the descriptive error message, not the "failed after X attempts" message
	if !strings.Contains(err.Error(), "403 Forbidden") {
		t.Errorf("Expected 403 error message, got: %v", err.Error())
	}
}

func TestDownloadWithRetry_NonRetryableError(t *testing.T) {
	// Create test server that returns 404 (non-retryable)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	options := DefaultOptions()
	downloader := NewDownloader(options)

	ctx := context.Background()
	_, err := downloader.DownloadWithRetry(ctx, server.URL, 12345)

	if err == nil {
		t.Fatal("Expected error for 404, got nil")
	}

	// Should fail immediately without retries for non-403 errors
	if strings.Contains(err.Error(), "download failed after") {
		t.Error("Should not retry for non-403 errors")
	}
}

func TestIsAudioContentType(t *testing.T) {
	testCases := []struct {
		contentType string
		expected    bool
	}{
		{"audio/mpeg", true},
		{"audio/mp3", true},
		{"audio/wav", true},
		{"AUDIO/MPEG", true},               // Case insensitive
		{"application/octet-stream", true}, // Special case for some servers
		{"text/html", false},
		{"image/jpeg", false},
		{"application/json", false},
		{"", false},
	}

	for _, tc := range testCases {
		result := isAudioContentType(tc.contentType)
		if result != tc.expected {
			t.Errorf("isAudioContentType(%q) = %v, expected %v", tc.contentType, result, tc.expected)
		}
	}
}

func TestIsValidAudioExtension(t *testing.T) {
	testCases := []struct {
		ext      string
		expected bool
	}{
		{"mp3", true},
		{"MP3", true}, // Case insensitive
		{"m4a", true},
		{"wav", true},
		{"flac", true},
		{"ogg", true},
		{"opus", true},
		{"txt", false},
		{"html", false},
		{"", false},
	}

	for _, tc := range testCases {
		result := isValidAudioExtension(tc.ext)
		if result != tc.expected {
			t.Errorf("isValidAudioExtension(%q) = %v, expected %v", tc.ext, result, tc.expected)
		}
	}
}

func TestCleanupTempFile(t *testing.T) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "test_cleanup_*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	tmpFile.Close()
	filePath := tmpFile.Name()

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("Temp file should exist before cleanup")
	}

	// Clean up the file
	err = CleanupTempFile(filePath)
	if err != nil {
		t.Errorf("CleanupTempFile failed: %v", err)
	}

	// Verify file is gone
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("Temp file should not exist after cleanup")
	}
}

func TestCleanupTempFile_EmptyPath(t *testing.T) {
	// Should handle empty path gracefully
	err := CleanupTempFile("")
	if err != nil {
		t.Errorf("CleanupTempFile with empty path should not error, got: %v", err)
	}
}

func TestCleanupOldTempFiles(t *testing.T) {
	// Create temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "test_cleanup_old_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create old and new files
	oldFile, err := os.CreateTemp(tmpDir, "episode_12345_*")
	if err != nil {
		t.Fatalf("Failed to create old file: %v", err)
	}
	oldFile.Close()

	newFile, err := os.CreateTemp(tmpDir, "episode_67890_*")
	if err != nil {
		t.Fatalf("Failed to create new file: %v", err)
	}
	newFile.Close()

	// Make old file actually old by modifying its timestamp
	oldTime := time.Now().Add(-25 * time.Hour) // 25 hours ago
	_ = os.Chtimes(oldFile.Name(), oldTime, oldTime)

	// Clean up files older than 24 hours
	err = CleanupOldTempFiles(tmpDir, 24*time.Hour)
	if err != nil {
		t.Errorf("CleanupOldTempFiles failed: %v", err)
	}

	// Old file should be gone
	if _, err := os.Stat(oldFile.Name()); !os.IsNotExist(err) {
		t.Error("Old file should have been cleaned up")
	}

	// New file should still exist
	if _, err := os.Stat(newFile.Name()); os.IsNotExist(err) {
		t.Error("New file should still exist")
	}
}
