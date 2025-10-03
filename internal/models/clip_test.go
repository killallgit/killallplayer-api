package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestClip_BeforeCreate(t *testing.T) {
	tests := []struct {
		name       string
		clip       Clip
		wantUUID   bool
		wantStatus string
	}{
		{
			name:       "generates UUID if empty",
			clip:       Clip{},
			wantUUID:   true,
			wantStatus: "pending",
		},
		{
			name: "keeps existing UUID",
			clip: Clip{
				UUID: "custom-uuid-123",
			},
			wantUUID:   true,
			wantStatus: "pending",
		},
		{
			name: "keeps existing status",
			clip: Clip{
				Status: "ready",
			},
			wantUUID:   true,
			wantStatus: "ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
				Logger: logger.Default.LogMode(logger.Silent),
			})

			err := tt.clip.BeforeCreate(db)
			require.NoError(t, err)

			if tt.wantUUID {
				assert.NotEmpty(t, tt.clip.UUID, "UUID should be generated")
			}

			assert.Equal(t, tt.wantStatus, tt.clip.Status)
		})
	}
}

func TestClip_GetOriginalDuration(t *testing.T) {
	clip := Clip{
		OriginalStartTime: 30.5,
		OriginalEndTime:   45.7,
	}

	duration := clip.GetOriginalDuration()
	assert.InDelta(t, 15.2, duration, 0.001, "Duration should be approximately 15.2")
}

func TestClip_IsReady(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"ready clip", "ready", true},
		{"processing clip", "processing", false},
		{"failed clip", "failed", false},
		{"queued clip", "queued", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clip := Clip{Status: tt.status}
			assert.Equal(t, tt.want, clip.IsReady())
		})
	}
}

func TestClip_IsFailed(t *testing.T) {
	tests := []struct {
		name   string
		status string
		want   bool
	}{
		{"failed clip", "failed", true},
		{"ready clip", "ready", false},
		{"processing clip", "processing", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clip := Clip{Status: tt.status}
			assert.Equal(t, tt.want, clip.IsFailed())
		})
	}
}

func TestClip_GetRelativePath(t *testing.T) {
	tests := []struct {
		name     string
		clip     Clip
		wantPath string
	}{
		{
			name: "with extracted filename",
			clip: Clip{
				Label:        "advertisement",
				ClipFilename: stringPtr("clip_abc123.wav"),
			},
			wantPath: "advertisement/clip_abc123.wav",
		},
		{
			name: "without extracted filename (auto-detected clip)",
			clip: Clip{
				Label:        "advertisement",
				ClipFilename: nil,
			},
			wantPath: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.clip.GetRelativePath()
			assert.Equal(t, tt.wantPath, path)
		})
	}
}

func TestClip_IsExtracted(t *testing.T) {
	tests := []struct {
		name          string
		clip          Clip
		wantExtracted bool
	}{
		{
			name: "extracted clip with filename",
			clip: Clip{
				Extracted:    true,
				ClipFilename: stringPtr("clip_abc123.wav"),
			},
			wantExtracted: true,
		},
		{
			name: "extracted flag true but no filename",
			clip: Clip{
				Extracted:    true,
				ClipFilename: nil,
			},
			wantExtracted: false,
		},
		{
			name: "not extracted",
			clip: Clip{
				Extracted:    false,
				ClipFilename: nil,
			},
			wantExtracted: false,
		},
		{
			name: "has filename but not marked extracted",
			clip: Clip{
				Extracted:    false,
				ClipFilename: stringPtr("clip_abc123.wav"),
			},
			wantExtracted: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantExtracted, tt.clip.IsExtracted())
		})
	}
}

func TestClip_ToExport(t *testing.T) {
	duration := 15.5
	confidence := 0.85

	tests := []struct {
		name       string
		clip       Clip
		wantExport ClipExport
	}{
		{
			name: "extracted clip with all fields",
			clip: Clip{
				UUID:              "test-uuid-123",
				Label:             "advertisement",
				AutoLabeled:       true,
				LabelConfidence:   &confidence,
				LabelMethod:       "peak_detection",
				ClipFilename:      stringPtr("clip_test.wav"),
				ClipDuration:      &duration,
				SourceEpisodeURL:  "https://example.com/episode.mp3",
				OriginalStartTime: 30.0,
				OriginalEndTime:   45.5,
			},
			wantExport: ClipExport{
				FilePath:          "advertisement/clip_test.wav",
				Label:             "advertisement",
				AutoLabeled:       true,
				LabelConfidence:   &confidence,
				LabelMethod:       "peak_detection",
				Duration:          15.5,
				SourceURL:         "https://example.com/episode.mp3",
				OriginalStartTime: 30.0,
				OriginalEndTime:   45.5,
				UUID:              "test-uuid-123",
			},
		},
		{
			name: "auto-detected clip without extraction",
			clip: Clip{
				UUID:              "test-uuid-456",
				Label:             "advertisement",
				AutoLabeled:       true,
				LabelConfidence:   &confidence,
				LabelMethod:       "peak_detection",
				ClipFilename:      nil,
				ClipDuration:      nil,
				SourceEpisodeURL:  "https://example.com/episode.mp3",
				OriginalStartTime: 60.0,
				OriginalEndTime:   75.0,
			},
			wantExport: ClipExport{
				FilePath:          "",
				Label:             "advertisement",
				AutoLabeled:       true,
				LabelConfidence:   &confidence,
				LabelMethod:       "peak_detection",
				Duration:          0.0,
				SourceURL:         "https://example.com/episode.mp3",
				OriginalStartTime: 60.0,
				OriginalEndTime:   75.0,
				UUID:              "test-uuid-456",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			export := tt.clip.ToExport()

			assert.Equal(t, tt.wantExport.FilePath, export.FilePath)
			assert.Equal(t, tt.wantExport.Label, export.Label)
			assert.Equal(t, tt.wantExport.AutoLabeled, export.AutoLabeled)
			assert.Equal(t, tt.wantExport.LabelConfidence, export.LabelConfidence)
			assert.Equal(t, tt.wantExport.LabelMethod, export.LabelMethod)
			assert.Equal(t, tt.wantExport.Duration, export.Duration)
			assert.Equal(t, tt.wantExport.SourceURL, export.SourceURL)
			assert.Equal(t, tt.wantExport.OriginalStartTime, export.OriginalStartTime)
			assert.Equal(t, tt.wantExport.OriginalEndTime, export.OriginalEndTime)
			assert.Equal(t, tt.wantExport.UUID, export.UUID)
		})
	}
}

func TestClip_DatabaseOperations(t *testing.T) {
	// Create in-memory database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)

	// Migrate schema
	err = db.AutoMigrate(&Clip{})
	require.NoError(t, err)

	t.Run("create clip with required fields only", func(t *testing.T) {
		clip := Clip{
			PodcastIndexEpisodeID: 12345,
			SourceEpisodeURL:      "https://example.com/episode.mp3",
			OriginalStartTime:     30.0,
			OriginalEndTime:       45.0,
			Label:                 "advertisement",
			AutoLabeled:           true,
			LabelMethod:           "peak_detection",
		}

		err := db.Create(&clip).Error
		require.NoError(t, err)
		assert.NotEmpty(t, clip.UUID, "UUID should be auto-generated")
		assert.Equal(t, "pending", clip.Status, "Status should default to pending")
		assert.False(t, clip.Extracted, "Extracted should default to false")
	})

	t.Run("create extracted clip with all fields", func(t *testing.T) {
		filename := "clip_xyz789.wav"
		duration := 15.0
		size := int64(480078)
		confidence := 0.85

		clip := Clip{
			PodcastIndexEpisodeID: 12345,
			SourceEpisodeURL:      "https://example.com/episode.mp3",
			OriginalStartTime:     60.0,
			OriginalEndTime:       75.0,
			Label:                 "music",
			AutoLabeled:           true,
			LabelConfidence:       &confidence,
			LabelMethod:           "peak_detection",
			ClipFilename:          &filename,
			ClipDuration:          &duration,
			ClipSizeBytes:         &size,
			Extracted:             true,
			Status:                "ready",
		}

		err := db.Create(&clip).Error
		require.NoError(t, err)
		assert.NotEmpty(t, clip.UUID)
		assert.Equal(t, "ready", clip.Status)
		assert.True(t, clip.Extracted)
		assert.Equal(t, filename, *clip.ClipFilename)
	})

	t.Run("query clips by episode ID", func(t *testing.T) {
		// Create multiple clips for same episode
		episodeID := int64(99999)
		for i := 0; i < 3; i++ {
			clip := Clip{
				PodcastIndexEpisodeID: episodeID,
				SourceEpisodeURL:      "https://example.com/episode2.mp3",
				OriginalStartTime:     float64(i * 30),
				OriginalEndTime:       float64(i*30 + 15),
				Label:                 "advertisement",
				AutoLabeled:           true,
			}
			err := db.Create(&clip).Error
			require.NoError(t, err)
		}

		// Query clips for episode
		var clips []Clip
		err := db.Where("podcast_index_episode_id = ?", episodeID).
			Order("original_start_time ASC").
			Find(&clips).Error
		require.NoError(t, err)
		assert.Len(t, clips, 3)

		// Verify ordering
		assert.Equal(t, 0.0, clips[0].OriginalStartTime)
		assert.Equal(t, 30.0, clips[1].OriginalStartTime)
		assert.Equal(t, 60.0, clips[2].OriginalStartTime)
	})
}

// Helper function
func stringPtr(s string) *string {
	return &s
}
