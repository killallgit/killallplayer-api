package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/killallgit/player-api/internal/models"
)

// SearchHandler handles podcast search requests
type SearchHandler struct {
	podcastClient PodcastSearcher
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(client PodcastSearcher) *SearchHandler {
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
		sendErrorResponse(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Query == "" {
		sendErrorResponse(w, "Query parameter is required", http.StatusBadRequest)
		return
	}

	// Validate and set limit
	if req.Limit <= 0 {
		req.Limit = 10 // Default limit
	} else if req.Limit > 100 {
		sendErrorResponse(w, "Limit must be between 1 and 100", http.StatusBadRequest)
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Search via Podcast Index API
	searchResp, err := h.podcastClient.Search(ctx, req.Query, req.Limit)
	if err != nil {
		// Log the actual error for debugging while returning generic message to client
		log.Printf("Search error for query '%s': %v", req.Query, err)
		sendErrorResponse(w, "Failed to search podcasts", http.StatusInternalServerError)
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

// sendErrorResponse sends a JSON error response
func sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "error",
		"message": message,
	})
}
