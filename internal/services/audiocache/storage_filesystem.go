package audiocache

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// FilesystemStorage implements StorageBackend for local filesystem storage
type FilesystemStorage struct {
	basePath string
}

// NewFilesystemStorage creates a new filesystem storage backend
func NewFilesystemStorage(basePath string) (StorageBackend, error) {
	// Ensure base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Create subdirectories for organization
	subdirs := []string{"original", "processed"}
	for _, subdir := range subdirs {
		path := filepath.Join(basePath, subdir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("failed to create subdirectory %s: %w", subdir, err)
		}
	}

	return &FilesystemStorage{
		basePath: basePath,
	}, nil
}

// Save saves data to filesystem
func (fs *FilesystemStorage) Save(ctx context.Context, data io.Reader, filename string) (string, error) {
	fullPath := filepath.Join(fs.basePath, filename)

	// Ensure directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data
	if _, err := io.Copy(file, data); err != nil {
		os.Remove(fullPath) // Clean up on error
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return fullPath, nil
}

// Load loads data from filesystem
func (fs *FilesystemStorage) Load(ctx context.Context, path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	return file, nil
}

// Delete removes data from filesystem
func (fs *FilesystemStorage) Delete(ctx context.Context, path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// Exists checks if a file exists
func (fs *FilesystemStorage) Exists(ctx context.Context, path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to stat file: %w", err)
	}
	return true, nil
}

// GetURL returns the file path (for local storage, this is just the path)
func (fs *FilesystemStorage) GetURL(ctx context.Context, path string) (string, error) {
	// For filesystem storage, we just return the path
	// In a cloud storage implementation, this would return a signed URL
	return path, nil
}
