package itunes

import (
	"fmt"
	"strings"
)

// transformToPodcast converts iTunes API result to our Podcast model
func transformToPodcast(result *iTunesResult) *Podcast {
	if result == nil {
		return nil
	}

	// Determine if content is explicit
	isExplicit := result.CollectionExplicitness == "explicit" ||
		result.TrackExplicitness == "explicit" ||
		result.ContentAdvisoryRating == "Explicit"

	// Use the best available artwork URL
	artworkURL := result.ArtworkURL600
	if artworkURL == "" {
		artworkURL = result.ArtworkURL100
	}
	if artworkURL == "" {
		artworkURL = result.ArtworkURL60
	}

	return &Podcast{
		ID:           result.CollectionID,
		Title:        result.CollectionName,
		Author:       result.ArtistName,
		Description:  result.TrackName, // iTunes often puts description in TrackName for podcasts
		FeedURL:      result.FeedURL,
		ArtworkURL:   artworkURL,
		EpisodeCount: result.TrackCount,
		ReleaseDate:  result.ReleaseDate,
		Genre:        result.PrimaryGenreName,
		Country:      result.Country,
		Explicit:     isExplicit,
		ITunesURL:    result.CollectionViewURL,
	}
}

// transformToEpisode converts iTunes API result to our Episode model
func transformToEpisode(result *iTunesResult, podcastID int64) *Episode {
	if result == nil || result.Kind != "podcast-episode" {
		return nil
	}

	// Use episode URL or preview URL
	audioURL := result.EpisodeURL
	if audioURL == "" {
		audioURL = result.PreviewURL
	}

	// Use best available artwork
	artworkURL := result.ArtworkURL600
	if artworkURL == "" {
		artworkURL = result.ArtworkURL100
	}

	return &Episode{
		ID:            result.TrackID,
		PodcastID:     podcastID,
		Title:         result.TrackName,
		Description:   result.Description,
		AudioURL:      audioURL,
		Duration:      result.TrackTimeMillis,
		ReleaseDate:   result.ReleaseDate,
		GUID:          result.EpisodeGUID,
		FileExtension: result.EpisodeFileExtension,
		ContentType:   result.EpisodeContentType,
		ArtworkURL:    artworkURL,
	}
}

// transformToPodcastWithEpisodes converts iTunes API response to PodcastWithEpisodes
func transformToPodcastWithEpisodes(resp *iTunesResponse) *PodcastWithEpisodes {
	if resp == nil || resp.ResultCount == 0 || len(resp.Results) == 0 {
		return nil
	}

	result := &PodcastWithEpisodes{
		Episodes: make([]*Episode, 0),
	}

	// First result is typically the podcast metadata
	if resp.Results[0].Kind == "podcast" {
		result.Podcast = transformToPodcast(&resp.Results[0])

		// Rest are episodes
		for i := 1; i < len(resp.Results); i++ {
			if episode := transformToEpisode(&resp.Results[i], result.Podcast.ID); episode != nil {
				result.Episodes = append(result.Episodes, episode)
			}
		}
	} else {
		// Sometimes all results are episodes, extract podcast info from first episode
		if len(resp.Results) > 0 {
			first := &resp.Results[0]
			result.Podcast = &Podcast{
				ID:         first.CollectionID,
				Title:      first.CollectionName,
				Author:     first.ArtistName,
				FeedURL:    first.FeedURL,
				ArtworkURL: first.ArtworkURL600,
				Country:    first.Country,
				ITunesURL:  first.CollectionViewURL,
			}

			// All results are episodes
			for i := 0; i < len(resp.Results); i++ {
				if episode := transformToEpisode(&resp.Results[i], result.Podcast.ID); episode != nil {
					result.Episodes = append(result.Episodes, episode)
				}
			}
		}
	}

	return result
}

// transformToSearchResults converts iTunes API response to SearchResults
func transformToSearchResults(query string, resp *iTunesResponse) *SearchResults {
	if resp == nil {
		return &SearchResults{
			Query:      query,
			TotalCount: 0,
			Podcasts:   []*Podcast{},
		}
	}

	results := &SearchResults{
		Query:      query,
		TotalCount: resp.ResultCount,
		Podcasts:   make([]*Podcast, 0, resp.ResultCount),
	}

	for i := range resp.Results {
		// Only include actual podcasts (not episodes) in search results
		if resp.Results[i].Kind == "podcast" || resp.Results[i].WrapperType == "track" {
			podcast := transformToPodcast(&resp.Results[i])
			if podcast != nil && podcast.ID > 0 {
				results.Podcasts = append(results.Podcasts, podcast)
			}
		}
	}

	return results
}

// ExtractPodcastIDFromURL extracts the iTunes podcast ID from an iTunes URL
// Example: https://podcasts.apple.com/us/podcast/the-daily/id1200361736 -> 1200361736
func ExtractPodcastIDFromURL(itunesURL string) (int64, bool) {
	if itunesURL == "" {
		return 0, false
	}

	// Look for pattern "id" followed by numbers
	parts := strings.Split(itunesURL, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "id") {
			// Remove "id" prefix and any query parameters
			idStr := strings.TrimPrefix(part, "id")
			if idx := strings.Index(idStr, "?"); idx > 0 {
				idStr = idStr[:idx]
			}

			// Parse the ID
			var id int64
			if _, err := fmt.Sscanf(idStr, "%d", &id); err == nil && id > 0 {
				return id, true
			}
		}
	}

	return 0, false
}

// NormalizeFeedURL normalizes a feed URL for comparison
func NormalizeFeedURL(feedURL string) string {
	if feedURL == "" {
		return ""
	}

	// Remove protocol
	feedURL = strings.TrimPrefix(feedURL, "https://")
	feedURL = strings.TrimPrefix(feedURL, "http://")

	// Remove trailing slash
	feedURL = strings.TrimSuffix(feedURL, "/")

	// Convert to lowercase for comparison
	return strings.ToLower(feedURL)
}