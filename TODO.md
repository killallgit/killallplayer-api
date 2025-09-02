# Waveform Generation & WebSocket Implementation TODO

## PHASE 1: Basic Waveform Endpoint (No WebSocket, No Processing)
**Goal:** Create foundation with static test data to verify client integration

- [x] **Step 1.1:** Create `/api/v1/episodes/{id}/waveform` GET endpoint ✅
  - Return static JSON with test waveform data
  - Test: `curl http://localhost:8080/api/v1/episodes/123/waveform`
  - Response: `{"peaks": [0.1, 0.5, 0.8...], "duration": 300, "resolution": 100}`
  - **Completed**: 2025-09-02 - Endpoint working with 1000 synthetic peaks

- [ ] **Step 1.2:** Add waveform database model
  - Create `waveforms` table (episode_id, peaks_data BLOB, duration, resolution, created_at)
  - Add migration
  - Test: Verify table creation with SQLite browser

- [ ] **Step 1.3:** Create WaveformService interface
  - Define GetWaveform(episodeID) method
  - Implement with hardcoded data initially
  - Test: Unit test the service returns expected format

## PHASE 2: Audio Processing with FFmpeg
**Goal:** Generate real waveforms from audio files

- [ ] **Step 2.1:** Add FFmpeg wrapper service
  - Install FFmpeg on dev machine
  - Create FFmpegService with GetAudioInfo() method
  - Test: Extract duration from a test MP3 file

- [ ] **Step 2.2:** Implement waveform extraction
  - Use FFmpeg to generate peaks: `ffmpeg -i audio.mp3 -filter_complex "showwavespic=s=1000x1:colors=white" -frames:v 1 output.png`
  - Parse output to numeric peaks array
  - Test: Generate waveform for a known audio file, verify peaks array

- [ ] **Step 2.3:** Integrate with WaveformService
  - Fetch audio URL from episode
  - Download audio to temp file
  - Process with FFmpeg
  - Return real waveform data
  - Test: Call endpoint with real episode ID, verify waveform generated

- [ ] **Step 2.4:** Add caching to database
  - Check cache before processing
  - Store generated waveforms
  - Add TTL/expiry logic
  - Test: Second request should be faster (from cache)

## PHASE 3: Background Processing Queue
**Goal:** Non-blocking waveform generation

- [ ] **Step 3.1:** Create job queue table
  - Add `waveform_jobs` table (episode_id, status, started_at, completed_at, error)
  - Test: Verify table structure

- [ ] **Step 3.2:** Implement background worker
  - Simple goroutine worker pool
  - Process jobs from queue
  - Update job status
  - Test: Submit job, verify it processes asynchronously

- [ ] **Step 3.3:** Add status endpoint
  - `/api/v1/episodes/{id}/waveform/status`
  - Return: `{"status": "processing", "progress": 45}`
  - Test: Check status while processing

## PHASE 4: WebSocket Infrastructure
**Goal:** Real-time updates for waveform generation

- [ ] **Step 4.1:** Add WebSocket dependencies
  - Add `github.com/gorilla/websocket` to go.mod
  - Test: Verify package installation

- [ ] **Step 4.2:** Create basic WebSocket endpoint
  - `/api/v1/ws/ping` for testing
  - Simple echo server
  - Test: Use wscat to verify connection: `wscat -c ws://localhost:8080/api/v1/ws/ping`

- [ ] **Step 4.3:** Implement stream WebSocket endpoint
  - `/api/v1/ws/stream/{episodeId}`
  - Send test messages every second
  - Test: Connect and receive periodic messages

- [ ] **Step 4.4:** Create message protocol
  - Match client's StreamMessage types
  - Implement JSON serialization
  - Test: Send and parse different message types

## PHASE 5: WebSocket + Waveform Integration
**Goal:** Real-time waveform generation updates

- [ ] **Step 5.1:** Connect waveform processing to WebSocket
  - Send "processing_started" message
  - Send progress updates (0-100%)
  - Send "waveform_complete" with data
  - Test: Monitor WebSocket while requesting waveform

- [ ] **Step 5.2:** Add connection management
  - Track active connections per episode
  - Broadcast to all connected clients
  - Clean up on disconnect
  - Test: Multiple clients receive same updates

- [ ] **Step 5.3:** Error handling
  - Send error messages on processing failure
  - Implement retry logic
  - Graceful fallback to HTTP polling
  - Test: Simulate FFmpeg failure, verify error message

## PHASE 6: Client Integration
**Goal:** Connect React Native client to new endpoints

- [ ] **Step 6.1:** Test waveform endpoint from client
  - Fetch waveform data via HTTP
  - Display in console
  - Test: Verify data received in React Native

- [ ] **Step 6.2:** Connect WebSocket in StreamContext
  - Establish connection on episode load
  - Handle incoming messages
  - Test: See WebSocket messages in console

- [ ] **Step 6.3:** Visualize waveform
  - Create WaveformView component
  - Render peaks as bars/lines
  - Test: See visual waveform in UI

- [ ] **Step 6.4:** Show processing progress
  - Display loading indicator
  - Update progress bar
  - Show completion
  - Test: User sees real-time progress

## PHASE 7: Optimization & Polish
**Goal:** Production-ready implementation

- [ ] **Step 7.1:** Add compression
  - Compress waveform data (gzip)
  - Compress WebSocket frames
  - Test: Measure bandwidth reduction

- [ ] **Step 7.2:** Implement rate limiting
  - Limit waveform requests per IP
  - Limit WebSocket connections
  - Test: Verify limits enforced

- [ ] **Step 7.3:** Add monitoring
  - Log processing times
  - Track success/failure rates
  - WebSocket connection metrics
  - Test: Review logs for insights

- [ ] **Step 7.4:** Performance optimization
  - Parallel processing for long audio
  - Adaptive resolution based on duration
  - Memory usage optimization
  - Test: Process 2-hour podcast efficiently

## Testing Checklist

### Unit Tests
- [ ] WaveformService methods
- [ ] FFmpeg wrapper functions
- [ ] WebSocket message serialization
- [ ] Database caching logic

### Integration Tests
- [ ] Full waveform generation flow
- [ ] WebSocket connection lifecycle
- [ ] Background job processing
- [ ] Cache hit/miss scenarios

### End-to-End Tests
- [ ] Client requests waveform → receives via WebSocket
- [ ] Multiple clients receive same updates
- [ ] Graceful degradation when WebSocket unavailable
- [ ] Large file processing (>100MB audio)

## Acceptance Criteria
- ✅ Waveform generation works for MP3, M4A, AAC formats
- ✅ Processing doesn't block API responses
- ✅ Client receives real-time progress updates
- ✅ Cached waveforms load instantly
- ✅ System handles 10 concurrent waveform requests
- ✅ WebSocket reconnects automatically on disconnect
- ✅ Fallback to HTTP polling when WebSocket fails

## Dependencies to Install
```bash
# Server
go get github.com/gorilla/websocket

# System (macOS)
brew install ffmpeg

# System (Linux)
apt-get install ffmpeg

# Testing tools
npm install -g wscat  # WebSocket testing
```

## Configuration Needed
```yaml
# config.yaml
waveform:
  cache_ttl: 86400  # 24 hours
  max_file_size: 500000000  # 500MB
  default_resolution: 1000  # peaks per waveform
  worker_pool_size: 3
  
websocket:
  max_connections_per_ip: 10
  ping_interval: 30s
  write_timeout: 10s
```

## Technical Notes

### Waveform Generation Approaches
1. **FFmpeg audiowaveform filter**: Most accurate, handles all formats
2. **Raw PCM analysis**: Faster but requires decoding first
3. **Pre-generated thumbnails**: Best for static content

### WebSocket vs Server-Sent Events (SSE)
- **WebSocket**: Bidirectional, more complex, better for interactive features
- **SSE**: Unidirectional (server→client), simpler, good enough for progress updates
- **Decision**: WebSocket for future extensibility (playback sync, live transcription)

### Caching Strategy
1. **Database**: Persistent, survives restarts
2. **Redis**: Faster access, optional enhancement
3. **CDN**: For popular episodes, future optimization

### Error Recovery
1. **WebSocket disconnection**: Automatic reconnect with exponential backoff
2. **Processing failure**: Retry up to 3 times with increasing delays
3. **Timeout**: Cancel processing after 5 minutes for any single file

## Progress Tracking

### Current Phase: 1
### Current Step: 1.2
### Status: In Progress
### Completed Steps: 
- ✅ Phase 1.1: Basic waveform endpoint with static data

---

*Last Updated: 2025-09-02*