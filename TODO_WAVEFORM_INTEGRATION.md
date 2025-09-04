# Waveform Integration - Next Steps

## Context
We've implemented an enhanced episode endpoint that embeds waveform status directly in the episode response. The architecture is ready but needs the job processing system to be wired up.

## Current State
- ✅ Enhanced episode endpoint (`GET /api/v1/episodes/{id}`) returns waveform status
- ✅ Download-to-temp-file pipeline implemented (`pkg/download/`)
- ✅ Enhanced waveform processor with progress tracking (`internal/services/workers/waveform_processor_enhanced.go`)
- ❌ JobService not initialized (causing "Waveform service unavailable" error)
- ❌ WorkerPool not running to process background jobs

## Implementation Tasks

### 1. Initialize JobService in routes.go
**File**: `api/routes.go`
**Location**: Around line 105-108 where WaveformService is initialized
**Task**: Add JobService initialization similar to WaveformService
```go
// Initialize job service if not set
if deps.JobService == nil {
    jobRepo := jobs.NewRepository(deps.DB.DB)
    deps.JobService = jobs.NewService(jobRepo)
}
```

### 2. Initialize WorkerPool with Enhanced Processor
**File**: `cmd/serve.go` or `api/server.go`
**Task**: Create and start worker pool after database initialization
```go
// Create FFmpeg instance
ffmpegInstance := ffmpeg.New(
    config.GetString("processing.ffmpeg_path"),
    config.GetString("processing.ffprobe_path"),
    config.GetDuration("processing.ffmpeg_timeout"),
)

// Create enhanced waveform processor
processor := workers.NewEnhancedWaveformProcessor(
    deps.JobService,
    deps.WaveformService,
    deps.EpisodeService,
    ffmpegInstance,
    ffmpeg.DefaultProcessingOptions(),
)

// Create and start worker pool
workerPool := workers.NewWorkerPool(
    config.GetInt("processing.workers"),
    config.GetInt("processing.max_queue_size"),
)
workerPool.RegisterProcessor(processor)
workerPool.Start(context.Background())
deps.WorkerPool = workerPool
```

### 3. Test Complete Flow
**Test Command**: 
```bash
curl -X GET "http://localhost:8080/api/v1/episodes/41461438162" | jq '.waveform'
```

**Expected Flow**:
1. First request returns `status: "downloading"` and triggers job
2. Background worker downloads audio to temp file
3. FFmpeg processes waveform from temp file
4. Subsequent requests show `status: "processing"` with progress
5. Final request shows `status: "ok"` with waveform data

### 4. Add Temp File Cleanup
**File**: Create `internal/services/cleanup/cleanup.go`
**Task**: Periodic cleanup of old temp files
```go
// Run cleanup every hour
ticker := time.NewTicker(1 * time.Hour)
go func() {
    for range ticker.C {
        download.CleanupOldTempFiles("/tmp", 1*time.Hour)
    }
}()
```

## Response Structure
The enhanced endpoint returns:
```json
{
  "id": 41461438162,
  "title": "Episode Title",
  "enclosureUrl": "https://...",
  "waveform": {
    "status": "ok|processing|downloading|error",
    "message": "Human readable status",
    "progress": 75,  // Optional: 0-100
    "data": {        // Only when status="ok"
      "peaks": [...],
      "duration": 300.5,
      "resolution": 1000,
      "sample_rate": 44100
    }
  }
}
```

## Key Files
- `api/episodes/get_by_id_enhanced.go` - Enhanced episode handler with waveform status
- `api/episodes/types.go` - Waveform status types and constants
- `pkg/download/download.go` - Audio download to temp file with progress
- `internal/services/workers/waveform_processor_enhanced.go` - Enhanced processor using temp files
- `api/routes.go` - Where services need to be initialized

## Testing Notes
- Episode ID 41461438162 is a valid test episode with audio
- Server should be running with `task serve`
- Check logs for download/processing progress
- Temp files are created as `/tmp/episode_{id}_{timestamp}.mp3`