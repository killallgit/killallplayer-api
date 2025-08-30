# API Specification

## Table of Contents
1. [Overview](#overview)
2. [REST API Endpoints](#rest-api-endpoints)
3. [WebSocket Protocol](#websocket-protocol)
4. [Error Handling](#error-handling)
5. [Authentication](#authentication)
6. [Rate Limiting](#rate-limiting)

## Overview

The Podcast Player API provides both REST endpoints for data transfer and WebSocket connections for real-time communication. The API follows RESTful principles for resource management while WebSocket handles control messages and real-time updates.

### Base URL
```
http://localhost:8080
```

### API Versioning
Currently v1, accessed via:
```
/api/v1/...
```

## REST API Endpoints

### Health Check

#### GET /health
Check if the service is running and healthy.

**Response:**
```json
{
  "status": "healthy",
  "version": "1.0.0",
  "uptime": 3600,
  "timestamp": "2024-01-01T00:00:00Z"
}
```

### Audio Streaming

#### GET /api/v1/stream/{episodeId}
Stream audio content with support for range requests.

**Headers:**
- `Range`: `bytes=start-end` (optional) - For partial content requests

**Response Headers:**
- `Content-Type`: `audio/mpeg` (or appropriate audio type)
- `Content-Length`: Size in bytes
- `Accept-Ranges`: `bytes`
- `Content-Range`: `bytes start-end/total` (for partial requests)

**Status Codes:**
- `200`: Full content
- `206`: Partial content
- `404`: Episode not found
- `416`: Range not satisfiable

**Example Request:**
```http
GET /api/v1/stream/episode-123
Range: bytes=1000-2000
```

### Waveform Data

#### GET /api/v1/episodes/{id}/waveform
Retrieve waveform data for visualization.

**Query Parameters:**
- `resolution`: Number of samples per pixel (256, 512, 1024)
- `format`: Response format (`json` or `binary`)

**Response (JSON):**
```json
{
  "episode_id": "episode-123",
  "duration": 3600,
  "sample_rate": 44100,
  "samples_per_pixel": 256,
  "bits": 8,
  "length": 14062,
  "data": [0.5, 0.7, 0.3, ...],
  "created_at": "2024-01-01T00:00:00Z"
}
```

### Transcription

#### GET /api/v1/episodes/{id}/transcript
Retrieve episode transcription with timestamps.

**Query Parameters:**
- `format`: Response format (`json`, `vtt`, `srt`)
- `start`: Start time in seconds (optional)
- `end`: End time in seconds (optional)

**Response (JSON):**
```json
{
  "episode_id": "episode-123",
  "language": "en",
  "confidence": 0.95,
  "full_text": "Complete transcript text...",
  "segments": [
    {
      "start": 0.0,
      "end": 5.2,
      "text": "Welcome to the podcast."
    }
  ],
  "created_at": "2024-01-01T00:00:00Z"
}
```

### Audio Tags

#### GET /api/v1/episodes/{id}/tags
List all tags for an episode.

**Query Parameters:**
- `start_time`: Filter tags after this time
- `end_time`: Filter tags before this time

**Response:**
```json
{
  "episode_id": "episode-123",
  "tags": [
    {
      "id": "tag-456",
      "start_time": 120.5,
      "end_time": 180.0,
      "label": "Important moment",
      "notes": "Discussion about...",
      "color": "#FF5733",
      "created_at": "2024-01-01T00:00:00Z"
    }
  ]
}
```

#### POST /api/v1/episodes/{id}/tags
Create a new tag.

**Request Body:**
```json
{
  "start_time": 120.5,
  "end_time": 180.0,
  "label": "Important moment",
  "notes": "Optional notes",
  "color": "#FF5733"
}
```

**Response:**
```json
{
  "id": "tag-789",
  "episode_id": "episode-123",
  "start_time": 120.5,
  "end_time": 180.0,
  "label": "Important moment",
  "notes": "Optional notes",
  "color": "#FF5733",
  "created_at": "2024-01-01T00:00:00Z"
}
```

#### PUT /api/v1/tags/{id}
Update an existing tag.

**Request Body:**
```json
{
  "start_time": 125.0,
  "end_time": 185.0,
  "label": "Updated label",
  "notes": "Updated notes",
  "color": "#00FF00"
}
```

#### DELETE /api/v1/tags/{id}
Delete a tag.

**Response:**
```json
{
  "message": "Tag deleted successfully"
}
```

## WebSocket Protocol

### Connection

#### Endpoint
```
ws://localhost:8080/ws
```

### Message Structure

All WebSocket messages follow this structure:

```json
{
  "id": "unique-message-id",
  "type": "message-type",
  "timestamp": "2024-01-01T00:00:00Z",
  "payload": {}
}
```

### Client → Server Messages

#### Search Request
Search for podcasts via Podcast Index.

```json
{
  "type": "search_request",
  "payload": {
    "query": "search terms",
    "limit": 25,
    "offset": 0,
    "category": "technology"
  }
}
```

#### Episode Selection
Select an episode for processing and playback.

```json
{
  "type": "episode_selected",
  "payload": {
    "episode_id": "episode-uuid",
    "podcast_id": "podcast-uuid",
    "force_reprocess": false
  }
}
```

#### Create Tag
Create a new audio tag.

```json
{
  "type": "create_tag",
  "payload": {
    "episode_id": "episode-uuid",
    "start_time": 120.5,
    "end_time": 180.0,
    "label": "Important moment",
    "notes": "Optional notes",
    "color": "#FF5733"
  }
}
```

#### Update Tag
Update an existing tag.

```json
{
  "type": "update_tag",
  "payload": {
    "tag_id": "tag-uuid",
    "start_time": 125.0,
    "end_time": 185.0,
    "label": "Updated label",
    "notes": "Updated notes"
  }
}
```

#### Delete Tag
Delete a tag.

```json
{
  "type": "delete_tag",
  "payload": {
    "tag_id": "tag-uuid"
  }
}
```

#### Get Status
Request processing status for an episode.

```json
{
  "type": "get_status",
  "payload": {
    "episode_id": "episode-uuid"
  }
}
```

#### Heartbeat
Keep connection alive.

```json
{
  "type": "heartbeat",
  "payload": {}
}
```

### Server → Client Messages

#### Search Results
Response to search request.

```json
{
  "type": "search_results",
  "payload": {
    "query": "original search",
    "total": 50,
    "results": [
      {
        "id": "podcast-uuid",
        "title": "Podcast Name",
        "author": "Author Name",
        "description": "Podcast description",
        "image_url": "https://...",
        "categories": ["Technology", "Science"],
        "episode_count": 100,
        "latest_episode": "2024-01-01T00:00:00Z"
      }
    ]
  }
}
```

#### Episode Details
Detailed episode information.

```json
{
  "type": "episode_details",
  "payload": {
    "id": "episode-uuid",
    "podcast_id": "podcast-uuid",
    "title": "Episode Title",
    "description": "Episode description",
    "duration": 3600,
    "pub_date": "2024-01-01T00:00:00Z",
    "audio_url": "https://...",
    "processed": true,
    "has_waveform": true,
    "has_transcript": true,
    "stream_url": "/api/v1/stream/episode-uuid",
    "waveform_url": "/api/v1/episodes/episode-uuid/waveform",
    "transcript_url": "/api/v1/episodes/episode-uuid/transcript"
  }
}
```

#### Processing Started
Notification that processing has begun.

```json
{
  "type": "processing_started",
  "payload": {
    "episode_id": "episode-uuid",
    "jobs": ["metadata", "waveform", "transcription"],
    "estimated_time": 120
  }
}
```

#### Processing Progress
Update on processing progress.

```json
{
  "type": "processing_progress",
  "payload": {
    "episode_id": "episode-uuid",
    "job": "waveform",
    "progress": 45,
    "status": "generating",
    "message": "Processing audio data..."
  }
}
```

#### Processing Complete
Notification that processing is complete.

```json
{
  "type": "processing_complete",
  "payload": {
    "episode_id": "episode-uuid",
    "success": true,
    "metadata": {
      "duration": 3600,
      "bitrate": 128000,
      "sample_rate": 44100
    },
    "waveform_url": "/api/v1/episodes/episode-uuid/waveform",
    "stream_url": "/api/v1/stream/episode-uuid",
    "transcript_available": true,
    "processing_time": 45.2
  }
}
```

#### Processing Failed
Notification of processing failure.

```json
{
  "type": "processing_failed",
  "payload": {
    "episode_id": "episode-uuid",
    "job": "transcription",
    "error": "Whisper API timeout",
    "retry_available": true
  }
}
```

#### Tag Created
Confirmation of tag creation.

```json
{
  "type": "tag_created",
  "payload": {
    "tag": {
      "id": "tag-uuid",
      "episode_id": "episode-uuid",
      "start_time": 120.5,
      "end_time": 180.0,
      "label": "Important moment",
      "notes": "Optional notes",
      "created_at": "2024-01-01T00:00:00Z"
    }
  }
}
```

#### Tag Updated
Confirmation of tag update.

```json
{
  "type": "tag_updated",
  "payload": {
    "tag": {
      "id": "tag-uuid",
      "start_time": 125.0,
      "end_time": 185.0,
      "label": "Updated label",
      "notes": "Updated notes",
      "updated_at": "2024-01-01T00:00:00Z"
    }
  }
}
```

#### Tag Deleted
Confirmation of tag deletion.

```json
{
  "type": "tag_deleted",
  "payload": {
    "tag_id": "tag-uuid"
  }
}
```

#### Error Message
Error notification.

```json
{
  "type": "error",
  "payload": {
    "code": "PROCESSING_FAILED",
    "message": "Failed to generate waveform",
    "details": {
      "episode_id": "episode-uuid",
      "job": "waveform"
    },
    "timestamp": "2024-01-01T00:00:00Z"
  }
}
```

#### Heartbeat Response
Response to heartbeat.

```json
{
  "type": "heartbeat_ack",
  "payload": {
    "server_time": "2024-01-01T00:00:00Z"
  }
}
```

## Error Handling

### Error Response Format

All error responses follow this structure:

```json
{
  "error": {
    "code": "ERROR_CODE",
    "message": "Human-readable error message",
    "details": {},
    "timestamp": "2024-01-01T00:00:00Z"
  }
}
```

### Error Codes

| Code | Description | HTTP Status |
|------|-------------|-------------|
| `INVALID_REQUEST` | Malformed request | 400 |
| `UNAUTHORIZED` | Missing or invalid auth | 401 |
| `FORBIDDEN` | Insufficient permissions | 403 |
| `NOT_FOUND` | Resource not found | 404 |
| `CONFLICT` | Resource conflict | 409 |
| `RATE_LIMITED` | Too many requests | 429 |
| `INTERNAL_ERROR` | Server error | 500 |
| `SERVICE_UNAVAILABLE` | External service down | 503 |

### WebSocket Error Codes

| Code | Description |
|------|-------------|
| `INVALID_MESSAGE` | Malformed message format |
| `UNKNOWN_TYPE` | Unknown message type |
| `PROCESSING_FAILED` | Processing job failed |
| `EPISODE_NOT_FOUND` | Episode doesn't exist |
| `TAG_OVERLAP` | Tag time overlaps existing |
| `QUOTA_EXCEEDED` | Processing quota exceeded |

## Authentication

Currently, the API operates in single-user mode without authentication. Future versions will implement:

- JWT-based authentication
- API key for external access
- Session management for WebSocket

## Rate Limiting

### Current Limits

| Endpoint | Limit | Window |
|----------|-------|--------|
| Search | 60 | 1 minute |
| Stream | 100 | 1 minute |
| Processing | 10 | 1 hour |
| WebSocket Messages | 100 | 1 minute |

### Headers

Rate limit information in response headers:

```
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 45
X-RateLimit-Reset: 1704067200
```

### Exceeded Response

```json
{
  "error": {
    "code": "RATE_LIMITED",
    "message": "Rate limit exceeded",
    "retry_after": 30
  }
}
```

## CORS Configuration

For web client access:

```
Access-Control-Allow-Origin: *
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Headers: Content-Type, Authorization, Range
Access-Control-Expose-Headers: Content-Range, Accept-Ranges
```

## API Versioning Strategy

- Version in URL path: `/api/v1/`
- Breaking changes increment major version
- Deprecation notices via headers
- Minimum 6-month deprecation period