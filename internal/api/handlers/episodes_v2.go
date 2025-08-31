package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/internal/services/episodes"
)

// EpisodeHandlerV2 handles episode requests with Podcast Index compatible responses
type EpisodeHandlerV2 struct {
	fetcher     *episodes.Fetcher
	repository  *episodes.CachedRepository
	transformer *episodes.Transformer
}

// NewEpisodeHandlerV2 creates a new episode handler with Podcast Index format support
func NewEpisodeHandlerV2(fetcher *episodes.Fetcher, repository *episodes.CachedRepository) *EpisodeHandlerV2 {
	return &EpisodeHandlerV2{
		fetcher:     fetcher,
		repository:  repository,
		transformer: episodes.NewTransformer(),
	}
}

// GetEpisodesByPodcastID returns episodes in Podcast Index format
func (h *EpisodeHandlerV2) GetEpisodesByPodcastID(c *gin.Context) {
	podcastIDStr := c.Param("id")
	podcastID, err := strconv.ParseInt(podcastIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, h.transformer.CreateErrorResponse("Invalid podcast ID"))
		return
	}

	// Get pagination parameters (Podcast Index uses 'max' not 'limit')
	max, _ := strconv.Atoi(c.DefaultQuery("max", "20"))
	if max < 1 || max > 1000 {
		max = 20
	}

	// Try to fetch from Podcast Index API first for fresh data
	piResponse, err := h.fetcher.GetEpisodesByPodcastID(c.Request.Context(), podcastID, max)
	if err == nil && piResponse != nil {
		// Store episodes in our database for caching
		go h.syncEpisodesToDatabase(piResponse.Items, uint(podcastID))
		
		// Return the Podcast Index response directly
		c.JSON(http.StatusOK, piResponse)
		return
	}

	// Fallback to database if API fails
	page := 1 // Podcast Index doesn't use pagination, but we'll use page 1
	episodes, total, dbErr := h.repository.GetEpisodesByPodcastID(c.Request.Context(), uint(podcastID), page, max)
	if dbErr != nil {
		c.JSON(http.StatusInternalServerError, h.transformer.CreateErrorResponse("Failed to fetch episodes"))
		return
	}

	// Transform database models to Podcast Index format
	response := h.transformer.CreateSuccessResponse(episodes, "")
	response.Query = podcastID
	
	// Add total count if different from items count
	if total > int64(len(episodes)) {
		response.Description = response.Description + ". Total available: " + strconv.FormatInt(total, 10)
	}

	c.JSON(http.StatusOK, response)
}

// GetEpisodeByID returns a single episode in Podcast Index format
func (h *EpisodeHandlerV2) GetEpisodeByID(c *gin.Context) {
	episodeIDStr := c.Param("id")
	episodeID, err := strconv.ParseUint(episodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, h.transformer.CreateErrorResponse("Invalid episode ID"))
		return
	}

	episode, err := h.repository.GetEpisodeByID(c.Request.Context(), uint(episodeID))
	if err != nil {
		c.JSON(http.StatusNotFound, h.transformer.CreateErrorResponse("Episode not found"))
		return
	}

	// Return single episode in Podcast Index format
	response := h.transformer.CreateSingleEpisodeResponse(episode)
	c.JSON(http.StatusOK, response)
}

// GetEpisodeByGUID returns episode by GUID in Podcast Index format
func (h *EpisodeHandlerV2) GetEpisodeByGUID(c *gin.Context) {
	guid := c.Query("guid")
	if guid == "" {
		c.JSON(http.StatusBadRequest, h.transformer.CreateErrorResponse("GUID parameter is required"))
		return
	}

	// Try Podcast Index API first
	piResponse, err := h.fetcher.GetEpisodeByGUID(c.Request.Context(), guid)
	if err == nil && piResponse != nil && piResponse.Status == "true" {
		c.JSON(http.StatusOK, piResponse)
		return
	}

	// Fallback to database
	episode, err := h.repository.GetEpisodeByGUID(c.Request.Context(), guid)
	if err != nil {
		c.JSON(http.StatusNotFound, h.transformer.CreateErrorResponse("Episode not found"))
		return
	}

	response := h.transformer.CreateSingleEpisodeResponse(episode)
	c.JSON(http.StatusOK, response)
}

// SyncEpisodesFromPodcastIndex syncs episodes and returns Podcast Index format
func (h *EpisodeHandlerV2) SyncEpisodesFromPodcastIndex(c *gin.Context) {
	podcastIDStr := c.Param("id")
	podcastID, err := strconv.ParseInt(podcastIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, h.transformer.CreateErrorResponse("Invalid podcast ID"))
		return
	}

	max, _ := strconv.Atoi(c.DefaultQuery("max", "50"))
	if max < 1 || max > 1000 {
		max = 50
	}

	// Fetch from Podcast Index
	piResponse, err := h.fetcher.GetEpisodesByPodcastID(c.Request.Context(), podcastID, max)
	if err != nil {
		c.JSON(http.StatusInternalServerError, h.transformer.CreateErrorResponse("Failed to fetch episodes from Podcast Index"))
		return
	}

	// Sync to database
	syncedCount := h.syncEpisodesToDatabase(piResponse.Items, uint(podcastID))

	// Add sync info to response
	piResponse.Description = piResponse.Description + ". Synced " + strconv.Itoa(syncedCount) + " episodes to database"
	
	c.JSON(http.StatusOK, piResponse)
}

// SearchEpisodes searches episodes and returns in Podcast Index format
func (h *EpisodeHandlerV2) SearchEpisodes(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, h.transformer.CreateErrorResponse("Query parameter 'q' is required"))
		return
	}

	max, _ := strconv.Atoi(c.DefaultQuery("max", "20"))
	if max < 1 || max > 100 {
		max = 20
	}

	// TODO: Implement actual search logic
	// For now, return empty results in correct format
	response := episodes.PodcastIndexResponse{
		Status:      "true",
		Items:       []episodes.PodcastIndexEpisode{},
		Count:       0,
		Query:       query,
		Description: "Episode search not yet implemented",
	}

	c.JSON(http.StatusOK, response)
}

// UpdatePlaybackState updates playback state (not part of Podcast Index API)
func (h *EpisodeHandlerV2) UpdatePlaybackState(c *gin.Context) {
	episodeIDStr := c.Param("id")
	episodeID, err := strconv.ParseUint(episodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, h.transformer.CreateErrorResponse("Invalid episode ID"))
		return
	}

	var request struct {
		Position int  `json:"position"`
		Played   bool `json:"played"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, h.transformer.CreateErrorResponse("Invalid request body"))
		return
	}

	episode, err := h.repository.GetEpisodeByID(c.Request.Context(), uint(episodeID))
	if err != nil {
		c.JSON(http.StatusNotFound, h.transformer.CreateErrorResponse("Episode not found"))
		return
	}

	episode.Position = request.Position
	episode.Played = request.Played

	if err := h.repository.UpdateEpisode(c.Request.Context(), episode); err != nil {
		c.JSON(http.StatusInternalServerError, h.transformer.CreateErrorResponse("Failed to update playback state"))
		return
	}

	// Return success in Podcast Index format
	response := episodes.PodcastIndexResponse{
		Status:      "true",
		Items:       []episodes.PodcastIndexEpisode{h.transformer.ModelToPodcastIndex(episode)},
		Count:       1,
		Description: "Playback state updated successfully",
	}

	c.JSON(http.StatusOK, response)
}

// Helper method to sync episodes to database
func (h *EpisodeHandlerV2) syncEpisodesToDatabase(piEpisodes []episodes.PodcastIndexEpisode, podcastID uint) int {
	syncedCount := 0
	for _, piEpisode := range piEpisodes {
		episode := h.transformer.PodcastIndexToModel(piEpisode, podcastID)
		
		// Check if episode exists
		existing, _ := h.repository.GetEpisodeByGUID(nil, episode.GUID)
		if existing != nil {
			// Update existing episode but preserve playback state
			episode.ID = existing.ID
			episode.Played = existing.Played
			episode.Position = existing.Position
			episode.CreatedAt = existing.CreatedAt
			
			if err := h.repository.UpdateEpisode(nil, episode); err == nil {
				syncedCount++
			}
		} else {
			// Create new episode
			if err := h.repository.CreateEpisode(nil, episode); err == nil {
				syncedCount++
			}
		}
	}
	return syncedCount
}

// GetRecentEpisodes returns recent episodes across all podcasts
func (h *EpisodeHandlerV2) GetRecentEpisodes(c *gin.Context) {
	max, _ := strconv.Atoi(c.DefaultQuery("max", "20"))
	if max < 1 || max > 100 {
		max = 20
	}

	episodes, err := h.repository.GetRecentEpisodes(c.Request.Context(), max)
	if err != nil {
		c.JSON(http.StatusInternalServerError, h.transformer.CreateErrorResponse("Failed to fetch recent episodes"))
		return
	}

	response := h.transformer.CreateSuccessResponse(episodes, "Recent episodes across all podcasts")
	c.JSON(http.StatusOK, response)
}