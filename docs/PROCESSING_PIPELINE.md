# Audio Processing Pipeline

## Overview

The audio processing pipeline transforms raw podcast audio into enriched, analyzable content through three main stages: metadata extraction, waveform generation, and transcription. Each stage is designed to be independent, allowing for graceful degradation and parallel processing.

## Pipeline Architecture

```
Episode Selected
       │
       ▼
┌──────────────┐
│ Check Cache  │───── Cached? ──────▶ Return Cached Data
└──────┬───────┘
       │ Not Cached
       ▼
┌──────────────┐
│ Create Jobs  │
└──────┬───────┘
       │
       ▼
┌──────────────┐     ┌─────────────┐
│ Job Queue    │────▶│ Worker Pool │
└──────────────┘     └──────┬──────┘
                            │
                ┌───────────┼───────────┐
                ▼           ▼           ▼
        ┌──────────┐ ┌──────────┐ ┌──────────┐
        │ Metadata │ │ Waveform │ │Transcript│
        └──────────┘ └──────────┘ └──────────┘
                │           │           │
                └───────────┼───────────┘
                            ▼
                    ┌──────────────┐
                    │Store Results │
                    └──────────────┘
```

## Processing Stages

### Stage 1: Metadata Extraction

#### Purpose
Extract essential information about the audio file quickly to enable immediate playback and UI updates.

#### Tool: FFprobe
```bash
ffprobe -v quiet -print_format json -show_format -show_streams audio.mp3
```

#### Data Extracted
- **Format Information**: Duration, bit rate, file size
- **Audio Stream**: Codec, sample rate, channels, bit depth
- **ID3 Tags**: Title, artist, album, year, genre, comment
- **Technical Details**: Encoder, creation time

#### Implementation
```go
type MetadataExtractor struct {
    ffprobePath string
    timeout     time.Duration
}

func (m *MetadataExtractor) Extract(audioPath string) (*Metadata, error) {
    ctx, cancel := context.WithTimeout(context.Background(), m.timeout)
    defer cancel()
    
    cmd := exec.CommandContext(ctx, m.ffprobePath,
        "-v", "quiet",
        "-print_format", "json",
        "-show_format",
        "-show_streams",
        audioPath,
    )
    
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("ffprobe failed: %w", err)
    }
    
    var result FFprobeResult
    if err := json.Unmarshal(output, &result); err != nil {
        return nil, fmt.Errorf("parse ffprobe output: %w", err)
    }
    
    return m.parseMetadata(result), nil
}
```

#### Optimization Strategies
1. **Partial Download**: Extract metadata from first 1MB of file
2. **Caching**: Store metadata indefinitely (doesn't change)
3. **Fallback**: Use HTTP HEAD requests if FFprobe fails
4. **Timeout**: 10-second timeout for extraction

### Stage 2: Waveform Generation

#### Purpose
Generate visual representation of audio amplitude over time for UI visualization and navigation.

#### Tool: BBC audiowaveform
```bash
audiowaveform -i audio.mp3 -o waveform.json --pixels-per-second 20 -b 8
```

#### Parameters
- **Pixels per second**: Resolution of waveform (10, 20, 50)
- **Bits**: Amplitude resolution (8 or 16 bit)
- **Output format**: JSON or binary (.dat)

#### Multi-Resolution Strategy
Generate multiple resolutions for different zoom levels:
```bash
# Overview (full episode view)
audiowaveform -i audio.mp3 -o waveform_256.json --pixels-per-second 256

# Standard view
audiowaveform -i audio.mp3 -o waveform_512.json --pixels-per-second 512

# Detailed view (zoomed in)
audiowaveform -i audio.mp3 -o waveform_1024.json --pixels-per-second 1024
```

#### Implementation
```go
type WaveformGenerator struct {
    audiowaveformPath string
    tempDir          string
}

func (w *WaveformGenerator) Generate(audioPath string, resolution int) (*Waveform, error) {
    outputPath := filepath.Join(w.tempDir, fmt.Sprintf("waveform_%d.json", resolution))
    
    cmd := exec.Command(w.audiowaveformPath,
        "-i", audioPath,
        "-o", outputPath,
        "--pixels-per-second", strconv.Itoa(resolution),
        "-b", "8",
    )
    
    if err := cmd.Run(); err != nil {
        return nil, fmt.Errorf("audiowaveform failed: %w", err)
    }
    
    data, err := os.ReadFile(outputPath)
    if err != nil {
        return nil, err
    }
    
    defer os.Remove(outputPath) // Clean up temp file
    
    var waveform Waveform
    if err := json.Unmarshal(data, &waveform); err != nil {
        return nil, err
    }
    
    return &waveform, nil
}
```

#### Progressive Generation
1. Generate low-resolution waveform first (fast)
2. Send to client immediately
3. Generate higher resolutions in background
4. Update client as better data becomes available

### Stage 3: Transcription

#### Purpose
Convert speech to text with timestamps for search, accessibility, and analysis.

#### Tool: OpenAI Whisper API

#### API Integration
```go
type WhisperTranscriber struct {
    apiKey     string
    apiURL     string
    model      string
    maxSize    int64 // 25MB
}

func (w *WhisperTranscriber) Transcribe(audioPath string) (*Transcription, error) {
    // Check file size
    info, err := os.Stat(audioPath)
    if err != nil {
        return nil, err
    }
    
    if info.Size() > w.maxSize {
        return w.transcribeChunked(audioPath)
    }
    
    return w.transcribeSingle(audioPath)
}

func (w *WhisperTranscriber) transcribeSingle(audioPath string) (*Transcription, error) {
    file, err := os.Open(audioPath)
    if err != nil {
        return nil, err
    }
    defer file.Close()
    
    // Create multipart form
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    
    part, err := writer.CreateFormFile("file", filepath.Base(audioPath))
    if err != nil {
        return nil, err
    }
    
    io.Copy(part, file)
    writer.WriteField("model", w.model)
    writer.WriteField("response_format", "verbose_json")
    writer.WriteField("timestamp_granularities", "segment")
    writer.Close()
    
    // Send request
    req, _ := http.NewRequest("POST", w.apiURL, body)
    req.Header.Set("Authorization", "Bearer " + w.apiKey)
    req.Header.Set("Content-Type", writer.FormDataContentType())
    
    // Handle response...
}
```

#### Chunking Strategy
For files > 25MB:
1. **Split at silence**: Use FFmpeg to detect silence
2. **Overlap chunks**: 5-second overlap for context
3. **Process in parallel**: Send multiple chunks simultaneously
4. **Merge results**: Reconcile timestamps and text

```bash
# Detect silence periods
ffmpeg -i audio.mp3 -af silencedetect=n=-30dB:d=0.5 -f null -

# Split at silence
ffmpeg -i audio.mp3 -ss 0 -to 600 -c copy chunk1.mp3
```

#### Cost Optimization
1. **Voice Activity Detection**: Skip pure silence/music
2. **Language Hints**: Provide language if known
3. **Caching**: Never reprocess same episode
4. **Compression**: Convert to lower bitrate before sending

## Job Queue System

### Queue Manager
```go
type QueueManager struct {
    workers    int
    jobQueue   chan *Job
    resultChan chan *JobResult
    wg         sync.WaitGroup
}

type Job struct {
    ID         string
    EpisodeID  string
    Type       JobType
    Priority   int
    RetryCount int
    CreatedAt  time.Time
}

func (q *QueueManager) Start() {
    for i := 0; i < q.workers; i++ {
        q.wg.Add(1)
        go q.worker(i)
    }
}

func (q *QueueManager) worker(id int) {
    defer q.wg.Done()
    
    for job := range q.jobQueue {
        result := q.processJob(job)
        q.resultChan <- result
        
        // Update progress via WebSocket
        q.sendProgress(job, result)
    }
}
```

### Priority System
```go
const (
    PriorityHigh   = 10  // User-initiated, current episode
    PriorityNormal = 5   // Background processing
    PriorityLow    = 1   // Batch reprocessing
)

// Priority queue implementation
type PriorityQueue []*Job

func (pq PriorityQueue) Less(i, j int) bool {
    if pq[i].Priority != pq[j].Priority {
        return pq[i].Priority > pq[j].Priority
    }
    return pq[i].CreatedAt.Before(pq[j].CreatedAt)
}
```

### Error Handling & Retry

#### Retry Logic
```go
func (q *QueueManager) processWithRetry(job *Job) error {
    var lastErr error
    
    for attempt := 0; attempt <= job.MaxRetries; attempt++ {
        if attempt > 0 {
            // Exponential backoff
            wait := time.Duration(math.Pow(2, float64(attempt))) * time.Second
            time.Sleep(wait)
        }
        
        err := q.processJob(job)
        if err == nil {
            return nil
        }
        
        lastErr = err
        if !isRetryable(err) {
            break
        }
    }
    
    return fmt.Errorf("job failed after %d attempts: %w", job.RetryCount, lastErr)
}

func isRetryable(err error) bool {
    // Network errors, timeouts are retryable
    // File not found, invalid format are not
    return errors.Is(err, context.DeadlineExceeded) ||
           errors.Is(err, syscall.ECONNREFUSED)
}
```

## Progress Tracking

### Progress Calculation
```go
type ProgressTracker struct {
    jobs map[string]*JobProgress
    mu   sync.RWMutex
}

type JobProgress struct {
    EpisodeID string
    Jobs      []JobType
    Progress  map[JobType]int
    Weights   map[JobType]float64
}

func (p *ProgressTracker) CalculateOverallProgress(episodeID string) int {
    p.mu.RLock()
    defer p.mu.RUnlock()
    
    jp := p.jobs[episodeID]
    if jp == nil {
        return 0
    }
    
    var totalProgress float64
    var totalWeight float64
    
    for jobType, progress := range jp.Progress {
        weight := jp.Weights[jobType]
        totalProgress += float64(progress) * weight
        totalWeight += weight
    }
    
    if totalWeight == 0 {
        return 0
    }
    
    return int(totalProgress / totalWeight)
}
```

### WebSocket Updates
```go
func (p *ProgressTracker) SendUpdate(episodeID string) {
    progress := p.CalculateOverallProgress(episodeID)
    
    msg := WSMessage{
        Type: "processing_progress",
        Payload: map[string]interface{}{
            "episode_id": episodeID,
            "progress":   progress,
            "jobs":       p.jobs[episodeID].Progress,
        },
    }
    
    p.wsHandler.Broadcast(msg)
}
```

## Resource Management

### CPU Management
```go
// Limit concurrent FFmpeg processes
var ffmpegSemaphore = make(chan struct{}, 2)

func runFFmpeg(args ...string) error {
    ffmpegSemaphore <- struct{}{}
    defer func() { <-ffmpegSemaphore }()
    
    return exec.Command("ffmpeg", args...).Run()
}
```

### Memory Management
```go
// Monitor memory usage
func monitorMemory() {
    var m runtime.MemStats
    runtime.ReadMemStats(&m)
    
    if m.Alloc > maxMemory {
        // Pause processing
        // Trigger garbage collection
        runtime.GC()
    }
}
```

### Disk Management
```go
// Clean up temporary files
func cleanupTempFiles(age time.Duration) {
    files, _ := filepath.Glob(filepath.Join(tempDir, "*"))
    
    for _, file := range files {
        info, err := os.Stat(file)
        if err != nil {
            continue
        }
        
        if time.Since(info.ModTime()) > age {
            os.Remove(file)
        }
    }
}
```

## Performance Metrics

### Processing Times (Target)
- Metadata extraction: < 2 seconds
- Waveform generation: < 10 seconds per hour
- Transcription: < 2 minutes per hour

### Monitoring
```go
type ProcessingMetrics struct {
    JobsProcessed   prometheus.Counter
    JobDuration     prometheus.Histogram
    JobErrors       prometheus.Counter
    QueueSize       prometheus.Gauge
    ActiveWorkers   prometheus.Gauge
}

func (m *ProcessingMetrics) RecordJob(jobType string, duration time.Duration, err error) {
    m.JobsProcessed.WithLabelValues(jobType).Inc()
    m.JobDuration.WithLabelValues(jobType).Observe(duration.Seconds())
    
    if err != nil {
        m.JobErrors.WithLabelValues(jobType).Inc()
    }
}
```

## Best Practices

1. **Process Isolation**: Run external tools in separate processes
2. **Timeout Everything**: Set reasonable timeouts for all operations
3. **Progressive Enhancement**: Deliver value incrementally
4. **Idempotency**: Ensure reprocessing produces same results
5. **Resource Limits**: Cap CPU, memory, and disk usage
6. **Error Recovery**: Gracefully handle and retry failures
7. **Monitoring**: Track all metrics for optimization
8. **Cleanup**: Remove temporary files promptly