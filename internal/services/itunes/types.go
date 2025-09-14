package itunes

import (
	"time"
)

// iTunesResponse represents the top-level response from iTunes API
type iTunesResponse struct {
	ResultCount int            `json:"resultCount"`
	Results     []iTunesResult `json:"results"`
}

// iTunesResult represents a single result from iTunes API (can be podcast or episode)
type iTunesResult struct {
	// Common fields
	WrapperType string `json:"wrapperType"`
	Kind        string `json:"kind"`

	// Podcast fields
	CollectionID           int64     `json:"collectionId"`
	TrackID                int64     `json:"trackId"`
	ArtistName             string    `json:"artistName"`
	CollectionName         string    `json:"collectionName"`
	TrackName              string    `json:"trackName"`
	CollectionCensoredName string    `json:"collectionCensoredName"`
	TrackCensoredName      string    `json:"trackCensoredName"`
	CollectionViewURL      string    `json:"collectionViewUrl"`
	FeedURL                string    `json:"feedUrl"`
	TrackViewURL           string    `json:"trackViewUrl"`
	ArtworkURL30           string    `json:"artworkUrl30"`
	ArtworkURL60           string    `json:"artworkUrl60"`
	ArtworkURL100          string    `json:"artworkUrl100"`
	ArtworkURL600          string    `json:"artworkUrl600"`
	CollectionPrice        float64   `json:"collectionPrice"`
	TrackPrice             float64   `json:"trackPrice"`
	ReleaseDate            time.Time `json:"releaseDate"`
	CollectionExplicitness string    `json:"collectionExplicitness"`
	TrackExplicitness      string    `json:"trackExplicitness"`
	TrackCount             int       `json:"trackCount"`
	TrackTimeMillis        int       `json:"trackTimeMillis"`
	Country                string    `json:"country"`
	Currency               string    `json:"currency"`
	PrimaryGenreName       string    `json:"primaryGenreName"`
	ContentAdvisoryRating  string    `json:"contentAdvisoryRating"`
	GenreIds               []string  `json:"genreIds"`
	Genres                 []string  `json:"genres"`

	// Episode-specific fields
	EpisodeURL       string                 `json:"episodeUrl,omitempty"`
	PreviewURL       string                 `json:"previewUrl,omitempty"`
	EpisodeGUID      string                 `json:"episodeGuid,omitempty"`
	Description      string                 `json:"description,omitempty"`
	ShortDescription string                 `json:"shortDescription,omitempty"`
	ClosedCaptioning string                 `json:"closedCaptioning,omitempty"`
	EpisodeFileExtension string             `json:"episodeFileExtension,omitempty"`
	EpisodeContentType   string             `json:"episodeContentType,omitempty"`
	ArtistIds        []int64                `json:"artistIds,omitempty"`
	GenresMap        []map[string]interface{} `json:"genres,omitempty"` // Can be array of objects for episodes
}

// Podcast represents a simplified podcast structure
type Podcast struct {
	ID            int64     `json:"id"`
	Title         string    `json:"title"`
	Author        string    `json:"author"`
	Description   string    `json:"description"`
	FeedURL       string    `json:"feedUrl"`
	ArtworkURL    string    `json:"artworkUrl"`
	EpisodeCount  int       `json:"episodeCount"`
	ReleaseDate   time.Time `json:"releaseDate"`
	Genre         string    `json:"genre"`
	Country       string    `json:"country"`
	Language      string    `json:"language,omitempty"`
	Explicit      bool      `json:"explicit"`
	ITunesURL     string    `json:"itunesUrl"`
}

// Episode represents a simplified episode structure
type Episode struct {
	ID               int64     `json:"id"`
	PodcastID        int64     `json:"podcastId"`
	Title            string    `json:"title"`
	Description      string    `json:"description"`
	AudioURL         string    `json:"audioUrl"`
	Duration         int       `json:"duration"` // milliseconds
	ReleaseDate      time.Time `json:"releaseDate"`
	GUID             string    `json:"guid"`
	FileExtension    string    `json:"fileExtension"`
	ContentType      string    `json:"contentType"`
	ArtworkURL       string    `json:"artworkUrl,omitempty"`
}

// PodcastWithEpisodes represents a podcast with its episodes
type PodcastWithEpisodes struct {
	Podcast  *Podcast   `json:"podcast"`
	Episodes []*Episode `json:"episodes"`
}

// SearchResults represents search results from iTunes
type SearchResults struct {
	Query       string     `json:"query"`
	TotalCount  int        `json:"totalCount"`
	Podcasts    []*Podcast `json:"podcasts"`
}

// SearchOptions represents options for searching
type SearchOptions struct {
	Media    string `json:"media,omitempty"`    // podcast, music, etc.
	Entity   string `json:"entity,omitempty"`   // podcast, podcastEpisode
	Country  string `json:"country,omitempty"`  // US, GB, etc.
	Limit    int    `json:"limit,omitempty"`    // Max results
	Language string `json:"language,omitempty"` // en_us, etc.
	Explicit string `json:"explicit,omitempty"` // Yes, No
}