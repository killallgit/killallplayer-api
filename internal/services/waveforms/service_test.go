package waveforms

import (
	"context"
	"errors"
	"testing"

	"github.com/killallgit/player-api/internal/models"
)

// mockWaveformRepository is a mock implementation of WaveformRepository for testing
type mockWaveformRepository struct {
	waveforms map[int64]*models.Waveform
	shouldErr bool
}

func newMockWaveformRepository() *mockWaveformRepository {
	return &mockWaveformRepository{
		waveforms: make(map[int64]*models.Waveform),
		shouldErr: false,
	}
}

func (m *mockWaveformRepository) GetByPodcastIndexEpisodeID(ctx context.Context, podcastIndexEpisodeID int64) (*models.Waveform, error) {
	if m.shouldErr {
		return nil, errors.New("mock database error")
	}

	waveform, exists := m.waveforms[podcastIndexEpisodeID]
	if !exists {
		return nil, ErrWaveformNotFound
	}

	return waveform, nil
}

func (m *mockWaveformRepository) Create(ctx context.Context, waveform *models.Waveform) error {
	if m.shouldErr {
		return errors.New("mock database error")
	}

	m.waveforms[waveform.PodcastIndexEpisodeID] = waveform
	return nil
}

func (m *mockWaveformRepository) Update(ctx context.Context, waveform *models.Waveform) error {
	if m.shouldErr {
		return errors.New("mock database error")
	}

	m.waveforms[waveform.PodcastIndexEpisodeID] = waveform
	return nil
}

func (m *mockWaveformRepository) Delete(ctx context.Context, podcastIndexEpisodeID int64) error {
	if m.shouldErr {
		return errors.New("mock database error")
	}

	_, exists := m.waveforms[podcastIndexEpisodeID]
	if !exists {
		return ErrWaveformNotFound
	}

	delete(m.waveforms, podcastIndexEpisodeID)
	return nil
}

func (m *mockWaveformRepository) Exists(ctx context.Context, podcastIndexEpisodeID int64) (bool, error) {
	if m.shouldErr {
		return false, errors.New("mock database error")
	}

	_, exists := m.waveforms[podcastIndexEpisodeID]
	return exists, nil
}

func TestNewService(t *testing.T) {
	repo := newMockWaveformRepository()
	service := NewService(repo)

	if service == nil {
		t.Error("NewService() returned nil")
	}
}

func TestService_GetWaveform(t *testing.T) {
	tests := []struct {
		name        string
		episodeID   int64
		setupRepo   func(*mockWaveformRepository)
		wantErr     bool
		expectedErr error
	}{
		{
			name:      "successful get",
			episodeID: 123,
			setupRepo: func(repo *mockWaveformRepository) {
				waveform := &models.Waveform{
					PodcastIndexEpisodeID: 123,
					Duration:              300.0,
					Resolution:            1000,
					SampleRate:            44100,
				}
				if err := waveform.SetPeaks([]float32{0.1, 0.5, 0.8}); err != nil {
					t.Fatalf("SetPeaks() error = %v", err)
				}
				repo.waveforms[123] = waveform
			},
			wantErr:     false,
			expectedErr: nil,
		},
		{
			name:        "waveform not found",
			episodeID:   999,
			setupRepo:   func(repo *mockWaveformRepository) {},
			wantErr:     true,
			expectedErr: ErrWaveformNotFound,
		},
		{
			name:        "invalid episode ID",
			episodeID:   0,
			setupRepo:   func(repo *mockWaveformRepository) {},
			wantErr:     true,
			expectedErr: ErrInvalidEpisodeID,
		},
		{
			name:      "repository error",
			episodeID: 123,
			setupRepo: func(repo *mockWaveformRepository) {
				repo.shouldErr = true
			},
			wantErr:     true,
			expectedErr: nil, // Any error is acceptable
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockWaveformRepository()
			tt.setupRepo(repo)

			service := NewService(repo)
			ctx := context.Background()

			waveform, err := service.GetWaveform(ctx, tt.episodeID)

			if tt.wantErr {
				if err == nil {
					t.Error("GetWaveform() expected error, got nil")
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("GetWaveform() error = %v, want %v", err, tt.expectedErr)
				}
				return
			}

			if err != nil {
				t.Errorf("GetWaveform() unexpected error = %v", err)
				return
			}

			if waveform == nil {
				t.Error("GetWaveform() returned nil waveform")
				return
			}

			if waveform.PodcastIndexEpisodeID != tt.episodeID {
				t.Errorf("GetWaveform() PodcastIndexEpisodeID = %v, want %v", waveform.PodcastIndexEpisodeID, tt.episodeID)
			}
		})
	}
}

func TestService_SaveWaveform(t *testing.T) {
	tests := []struct {
		name        string
		waveform    *models.Waveform
		setupRepo   func(*mockWaveformRepository)
		wantErr     bool
		expectedErr error
	}{
		{
			name: "successful create",
			waveform: &models.Waveform{
				PodcastIndexEpisodeID: 123,
				Duration:              300.0,
				Resolution:            3,
				SampleRate:            44100,
			},
			setupRepo: func(repo *mockWaveformRepository) {
				// Waveform doesn't exist yet
			},
			wantErr: false,
		},
		{
			name: "successful update",
			waveform: &models.Waveform{
				PodcastIndexEpisodeID: 123,
				Duration:              400.0,
				Resolution:            3,
				SampleRate:            48000,
			},
			setupRepo: func(repo *mockWaveformRepository) {
				// Existing waveform
				existing := &models.Waveform{PodcastIndexEpisodeID: 123, Duration: 300.0}
				err := existing.SetPeaks([]float32{0.1, 0.2, 0.3})
				if err != nil {
					t.Fatalf("SetPeaks() error = %v", err)
				}
				repo.waveforms[123] = existing
			},
			wantErr: false,
		},
		{
			name: "invalid episode ID",
			waveform: &models.Waveform{
				PodcastIndexEpisodeID: 0,
			},
			setupRepo:   func(repo *mockWaveformRepository) {},
			wantErr:     true,
			expectedErr: ErrInvalidEpisodeID,
		},
		{
			name: "invalid peaks data",
			waveform: &models.Waveform{
				PodcastIndexEpisodeID: 123,
				PeaksData:             []byte{}, // Empty peaks data
			},
			setupRepo:   func(repo *mockWaveformRepository) {},
			wantErr:     true,
			expectedErr: ErrInvalidPeaksData,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockWaveformRepository()
			tt.setupRepo(repo)

			// Set peaks data if not already set and not testing empty peaks
			if tt.name != "invalid peaks data" && len(tt.waveform.PeaksData) == 0 {
				err := tt.waveform.SetPeaks([]float32{0.1, 0.5, 0.8})
				if err != nil {
					t.Fatalf("SetPeaks() error = %v", err)
				}
			}

			service := NewService(repo)
			ctx := context.Background()

			err := service.SaveWaveform(ctx, tt.waveform)

			if tt.wantErr {
				if err == nil {
					t.Error("SaveWaveform() expected error, got nil")
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("SaveWaveform() error = %v, want %v", err, tt.expectedErr)
				}
				return
			}

			if err != nil {
				t.Errorf("SaveWaveform() unexpected error = %v", err)
				return
			}

			// Verify waveform was saved
			saved, exists := repo.waveforms[tt.waveform.PodcastIndexEpisodeID]
			if !exists {
				t.Error("SaveWaveform() waveform not found in repository")
				return
			}

			if saved.Duration != tt.waveform.Duration {
				t.Errorf("SaveWaveform() Duration = %v, want %v", saved.Duration, tt.waveform.Duration)
			}
		})
	}
}

func TestService_DeleteWaveform(t *testing.T) {
	tests := []struct {
		name        string
		episodeID   int64
		setupRepo   func(*mockWaveformRepository)
		wantErr     bool
		expectedErr error
	}{
		{
			name:      "successful delete",
			episodeID: 123,
			setupRepo: func(repo *mockWaveformRepository) {
				waveform := &models.Waveform{PodcastIndexEpisodeID: 123}
				if err := waveform.SetPeaks([]float32{0.1, 0.5, 0.8}); err != nil {
					t.Fatalf("SetPeaks() error = %v", err)
				}
				repo.waveforms[123] = waveform
			},
			wantErr: false,
		},
		{
			name:        "waveform not found",
			episodeID:   999,
			setupRepo:   func(repo *mockWaveformRepository) {},
			wantErr:     true,
			expectedErr: ErrWaveformNotFound,
		},
		{
			name:        "invalid episode ID",
			episodeID:   0,
			setupRepo:   func(repo *mockWaveformRepository) {},
			wantErr:     true,
			expectedErr: ErrInvalidEpisodeID,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockWaveformRepository()
			tt.setupRepo(repo)

			service := NewService(repo)
			ctx := context.Background()

			err := service.DeleteWaveform(ctx, tt.episodeID)

			if tt.wantErr {
				if err == nil {
					t.Error("DeleteWaveform() expected error, got nil")
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("DeleteWaveform() error = %v, want %v", err, tt.expectedErr)
				}
				return
			}

			if err != nil {
				t.Errorf("DeleteWaveform() unexpected error = %v", err)
				return
			}

			// Verify waveform was deleted
			_, exists := repo.waveforms[tt.episodeID]
			if exists {
				t.Error("DeleteWaveform() waveform still exists in repository")
			}
		})
	}
}

func TestService_WaveformExists(t *testing.T) {
	tests := []struct {
		name        string
		episodeID   int64
		setupRepo   func(*mockWaveformRepository)
		wantExists  bool
		wantErr     bool
		expectedErr error
	}{
		{
			name:      "waveform exists",
			episodeID: 123,
			setupRepo: func(repo *mockWaveformRepository) {
				waveform := &models.Waveform{PodcastIndexEpisodeID: 123}
				if err := waveform.SetPeaks([]float32{0.1, 0.5, 0.8}); err != nil {
					t.Fatalf("SetPeaks() error = %v", err)
				}
				repo.waveforms[123] = waveform
			},
			wantExists: true,
			wantErr:    false,
		},
		{
			name:       "waveform does not exist",
			episodeID:  999,
			setupRepo:  func(repo *mockWaveformRepository) {},
			wantExists: false,
			wantErr:    false,
		},
		{
			name:        "invalid episode ID",
			episodeID:   0,
			setupRepo:   func(repo *mockWaveformRepository) {},
			wantExists:  false,
			wantErr:     true,
			expectedErr: ErrInvalidEpisodeID,
		},
		{
			name:      "repository error",
			episodeID: 123,
			setupRepo: func(repo *mockWaveformRepository) {
				repo.shouldErr = true
			},
			wantExists: false,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockWaveformRepository()
			tt.setupRepo(repo)

			service := NewService(repo)
			ctx := context.Background()

			exists, err := service.WaveformExists(ctx, tt.episodeID)

			if tt.wantErr {
				if err == nil {
					t.Error("WaveformExists() expected error, got nil")
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("WaveformExists() error = %v, want %v", err, tt.expectedErr)
				}
				return
			}

			if err != nil {
				t.Errorf("WaveformExists() unexpected error = %v", err)
				return
			}

			if exists != tt.wantExists {
				t.Errorf("WaveformExists() = %v, want %v", exists, tt.wantExists)
			}
		})
	}
}
