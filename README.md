# Podcast Player API

A robust REST API for podcast discovery, episode management, and audio streaming built with Go, using the Podcast Index API for content discovery.

## Features

- 🎙️ **Podcast Discovery** - Search and discover podcasts via Podcast Index API
- 📋 **Episode Management** - Sync, store, and retrieve podcast episodes
- 🎵 **Audio Streaming** - HTTP audio streaming with range request support (seeking)
- 🆔 **Podcast Index IDs** - Uses Podcast Index IDs throughout, no ID mapping needed
- 🔗 **Direct URL Streaming** - Stream any audio URL without database storage
- 💾 **Local Database** - SQLite storage for offline episode access
- ⚡ **Rate Limiting** - Built-in rate limiting for API endpoints
- 🔄 **CORS Support** - Full CORS support for web clients

## Quick Start

### Prerequisites

- Go 1.21+
- SQLite 3.35+
- Podcast Index API credentials ([get them here](https://api.podcastindex.org))

### Installation

1. Clone the repository:
```bash
git clone https://github.com/killallgit/killallplayer-api.git
cd killallplayer-api
```

2. Install dependencies:
```bash
go mod download
```

3. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your API credentials
```

4. Run the server:
```bash
task serve
# Or without Task: source .env && go run main.go serve
```

The API will be available at `http://localhost:8080`

## API Documentation

### Swagger UI

Interactive API documentation is available via Swagger UI at:
```
http://localhost:8080/swagger/index.html
```

**Authentication Required**: Use the static token `swagger-api-token-2025` in the Authorization header:
- Click the "Authorize" button in Swagger UI
- Enter: `swagger-api-token-2025` (without "Bearer " prefix)

### Complete Documentation

Complete API documentation is also available in [docs/API_SPECIFICATION.md](docs/API_SPECIFICATION.md).

### Key Endpoints

- `GET /health` - Health check
- `POST /api/v1/search` - Search podcasts
- `GET /api/v1/episodes/:id` - Get episode details (using Podcast Index ID)
- `GET /api/v1/stream/:id` - Stream audio with range support (using Podcast Index ID)
- `GET /api/v1/stream/direct?url=` - Stream audio from any URL
- `POST /api/v1/podcasts/:id/episodes/sync` - Sync episodes from Podcast Index

### Example Usage

```bash
# Search for podcasts
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query": "technology", "limit": 5}'

# Sync episodes for a podcast (using Podcast Index podcast ID)
curl -X POST http://localhost:8080/api/v1/podcasts/41506/episodes/sync

# Stream audio using Podcast Index episode ID (supports seeking)
curl http://localhost:8080/api/v1/stream/41928435424
curl -H "Range: bytes=1024000-2048000" http://localhost:8080/api/v1/stream/41928435424
```

## Project Structure

```
killallplayer-api/
├── api/                  # API handlers and routes
│   ├── episodes/        # Episode endpoints
│   ├── podcasts/        # Podcast endpoints
│   ├── search/          # Search functionality
│   ├── stream/          # Audio streaming
│   └── types/           # Shared types
├── cmd/                 # CLI commands
├── internal/            # Internal packages
│   ├── database/        # Database layer
│   ├── models/          # Data models
│   └── services/        # Business logic
├── pkg/                 # Public packages
│   └── config/          # Configuration management
└── docs/                # Documentation
```

## Configuration

The application uses Viper for configuration management and supports:

1. **Environment variables** (prefix: `KILLALL_`)
2. **Configuration file** (`config.yaml`)
3. **Default values**

### Environment Variables

```bash
# Podcast Index API (required)
KILLALL_PODCAST_INDEX_API_KEY=your_api_key
KILLALL_PODCAST_INDEX_API_SECRET=your_api_secret

# Server (optional)
KILLALL_SERVER_PORT=8080
KILLALL_SERVER_HOST=0.0.0.0

# Database (optional)
KILLALL_DATABASE_PATH=./data/podcast.db
```

## Development

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for detailed development instructions.

### Running Tests

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# Specific package
go test ./pkg/config
```

### Building

```bash
# Build binary
go build -o bin/player-api

# Build with optimizations
CGO_ENABLED=1 go build -ldflags="-s -w" -o bin/player-api

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o bin/player-api-linux
```

## Architecture

The API follows a clean architecture pattern:

- **Handlers** - HTTP request/response handling
- **Services** - Business logic and external API integration
- **Repositories** - Data persistence layer
- **Models** - Domain entities

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for detailed architecture documentation.

## Rate Limiting

| Endpoint | Requests/Second | Burst |
|----------|-----------------|-------|
| Search | 5 | 10 |
| Episodes | 10 | 20 |
| Stream | 20 | 30 |
| Sync | 1 | 2 |

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [Podcast Index](https://podcastindex.org) for the podcast API
- [Gin Web Framework](https://gin-gonic.com) for HTTP routing
- [GORM](https://gorm.io) for database ORM
- [Viper](https://github.com/spf13/viper) for configuration management