# Development Guide

## Prerequisites

### Required Software
- Go 1.21+ (install via [official site](https://golang.org/dl/) or package manager)
- SQLite 3.35+ (usually pre-installed on macOS/Linux)
- FFmpeg 4.0+ with FFprobe
- BBC audiowaveform ([installation guide](https://github.com/bbc/audiowaveform))
- Git

### Optional Tools
- Docker & Docker Compose (for containerized development)
- Air (for hot reload during development)
- golangci-lint (for code quality)
- Task (task runner, alternative to Make)

### Installation Commands

#### macOS (Homebrew)
```bash
# Core tools
brew install go sqlite ffmpeg

# audiowaveform
brew tap bbc/audiowaveform
brew install audiowaveform

# Development tools
go install github.com/air-verse/air@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
brew install go-task
```

#### Linux (Ubuntu/Debian)
```bash
# Core tools
sudo apt update
sudo apt install golang sqlite3 ffmpeg

# audiowaveform (build from source)
sudo apt install git cmake libboost-all-dev libsndfile1-dev libgd-dev libmad0-dev libid3tag0-dev
git clone https://github.com/bbc/audiowaveform.git
cd audiowaveform
mkdir build && cd build
cmake ..
make
sudo make install

# Development tools
go install github.com/air-verse/air@latest
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
sudo snap install task --classic
```

## Project Setup

### 1. Clone Repository
```bash
git clone https://github.com/killallgit/killallplayer-api.git
cd killallplayer-api
```

### 2. Install Dependencies
```bash
go mod download
go mod verify
```

### 3. Configure Environment
```bash
# Copy environment template
cp .env.example .env

# Edit with your API keys
vim .env
```

Required environment variables:
```bash
# Podcast Index API
PODCAST_INDEX_API_KEY=your_api_key
PODCAST_INDEX_API_SECRET=your_api_secret

# OpenAI API (for Whisper)
OPENAI_API_KEY=your_openai_key

# Optional
DATABASE_PATH=./data/podcast.db
PORT=8080
LOG_LEVEL=info
```

### 4. Initialize Database
```bash
# Create data directory
mkdir -p data

# Run migrations
go run main.go migrate up
```

### 5. Verify Installation
```bash
# Check FFmpeg
ffmpeg -version
ffprobe -version

# Check audiowaveform
audiowaveform --version

# Run tests
go test ./...
```

## Development Workflow

### Running the Server

#### Development Mode (with hot reload)
```bash
# Using Air for auto-reload
air

# Or using Task (RECOMMENDED - loads .env file automatically)
task serve
```

#### Standard Mode
```bash
# Build and run
go build -o bin/player-api
./bin/player-api serve

# Or directly
go run main.go serve

# Note: When not using Task, ensure environment variables are loaded:
source .env && go run main.go serve
```

#### Debug Mode
```bash
# With detailed logging
LOG_LEVEL=debug go run main.go serve

# With delve debugger
dlv debug -- serve
```

### Project Structure
```
killallplayer-api/
├── cmd/                    # CLI commands
│   ├── root.go            # Root command
│   ├── serve.go           # Server command
│   └── migrate.go         # Migration command
├── internal/              # Private application code
│   ├── api/              # HTTP/WebSocket handlers
│   │   ├── handlers/     # Request handlers
│   │   ├── middleware/   # HTTP middleware
│   │   └── websocket/    # WebSocket implementation
│   ├── models/           # Data models
│   ├── services/         # Business logic
│   │   ├── podcast/      # Podcast service
│   │   ├── audio/        # Audio processing
│   │   ├── transcription/# Whisper integration
│   │   └── queue/        # Job queue
│   ├── database/         # Database layer
│   │   ├── migrations/   # SQL migrations
│   │   └── repositories/ # Data access
│   └── config/           # Configuration
├── pkg/                  # Public packages
│   ├── ffmpeg/          # FFmpeg wrapper
│   ├── waveform/        # Waveform generation
│   └── cache/           # Caching utilities
├── docs/                # Documentation
├── tests/               # Integration tests
├── scripts/             # Utility scripts
└── deployments/         # Deployment configs
```

### Code Style

#### Go Conventions
- Follow [Effective Go](https://golang.org/doc/effective_go.html)
- Use `gofmt` for formatting
- Use `golangci-lint` for linting
- Write tests for all business logic
- Document exported functions

#### Naming Conventions
- Packages: lowercase, no underscores
- Files: lowercase with underscores
- Types: PascalCase
- Functions/Methods: PascalCase (exported), camelCase (private)
- Constants: PascalCase or SCREAMING_SNAKE_CASE
- Variables: camelCase

### Testing

#### Unit Tests
```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# Verbose output
go test -v ./...

# Specific package
go test ./internal/services/podcast

# With race detection
go test -race ./...
```

#### Integration Tests
```bash
# Run integration tests
go test -tags=integration ./tests/...

# With test database
TEST_DB=./test.db go test -tags=integration ./tests/...
```

#### Test Structure
```go
func TestPodcastService_Search(t *testing.T) {
    // Arrange
    service := NewPodcastService(mockClient)
    
    // Act
    result, err := service.Search("golang")
    
    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.Equal(t, 10, len(result.Podcasts))
}
```

### Database Management

#### Migrations
```bash
# Create new migration
go run main.go migrate create add_user_table

# Run migrations
go run main.go migrate up

# Rollback last migration
go run main.go migrate down

# Check migration status
go run main.go migrate status
```

#### Database Access
```bash
# SQLite CLI
sqlite3 data/podcast.db

# Common queries
.tables                    # List tables
.schema episodes          # Show table schema
SELECT * FROM episodes;   # Query data
.exit                     # Exit
```

### Debugging

#### Logging
```go
import "github.com/rs/zerolog/log"

// Different log levels
log.Debug().Str("key", "value").Msg("Debug message")
log.Info().Str("episode", episodeID).Msg("Processing started")
log.Warn().Err(err).Msg("Retryable error occurred")
log.Error().Err(err).Str("job", jobID).Msg("Processing failed")
```

#### Performance Profiling
```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=.
go tool pprof mem.prof

# Runtime profiling (with pprof endpoint)
go tool pprof http://localhost:8080/debug/pprof/profile
```

#### Request Tracing
```go
// Add request ID to context
ctx := context.WithValue(r.Context(), "request_id", uuid.New())

// Log with request ID
log.Info().
    Str("request_id", ctx.Value("request_id").(string)).
    Msg("Processing request")
```

## Common Tasks

### Adding a New Endpoint
1. Define handler in `internal/api/handlers/`
2. Add route in `internal/api/router.go`
3. Add request/response types in `internal/models/`
4. Write tests in `internal/api/handlers/handler_test.go`
5. Update API documentation

### Adding a New WebSocket Message
1. Define message type in `internal/api/websocket/messages.go`
2. Add handler in `internal/api/websocket/handlers.go`
3. Update client message router
4. Add tests
5. Update API documentation

### Adding External Service
1. Create client in `internal/services/`
2. Add configuration in `internal/config/`
3. Implement circuit breaker and retry logic
4. Add caching layer
5. Write integration tests
6. Document in `docs/EXTERNAL_APIS.md`

## Configuration

### Configuration File (config.yaml)
```yaml
server:
  host: 0.0.0.0
  port: 8080
  read_timeout: 30s
  write_timeout: 30s

database:
  path: ./data/podcast.db
  max_connections: 10

processing:
  workers: 2
  ffmpeg_path: /usr/local/bin/ffmpeg
  ffprobe_path: /usr/local/bin/ffprobe
  audiowaveform_path: /usr/local/bin/audiowaveform

cache:
  ttl: 1h
  max_size: 100MB

logging:
  level: info
  format: json
```

### Environment Variables
Environment variables override config file values:
```bash
export PORT=9090
export LOG_LEVEL=debug
export DATABASE_PATH=/custom/path/podcast.db
```

## Troubleshooting

### Common Issues

#### Port Already in Use
```bash
# Find process using port 8080
lsof -i :8080

# Kill process
kill -9 <PID>

# Or kill all player-api processes
pkill -f "player-api serve"
```

#### Episode "Record Not Found" Errors
- Ensure episodes are synced: `curl -X POST http://localhost:8080/api/v1/podcasts/{id}/episodes/sync`
- Check database has episodes: `sqlite3 data/podcast.db "SELECT COUNT(*) FROM episodes;"`
- Verify streaming endpoint is implemented at `/api/v1/stream/:id`

#### Database Lock Error
```bash
# Check for lock file
ls -la data/*.db-wal data/*.db-shm

# Remove if stale
rm data/*.db-wal data/*.db-shm
```

#### FFmpeg Not Found
```bash
# Check FFmpeg in PATH
which ffmpeg

# Set custom path in config
processing:
  ffmpeg_path: /custom/path/to/ffmpeg
```

#### Memory Issues
```bash
# Increase Go memory limit
GOMEMLIMIT=2GiB go run main.go serve

# Profile memory usage
go tool pprof -http=:8081 http://localhost:8080/debug/pprof/heap
```

### Debug Commands
```bash
# Check service health
curl http://localhost:8080/health

# Test streaming endpoint
curl -I http://localhost:8080/api/v1/stream/1  # HEAD request for metadata
curl -H "Range: bytes=0-1000" http://localhost:8080/api/v1/stream/1  # Range request

# Search for podcasts
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query": "technology", "limit": 5}'

# Sync episodes
curl -X POST http://localhost:8080/api/v1/podcasts/217331/episodes/sync

# Monitor logs
tail -f logs/app.log | jq '.'

# Database integrity check
sqlite3 data/podcast.db "PRAGMA integrity_check;"
```

## Deployment

### Building for Production
```bash
# Build with optimizations
CGO_ENABLED=1 go build -ldflags="-s -w" -o bin/player-api

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o bin/player-api-linux

# Build Docker image
docker build -t player-api:latest .
```

### Running in Production
```bash
# With systemd service
sudo systemctl start player-api
sudo systemctl status player-api

# With Docker
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/data:/app/data \
  -v $(pwd)/config:/app/config \
  --env-file .env \
  player-api:latest
```

## Contributing

### Pull Request Process
1. Fork the repository
2. Create feature branch (`git checkout -b feature/amazing-feature`)
3. Write tests for new functionality
4. Ensure all tests pass (`go test ./...`)
5. Run linter (`golangci-lint run`)
6. Commit changes (`git commit -m 'Add amazing feature'`)
7. Push to branch (`git push origin feature/amazing-feature`)
8. Open Pull Request

### Code Review Checklist
- [ ] Tests written and passing
- [ ] Documentation updated
- [ ] No linting errors
- [ ] Performance impact considered
- [ ] Security implications reviewed
- [ ] Error handling implemented
- [ ] Logging added appropriately