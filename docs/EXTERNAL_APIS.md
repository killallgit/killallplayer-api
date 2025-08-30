# External API Integration Guide

## Overview

The Podcast Player API integrates with two primary external services:
1. **Podcast Index API** - For podcast discovery and metadata
2. **OpenAI Whisper API** - For audio transcription

Both integrations follow similar patterns: authentication, rate limiting, error handling, and caching.

## Podcast Index API

### Authentication

The Podcast Index API uses a combination of API key, secret, and request signing.

#### Configuration
```yaml
podcast_index:
  api_key: "YOUR_API_KEY"
  api_secret: "YOUR_API_SECRET"
  base_url: "https://api.podcastindex.org/api/1.0"
  user_agent: "PodcastPlayerAPI/1.0"
```

#### Request Signing
```go
type PodcastIndexClient struct {
    apiKey    string
    apiSecret string
    baseURL   string
}

func (c *PodcastIndexClient) signRequest(req *http.Request) {
    // Generate auth headers
    authTime := strconv.FormatInt(time.Now().Unix(), 10)
    authHash := c.generateHash(c.apiKey + c.apiSecret + authTime)
    
    // Add headers
    req.Header.Set("X-Auth-Date", authTime)
    req.Header.Set("X-Auth-Key", c.apiKey)
    req.Header.Set("Authorization", authHash)
    req.Header.Set("User-Agent", c.userAgent)
}

func (c *PodcastIndexClient) generateHash(data string) string {
    h := sha1.New()
    h.Write([]byte(data))
    return hex.EncodeToString(h.Sum(nil))
}
```

### API Endpoints

#### Search Podcasts
```go
func (c *PodcastIndexClient) SearchPodcasts(query string, limit int) (*SearchResult, error) {
    endpoint := fmt.Sprintf("%s/search/byterm?q=%s&max=%d", 
        c.baseURL, url.QueryEscape(query), limit)
    
    req, err := http.NewRequest("GET", endpoint, nil)
    if err != nil {
        return nil, err
    }
    
    c.signRequest(req)
    
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("API error: %d", resp.StatusCode)
    }
    
    var result SearchResult
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    return &result, nil
}
```

#### Get Podcast Episodes
```go
func (c *PodcastIndexClient) GetEpisodes(podcastID string, limit int) (*EpisodesResult, error) {
    endpoint := fmt.Sprintf("%s/episodes/byfeedid?id=%s&max=%d",
        c.baseURL, podcastID, limit)
    
    // Similar request pattern...
}
```

#### Get Podcast by Feed URL
```go
func (c *PodcastIndexClient) GetByFeedURL(feedURL string) (*Podcast, error) {
    endpoint := fmt.Sprintf("%s/podcasts/byfeedurl?url=%s",
        c.baseURL, url.QueryEscape(feedURL))
    
    // Similar request pattern...
}
```

### Rate Limiting

Podcast Index API limits:
- 10,000 requests per month (free tier)
- No published rate limit per second

#### Implementation
```go
type RateLimiter struct {
    limiter *rate.Limiter
    mu      sync.Mutex
}

func NewRateLimiter(rps int) *RateLimiter {
    return &RateLimiter{
        limiter: rate.NewLimiter(rate.Limit(rps), 1),
    }
}

func (r *RateLimiter) Wait(ctx context.Context) error {
    return r.limiter.Wait(ctx)
}

// Use in client
func (c *PodcastIndexClient) doRequest(req *http.Request) (*http.Response, error) {
    // Wait for rate limit
    if err := c.rateLimiter.Wait(req.Context()); err != nil {
        return nil, err
    }
    
    return c.httpClient.Do(req)
}
```

### Circuit Breaker

Prevent cascading failures when API is down.

```go
type CircuitBreaker struct {
    maxFailures  int
    resetTimeout time.Duration
    failures     int
    lastFailTime time.Time
    state        State
    mu           sync.RWMutex
}

type State int

const (
    StateClosed State = iota
    StateOpen
    StateHalfOpen
)

func (cb *CircuitBreaker) Call(fn func() error) error {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    
    // Check if circuit is open
    if cb.state == StateOpen {
        if time.Since(cb.lastFailTime) > cb.resetTimeout {
            cb.state = StateHalfOpen
            cb.failures = 0
        } else {
            return ErrCircuitOpen
        }
    }
    
    // Execute function
    err := fn()
    
    if err != nil {
        cb.failures++
        cb.lastFailTime = time.Now()
        
        if cb.failures >= cb.maxFailures {
            cb.state = StateOpen
        }
        return err
    }
    
    // Success - reset
    cb.failures = 0
    cb.state = StateClosed
    return nil
}
```

### Caching Strategy

```go
type PodcastCache struct {
    cache *cache.Cache
    ttl   time.Duration
}

func (pc *PodcastCache) GetOrFetch(key string, fetcher func() (interface{}, error)) (interface{}, error) {
    // Check cache
    if cached, found := pc.cache.Get(key); found {
        return cached, nil
    }
    
    // Fetch from API
    data, err := fetcher()
    if err != nil {
        // On error, try to return stale cache if available
        if stale, found := pc.cache.Get(key + ":stale"); found {
            return stale, nil
        }
        return nil, err
    }
    
    // Update cache
    pc.cache.Set(key, data, pc.ttl)
    pc.cache.Set(key+":stale", data, pc.ttl*2) // Keep stale copy longer
    
    return data, nil
}
```

## OpenAI Whisper API

### Authentication

Simple Bearer token authentication.

#### Configuration
```yaml
whisper:
  api_key: "${OPENAI_API_KEY}"
  api_url: "https://api.openai.com/v1/audio/transcriptions"
  model: "whisper-1"
  temperature: 0
```

#### Client Setup
```go
type WhisperClient struct {
    apiKey      string
    apiURL      string
    model       string
    httpClient  *http.Client
}

func (w *WhisperClient) setAuthHeader(req *http.Request) {
    req.Header.Set("Authorization", "Bearer " + w.apiKey)
}
```

### API Integration

#### Basic Transcription
```go
func (w *WhisperClient) Transcribe(audioFile io.Reader, filename string) (*TranscriptionResult, error) {
    // Create multipart form
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    
    // Add file
    part, err := writer.CreateFormFile("file", filename)
    if err != nil {
        return nil, err
    }
    io.Copy(part, audioFile)
    
    // Add parameters
    writer.WriteField("model", w.model)
    writer.WriteField("response_format", "verbose_json")
    writer.WriteField("timestamp_granularities[]", "segment")
    writer.WriteField("timestamp_granularities[]", "word")
    
    writer.Close()
    
    // Create request
    req, err := http.NewRequest("POST", w.apiURL, body)
    if err != nil {
        return nil, err
    }
    
    w.setAuthHeader(req)
    req.Header.Set("Content-Type", writer.FormDataContentType())
    
    // Send request
    resp, err := w.httpClient.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    // Parse response
    var result TranscriptionResult
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    return &result, nil
}
```

#### Chunked Transcription
For files > 25MB:

```go
func (w *WhisperClient) TranscribeChunked(audioPath string) (*TranscriptionResult, error) {
    chunks, err := w.splitAudio(audioPath)
    if err != nil {
        return nil, err
    }
    
    var wg sync.WaitGroup
    results := make([]*TranscriptionResult, len(chunks))
    errors := make([]error, len(chunks))
    
    for i, chunk := range chunks {
        wg.Add(1)
        go func(index int, chunkPath string) {
            defer wg.Done()
            
            file, err := os.Open(chunkPath)
            if err != nil {
                errors[index] = err
                return
            }
            defer file.Close()
            
            result, err := w.Transcribe(file, filepath.Base(chunkPath))
            if err != nil {
                errors[index] = err
                return
            }
            
            results[index] = result
        }(i, chunk)
    }
    
    wg.Wait()
    
    // Check for errors
    for _, err := range errors {
        if err != nil {
            return nil, err
        }
    }
    
    // Merge results
    return w.mergeTranscriptions(results), nil
}

func (w *WhisperClient) splitAudio(audioPath string) ([]string, error) {
    // Use FFmpeg to split at silence
    cmd := exec.Command("ffmpeg",
        "-i", audioPath,
        "-f", "segment",
        "-segment_time", "600", // 10 minutes
        "-c", "copy",
        "chunk_%03d.mp3",
    )
    
    if err := cmd.Run(); err != nil {
        return nil, err
    }
    
    // Return chunk paths
    chunks, _ := filepath.Glob("chunk_*.mp3")
    return chunks, nil
}
```

### Cost Management

```go
type CostTracker struct {
    mu          sync.RWMutex
    dailyCosts  map[string]float64
    monthlyLimit float64
}

func (ct *CostTracker) TrackUsage(duration time.Duration, cost float64) error {
    ct.mu.Lock()
    defer ct.mu.Unlock()
    
    today := time.Now().Format("2006-01-02")
    ct.dailyCosts[today] += cost
    
    // Check monthly total
    var monthlyTotal float64
    for date, dailyCost := range ct.dailyCosts {
        if strings.HasPrefix(date, time.Now().Format("2006-01")) {
            monthlyTotal += dailyCost
        }
    }
    
    if monthlyTotal > ct.monthlyLimit {
        return ErrQuotaExceeded
    }
    
    return nil
}

func (ct *CostTracker) CalculateCost(duration time.Duration) float64 {
    // Whisper API: $0.006 per minute
    minutes := duration.Minutes()
    return minutes * 0.006
}
```

### Error Handling

```go
type WhisperError struct {
    Type    string `json:"type"`
    Message string `json:"message"`
    Code    string `json:"code"`
}

func (w *WhisperClient) handleError(resp *http.Response) error {
    var apiError struct {
        Error WhisperError `json:"error"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&apiError); err != nil {
        return fmt.Errorf("API error: status %d", resp.StatusCode)
    }
    
    switch apiError.Error.Code {
    case "insufficient_quota":
        return ErrQuotaExceeded
    case "model_not_found":
        return ErrInvalidModel
    case "invalid_api_key":
        return ErrInvalidAPIKey
    case "rate_limit_exceeded":
        return ErrRateLimited
    default:
        return fmt.Errorf("API error: %s", apiError.Error.Message)
    }
}
```

## Fallback Strategies

### Podcast Index Fallback
```go
func (s *PodcastService) SearchWithFallback(query string) (*SearchResult, error) {
    // Try Podcast Index API
    result, err := s.podcastIndex.Search(query)
    if err == nil {
        return result, nil
    }
    
    // Fallback to cached results
    if cached := s.cache.GetStale(query); cached != nil {
        return cached.(*SearchResult), nil
    }
    
    // Fallback to local database search
    return s.db.SearchPodcasts(query)
}
```

### Whisper API Fallback
```go
func (s *TranscriptionService) TranscribeWithFallback(audioPath string) (*Transcription, error) {
    // Try Whisper API
    result, err := s.whisper.Transcribe(audioPath)
    if err == nil {
        return result, nil
    }
    
    // Check if quota exceeded
    if errors.Is(err, ErrQuotaExceeded) {
        // Queue for later processing
        s.queue.Add(audioPath, PriorityLow)
        return nil, ErrTranscriptionQueued
    }
    
    // For other errors, could fall back to:
    // 1. Local Whisper (if available)
    // 2. Alternative transcription service
    // 3. Return partial result without transcription
    
    return nil, err
}
```

## Monitoring & Alerting

```go
type APIMetrics struct {
    RequestCount    *prometheus.CounterVec
    RequestDuration *prometheus.HistogramVec
    ErrorCount      *prometheus.CounterVec
    CacheHitRate    *prometheus.GaugeVec
}

func (m *APIMetrics) RecordRequest(api, endpoint string, duration time.Duration, err error) {
    m.RequestCount.WithLabelValues(api, endpoint).Inc()
    m.RequestDuration.WithLabelValues(api, endpoint).Observe(duration.Seconds())
    
    if err != nil {
        m.ErrorCount.WithLabelValues(api, endpoint, errorType(err)).Inc()
    }
}

func errorType(err error) string {
    switch {
    case errors.Is(err, ErrRateLimited):
        return "rate_limited"
    case errors.Is(err, ErrQuotaExceeded):
        return "quota_exceeded"
    case errors.Is(err, context.DeadlineExceeded):
        return "timeout"
    default:
        return "unknown"
    }
}
```

## Best Practices

1. **Always Cache**: Cache everything with appropriate TTLs
2. **Graceful Degradation**: Always have a fallback plan
3. **Monitor Everything**: Track API usage, errors, and costs
4. **Respect Rate Limits**: Self-impose stricter limits than API
5. **Handle Errors Gracefully**: Distinguish between retryable and permanent errors
6. **Use Circuit Breakers**: Prevent cascading failures
7. **Batch When Possible**: Reduce API calls through batching
8. **Validate Responses**: Always validate API responses before use
9. **Secure Credentials**: Never commit API keys to source control
10. **Document Costs**: Track and alert on API costs