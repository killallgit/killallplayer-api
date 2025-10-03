# Podcast Player API

A robust REST API for podcast discovery and episode management built with Go, using the Podcast Index API for content discovery.

## Features

- 🎙️ **Podcast Discovery** - Search and browse trending podcasts via Podcast Index API
- 📋 **Episode Management** - Sync, store, and retrieve podcast episodes
- 🔖 **Playback Regions** - Save bookmarks and regions within episodes
- 📊 **Waveform Generation** - Generate audio waveforms for visual representation
- 🎯 **ML Training Clips** - Extract labeled audio segments for machine learning
- 🤖 **Auto-Analysis** - Automatic episode analysis for segment detection
- 🔐 **Supabase Authentication** - JWT-based authentication with custom permissions via JWKS
- 🛡️ **Permission System** - Role-based access control with scoped permissions
- 🆔 **Podcast Index IDs** - Uses Podcast Index IDs throughout, no ID mapping needed
- 💾 **Local Database** - SQLite storage for offline episode access
- ⚡ **Rate Limiting** - Built-in rate limiting for API endpoints
- 🔄 **CORS Support** - Full CORS support for web clients
- 🔄 **Job Queue System** - Async processing with status tracking

## Quick Start

### Prerequisites

- Go 1.23.6+
- SQLite 3.35+
- Podcast Index API credentials ([get them here](https://api.podcastindex.org))
- Supabase project with configured authentication

### Installation

1. Clone the repository:
```bash
git clone https://github.com/killallgit/killallplayer-api.git
cd killallplayer-api
```

2. Install dependencies and tools:
```bash
# Download Go module dependencies
go mod download

# Install development tools (linting, etc.)
task install-tools
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

The API will be available at `http://localhost:9000`

## API Documentation

Interactive API documentation is available via Swagger UI at `http://localhost:9000/docs` when the server is running.
The OpenAPI specification is generated automatically from code annotations and available in `docs/swagger.json` and `docs/swagger.yaml`.

### Key Endpoints

#### Public Endpoints
- `GET /health` - Health check

#### Authenticated Endpoints (require Bearer token)
- `GET /api/v1/me` - Get current user information
- `POST /api/v1/search` - Search podcasts
- `POST /api/v1/trending` - Get trending podcasts
- `GET /api/v1/podcasts/:id/episodes` - Get episodes for a podcast (using Podcast Index feedId)
- `GET /api/v1/episodes/:id` - Get episode details (using Podcast Index ID)
- `GET /api/v1/episodes/:id/reviews` - Get iTunes reviews for the podcast
- `GET /api/v1/episodes/:id/waveform` - Generate/retrieve waveform data with status
- `POST /api/v1/episodes/:id/analyze` - Auto-analyze episode for clips
- `GET /api/v1/episodes/:id/clips` - Get clips for an episode
- `POST /api/v1/clips` - Create ML training audio clip
- `GET /api/v1/clips` - List all clips with optional filters
- `GET /api/v1/clips/:uuid` - Get specific clip details
- `PUT /api/v1/clips/:uuid/label` - Update clip label
- `DELETE /api/v1/clips/:uuid` - Delete a clip
- `GET /api/v1/clips/export` - Export clips dataset as ZIP
- `POST /api/v1/regions` - Save playback regions/bookmarks
- `GET /api/v1/regions?episodeId=` - Get regions for an episode

### Example Usage

**Note:** Most endpoints require authentication. Get a JWT token from your Supabase client, then include it in the Authorization header.

```bash
# Get current user info (requires authentication)
curl -X GET http://localhost:9000/api/v1/me \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# Search for podcasts (requires authentication)
curl -X POST http://localhost:9000/api/v1/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{"query": "technology", "limit": 5}'

# Get trending podcasts (requires authentication)
curl -X POST http://localhost:9000/api/v1/trending \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{"max": 3}'

# Get episodes for a podcast (requires authentication)
curl -X GET http://localhost:9000/api/v1/podcasts/41506/episodes \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# Save a playbook region/bookmark (requires authentication)
curl -X POST http://localhost:9000/api/v1/regions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{"episodeId": 41951637359, "startTime": 10.5, "endTime": 45.8, "label": "Important"}'
```

## Project Structure

```
killallplayer-api/
├── api/                  # API handlers and routes
│   ├── auth/            # Authentication endpoints and middleware
│   ├── clips/           # ML training clips endpoints
│   ├── episodes/        # Episode endpoints
│   ├── podcasts/        # Podcast endpoints
│   ├── search/          # Search functionality
│   ├── trending/        # Trending podcasts
│   └── types/           # Shared types
├── cmd/                 # CLI commands
├── internal/            # Internal packages
│   ├── database/        # Database layer
│   ├── models/          # Data models
│   └── services/        # Business logic
│       ├── auth/        # Supabase JWT authentication service
│       ├── audiocache/  # Local audio caching
│       ├── autolabel/   # Automatic clip labeling
│       ├── clips/       # Clip extraction and storage
│       ├── episode_analysis/ # Episode analysis orchestration
│       ├── episodes/    # Episode service
│       ├── jobs/        # Job queue management
│       ├── transcription/ # Audio transcription
│       ├── waveforms/   # Waveform generation
│       └── workers/     # Background job processors
├── pkg/                 # Public packages
│   ├── config/          # Configuration management
│   ├── download/        # HTTP download utilities
│   ├── ffmpeg/          # FFmpeg wrappers
│   └── transcript/      # Transcript fetching & parsing
├── scripts/             # Setup and testing scripts
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
KILLALL_SERVER_PORT=9000
KILLALL_SERVER_HOST=0.0.0.0

# Database (optional)
KILLALL_DATABASE_PATH=./data/podcast.db

# Supabase Authentication (required for protected endpoints)
KILLALL_SUPABASE_JWKS_URL=https://your-project.supabase.co/auth/v1/.well-known/jwks.json

# Optional: Development authentication bypass
KILLALL_DEV_AUTH_ENABLED=false
KILLALL_DEV_AUTH_TOKEN=dev-token-for-testing
```

## Authentication

The API uses **Supabase JWT authentication** with custom permissions. Users must be manually provisioned with appropriate permissions to access the API.

### Overview

- **JWT Validation**: Uses JWKS (JSON Web Key Set) with ES256 (ECDSA) signatures
- **Permission System**: Role-based access control with scoped permissions
- **No Local User Storage**: Supabase is the single source of truth for user data
- **Performance**: Permissions embedded in JWT claims (no database lookups per request)

### Permission System

The API uses a hierarchical permission system with the following scopes:

| Permission | Description |
|------------|-------------|
| `podcasts:read` | Read access to podcast data, search, and episode information |
| `podcasts:write` | Write access to create clips, regions, and user data |
| `podcasts:admin` | Full administrative access to all endpoints and features |

**Authorization Requirements:**
- All `/api/v1/*` endpoints require authentication (except `/api/v1/auth/dev-login` in dev mode)
- Users must have at least one `podcasts:*` permission to access the API
- Users without any permissions are automatically denied access

### Setup Instructions

#### 1. Configure Supabase Project

1. **Create Supabase Project**: Set up a new project at [supabase.com](https://supabase.com)

2. **Enable ECC Keys**: In your Supabase dashboard:
   - Go to **Settings > API > JWT Settings**
   - Switch from legacy HS256 to **ECC (P-256)** keys
   - This enables ES256 signatures required by the API

3. **Get JWKS URL**: Copy your JWKS URL:
   ```
   https://your-project.supabase.co/auth/v1/.well-known/jwks.json
   ```

#### 2. Configure API Environment

Add to your `.env` file:
```bash
KILLALL_SUPABASE_JWKS_URL=https://your-project.supabase.co/auth/v1/.well-known/jwks.json
```

#### 3. User Provisioning

Users must be manually provisioned with permissions. Use the included scripts:

**Add Basic User Permissions:**
```bash
# Set user credentials in .env
SUPABASE_ADMIN_USER=admin@example.com
SUPABASE_ADMIN_PASSWORD=your-password
SUPABASE_API_KEY=your-service-role-key

# Add permissions to existing user
./scripts/setup-user-permissions.sh
```

**Upgrade User to Admin:**
```bash
./scripts/upgrade-to-admin.sh
```

**Manual User Creation (via Supabase Admin API):**
```bash
# Create user with permissions
curl -X POST "https://your-project.supabase.co/auth/v1/admin/users" \
  -H "Authorization: Bearer YOUR_SERVICE_ROLE_KEY" \
  -H "apikey: YOUR_SERVICE_ROLE_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "email": "user@example.com",
    "password": "secure-password",
    "email_confirm": true,
    "app_metadata": {
      "permissions": ["podcasts:read", "podcasts:write"],
      "role": "user"
    }
  }'
```

#### 4. Client Integration

**Frontend (JavaScript/TypeScript):**
```javascript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient(
  'https://your-project.supabase.co',
  'your-anon-key'
)

// Login user
const { data, error } = await supabase.auth.signInWithPassword({
  email: 'user@example.com',
  password: 'password'
})

// Get JWT token for API calls
const token = data.session?.access_token

// Use token with API
const response = await fetch('http://localhost:8080/api/v1/me', {
  headers: {
    'Authorization': `Bearer ${token}`,
    'Content-Type': 'application/json'
  }
})
```

**Mobile (React Native/Expo):**
```javascript
import { createClient } from '@supabase/supabase-js'

const supabase = createClient(
  'https://your-project.supabase.co',
  'your-anon-key'
)

// Handle auth state changes
supabase.auth.onAuthStateChange((event, session) => {
  if (session) {
    const token = session.access_token
    // Store token for API calls
  }
})
```

### Development Mode

For development and testing, you can enable auth bypass:

```bash
# In .env file
KILLALL_DEV_AUTH_ENABLED=true
KILLALL_DEV_AUTH_TOKEN=dev-token-for-testing
```

Then use the dev token in requests:
```bash
curl -X GET http://localhost:8080/api/v1/me \
  -H "Authorization: Bearer dev-token-for-testing"
```

### Testing Authentication

Use the included test scripts to verify your setup:

```bash
# Test real Supabase authentication
./scripts/test-supabase-auth.sh

# Test admin access to all endpoints
./scripts/test-admin-endpoints.sh
```

## Development

### Running Tests

```bash
# Run all tests using task
task test

# Or run tests directly
go test ./...

# With coverage
go test -cover ./...

# Run linting
task lint
```

### Building

```bash
# Build using task (recommended)
task build

# Or build manually
go build -o bin/killallplayer-api

# Build with optimizations
CGO_ENABLED=1 go build -ldflags="-s -w" -o bin/killallplayer-api

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 go build -o bin/killallplayer-api-linux
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