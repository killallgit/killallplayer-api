package audiocache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"gorm.io/gorm"
)

// ServiceImpl implements the Service interface
type ServiceImpl struct {
	repository Repository
	storage    StorageBackend
}

// NewService creates a new audio cache service
func NewService(repository Repository, storage StorageBackend) Service {
	return &ServiceImpl{
		repository: repository,
		storage:    storage,
	}
}

// GetOrDownloadAudio retrieves cached audio or downloads if not present
func (s *ServiceImpl) GetOrDownloadAudio(ctx context.Context, podcastIndexEpisodeID int64, audioURL string) (*models.AudioCache, error) {
	// Check if already cached
	cache, err := s.repository.GetByPodcastIndexEpisodeID(ctx, podcastIndexEpisodeID)
	if err == nil && cache != nil {
		// Update last used timestamp
		if err := s.UpdateLastUsed(ctx, cache.ID); err != nil {
			log.Printf("[WARN] Failed to update last used timestamp: %v", err)
		}
		return cache, nil
	}

	// Not cached, download and process
	log.Printf("[INFO] Downloading audio for Podcast Index episode %d from %s", podcastIndexEpisodeID, audioURL)

	// Download audio to temp file
	tempFile, err := s.downloadAudio(ctx, audioURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download audio: %w", err)
	}
	defer os.Remove(tempFile)

	// Calculate SHA256 of original file
	sha256Hash, err := s.calculateSHA256(tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate SHA256: %w", err)
	}

	// Check if this audio already exists (by SHA256)
	existingCache, err := s.repository.GetBySHA256(ctx, sha256Hash)
	if err == nil && existingCache != nil {
		log.Printf("[INFO] Audio already cached with SHA256 %s, linking to Podcast Index episode %d", sha256Hash, podcastIndexEpisodeID)

		// Create new cache entry for this episode linking to existing files
		newCache := &models.AudioCache{
			PodcastIndexEpisodeID: podcastIndexEpisodeID,
			OriginalURL:           audioURL,
			OriginalSHA256:        existingCache.OriginalSHA256,
			OriginalPath:          existingCache.OriginalPath,
			OriginalSize:          existingCache.OriginalSize,
			ProcessedPath:         existingCache.ProcessedPath,
			ProcessedSHA256:       existingCache.ProcessedSHA256,
			ProcessedSize:         existingCache.ProcessedSize,
			DurationSeconds:       existingCache.DurationSeconds,
			SampleRate:            existingCache.SampleRate,
		}

		if err := s.repository.Create(ctx, newCache); err != nil {
			return nil, fmt.Errorf("failed to create cache entry: %w", err)
		}

		return newCache, nil
	}

	// Get file info
	fileInfo, err := os.Stat(tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to stat temp file: %w", err)
	}

	// Save original file to storage
	originalFilename := fmt.Sprintf("original/%d_%s.mp3", podcastIndexEpisodeID, sha256Hash[:8])
	originalFile, err := os.Open(tempFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open temp file: %w", err)
	}
	defer originalFile.Close()

	originalPath, err := s.storage.Save(ctx, originalFile, originalFilename)
	if err != nil {
		return nil, fmt.Errorf("failed to save original audio: %w", err)
	}

	// Process audio for ML (16kHz mono)
	processedFilename := fmt.Sprintf("processed/%d_%s_16khz.mp3", podcastIndexEpisodeID, sha256Hash[:8])
	processedTempFile := tempFile + "_processed.mp3"

	if err := s.ProcessAudioForML(ctx, tempFile, processedTempFile); err != nil {
		// Clean up original file on error
		if delErr := s.storage.Delete(ctx, originalPath); delErr != nil {
			log.Printf("[WARN] Failed to cleanup original file after processing error: %v", delErr)
		}
		return nil, fmt.Errorf("failed to process audio for ML: %w", err)
	}
	defer os.Remove(processedTempFile)

	// Calculate SHA256 of processed file
	processedSHA256, err := s.calculateSHA256(processedTempFile)
	if err != nil {
		if delErr := s.storage.Delete(ctx, originalPath); delErr != nil {
			log.Printf("[WARN] Failed to cleanup original file after SHA256 error: %v", delErr)
		}
		return nil, fmt.Errorf("failed to calculate processed SHA256: %w", err)
	}

	// Save processed file to storage
	processedFile, err := os.Open(processedTempFile)
	if err != nil {
		if delErr := s.storage.Delete(ctx, originalPath); delErr != nil {
			log.Printf("[WARN] Failed to cleanup original file after processed file open error: %v", delErr)
		}
		return nil, fmt.Errorf("failed to open processed file: %w", err)
	}
	defer processedFile.Close()

	processedPath, err := s.storage.Save(ctx, processedFile, processedFilename)
	if err != nil {
		if delErr := s.storage.Delete(ctx, originalPath); delErr != nil {
			log.Printf("[WARN] Failed to cleanup original file after processed file save error: %v", delErr)
		}
		return nil, fmt.Errorf("failed to save processed audio: %w", err)
	}

	// Get processed file info
	processedInfo, err := os.Stat(processedTempFile)
	if err != nil {
		if delErr := s.storage.Delete(ctx, originalPath); delErr != nil {
			log.Printf("[WARN] Failed to cleanup original file after stat error: %v", delErr)
		}
		if delErr := s.storage.Delete(ctx, processedPath); delErr != nil {
			log.Printf("[WARN] Failed to cleanup processed file after stat error: %v", delErr)
		}
		return nil, fmt.Errorf("failed to stat processed file: %w", err)
	}

	// Get audio duration
	duration, err := s.getAudioDuration(processedTempFile)
	if err != nil {
		log.Printf("[WARN] Failed to get audio duration: %v", err)
		duration = 0
	}

	// Create cache entry
	cache = &models.AudioCache{
		PodcastIndexEpisodeID: podcastIndexEpisodeID,
		OriginalURL:           audioURL,
		OriginalSHA256:        sha256Hash,
		OriginalPath:          originalPath,
		OriginalSize:          fileInfo.Size(),
		ProcessedPath:         processedPath,
		ProcessedSHA256:       processedSHA256,
		ProcessedSize:         processedInfo.Size(),
		DurationSeconds:       duration,
		SampleRate:            16000,
	}

	if err := s.repository.Create(ctx, cache); err != nil {
		// Clean up files on error
		if delErr := s.storage.Delete(ctx, originalPath); delErr != nil {
			log.Printf("[WARN] Failed to cleanup original file after database error: %v", delErr)
		}
		if delErr := s.storage.Delete(ctx, processedPath); delErr != nil {
			log.Printf("[WARN] Failed to cleanup processed file after database error: %v", delErr)
		}
		return nil, fmt.Errorf("failed to create cache entry: %w", err)
	}

	log.Printf("[INFO] Successfully cached audio for Podcast Index episode %d", podcastIndexEpisodeID)
	return cache, nil
}

// GetCachedAudio retrieves cached audio without downloading
func (s *ServiceImpl) GetCachedAudio(ctx context.Context, podcastIndexEpisodeID int64) (*models.AudioCache, error) {
	cache, err := s.repository.GetByPodcastIndexEpisodeID(ctx, podcastIndexEpisodeID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil // Return nil without error if not found
		}
		return nil, err
	}

	// Update last used timestamp
	if err := s.UpdateLastUsed(ctx, cache.ID); err != nil {
		log.Printf("[WARN] Failed to update last used timestamp: %v", err)
	}

	return cache, nil
}

// ProcessAudioForML converts audio to 16kHz mono for ML training
func (s *ServiceImpl) ProcessAudioForML(ctx context.Context, originalPath string, outputPath string) error {
	// Use ffmpeg to convert to 16kHz mono
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", originalPath,
		"-ar", "16000", // Sample rate 16kHz
		"-ac", "1", // Mono
		"-b:a", "64k", // Bitrate
		"-f", "mp3", // Output format
		"-y", // Overwrite output
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// UpdateLastUsed updates the last used timestamp for cache entry
func (s *ServiceImpl) UpdateLastUsed(ctx context.Context, cacheID uint) error {
	cache := &models.AudioCache{ID: cacheID}
	cache.LastUsedAt = time.Now()
	return s.repository.Update(ctx, cache)
}

// CleanupOldCache removes cache entries older than specified days
func (s *ServiceImpl) CleanupOldCache(ctx context.Context, olderThanDays int) error {
	caches, err := s.repository.GetOlderThan(ctx, olderThanDays)
	if err != nil {
		return fmt.Errorf("failed to get old cache entries: %w", err)
	}

	for _, cache := range caches {
		// Delete files from storage
		if cache.OriginalPath != "" {
			if err := s.storage.Delete(ctx, cache.OriginalPath); err != nil {
				log.Printf("[WARN] Failed to delete original file %s: %v", cache.OriginalPath, err)
			}
		}

		if cache.ProcessedPath != "" {
			if err := s.storage.Delete(ctx, cache.ProcessedPath); err != nil {
				log.Printf("[WARN] Failed to delete processed file %s: %v", cache.ProcessedPath, err)
			}
		}

		// Delete database entry
		if err := s.repository.Delete(ctx, cache.ID); err != nil {
			log.Printf("[WARN] Failed to delete cache entry %d: %v", cache.ID, err)
		} else {
			log.Printf("[INFO] Deleted cache entry %d (Podcast Index episode %d)", cache.ID, cache.PodcastIndexEpisodeID)
		}
	}

	return nil
}

// GetCacheStats returns statistics about the cache
func (s *ServiceImpl) GetCacheStats(ctx context.Context) (*CacheStats, error) {
	return s.repository.GetStats(ctx)
}

// downloadAudio downloads audio from URL to temp file
func (s *ServiceImpl) downloadAudio(ctx context.Context, url string) (string, error) {
	// Create temp file
	tempFile, err := os.CreateTemp("", "audio_download_*.mp3")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tempFile.Close()

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Make request
	client := &http.Client{
		Timeout: 30 * time.Minute, // Long timeout for large files
	}

	resp, err := client.Do(req)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// Copy to temp file
	_, err = io.Copy(tempFile, resp.Body)
	if err != nil {
		os.Remove(tempFile.Name())
		return "", fmt.Errorf("failed to save audio: %w", err)
	}

	return tempFile.Name(), nil
}

// calculateSHA256 calculates SHA256 hash of file
func (s *ServiceImpl) calculateSHA256(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// getAudioDuration gets duration of audio file in seconds
func (s *ServiceImpl) getAudioDuration(filepath string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		filepath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	durationStr := strings.TrimSpace(string(output))
	duration, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse duration: %w", err)
	}

	return duration, nil
}
