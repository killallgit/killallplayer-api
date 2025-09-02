package waveform

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
	"github.com/killallgit/player-api/internal/services/waveforms"
)

// WaveformData represents the waveform peaks for an audio file
type WaveformData struct {
	EpisodeID  int64     `json:"episode_id"`
	Peaks      []float32 `json:"peaks"`
	Duration   float64   `json:"duration"`   // Duration in seconds
	Resolution int       `json:"resolution"` // Number of peaks
	SampleRate int       `json:"sample_rate,omitempty"`
	Cached     bool      `json:"cached"`
}

// GetWaveform returns waveform data for an episode
func GetWaveform(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")
		
		// Parse episode ID
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || episodeID < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode ID"})
			return
		}

		// Check if WaveformService is available
		if deps.WaveformService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Waveform service not available",
				"episode_id": episodeID,
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Get waveform from database
		waveformModel, err := deps.WaveformService.GetWaveform(ctx, uint(episodeID))
		if err != nil {
			if errors.Is(err, waveforms.ErrWaveformNotFound) {
				c.JSON(http.StatusNotFound, gin.H{
					"error":      "Waveform not found for episode",
					"episode_id": episodeID,
				})
				return
			}

			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to retrieve waveform",
				"episode_id": episodeID,
			})
			return
		}

		// Decode peaks data
		peaks, err := waveformModel.Peaks()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to decode waveform data",
				"episode_id": episodeID,
			})
			return
		}

		// Convert to response format
		waveformData := &WaveformData{
			EpisodeID:  episodeID,
			Peaks:      peaks,
			Duration:   waveformModel.Duration,
			Resolution: waveformModel.Resolution,
			SampleRate: waveformModel.SampleRate,
			Cached:     true, // Always true since it's from database
		}

		c.JSON(http.StatusOK, waveformData)
	}
}

// generateTestWaveform creates synthetic waveform data for testing
func generateTestWaveform(episodeID int64) *WaveformData {
	resolution := 1000 // 1000 peaks for the entire audio
	peaks := make([]float32, resolution)
	
	// Generate a realistic-looking waveform pattern
	// Using sine waves with some randomness to simulate audio
	for i := 0; i < resolution; i++ {
		t := float64(i) / float64(resolution)
		
		// Combine multiple sine waves for complexity
		wave1 := math.Sin(2 * math.Pi * 3 * t)   // Low frequency
		wave2 := math.Sin(2 * math.Pi * 10 * t)  // Mid frequency
		wave3 := math.Sin(2 * math.Pi * 50 * t)  // High frequency
		
		// Add envelope to simulate song structure (intro, verse, chorus, etc.)
		envelope := 0.3 + 0.7*math.Sin(math.Pi*t)
		
		// Combine waves with different weights
		value := (0.6*wave1 + 0.3*wave2 + 0.1*wave3) * envelope
		
		// Ensure values are between 0 and 1 (normalized)
		peaks[i] = float32(math.Abs(value))
	}
	
	return &WaveformData{
		EpisodeID:  episodeID,
		Peaks:      peaks,
		Duration:   300.0, // 5 minutes test duration
		Resolution: resolution,
		SampleRate: 44100, // Standard CD quality
		Cached:     false,  // Will be true when we implement caching
	}
}

// GetWaveformStatus returns the processing status of a waveform
func GetWaveformStatus(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		episodeIDStr := c.Param("id")
		
		// Parse episode ID
		episodeID, err := strconv.ParseInt(episodeIDStr, 10, 64)
		if err != nil || episodeID < 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid episode ID"})
			return
		}

		// Check if WaveformService is available
		if deps.WaveformService == nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Waveform service not available",
				"episode_id": episodeID,
			})
			return
		}

		// Create context with timeout
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Check if waveform exists
		exists, err := deps.WaveformService.WaveformExists(ctx, uint(episodeID))
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":      "Failed to check waveform status",
				"episode_id": episodeID,
			})
			return
		}

		if exists {
			status := gin.H{
				"episode_id": episodeID,
				"status":     "completed",
				"progress":   100,
				"message":    "Waveform ready",
			}
			c.JSON(http.StatusOK, status)
		} else {
			status := gin.H{
				"episode_id": episodeID,
				"status":     "not_found",
				"progress":   0,
				"message":    "Waveform not available",
			}
			c.JSON(http.StatusNotFound, status)
		}
	}
}