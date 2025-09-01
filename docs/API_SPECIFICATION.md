# Podcast Player API Documentation

Base URL: `http://localhost:8080`

## Table of Contents
1. [Overview](#overview)
2. [Health Check](#health-check)
3. [Search](#search)
4. [Episodes](#episodes)
5. [Podcasts](#podcasts)
6. [Audio Streaming](#audio-streaming)
7. [Rate Limiting](#rate-limiting)
8. [Error Handling](#error-handling)
9. [CORS Configuration](#cors-configuration)

## Overview

The Podcast Player API provides RESTful endpoints for podcast discovery, episode management, and audio streaming. The API follows RESTful principles and supports range requests for audio streaming.

**Important:** All episode IDs in this API are Podcast Index IDs. The API does not expose internal database IDs to clients. Episode IDs received from search results can be used directly for streaming, playback updates, and other operations.

### API Versioning
Currently v1, accessed via:
```
/api/v1/...
```

## Health Check

### GET /health
Check if the service is running and healthy.

**Response:**
```json
{
  "status": "ok",
  "timestamp": "2025-09-01T15:00:00Z",
  "database": {"status": "healthy"}
}
```

### GET /
Get API version information.

**Response:**
```json
{
  "name": "Podcast Player API",
  "version": "1.0.0",
  "status": "running"
}
```

## Search

### POST /api/v1/search
Search for podcasts via Podcast Index API.

**Request Body:**
```json
{
  "query": "string",  // Required: search term
  "limit": 10         // Optional: max results (1-100, default: 10)
}
```

**Response:**
```json
{
  "podcasts": [
    {
      "id": "123456",
      "title": "Podcast Name",
      "author": "Author Name",
      "description": "Podcast description",
      "image": "https://...",
      "url": "https://..."
    }
  ]
}
```

**Rate Limit:** 5 requests/second, burst of 10

## Episodes

### GET /api/v1/episodes
Get all episodes (recent episodes across all podcasts).

**Query Parameters:**
- `limit` (optional): Max episodes to return (1-1000, default: 50)

**Response:**
```json
{
  "status": "true",
  "items": [
    {
      "id": 41928435424,
      "title": "Episode Title",
      "link": "https://...",
      "description": "Episode description",
      "guid": "unique-guid",
      "datePublished": 1756706400,
      "datePublishedPretty": "August 31, 2025 11:00pm",
      "dateCrawled": 1756723804,
      "enclosureUrl": "https://...",
      "enclosureType": "audio/mpeg",
      "enclosureLength": 11535016,
      "duration": 718,
      "episode": 493,
      "episodeType": "full",
      "season": 2,
      "image": "https://...",
      "feedItunesId": 1139480348,
      "feedImage": "https://...",
      "feedId": 52216,
      "feedLanguage": "en-us",
      "transcriptUrl": "https://..."
    }
  ],
  "count": 50,
  "description": "All episodes"
}
```

### GET /api/v1/episodes/recent
Get recent episodes across all podcasts.

**Query Parameters:**
- `max` (optional): Max episodes (1-100, default: 20)

**Response:** Same as GET /api/v1/episodes

### GET /api/v1/episodes/byfeedid
Get episodes by podcast/feed ID.

**Query Parameters:**
- `id` (required): Podcast ID
- `max` (optional): Max episodes (1-1000, default: 20)

**Response:** Same as episodes list with `query` field containing podcast ID

### GET /api/v1/episodes/byguid
Get single episode by GUID.

**Query Parameters:**
- `guid` (required): Episode GUID

**Response:** Single episode in Podcast Index format

### GET /api/v1/episodes/:id
Get single episode by Podcast Index ID.

**URL Parameters:**
- `id`: Episode ID (Podcast Index ID, numeric int64)

**Response:**
```json
{
  "status": "true",
  "episode": {
    "id": 41928435424,
    "title": "Episode Title",
    "link": "https://...",
    "description": "Episode description",
    "guid": "unique-guid",
    "datePublished": 1756706400,
    "datePublishedPretty": "August 31, 2025 11:00pm",
    "enclosureUrl": "https://...",
    "enclosureType": "audio/mpeg",
    "enclosureLength": 11535016,
    "duration": 718,
    "episode": 493,
    "episodeType": "full",
    "season": 2,
    "image": "https://...",
    "feedItunesId": 1139480348,
    "feedImage": "https://...",
    "feedId": 52216,
    "feedLanguage": "en-us",
    "transcriptUrl": "https://..."
  },
  "description": "Episode found"
}
```

### PUT /api/v1/episodes/:id/playback
Update episode playback state.

**URL Parameters:**
- `id`: Episode ID (Podcast Index ID, numeric int64)

**Request Body:**
```json
{
  "position": 120,  // Position in seconds
  "played": true    // Whether episode is marked as played
}
```

**Response:**
```json
{
  "status": "success",
  "message": "Playback state updated",
  "data": {
    "episode_id": 41928435424,
    "position": 120,
    "played": true
  }
}
```

**Rate Limit:** 10 requests/second, burst of 20

## Podcasts

### GET /api/v1/podcasts/:id/episodes
Get episodes for specific podcast.

**URL Parameters:**
- `id`: Podcast ID

**Query Parameters:**
- `max` (optional): Max episodes (1-1000, default: 20)

**Response:** Same as episodes list

### POST /api/v1/podcasts/:id/episodes/sync
Sync episodes from Podcast Index API for a specific podcast.

**URL Parameters:**
- `id`: Podcast ID

**Query Parameters:**
- `max` (optional): Max episodes to sync (1-1000, default: 50)

**Response:** Updated episodes list

**Rate Limit:** 1 request/second, burst of 2

## Audio Streaming

### GET /api/v1/stream/:id
Stream audio content with support for range requests. Acts as a proxy to the episode's audio URL, handling redirects transparently.

**URL Parameters:**
- `id`: Episode ID (Podcast Index ID, numeric int64)

**Request Headers:**
- `Range`: `bytes=start-end` (optional) - For partial content requests/seeking

**Response Headers:**
- `Content-Type`: `audio/mpeg` (or appropriate audio type from source)
- `Content-Length`: Size in bytes
- `Accept-Ranges`: `bytes`
- `Content-Range`: `bytes start-end/total` (for partial requests)
- `ETag`: Entity tag from source
- `Last-Modified`: Last modified date from source
- `Cache-Control`: Caching directives from source
- `Access-Control-Allow-Origin`: `*`
- `Access-Control-Allow-Methods`: `GET, HEAD, OPTIONS`
- `Access-Control-Allow-Headers`: `Range`
- `Access-Control-Expose-Headers`: `Content-Length, Content-Range, Accept-Ranges`

**Status Codes:**
- `200`: Full content delivery
- `206`: Partial content (range request)
- `400`: Invalid episode ID
- `404`: Episode not found or no audio URL
- `502`: Failed to fetch audio from source

**Example Requests:**
```bash
# Full stream (using Podcast Index ID)
curl http://localhost:8080/api/v1/stream/41928435424

# Range request for seeking (bytes 1MB-2MB)
curl -H "Range: bytes=1024000-2048000" http://localhost:8080/api/v1/stream/41928435424

# Test partial content with first 1KB
curl -H "Range: bytes=0-1000" http://localhost:8080/api/v1/stream/41928435424
```

**Rate Limit:** 20 requests/second, burst of 30

### HEAD /api/v1/stream/:id
Get audio metadata without downloading the content body.

**URL Parameters:**
- `id`: Episode ID (Podcast Index ID, numeric int64)

**Response:** Same headers as GET but without body

**Example Request:**
```bash
curl -I http://localhost:8080/api/v1/stream/41928435424
```

### OPTIONS /api/v1/stream/:id
CORS preflight request handler.

**Response Headers:**
- `Access-Control-Allow-Origin`: `*`
- `Access-Control-Allow-Methods`: `GET, HEAD, OPTIONS`
- `Access-Control-Allow-Headers`: `Range, Content-Type`
- `Access-Control-Max-Age`: `86400`

**Status Code:**
- `204`: No Content

## Rate Limiting

### Current Limits

| Endpoint | Requests/Second | Burst |
|----------|-----------------|-------|
| Search | 5 | 10 |
| Episodes | 10 | 20 |
| Stream | 20 | 30 |
| Sync | 1 | 2 |

### Rate Limit Response

When rate limit is exceeded:
```json
{
  "error": "Rate limit exceeded",
  "status": 429
}
```

## Error Handling

### Error Response Format

All error responses follow this structure:

```json
{
  "status": "error",
  "message": "Human-readable error message"
}
```

### Common HTTP Status Codes

| Code | Description |
|------|-------------|
| `200` | Success |
| `204` | No Content (for OPTIONS) |
| `206` | Partial Content (range requests) |
| `400` | Bad Request - Invalid parameters |
| `404` | Not Found - Resource doesn't exist |
| `429` | Too Many Requests - Rate limit exceeded |
| `500` | Internal Server Error |
| `502` | Bad Gateway - External service error |

## CORS Configuration

The API includes CORS headers for web client access:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, HEAD, OPTIONS
Access-Control-Allow-Headers: Content-Type, Range
Access-Control-Expose-Headers: Content-Length, Content-Range, Accept-Ranges
Access-Control-Max-Age: 86400
```

## Authentication

Currently, the API operates without authentication. The Podcast Index API credentials are configured server-side through environment variables.

## Environment Configuration

Server configuration via environment variables (`.env` file):

```bash
# Podcast Index API Credentials
KILLALL_PODCAST_INDEX_API_KEY=your_api_key
KILLALL_PODCAST_INDEX_API_SECRET=your_api_secret
KILLALL_PODCAST_INDEX_API_URL=https://api.podcastindex.org/api/1.0

# Database
KILLALL_DATABASE_PATH=./data/podcast.db

# Server
KILLALL_SERVER_PORT=8080
```

## Example Usage

### Search for Podcasts
```bash
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query": "technology", "limit": 5}'
```

### Sync Episodes for a Podcast
```bash
curl -X POST http://localhost:8080/api/v1/podcasts/217331/episodes/sync?max=10
```

### Get Episode Details
```bash
# Using Podcast Index ID from search results
curl http://localhost:8080/api/v1/episodes/41928435424
```

### Stream Audio
```bash
# Stream full episode using Podcast Index ID
curl http://localhost:8080/api/v1/stream/41928435424 --output episode.mp3

# Stream with range (for seeking)
curl -H "Range: bytes=1024000-" http://localhost:8080/api/v1/stream/41928435424
```

### Update Playback Position
```bash
curl -X PUT http://localhost:8080/api/v1/episodes/41928435424/playback \
  -H "Content-Type: application/json" \
  -d '{"position": 300, "played": false}'
```