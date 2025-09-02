package waveforms

import (
	"context"
	"errors"
	"testing"

	"github.com/killallgit/player-api/internal/models"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupTestDB(t *testing.T) *gorm.DB {
	// Create in-memory SQLite database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to test database: %v", err)
	}

	// Run migrations
	err = db.AutoMigrate(&models.Episode{}, &models.Waveform{})
	if err != nil {
		t.Fatalf("Failed to migrate test database: %v", err)
	}

	return db
}

func createTestEpisode(t *testing.T, db *gorm.DB, id uint) *models.Episode {
	episode := &models.Episode{
		Model:          gorm.Model{ID: id},
		Title:          "Test Episode",
		AudioURL:       "https://example.com/audio.mp3",
		Duration:       func() *int { d := 300; return &d }(),
		EnclosureType:  "audio/mpeg",
		EnclosureLength: 12345678,
	}

	if err := db.Create(episode).Error; err != nil {
		t.Fatalf("Failed to create test episode: %v", err)
	}

	return episode
}

func TestNewRepository(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)

	if repo == nil {
		t.Error("NewRepository() returned nil")
	}
}

func TestRepository_Create(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create test episode first
	episode := createTestEpisode(t, db, 1)

	waveform := &models.Waveform{
		EpisodeID:  episode.ID,
		Duration:   300.0,
		Resolution: 1000,
		SampleRate: 44100,
	}
	err := waveform.SetPeaks([]float32{0.1, 0.5, 0.8, 0.3, 0.9})
	if err != nil {
		t.Fatalf("Failed to set peaks: %v", err)
	}

	err = repo.Create(ctx, waveform)
	if err != nil {
		t.Errorf("Create() error = %v", err)
		return
	}

	// Verify the waveform was created
	if waveform.ID == 0 {
		t.Error("Create() did not set ID")
	}

	// Verify we can retrieve it
	retrieved, err := repo.GetByEpisodeID(ctx, episode.ID)
	if err != nil {
		t.Errorf("GetByEpisodeID() after Create() error = %v", err)
		return
	}

	if retrieved.EpisodeID != episode.ID {
		t.Errorf("Retrieved waveform EpisodeID = %v, want %v", retrieved.EpisodeID, episode.ID)
	}

	if retrieved.Duration != 300.0 {
		t.Errorf("Retrieved waveform Duration = %v, want %v", retrieved.Duration, 300.0)
	}

	// Verify peaks data
	peaks, err := retrieved.Peaks()
	if err != nil {
		t.Errorf("Retrieved waveform Peaks() error = %v", err)
		return
	}

	expectedPeaks := []float32{0.1, 0.5, 0.8, 0.3, 0.9}
	if len(peaks) != len(expectedPeaks) {
		t.Errorf("Retrieved peaks length = %v, want %v", len(peaks), len(expectedPeaks))
		return
	}

	for i, peak := range peaks {
		if peak != expectedPeaks[i] {
			t.Errorf("Retrieved peaks[%d] = %v, want %v", i, peak, expectedPeaks[i])
		}
	}
}

func TestRepository_GetByEpisodeID(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create test episode
	episode := createTestEpisode(t, db, 1)

	// Test getting non-existent waveform
	_, err := repo.GetByEpisodeID(ctx, episode.ID)
	if err == nil {
		t.Error("GetByEpisodeID() expected error for non-existent waveform, got nil")
	}
	if !errors.Is(err, ErrWaveformNotFound) {
		t.Errorf("GetByEpisodeID() error = %v, want %v", err, ErrWaveformNotFound)
	}

	// Create a waveform
	waveform := &models.Waveform{
		EpisodeID:  episode.ID,
		Duration:   300.0,
		Resolution: 3,
		SampleRate: 44100,
	}
	waveform.SetPeaks([]float32{0.1, 0.5, 0.8})
	
	err = repo.Create(ctx, waveform)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Test getting existing waveform
	retrieved, err := repo.GetByEpisodeID(ctx, episode.ID)
	if err != nil {
		t.Errorf("GetByEpisodeID() error = %v", err)
		return
	}

	if retrieved.EpisodeID != episode.ID {
		t.Errorf("GetByEpisodeID() EpisodeID = %v, want %v", retrieved.EpisodeID, episode.ID)
	}

	if retrieved.Duration != 300.0 {
		t.Errorf("GetByEpisodeID() Duration = %v, want %v", retrieved.Duration, 300.0)
	}
}

func TestRepository_Update(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create test episode
	episode := createTestEpisode(t, db, 1)

	// Create a waveform
	waveform := &models.Waveform{
		EpisodeID:  episode.ID,
		Duration:   300.0,
		Resolution: 3,
		SampleRate: 44100,
	}
	waveform.SetPeaks([]float32{0.1, 0.5, 0.8})
	
	err := repo.Create(ctx, waveform)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Update the waveform
	waveform.Duration = 400.0
	waveform.SampleRate = 48000
	waveform.SetPeaks([]float32{0.2, 0.6, 0.9})

	err = repo.Update(ctx, waveform)
	if err != nil {
		t.Errorf("Update() error = %v", err)
		return
	}

	// Verify the update
	retrieved, err := repo.GetByEpisodeID(ctx, episode.ID)
	if err != nil {
		t.Errorf("GetByEpisodeID() after Update() error = %v", err)
		return
	}

	if retrieved.Duration != 400.0 {
		t.Errorf("Updated waveform Duration = %v, want %v", retrieved.Duration, 400.0)
	}

	if retrieved.SampleRate != 48000 {
		t.Errorf("Updated waveform SampleRate = %v, want %v", retrieved.SampleRate, 48000)
	}

	// Verify updated peaks
	peaks, err := retrieved.Peaks()
	if err != nil {
		t.Errorf("Updated waveform Peaks() error = %v", err)
		return
	}

	expectedPeaks := []float32{0.2, 0.6, 0.9}
	if len(peaks) != len(expectedPeaks) {
		t.Errorf("Updated peaks length = %v, want %v", len(peaks), len(expectedPeaks))
		return
	}

	for i, peak := range peaks {
		if peak != expectedPeaks[i] {
			t.Errorf("Updated peaks[%d] = %v, want %v", i, peak, expectedPeaks[i])
		}
	}
}

func TestRepository_Delete(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create test episode
	episode := createTestEpisode(t, db, 1)

	// Test deleting non-existent waveform
	err := repo.Delete(ctx, episode.ID)
	if err == nil {
		t.Error("Delete() expected error for non-existent waveform, got nil")
	}
	if !errors.Is(err, ErrWaveformNotFound) {
		t.Errorf("Delete() error = %v, want %v", err, ErrWaveformNotFound)
	}

	// Create a waveform
	waveform := &models.Waveform{
		EpisodeID:  episode.ID,
		Duration:   300.0,
		Resolution: 3,
		SampleRate: 44100,
	}
	waveform.SetPeaks([]float32{0.1, 0.5, 0.8})
	
	err = repo.Create(ctx, waveform)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Delete the waveform
	err = repo.Delete(ctx, episode.ID)
	if err != nil {
		t.Errorf("Delete() error = %v", err)
		return
	}

	// Verify it was deleted
	_, err = repo.GetByEpisodeID(ctx, episode.ID)
	if err == nil {
		t.Error("GetByEpisodeID() after Delete() expected error, got nil")
	}
	if !errors.Is(err, ErrWaveformNotFound) {
		t.Errorf("GetByEpisodeID() after Delete() error = %v, want %v", err, ErrWaveformNotFound)
	}
}

func TestRepository_Exists(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create test episode
	episode := createTestEpisode(t, db, 1)

	// Test non-existent waveform
	exists, err := repo.Exists(ctx, episode.ID)
	if err != nil {
		t.Errorf("Exists() error = %v", err)
		return
	}
	if exists {
		t.Error("Exists() = true for non-existent waveform, want false")
	}

	// Create a waveform
	waveform := &models.Waveform{
		EpisodeID:  episode.ID,
		Duration:   300.0,
		Resolution: 3,
		SampleRate: 44100,
	}
	waveform.SetPeaks([]float32{0.1, 0.5, 0.8})
	
	err = repo.Create(ctx, waveform)
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// Test existing waveform
	exists, err = repo.Exists(ctx, episode.ID)
	if err != nil {
		t.Errorf("Exists() error = %v", err)
		return
	}
	if !exists {
		t.Error("Exists() = false for existing waveform, want true")
	}

	// Delete and test again
	err = repo.Delete(ctx, episode.ID)
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	exists, err = repo.Exists(ctx, episode.ID)
	if err != nil {
		t.Errorf("Exists() after Delete() error = %v", err)
		return
	}
	if exists {
		t.Error("Exists() after Delete() = true, want false")
	}
}

func TestRepository_UniqueConstraint(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	ctx := context.Background()

	// Create test episode
	episode := createTestEpisode(t, db, 1)

	// Create first waveform
	waveform1 := &models.Waveform{
		EpisodeID:  episode.ID,
		Duration:   300.0,
		Resolution: 3,
		SampleRate: 44100,
	}
	waveform1.SetPeaks([]float32{0.1, 0.5, 0.8})
	
	err := repo.Create(ctx, waveform1)
	if err != nil {
		t.Fatalf("Create() first waveform error = %v", err)
	}

	// Try to create second waveform with same episode ID
	waveform2 := &models.Waveform{
		EpisodeID:  episode.ID,
		Duration:   400.0,
		Resolution: 3,
		SampleRate: 48000,
	}
	waveform2.SetPeaks([]float32{0.2, 0.6, 0.9})
	
	err = repo.Create(ctx, waveform2)
	if err == nil {
		t.Error("Create() second waveform expected error due to unique constraint, got nil")
	}

	// Verify only the first waveform exists
	retrieved, err := repo.GetByEpisodeID(ctx, episode.ID)
	if err != nil {
		t.Errorf("GetByEpisodeID() error = %v", err)
		return
	}

	if retrieved.Duration != 300.0 {
		t.Errorf("Retrieved waveform Duration = %v, want %v (should be first waveform)", retrieved.Duration, 300.0)
	}
}