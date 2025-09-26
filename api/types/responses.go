package types

// Status constants for API responses
const (
	StatusOK         = "ok"
	StatusError      = "error"
	StatusProcessing = "processing"
	StatusFailed     = "failed"
	StatusQueued     = "queued"
)

// BaseResponse contains fields common to all API responses
type BaseResponse struct {
	Status  string `json:"status"`  // One of the Status constants above
	Message string `json:"message"` // Human-readable message
}

// PodcastSearchResponse for search endpoints
type PodcastSearchResponse struct {
	BaseResponse
	Podcasts []Podcast `json:"podcasts"`
	Query    string    `json:"query"`
	Count    int       `json:"count"`           // Number of results in this response
	Total    int       `json:"total,omitempty"` // Total available results (if known)
	Offset   int       `json:"offset,omitempty"`
}

// TrendingPodcastsResponse for trending endpoint
type TrendingPodcastsResponse struct {
	BaseResponse
	Podcasts []Podcast `json:"podcasts"`
	Since    int       `json:"since"` // Hours back for trending calculation
	Count    int       `json:"count"` // Number of results in this response
}

// SinglePodcastResponse for getting a single podcast
type SinglePodcastResponse struct {
	BaseResponse
	Podcast *Podcast `json:"podcast"`
}

// PodcastsResponse for generic podcast lists
type PodcastsResponse struct {
	BaseResponse
	Podcasts []Podcast `json:"podcasts"`
	Count    int       `json:"count"`           // Number of results in this response
	Total    int       `json:"total,omitempty"` // Total available (if known)
	Offset   int       `json:"offset,omitempty"`
}

// EpisodesResponse for episode lists
type EpisodesResponse struct {
	BaseResponse
	Episodes []Episode `json:"episodes"`
	Count    int       `json:"count"`           // Number of results in this response
	Total    int       `json:"total,omitempty"` // Total available (if known)
	Offset   int       `json:"offset,omitempty"`
}

// SingleEpisodeResponse for getting a single episode
type SingleEpisodeResponse struct {
	BaseResponse
	Episode *Episode `json:"episode"`
}

// WaveformResponse for waveform data
type WaveformResponse struct {
	BaseResponse
	Waveform *Waveform `json:"waveform"`
}

// TranscriptionResponse for transcription data
type TranscriptionResponse struct {
	BaseResponse
	Transcription *Transcription `json:"transcription"`
}

// ReviewsResponse for podcast reviews
type ReviewsResponse struct {
	BaseResponse
	Reviews  *ReviewData `json:"reviews,omitempty"`
	ITunesID int64       `json:"itunesId,omitempty"`
}

// CategoriesResponse for category lists
type CategoriesResponse struct {
	BaseResponse
	Categories []Category `json:"categories"`
	Count      int        `json:"count"`
}

// ErrorResponse for detailed error information
type ErrorResponse struct {
	Status  string      `json:"status"`
	Message string      `json:"message"`
	Error   string      `json:"error,omitempty"`   // Error code/type
	Details interface{} `json:"details,omitempty"` // Additional error details
}

// HealthResponse for health check endpoint
type HealthResponse struct {
	BaseResponse
	Version  string                 `json:"version,omitempty"`
	Services map[string]interface{} `json:"services,omitempty"`
}

// JobResponse for async job status
type JobResponse struct {
	BaseResponse
	JobID    string      `json:"jobId"`
	Progress float64     `json:"progress,omitempty"` // 0-100
	Result   interface{} `json:"result,omitempty"`
}
