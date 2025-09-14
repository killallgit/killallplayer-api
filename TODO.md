- make sure waveform GET sends response of "queued" or others for the client
- We're logging failure over and over after we've finished the jobs. We need to completely remove / cleanup the queue and maybe disable the logs that it's checking because it's noisy
- Test fixtures

## PHASE 4: Testing Implementation (🚧 IN PROGRESS)
**Goal:** Comprehensive testing of waveform generation system

- [x] **Step 4.1:** Create test audio clips ✅
  - Extract small clips from 2-hour audio file for testing
  - Created `./data/tests/clips/test-5s.mp3` (10KB, 5 seconds)
  - Created `./data/tests/clips/test-30s.mp3` (60KB, 30 seconds)
  - **Completed**: 2025-09-04 - Test clips ready for unit/integration tests

- [ ] **Step 4.2:** Basic FFmpeg unit tests
  - Test metadata extraction with real audio files
  - Test waveform generation end-to-end with small clips
  - Verify peak values and data format
  - Test: `go test ./pkg/ffmpeg/ -v`

- [ ] **Step 4.3:** Worker system integration tests
  - Test job creation and processing with real audio
  - Test waveform storage and retrieval
  - Test API endpoints with background processing
  - Test: Complete workflow from HTTP request to database storage

- [ ] **Step 4.4:** End-to-end system tests
  - Start server with worker pool
  - Make HTTP requests to waveform endpoints
  - Verify job processing and waveform generation
  - Test: Manual verification with `curl` commands

**✅ PHASE 4 PROGRESS: Step 1 Complete**
- Real audio test files extracted and ready
- Foundation established for comprehensive testing

## PHASE 5: WebSocket Infrastructure (FUTURE)
**Goal:** Real-time updates for waveform generation

- [ ] **Step 5.1:** Add WebSocket dependencies
  - Add `github.com/gorilla/websocket` to go.mod
  - Test: Verify package installation

- [ ] **Step 5.2:** Create basic WebSocket endpoint
  - `/api/v1/ws/ping` for testing
  - Simple echo server
  - Test: Use wscat to verify connection: `wscat -c ws://localhost:8080/api/v1/ws/ping`

- [ ] **Step 5.3:** Implement stream WebSocket endpoint
  - `/api/v1/ws/stream/{episodeId}`
  - Send test messages every second
  - Test: Connect and receive periodic messages

- [ ] **Step 5.4:** Create message protocol
  - Match client's StreamMessage types
  - Implement JSON serialization
  - Test: Send and parse different message types

## PHASE 6: WebSocket + Waveform Integration (FUTURE)
**Goal:** Real-time waveform generation updates

- [ ] **Step 6.1:** Connect waveform processing to WebSocket
  - Send "processing_started" message
  - Send progress updates (0-100%)
  - Send "waveform_complete" with data
  - Test: Monitor WebSocket while requesting waveform

- [ ] **Step 6.2:** Add connection management
  - Track active connections per episode
  - Broadcast to all connected clients
  - Clean up on disconnect
  - Test: Multiple clients receive same updates

- [ ] **Step 6.3:** Error handling
  - Send error messages on processing failure
  - Implement retry logic
  - Graceful fallback to HTTP polling
  - Test: Simulate FFmpeg failure, verify error message

## PHASE 7: Client Integration (FUTURE)
**Goal:** Connect React Native client to new endpoints

- [ ] **Step 7.1:** Test waveform endpoint from client
  - Fetch waveform data via HTTP
  - Display in console
  - Test: Verify data received in React Native

- [ ] **Step 7.2:** Connect WebSocket in StreamContext
  - Establish connection on episode load
  - Handle incoming messages
  - Test: See WebSocket messages in console

- [ ] **Step 7.3:** Visualize waveform
  - Create WaveformView component
  - Render peaks as bars/lines
  - Test: See visual waveform in UI

- [ ] **Step 7.4:** Show processing progress
  - Display loading indicator
  - Update progress bar
  - Show completion
  - Test: User sees real-time progress

## PHASE 8: Optimization & Polish (FUTURE)
**Goal:** Production-ready implementation

- [ ] **Step 8.1:** Add compression
  - Compress waveform data (gzip)
  - Compress WebSocket frames
  - Test: Measure bandwidth reduction

- [ ] **Step 8.2:** Implement rate limiting
  - Limit waveform requests per IP
  - Limit WebSocket connections
  - Test: Verify limits enforced

- [ ] **Step 8.3:** Add monitoring
  - Log processing times
  - Track success/failure rates
  - WebSocket connection metrics
  - Test: Review logs for insights

- [ ] **Step 8.4:** Performance optimization
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

### Current Phase: 4 (Testing Implementation - IN PROGRESS)
### Current Step: 4.2 (Next: Basic FFmpeg unit tests with real audio)
### Status: Phases 1-3 Complete, Phase 4 Step 1 Complete

### Completed Phases: 
- ✅ **Phase 1**: Basic waveform endpoint with database integration
- ✅ **Phase 2**: FFmpeg audio processing with advanced PCM analysis
- ✅ **Phase 3**: Background job queue with worker pool system
- 🚧 **Phase 4**: Testing implementation (Step 1/4 complete - test clips ready)

### Major Implementation Summary:
**Phases 1-3 completed with advanced features!** ✅

#### What was implemented beyond the original plan:
- **Advanced Job System**: Full job queue with types, priorities, retries, and progress tracking
- **Worker Pool Architecture**: Extensible processor system for multiple job types
- **Direct URL Processing**: FFmpeg can process URLs directly without pre-download
- **Smart Job Management**: Automatic job creation when waveforms are requested
- **Enhanced API Responses**: Intelligent status reporting based on job states
- **Comprehensive Error Handling**: Proper error propagation and retry logic

#### Technical Architecture Implemented:
- `pkg/ffmpeg/` - Complete FFmpeg wrapper with metadata extraction and waveform generation
- `internal/services/workers/` - Worker pool system with job processors
- `internal/services/jobs/` - Job queue service with comprehensive management
- Enhanced API handlers with automatic job enqueueing and status reporting

---

*Last Updated: 2025-09-04*

## 🧪 PHASE 4: Testing in Progress
**Testing the complete waveform generation system!**

With Phases 1-3 complete, we have:
- ✅ **Complete waveform generation pipeline** (FFmpeg + Database + Job Queue)  
- ✅ **Background processing** (Worker pool + Progress tracking)
- ✅ **Smart API endpoints** (Automatic job creation + Status reporting)
- ✅ **Test audio clips** extracted and ready for comprehensive testing

**Current focus**: Build comprehensive test suite to validate the complete system before moving to WebSocket implementation.

### Test Files Available:
- `./data/tests/clips/test-5s.mp3` (10KB, 5 seconds) - Quick unit tests
- `./data/tests/clips/test-30s.mp3` (60KB, 30 seconds) - Standard tests

**Next step**: Create FFmpeg unit tests with real audio files to validate the complete pipeline.


- Match the Podcast index endpoints! there's something off about trending and i think maybe we've got wrong shared types
- unified responses. there should be a podcast object, an episode object, a waveform object. every endpoint needs to make sure to return its proper type
