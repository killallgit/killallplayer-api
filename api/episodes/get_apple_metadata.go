package episodes

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/api/types"
)

// GetAppleMetadata fetches Apple-specific metadata for the podcast containing this episode
// @Summary      Get Apple Podcasts metadata
// @Description  Fetch Apple-specific metadata including artwork, genres, and chart position
// @Tags         episodes
// @Accept       json
// @Produce      json
// @Param        id path int true "Episode ID (Podcast Index ID)"
// @Success      200 {object} AppleMetadataResponse "Apple metadata"
// @Failure      404 {object} types.ErrorResponse "Episode not found or no iTunes ID available"
// @Failure      500 {object} types.ErrorResponse "Internal server error"
// @Router       /api/v1/episodes/{id}/apple-metadata [get]
func GetAppleMetadata(deps *types.Dependencies) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Parse episode ID
		episodeID, ok := types.ParseInt64Param(c, "id")
		if !ok {
			return
		}

		// Get episode to extract iTunes ID
		episode, err := deps.EpisodeService.GetEpisodeByPodcastIndexID(c.Request.Context(), episodeID)
		if err != nil {
			if IsNotFound(err) {
				types.SendNotFound(c, "Episode not found")
			} else {
				log.Printf("[ERROR] Failed to fetch episode %d: %v", episodeID, err)
				types.SendInternalError(c, "Failed to fetch episode")
			}
			return
		}

		// Check if we have an iTunes ID
		if episode.FeedItunesID == nil || *episode.FeedItunesID == 0 {
			c.JSON(http.StatusOK, AppleMetadataResponse{
				Status:    "success",
				EpisodeID: episodeID,
				Message:   "No iTunes ID available for this podcast",
			})
			return
		}

		itunesID := *episode.FeedItunesID

		// Check if iTunes client is available
		if deps.ITunesClient == nil {
			log.Printf("[WARN] iTunes client not configured")
			c.JSON(http.StatusOK, AppleMetadataResponse{
				Status:    "success",
				EpisodeID: episodeID,
				ITunesID:  itunesID,
				Message:   "Apple metadata service not available",
			})
			return
		}

		// Fetch metadata from Apple
		metadata, err := fetchAppleMetadata(c.Request.Context(), deps, itunesID)
		if err != nil {
			log.Printf("[ERROR] Failed to fetch Apple metadata for iTunes ID %d: %v", itunesID, err)
			c.JSON(http.StatusOK, AppleMetadataResponse{
				Status:    "success",
				EpisodeID: episodeID,
				ITunesID:  itunesID,
				Message:   "Could not fetch metadata at this time",
			})
			return
		}

		now := time.Now()
		c.JSON(http.StatusOK, AppleMetadataResponse{
			Status:    "success",
			EpisodeID: episodeID,
			ITunesID:  itunesID,
			Metadata:  metadata,
			CachedAt:  &now,
		})
	}
}

// fetchAppleMetadata fetches metadata from Apple iTunes API
func fetchAppleMetadata(ctx context.Context, deps *types.Dependencies, itunesID int64) (*AppleMetadata, error) {
	// Fetch podcast metadata from Apple
	podcast, err := deps.ITunesClient.LookupPodcast(ctx, itunesID)
	if err != nil {
		return nil, err
	}

	// Build metadata response
	metadata := &AppleMetadata{
		TrackCount:        podcast.EpisodeCount,
		CollectionViewURL: podcast.ITunesURL,
		Country:           podcast.Country,
	}

	// Add artwork URLs if available
	if podcast.ArtworkURL != "" {
		metadata.ArtworkURLs = &AppleArtwork{
			Large:   podcast.ArtworkURL,
			Default: podcast.ArtworkURL,
			// The simplified Podcast type only has one artwork URL
			Medium: podcast.ArtworkURL,
			Small:  podcast.ArtworkURL,
		}
	}

	// Add genre information
	if podcast.Genre != "" {
		metadata.GenreNames = []string{podcast.Genre}
	}

	// Set content rating based on explicit flag
	if podcast.Explicit {
		metadata.ContentRating = "Explicit"
	} else {
		metadata.ContentRating = "Clean"
	}

	// TODO: Add chart position by checking genre-specific charts
	// This would require additional API calls to chart endpoints
	// TODO: Add genre IDs by making a raw iTunes API call

	return metadata, nil
}