package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/killallgit/player-api/internal/services/episodes"
)

type EpisodeHandler struct {
	fetcher    *episodes.Fetcher
	repository *episodes.CachedRepository
}

func NewEpisodeHandler(fetcher *episodes.Fetcher, repository *episodes.CachedRepository) *EpisodeHandler {
	return &EpisodeHandler{
		fetcher:    fetcher,
		repository: repository,
	}
}

func (h *EpisodeHandler) GetEpisodesByPodcastID(c *gin.Context) {
	podcastIDStr := c.Param("id")
	podcastID, err := strconv.ParseUint(podcastIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid podcast ID",
		})
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}

	episodes, total, err := h.repository.GetEpisodesByPodcastID(c.Request.Context(), uint(podcastID), page, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch episodes",
		})
		return
	}

	totalPages := (int(total) + limit - 1) / limit

	c.JSON(http.StatusOK, gin.H{
		"episodes": episodes,
		"pagination": gin.H{
			"page":        page,
			"limit":       limit,
			"total":       total,
			"total_pages": totalPages,
			"has_next":    page < totalPages,
			"has_prev":    page > 1,
		},
	})
}

func (h *EpisodeHandler) GetEpisodeByID(c *gin.Context) {
	episodeIDStr := c.Param("id")
	episodeID, err := strconv.ParseUint(episodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid episode ID",
		})
		return
	}

	episode, err := h.repository.GetEpisodeByID(c.Request.Context(), uint(episodeID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Episode not found",
		})
		return
	}

	c.JSON(http.StatusOK, episode)
}

func (h *EpisodeHandler) SyncEpisodesFromPodcastIndex(c *gin.Context) {
	podcastIDStr := c.Param("id")
	podcastID, err := strconv.ParseInt(podcastIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid podcast ID",
		})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 200 {
		limit = 50
	}

	fetchedEpisodes, err := h.fetcher.GetEpisodesByPodcastID(c.Request.Context(), podcastID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch episodes from Podcast Index",
		})
		return
	}

	var syncedCount int
	var errors []string

	for _, episode := range fetchedEpisodes {
		episode.PodcastID = uint(podcastID)
		
		existing, _ := h.repository.GetEpisodeByGUID(c.Request.Context(), episode.GUID)
		if existing != nil {
			episode.ID = existing.ID
			episode.Played = existing.Played
			episode.Position = existing.Position
			err = h.repository.UpdateEpisode(c.Request.Context(), &episode)
		} else {
			err = h.repository.CreateEpisode(c.Request.Context(), &episode)
		}

		if err != nil {
			errors = append(errors, err.Error())
		} else {
			syncedCount++
		}
	}

	response := gin.H{
		"synced":       syncedCount,
		"total":        len(fetchedEpisodes),
		"podcast_id":   podcastID,
		"synced_at":    time.Now(),
	}

	if len(errors) > 0 {
		response["errors"] = errors
	}

	c.JSON(http.StatusOK, response)
}

func (h *EpisodeHandler) UpdatePlaybackState(c *gin.Context) {
	episodeIDStr := c.Param("id")
	episodeID, err := strconv.ParseUint(episodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid episode ID",
		})
		return
	}

	var request struct {
		Position int  `json:"position"`
		Played   bool `json:"played"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	episode, err := h.repository.GetEpisodeByID(c.Request.Context(), uint(episodeID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Episode not found",
		})
		return
	}

	episode.Position = request.Position
	episode.Played = request.Played

	if err := h.repository.UpdateEpisode(c.Request.Context(), episode); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to update playback state",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "Playback state updated",
		"episode":  episode,
	})
}

func (h *EpisodeHandler) GetEpisodeMetadata(c *gin.Context) {
	episodeIDStr := c.Param("id")
	episodeID, err := strconv.ParseUint(episodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid episode ID",
		})
		return
	}

	episode, err := h.repository.GetEpisodeByID(c.Request.Context(), uint(episodeID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Episode not found",
		})
		return
	}

	metadata, err := h.fetcher.GetEpisodeMetadata(c.Request.Context(), episode.AudioURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch metadata",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"episode":  episode,
		"metadata": metadata,
	})
}

func (h *EpisodeHandler) SearchEpisodes(c *gin.Context) {
	var request struct {
		Query     string `json:"query" binding:"required"`
		PodcastID uint   `json:"podcast_id,omitempty"`
		Limit     int    `json:"limit,omitempty"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid request body",
		})
		return
	}

	if request.Limit == 0 {
		request.Limit = 20
	} else if request.Limit > 100 {
		request.Limit = 100
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "Episode search not yet implemented",
		"query":   request.Query,
	})
}