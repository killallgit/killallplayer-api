# Podcast Player API Documentation

Base URL: `http://localhost:8080`

## Health Check

### GET /health
Returns server health status.

**Response:**
```json
{
  "status": "ok",
  "timestamp": "2025-09-01T15:00:00Z",
  "database": {"status": "healthy"}
}
```

## Search

### POST /api/v1/search
Search for podcasts.

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

## Episodes

### GET /api/v1/episodes
Get all episodes (recent episodes across all podcasts).

**Query Parameters:**
- `limit` (optional): Max episodes to return (1-1000, default: 50)

**Response:**
```json
{
  "status": "true",
  "items": [...],  // Array of episodes
  "count": 50,
  "description": "All episodes"
}
```

### GET /api/v1/episodes/recent
Get recent episodes across all podcasts.

**Query Parameters:**
- `max` (optional): Max episodes (1-100, default: 20)

**Response:** Same as above

### GET /api/v1/episodes/byfeedid
Get episodes by podcast/feed ID.

**Query Parameters:**
- `id` (required): Podcast ID
- `max` (optional): Max episodes (1-1000, default: 20)

**Response:** Same as episodes with `query` field containing podcast ID

### GET /api/v1/episodes/byguid
Get single episode by GUID.

**Query Parameters:**
- `guid` (required): Episode GUID

**Response:** Single episode in Podcast Index format

### GET /api/v1/episodes/:id
Get single episode by ID.

**Response:** Single episode

### PUT /api/v1/episodes/:id/playback
Update episode playback state.

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
    "episode_id": 123,
    "position": 120,
    "played": true
  }
}
```

## Podcasts

### GET /api/v1/podcasts/:id/episodes
Get episodes for specific podcast.

**Query Parameters:**
- `max` (optional): Max episodes (1-1000, default: 20)

**Response:** Same as episodes list

### POST /api/v1/podcasts/:id/episodes/sync
Sync episodes from Podcast Index API.

**Query Parameters:**
- `max` (optional): Max episodes to sync (1-1000, default: 50)

**Response:** Updated episodes list

## Rate Limits

- Search: 5 requests/second, burst of 10
- Episodes/Podcasts: 10 requests/second, burst of 20  
- Sync: 1 request/second, burst of 2

## Error Responses

All errors return:
```json
{
  "status": "error",
  "message": "Error description"
}
```

Common HTTP status codes:
- 200: Success
- 400: Bad Request
- 404: Not Found
- 429: Rate Limit Exceeded
- 500: Internal Server Error