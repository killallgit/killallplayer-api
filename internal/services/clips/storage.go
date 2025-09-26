package clips

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ClipStorage defines the interface for clip storage operations
type ClipStorage interface {
	// SaveClip saves a clip file to storage
	SaveClip(ctx context.Context, label, filename string, data io.Reader) error

	// GetClip retrieves a clip file from storage
	GetClip(ctx context.Context, label, filename string) (io.ReadCloser, error)

	// DeleteClip removes a clip file from storage
	DeleteClip(ctx context.Context, label, filename string) error

	// MoveClip moves a clip from one label to another (for label updates)
	MoveClip(ctx context.Context, oldLabel, newLabel, filename string) error

	// GetClipPath returns the full path to a clip (for local storage)
	GetClipPath(label, filename string) string

	// ListClipsByLabel lists all clips for a given label
	ListClipsByLabel(ctx context.Context, label string) ([]string, error)

	// ExportDataset exports all clips to a directory for ML training
	ExportDataset(ctx context.Context, exportPath string, labels []string) error
}

// LocalClipStorage implements ClipStorage using the local filesystem
type LocalClipStorage struct {
	basePath string // Base directory for all clips
}

// NewLocalClipStorage creates a new local filesystem storage
func NewLocalClipStorage(basePath string) (*LocalClipStorage, error) {
	// Ensure base path exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create base storage directory: %w", err)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	return &LocalClipStorage{
		basePath: absPath,
	}, nil
}

// SaveClip saves a clip file to the filesystem
func (s *LocalClipStorage) SaveClip(ctx context.Context, label, filename string, data io.Reader) error {
	// Sanitize label to be filesystem-safe
	label = s.sanitizeLabel(label)

	// Create label directory if it doesn't exist
	labelDir := filepath.Join(s.basePath, label)
	if err := os.MkdirAll(labelDir, 0755); err != nil {
		return fmt.Errorf("failed to create label directory: %w", err)
	}

	// Create the file
	filePath := filepath.Join(labelDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy data to file
	if _, err := io.Copy(file, data); err != nil {
		os.Remove(filePath) // Clean up on failure
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// GetClip retrieves a clip file from the filesystem
func (s *LocalClipStorage) GetClip(ctx context.Context, label, filename string) (io.ReadCloser, error) {
	label = s.sanitizeLabel(label)
	filePath := filepath.Join(s.basePath, label, filename)

	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("clip not found: %s/%s", label, filename)
		}
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	return file, nil
}

// DeleteClip removes a clip file from the filesystem
func (s *LocalClipStorage) DeleteClip(ctx context.Context, label, filename string) error {
	label = s.sanitizeLabel(label)
	filePath := filepath.Join(s.basePath, label, filename)

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return nil // Already deleted, not an error
		}
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Try to remove label directory if empty
	labelDir := filepath.Join(s.basePath, label)
	_ = os.Remove(labelDir) // Ignore error if directory is not empty

	return nil
}

// MoveClip moves a clip from one label directory to another
func (s *LocalClipStorage) MoveClip(ctx context.Context, oldLabel, newLabel, filename string) error {
	oldLabel = s.sanitizeLabel(oldLabel)
	newLabel = s.sanitizeLabel(newLabel)

	// Create new label directory if needed
	newLabelDir := filepath.Join(s.basePath, newLabel)
	if err := os.MkdirAll(newLabelDir, 0755); err != nil {
		return fmt.Errorf("failed to create new label directory: %w", err)
	}

	// Move the file
	oldPath := filepath.Join(s.basePath, oldLabel, filename)
	newPath := filepath.Join(s.basePath, newLabel, filename)

	if err := os.Rename(oldPath, newPath); err != nil {
		// If rename fails (e.g., across filesystems), try copy and delete
		if err := s.copyFile(oldPath, newPath); err != nil {
			return fmt.Errorf("failed to move file: %w", err)
		}
		if err := os.Remove(oldPath); err != nil {
			// Log but don't fail if we can't delete the original
			fmt.Printf("Warning: failed to delete original file after copy: %v\n", err)
		}
	}

	// Try to remove old label directory if empty
	oldLabelDir := filepath.Join(s.basePath, oldLabel)
	_ = os.Remove(oldLabelDir) // Ignore error if directory is not empty

	return nil
}

// GetClipPath returns the full filesystem path to a clip
func (s *LocalClipStorage) GetClipPath(label, filename string) string {
	label = s.sanitizeLabel(label)
	return filepath.Join(s.basePath, label, filename)
}

// ListClipsByLabel lists all clip filenames for a given label
func (s *LocalClipStorage) ListClipsByLabel(ctx context.Context, label string) ([]string, error) {
	label = s.sanitizeLabel(label)
	labelDir := filepath.Join(s.basePath, label)

	// Check if directory exists
	if _, err := os.Stat(labelDir); os.IsNotExist(err) {
		return []string{}, nil // Return empty list if label doesn't exist
	}

	entries, err := os.ReadDir(labelDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var clips []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".wav") {
			clips = append(clips, entry.Name())
		}
	}

	return clips, nil
}

// ExportDataset exports all clips to a directory structure for ML training
func (s *LocalClipStorage) ExportDataset(ctx context.Context, exportPath string, labels []string) error {
	// Create export directory
	if err := os.MkdirAll(exportPath, 0755); err != nil {
		return fmt.Errorf("failed to create export directory: %w", err)
	}

	// Copy clips for each label
	for _, label := range labels {
		sanitizedLabel := s.sanitizeLabel(label)
		sourceLabelDir := filepath.Join(s.basePath, sanitizedLabel)
		destLabelDir := filepath.Join(exportPath, sanitizedLabel)

		// Skip if source doesn't exist
		if _, err := os.Stat(sourceLabelDir); os.IsNotExist(err) {
			continue
		}

		// Create destination label directory
		if err := os.MkdirAll(destLabelDir, 0755); err != nil {
			return fmt.Errorf("failed to create export label directory %s: %w", label, err)
		}

		// Copy all clips in this label
		clips, err := s.ListClipsByLabel(ctx, label)
		if err != nil {
			return fmt.Errorf("failed to list clips for label %s: %w", label, err)
		}

		for _, clipName := range clips {
			srcPath := filepath.Join(sourceLabelDir, clipName)
			destPath := filepath.Join(destLabelDir, clipName)

			if err := s.copyFile(srcPath, destPath); err != nil {
				return fmt.Errorf("failed to copy clip %s: %w", clipName, err)
			}
		}
	}

	return nil
}

// sanitizeLabel makes a label safe for use as a directory name
func (s *LocalClipStorage) sanitizeLabel(label string) string {
	// Replace spaces with underscores
	label = strings.ReplaceAll(label, " ", "_")

	// Replace problematic characters
	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
		".", "_",
	)
	label = replacer.Replace(label)

	// Convert to lowercase for consistency
	label = strings.ToLower(label)

	// Trim any leading/trailing whitespace or dashes
	label = strings.Trim(label, " -_")

	// If label is empty after sanitization, use "unknown"
	if label == "" {
		label = "unknown"
	}

	return label
}

// copyFile copies a file from src to dst
func (s *LocalClipStorage) copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	if _, err := io.Copy(destFile, sourceFile); err != nil {
		return err
	}

	// Sync to ensure write is complete
	return destFile.Sync()
}
