# Clip System - Client Flow Guide

## Quick Start
The clip system allows clients to extract, label, and export audio segments for ML training. All operations are asynchronous and return immediately with a status field.

## Basic Flow

### 1. Create a Clip
```javascript
POST /api/v1/clips
{
  "source_episode_url": "https://example.com/episode.mp3",
  "start_time": 30,    // seconds
  "end_time": 45,      // seconds
  "label": "advertisement"
}

Response (202 Accepted):
{
  "uuid": "052f3b9b-cc02-418c-a9ab-8f49534c01c8",
  "status": "processing",  // Will change to "ready" or "failed"
  "label": "advertisement",
  ...
}
```

### 2. Check Processing Status
```javascript
GET /api/v1/clips/052f3b9b-cc02-418c-a9ab-8f49534c01c8

Response:
{
  "uuid": "052f3b9b-cc02-418c-a9ab-8f49534c01c8",
  "status": "ready",       // "processing", "ready", or "failed"
  "duration": 15,          // Always 15 seconds when ready
  "size_bytes": 480078,    // ~480KB for 15s clip
  "error_message": null,   // Contains error if status="failed"
  ...
}
```

### 3. List and Filter Clips
```javascript
GET /api/v1/clips?label=advertisement&status=ready&limit=10

Response: Array of clip objects
```

### 4. Update Labels
```javascript
PUT /api/v1/clips/052f3b9b-cc02-418c-a9ab-8f49534c01c8/label
{
  "label": "music"
}
```

### 5. Export Dataset
```javascript
GET /api/v1/clips/export

Response: ZIP file download containing:
- Audio files organized by label directories
- manifest.jsonl with metadata for each clip
```

## UI Components Needed

### Clip Creator
- Time range selector (start/end in seconds)
- Label input/dropdown (common: "advertisement", "music", "speech", "silence")
- Source URL/episode selector
- Submit button

### Clip Status Monitor
- Show "processing" state with spinner
- Poll every 2-3 seconds until status changes
- Display error messages if failed
- Show success when ready

### Clip Manager
- Table/list view with filters
  - Filter by label (dropdown)
  - Filter by status (processing/ready/failed)
- Actions per clip:
  - View details
  - Edit label
  - Delete
- Bulk export button

### Processing States
```javascript
// Recommended polling logic
async function waitForClip(uuid) {
  const maxAttempts = 60;  // 3 minutes max
  let attempts = 0;

  while (attempts < maxAttempts) {
    const response = await fetch(`/api/v1/clips/${uuid}`);
    const clip = await response.json();

    if (clip.status === 'ready') {
      return { success: true, clip };
    }
    if (clip.status === 'failed') {
      return { success: false, error: clip.error_message };
    }

    await sleep(3000);  // Wait 3 seconds
    attempts++;
  }

  return { success: false, error: 'Timeout' };
}
```

## Important Notes

1. **No Trailing Slashes**: The API does not require trailing slashes on endpoints
2. **Async Processing**: All clips start with status="processing"
3. **Fixed Duration**: All clips are normalized to exactly 15 seconds
4. **Audio Format**: 16kHz mono WAV (optimal for ML models)
5. **Error Handling**: Check error_message field when status="failed"
6. **CDN Issues**: Some podcast URLs may fail due to hotlink protection (403 errors)

## Label Suggestions
Common labels for ML training:
- `advertisement` - Ads and sponsors
- `music` - Music segments
- `speech` - Spoken content
- `silence` - Quiet periods
- `applause` - Audience reactions
- `laughter` - Comedy segments
- Custom labels welcome

## Error Scenarios

### Common Errors
- **403 Forbidden**: Source URL blocks direct download (CDN protection)
- **Invalid Range**: end_time <= start_time
- **404 Not Found**: Clip UUID doesn't exist
- **Processing Failed**: Check error_message for details

### Handling Failed Clips
```javascript
if (clip.status === 'failed') {
  if (clip.error_message.includes('403')) {
    showError('This podcast provider blocks direct downloads');
  } else {
    showError(clip.error_message);
  }
}
```

## Complete Example Flow
```javascript
// 1. Create clip
const createResponse = await fetch('/api/v1/clips', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    source_episode_url: episodeUrl,
    start_time: 30,
    end_time: 45,
    label: 'advertisement'
  })
});

const { uuid } = await createResponse.json();

// 2. Wait for processing
const result = await waitForClip(uuid);

// 3. Handle result
if (result.success) {
  showSuccess('Clip ready!');
  refreshClipList();
} else {
  showError(result.error);
}
```