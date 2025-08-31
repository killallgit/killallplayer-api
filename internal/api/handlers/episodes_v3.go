package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/internal/services/episodes"
)

// EpisodeHandlerV3 handles episode requests with clean separation of concerns
type EpisodeHandlerV3 struct {
	service     episodes.EpisodeService
	transformer episodes.EpisodeTransformer
}

// NewEpisodeHandlerV3 creates a new episode handler with service layer
func NewEpisodeHandlerV3(service episodes.EpisodeService, transformer episodes.EpisodeTransformer) *EpisodeHandlerV3 {
	return &EpisodeHandlerV3{
		service:     service,
		transformer: transformer,
	}
}

// GetEpisodesByPodcastID returns episodes in Podcast Index format
func (h *EpisodeHandlerV3) GetEpisodesByPodcastID(c *gin.Context) {
	// Parse and validate podcast ID
	podcastIDStr := c.Param("id")
	if podcastIDStr == "" {
		podcastIDStr = c.Query("id") // Support query param for Podcast Index compatibility
	}

	podcastID, err := strconv.ParseInt(podcastIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, h.transformer.CreateErrorResponse("Invalid podcast ID"))
		return
	}

	// Parse pagination
	max, _ := strconv.Atoi(c.DefaultQuery("max", "20"))
	if max < 1 || max > 1000 {
		max = 20
	}

	// Try to fetch fresh data from API and sync
	apiResponse, err := h.service.FetchAndSyncEpisodes(c.Request.Context(), podcastID, max)
	if err == nil && apiResponse != nil {
		c.JSON(http.StatusOK, apiResponse)
		return
	}

	// Fallback to database
	page := 1
	episodes, total, dbErr := h.service.GetEpisodesByPodcastID(c.Request.Context(), uint(podcastID), page, max)
	if dbErr != nil {
		c.JSON(http.StatusInternalServerError, h.transformer.CreateErrorResponse("Failed to fetch episodes"))
		return
	}

	// Transform and return
	response := h.transformer.CreateSuccessResponse(episodes, "")
	response.Query = podcastID

	if total > int64(len(episodes)) {
		response.Description = response.Description + ". Total available: " + strconv.FormatInt(total, 10)
	}

	c.JSON(http.StatusOK, response)
}

// GetEpisodeByID returns a single episode
func (h *EpisodeHandlerV3) GetEpisodeByID(c *gin.Context) {
	episodeIDStr := c.Param("id")
	episodeID, err := strconv.ParseUint(episodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, h.transformer.CreateErrorResponse("Invalid episode ID"))
		return
	}

	episode, err := h.service.GetEpisodeByID(c.Request.Context(), uint(episodeID))
	if err != nil {
		if IsNotFound(err) {
			c.JSON(http.StatusNotFound, h.transformer.CreateErrorResponse("Episode not found"))
		} else {
			c.JSON(http.StatusInternalServerError, h.transformer.CreateErrorResponse("Failed to fetch episode"))
		}
		return
	}

	response := h.transformer.CreateSingleEpisodeResponse(episode)
	c.JSON(http.StatusOK, response)
}

// GetEpisodeByGUID returns episode by GUID
func (h *EpisodeHandlerV3) GetEpisodeByGUID(c *gin.Context) {
	guid := c.Query("guid")
	if guid == "" {
		c.JSON(http.StatusBadRequest, h.transformer.CreateErrorResponse("GUID parameter is required"))
		return
	}

	episode, err := h.service.GetEpisodeByGUID(c.Request.Context(), guid)
	if err != nil {
		if IsNotFound(err) {
			c.JSON(http.StatusNotFound, h.transformer.CreateErrorResponse("Episode not found"))
		} else {
			c.JSON(http.StatusInternalServerError, h.transformer.CreateErrorResponse("Failed to fetch episode"))
		}
		return
	}

	response := h.transformer.CreateSingleEpisodeResponse(episode)
	c.JSON(http.StatusOK, response)
}

// GetRecentEpisodes returns recent episodes across all podcasts
func (h *EpisodeHandlerV3) GetRecentEpisodes(c *gin.Context) {
	max, _ := strconv.Atoi(c.DefaultQuery("max", "20"))
	if max < 1 || max > 100 {
		max = 20
	}

	episodes, err := h.service.GetRecentEpisodes(c.Request.Context(), max)
	if err != nil {
		c.JSON(http.StatusInternalServerError, h.transformer.CreateErrorResponse("Failed to fetch recent episodes"))
		return
	}

	response := h.transformer.CreateSuccessResponse(episodes, "Recent episodes across all podcasts")
	c.JSON(http.StatusOK, response)
}

// SyncEpisodesFromPodcastIndex manually triggers episode sync
func (h *EpisodeHandlerV3) SyncEpisodesFromPodcastIndex(c *gin.Context) {
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

	// Fetch and sync
	response, err := h.service.FetchAndSyncEpisodes(c.Request.Context(), podcastID, max)
	if err != nil {
		c.JSON(http.StatusInternalServerError, h.transformer.CreateErrorResponse("Failed to sync episodes"))
		return
	}

	c.JSON(http.StatusOK, response)
}

// UpdatePlaybackState updates playback position and played status
func (h *EpisodeHandlerV3) UpdatePlaybackState(c *gin.Context) {
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

	err = h.service.UpdatePlaybackState(c.Request.Context(), uint(episodeID), request.Position, request.Played)
	if err != nil {
		if IsNotFound(err) {
			c.JSON(http.StatusNotFound, h.transformer.CreateErrorResponse("Episode not found"))
		} else {
			c.JSON(http.StatusInternalServerError, h.transformer.CreateErrorResponse("Failed to update playback state"))
		}
		return
	}

	// Return success response
	episode, _ := h.service.GetEpisodeByID(c.Request.Context(), uint(episodeID))
	response := episodes.PodcastIndexResponse{
		Status:      "true",
		Items:       []episodes.PodcastIndexEpisode{h.transformer.ModelToPodcastIndex(episode)},
		Count:       1,
		Description: "Playback state updated successfully",
	}

	c.JSON(http.StatusOK, response)
}

// IsNotFound is a helper to check if an error is a not found error
func IsNotFound(err error) bool {
	return episodes.IsNotFound(err)
}

