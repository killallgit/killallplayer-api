package cleanup

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Service handles cleanup of temporary files
type Service struct {
	tempDir         string
	maxAge          time.Duration
	cleanupInterval time.Duration
	cancel          context.CancelFunc
}

// NewService creates a new cleanup service
func NewService(tempDir string, maxAge, cleanupInterval time.Duration) *Service {
	return &Service{
		tempDir:         tempDir,
		maxAge:          maxAge,
		cleanupInterval: cleanupInterval,
	}
}

// Start begins the cleanup service
func (s *Service) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	ticker := time.NewTicker(s.cleanupInterval)
	defer ticker.Stop()

	// Run initial cleanup
	s.cleanup()

	// Run periodic cleanup
	go func() {
		for {
			select {
			case <-ticker.C:
				s.cleanup()
			case <-ctx.Done():
				log.Println("[INFO] Cleanup service stopped")
				return
			}
		}
	}()

	log.Printf("[INFO] Cleanup service started (interval: %v, max age: %v)", s.cleanupInterval, s.maxAge)
}

// Stop stops the cleanup service
func (s *Service) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
}

// cleanup removes old temporary files
func (s *Service) cleanup() {
	// Check if temp directory exists
	if _, err := os.Stat(s.tempDir); os.IsNotExist(err) {
		return
	}

	// Walk through temp directory
	err := filepath.Walk(s.tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files with errors
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file is an episode temp file (pattern: episode_*_*.mp3)
		if strings.HasPrefix(info.Name(), "episode_") && strings.HasSuffix(info.Name(), ".mp3") {
			// Check age
			if time.Since(info.ModTime()) > s.maxAge {
				log.Printf("[DEBUG] Removing old temp file: %s", path)
				if err := os.Remove(path); err != nil {
					log.Printf("[WARN] Failed to remove temp file %s: %v", path, err)
				}
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("[ERROR] Cleanup walk error: %v", err)
	}
}

// CleanupSingleFile removes a specific temp file
func CleanupSingleFile(path string) {
	if path == "" {
		return
	}

	// Safety check: only remove files in temp directories
	if !strings.Contains(path, "/tmp/") && !strings.Contains(path, "\\temp\\") {
		log.Printf("[WARN] Refusing to cleanup file outside temp directory: %s", path)
		return
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		log.Printf("[DEBUG] Failed to cleanup temp file %s: %v", path, err)
	}
}
