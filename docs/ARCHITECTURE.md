# Podcast Player API - Architecture Specification

## Table of Contents
1. [System Overview](#system-overview)
2. [Design Philosophy](#design-philosophy)
3. [System Architecture](#system-architecture)
4. [Component Analysis](#component-analysis)
5. [System-Wide Trade-offs](#system-wide-trade-offs)
6. [Hidden Complexities](#hidden-complexities)
7. [Future Evolution](#future-evolution)

## System Overview

The Podcast Player API is a Go-based backend service designed to transform passive podcast listening into an active knowledge creation experience. It serves as an **audio content enrichment platform** that proxies podcast content, processes audio for enhanced playback features, and provides real-time updates to a mobile client via WebSockets.

### Core Capabilities
- **Podcast Discovery**: Search and retrieve podcast information via Podcast Index API proxy
- **Real-time Communication**: WebSocket-based bidirectional communication for instant updates
- **Audio Streaming**: HTTP-based audio streaming with range request support
- **Audio Processing**: Automated waveform generation and transcription
- **Knowledge Management**: Time-based audio tagging system for annotations
- **Intelligent Caching**: Multi-layer caching to minimize reprocessing and API calls

### Key Design Principles
- **Progressive Enhancement**: Each processing stage adds value independently
- **Graceful Degradation**: System remains useful even when degraded
- **Separation of Concerns**: Clear boundaries between control plane and data plane
- **Single Responsibility**: Each component has one clear purpose
- **Fail-Safe Defaults**: System favors availability over perfect consistency

## Design Philosophy

### Not Just Another Podcast Player
This architecture reflects a vision beyond simple playback:
- **Not just streaming** → Processing pipeline for added value
- **Not just playback** → Tagging system for knowledge management
- **Not just single-session** → Persistent enrichment that grows over time
- **Not just consumption** → Interactive annotation and analysis

### Architectural Principles

1. **Simplicity Over Complexity**: Single binary deployment with embedded SQLite
2. **Performance Through Caching**: Multi-layer caching minimizes external calls
3. **Reliability Through Isolation**: External processes (FFmpeg) run in isolation
4. **Flexibility Through Modularity**: Components can be swapped without system redesign
5. **Observability By Design**: Structured logging and metrics at every layer

## System Architecture

```
┌─────────────────┐         ┌──────────────────┐
│  Mobile Client  │────────▶│  WebSocket       │
│  (iOS/Android)  │◀────────│  Connection      │
└─────────────────┘         └──────────────────┘
                                     │
                    ┌────────────────┼────────────────┐
                    │                │                │
              ┌─────▼──────┐  ┌─────▼──────┐  ┌─────▼──────┐
              │  WebSocket  │  │   HTTP     │  │   Stream   │
              │   Handler   │  │  Router    │  │   Proxy    │
              └─────┬───────┘  └─────┬──────┘  └─────┬──────┘
                    │                │                │
        ┌───────────┴────────────────┴────────────────┴───────────┐
        │                    Service Layer                        │
        ├──────────────────────────────────────────────────────────┤
        │  • Podcast Service  • Audio Processor  • Tag Service    │
        │  • Queue Manager    • Transcription    • Cache Service  │
        └───────────┬──────────────────────────────────────────┬──────────┘
                    │                                  │
        ┌───────────▼──────────┐          ┌───────────▼──────────┐
        │   External APIs      │          │    Data Layer        │
        ├──────────────────────┤          ├──────────────────────┤
        │  • Podcast Index     │          │  • SQLite Database   │
        │  • OpenAI Whisper    │          │  • File Cache        │
        └──────────────────────┘          └──────────────────────┘
```

### Architectural Layers

#### Presentation Layer
- **WebSocket Handler**: Maintains persistent connection for real-time updates
- **HTTP Router**: RESTful endpoints for data transfer
- **Stream Proxy**: Efficient audio streaming with range support

#### Service Layer
- **Business Logic**: Encapsulates all domain-specific operations
- **Orchestration**: Coordinates between multiple services
- **Transaction Management**: Ensures data consistency

#### Data Layer
- **Persistence**: SQLite for structured data storage
- **Caching**: Multi-level cache hierarchy
- **File Storage**: Temporary and cached file management

#### External Integration Layer
- **API Clients**: Wrapped external service calls
- **Circuit Breakers**: Fault tolerance for external dependencies
- **Rate Limiting**: Respect external API limits

## Component Analysis

### 1. WebSocket Handler - The Real-time Nervous System

**Purpose**: Maintains stateful, persistent communication channel between client and server.

**Key Design Decisions**:
- **Single Client Optimization**: Since targeting single user, connection management is simplified
- **Heartbeat Mechanism (30s)**: Keeps connection alive through proxies and provides early failure detection
- **Message Queuing**: Ensures ordered delivery of async processing updates
- **Automatic Reconnection**: Handles mobile network instability gracefully

**Critical Considerations**:
- Mobile apps lose connection frequently (backgrounding, network switches)
- Large data (waveforms) might exceed message limits - use reference URLs
- WebSocket doesn't guarantee delivery - need application-level acknowledgments
- Connection state must be restored after reconnection

**Integration Points**:
- Receives events from processing queue
- Triggers service layer operations
- Maintains reference to active jobs
- Coordinates with stream proxy

### 2. HTTP Router & Stream Proxy - The Content Delivery Pipeline

**Purpose**: Handles heavy data transfer separately from control messages.

**Key Design Decisions**:
- **Range Request Support**: Essential for seeking and resume functionality
  - Partial content (206) responses
  - Proper Content-Range headers
  - Handle multiple byte ranges
- **No Transcoding**: Preserves original quality, reduces latency
- **Chunked Transfer Encoding**: Enables streaming before knowing file size
- **Separation of Concerns**: Control plane (WebSocket) vs Data plane (HTTP)

**Critical Considerations**:
- Podcast servers might be slow/unreliable - need timeouts
- Some hosts use expiring signed URLs - need refresh logic
- Mobile clients might open multiple connections - need limiting
- CORS headers required for web client access

### 3. Service Layer - The Business Logic Orchestra

**Components**:

#### Podcast Service
- **Caching Strategy**: 1-hour TTL for search results
- **Rate Limiting**: Self-imposed to stay under API limits
- **Fallback Behavior**: Serve stale cache when API unavailable
- **Data Transformation**: Maps external schema to internal

#### Queue Manager
- **Worker Pool Pattern**: 2 workers default (CPU-intensive FFmpeg)
- **Job Prioritization**: User-initiated jobs get priority
- **Failure Handling**: Retry with exponential backoff
- **Resource Management**: Limit concurrent FFmpeg processes

#### Audio Processor
- **Process Isolation**: FFmpeg via exec.Command for stability
- **Streaming Processing**: Process while downloading for large files
- **Progressive Enhancement**: Low-res waveform first, then high-res
- **Error Recovery**: Process timeouts and cleanup

#### Transcription Service
- **Chunking Strategy**: 25MB limit, smaller chunks for parallelism
- **Cost Optimization**: Skip silence, use voice activity detection
- **Language Detection**: Auto-detect with fallback to metadata
- **Timestamp Alignment**: Handle drift in long episodes

### 4. Audio Processing Pipeline - The Value Creation Engine

**Processing Stages**:

#### Stage 1: Metadata Extraction (FFprobe)
- **Why First**: Fast extraction, immediately useful
- **Data Retrieved**: Duration, bitrate, codec, ID3 tags
- **Failure Mode**: Fallback to HTTP HEAD for basic info
- **Optimization**: Process first MB only when possible

#### Stage 2: Waveform Generation (audiowaveform)
- **Tool Choice**: BBC audiowaveform for production reliability
- **Multi-resolution**: Multiple zoom levels (256, 512, 1024 samples/pixel)
- **Progressive Delivery**: Stream updates as available
- **Resource Management**: Memory scales with audio length

#### Stage 3: Transcription (Whisper)
- **Why Last**: Most expensive, longest processing
- **Chunking Complexity**: Split at silence, overlap for context
- **Quality Factors**: Audio quality varies, music interference
- **Progressive Delivery**: Send partial transcripts as ready

**Pipeline Coordination**:
- Metadata must complete first
- Waveform and transcription can parallelize
- Weighted progress tracking
- Graceful degradation on partial failure

### 5. Database Layer & Caching - The Memory Foundation

**SQLite Choice Rationale**:
- Zero operational complexity
- Sufficient performance for single-user
- Simple backup/restore (file copy)
- Embedded reduces latency

**Caching Hierarchy**:

#### L1 - In-Memory Cache
- Hot data: Active episode metadata
- TTL: 5-10 minutes
- Size limit: ~100MB
- Invalidation: On database writes

#### L2 - SQLite Cache Tables
- API responses: Search results
- Processed data: Waveforms, transcriptions
- TTL: Variable by type
- Size: Unbounded

#### L3 - File System Cache
- Temporary audio during processing
- Generated assets
- Cleanup: On completion/timeout

**Schema Design Decisions**:
- Normalized core relationships
- JSON fields for flexibility
- Strategic denormalization for performance
- Comprehensive indexing strategy

### 6. External API Integration - The Dependency Layer

**Podcast Index API**:
- **Authentication**: Request signing with timestamp
- **Reliability**: Circuit breaker, retry logic, fallbacks
- **Caching**: Aggressive to minimize calls
- **Data Quality**: Handle missing/inconsistent data

**OpenAI Whisper API**:
- **Cost Management**: $0.006/minute tracking
- **Technical Challenges**: 25MB limit, variable response times
- **Quality Control**: Confidence scores, language detection
- **Optimization**: Remove silence before submission

**Integration Patterns**:
- **Adapter Pattern**: Wrap APIs in interfaces
- **Repository Pattern**: Cache external data locally
- **Gateway Pattern**: Single entry point per service
- **Fallback Strategies**: Graceful degradation

### 7. Message Flow & State Management - The Orchestration Layer

**Critical User Flows**:

1. **Podcast Discovery**: Search → Cache Check → API → Transform → Store → Return
2. **Episode Processing**: Select → Check Processed → Create Jobs → Process → Update
3. **Audio Streaming**: Request → Authorize → Fetch Metadata → Proxy → Stream
4. **Tag Management**: Create → Validate → Store → Broadcast

**State Management**:
- **Connection State**: WebSocket status and recovery
- **Processing State**: Job queue and progress
- **Playback State**: Position and buffer status
- **Application State**: Preferences and UI state

**Synchronization Strategies**:
- Optimistic updates with rollback
- Eventual consistency for non-critical data
- Last-write-wins conflict resolution
- Periodic full state sync

## System-Wide Trade-offs

### SQLite vs PostgreSQL
| Choice | Pros | Cons |
|--------|------|------|
| SQLite | Zero ops, Simple deployment, Fast for single-user | No horizontal scaling, Single writer, Size limitations |
| **Verdict** | Perfect for MVP and single-user scenario | Will need migration for multi-user |

### FFmpeg Execution vs Libraries
| Choice | Pros | Cons |
|--------|------|------|
| exec.Command | Process isolation, Always current, Stable | Process overhead, Complex error handling |
| **Verdict** | Reliability trumps performance for audio processing |

### WebSocket vs REST
| Choice | Pros | Cons |
|--------|------|------|
| WebSocket | Real-time natural, Persistent connection, Lower latency | Complex client, Connection management |
| **Verdict** | Essential for progressive updates and real-time features |

## Hidden Complexities

### 1. Distributed Transaction Problem
Without database transactions across services:
- Implement Saga pattern with compensation
- Ensure idempotent operations
- Use event sourcing for recovery

### 2. Cache Invalidation
The hardest problem in computer science:
- When does podcast data become stale?
- How to invalidate dependent caches?
- Handling partial updates

### 3. Progress Calculation
Simple concept, complex implementation:
- Different operations take different times
- Network speed affects streaming
- Estimating remaining time accurately

### 4. State Synchronization
Managing state across async operations:
- Race conditions between parallel operations
- Client-server state drift
- Reconnection state restoration

### 5. Resource Management
Balancing performance and stability:
- FFmpeg memory consumption
- Concurrent job limits
- Disk space for temporary files

## Future Evolution

### Scaling Up
1. **Database**: Migrate SQLite → PostgreSQL
2. **Caching**: Add Redis for distributed cache
3. **Queue**: Implement RabbitMQ/Kafka
4. **Deployment**: Containerize with Kubernetes
5. **CDN**: Edge caching for global distribution

### Feature Growth
1. **Multi-user**: Authentication and authorization
2. **Playlists**: Curated episode collections
3. **Social**: Share tags and annotations
4. **Analytics**: Listening patterns and insights
5. **Subscriptions**: RSS feed monitoring

### Technical Improvements
1. **GraphQL**: Flexible query interface
2. **SSE**: Alternative to WebSocket
3. **ML**: Content recommendations
4. **Search**: Elasticsearch for transcripts
5. **Monitoring**: Prometheus + Grafana

### Architecture Evolution
1. **Microservices**: Split monolith when needed
2. **Event-Driven**: Event bus for loose coupling
3. **CQRS**: Separate read/write paths
4. **Service Mesh**: Advanced networking
5. **Serverless**: Functions for processing

## Architectural Strengths

1. **Modularity**: Single responsibility components
2. **Testability**: Clear interfaces and boundaries
3. **Observability**: Comprehensive logging/metrics
4. **Flexibility**: Easy component substitution
5. **Performance**: Multi-layer caching strategy
6. **Reliability**: Graceful degradation patterns
7. **Simplicity**: No unnecessary complexity

## Architectural Weaknesses

1. **Single Point of Failure**: SQLite database
2. **External Dependencies**: Internet requirement
3. **Resource Intensive**: CPU/memory spikes
4. **Limited Scalability**: Single-user focus
5. **Complex State**: Async coordination challenges

## Conclusion

This architecture balances pragmatism with ambition. It's simple enough to build quickly but structured for growth. Each component has a clear purpose and fits into the whole coherently. The real elegance lies in what's NOT there - no unnecessary complexity, no premature optimization, no architecture astronautics.

The system is ready to be built, tested, and evolved based on real user feedback rather than imagined requirements. It serves the vision of transforming podcast listening into active knowledge creation while maintaining operational simplicity.