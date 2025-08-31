package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/internal/services/podcastindex"
)

// SearchHandler handles podcast search requests
type SearchHandler struct {
	podcastClient *podcastindex.Client
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(client *podcastindex.Client) *SearchHandler {
	return &SearchHandler{
		podcastClient: client,
	}
}

// ServeHTTP handles the search endpoint
func (h *SearchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req models.SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Query == "" {
		http.Error(w, "Query parameter is required", http.StatusBadRequest)
		return
	}

	// Set default limit if not provided
	if req.Limit <= 0 {
		req.Limit = 25
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Search via Podcast Index API
	searchResp, err := h.podcastClient.Search(ctx, req.Query, req.Limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Search failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to our response format
	response := models.SearchResponse{
		Podcasts: make([]models.PodcastSearchResult, 0, len(searchResp.Feeds)),
	}

	for _, feed := range searchResp.Feeds {
		response.Podcasts = append(response.Podcasts, models.PodcastSearchResult{
			ID:          strconv.Itoa(feed.ID),
			Title:       feed.Title,
			Author:      feed.Author,
			Description: feed.Description,
			Image:       feed.Image,
			URL:         feed.URL,
		})
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log error but response is already partially written
		// In production, you'd want proper logging here
		return
	}
}
