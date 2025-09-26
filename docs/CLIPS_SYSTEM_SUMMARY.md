# Clip System - ML Training Data Pipeline

## ✅ System Status: FULLY OPERATIONAL

All tests passing - the system is ready for production use.

## Overview

The new clip system provides a streamlined pipeline for extracting labeled audio segments from podcast episodes for machine learning training. It replaces the complex annotation/dataset system with a simpler, more direct approach.

## Key Features

### Audio Processing
- **Format**: 16kHz mono WAV (optimal for Whisper/Wav2Vec2)
- **Duration**: Exactly 15 seconds (padded or cropped automatically)
- **Extraction**: FFmpeg-based with async processing
- **Storage**: Organized by label directories

### API Endpoints
All endpoints work WITHOUT trailing slashes (client-friendly):

- `POST /api/v1/clips` - Create new clip
- `GET /api/v1/clips` - List clips (with filters)
- `GET /api/v1/clips/{uuid}` - Get specific clip
- `PUT /api/v1/clips/{uuid}/label` - Update label
- `DELETE /api/v1/clips/{uuid}` - Delete clip
- `GET /api/v1/clips/export` - Export dataset as ZIP

### Label System
- **Flexible**: Any string allowed as label
- **Organized**: Clips stored in `clips/{label}/` directories
- **Portable**: Labels preserved in export

### Dataset Export
- **Format**: ZIP archive with:
  - Audio files organized by label directories
  - JSONL manifest with metadata
- **Manifest Fields**:
  - `file_path`: Relative path to audio file
  - `label`: Training label
  - `duration`: Clip duration (always 15s)
  - `source_url`: Original episode URL
  - `original_start_time/end_time`: Source timestamps
  - `uuid`: Unique identifier
  - `created_at`: Timestamp

## Testing

Two test scripts are available:

1. **Quick Test**: `./test-clips.sh`
   - Basic CRUD operations
   - Label filtering
   - Export verification

2. **Comprehensive Test**: `./test-clips-comprehensive.sh`
   - 14 automated tests
   - Audio format verification
   - Full workflow testing
   - All tests currently passing ✅

## Directory Structure

```
clips/
├── advertisement/
│   └── clip_xxx.wav
├── music/
│   └── clip_yyy.wav
├── speech/
│   └── clip_zzz.wav
└── [other_labels]/
```

## Technical Details

### Processing Flow
1. Client sends clip request with source URL and time range
2. Server creates database record with status "processing"
3. Async worker:
   - Downloads audio (if URL) or reads local file
   - Extracts segment using FFmpeg
   - Converts to 16kHz mono WAV
   - Pads/crops to 15 seconds
   - Saves to storage
   - Updates database status to "ready"

### Error Handling
- Failed downloads (403 errors from CDNs)
- Invalid time ranges
- Processing failures
- All errors stored in database with messages

### Performance
- Async processing (non-blocking API)
- Typical processing time: 2-5 seconds
- Storage: ~480KB per 15-second clip

## Migration from Old System

### Removed Components
- ❌ `api/annotations/*` - Annotation handlers
- ❌ `internal/services/annotations/*` - Annotation service
- ❌ `internal/services/dataset/*` - Dataset service
- ❌ `internal/models/annotation.go` - Annotation model

### New Components
- ✅ `api/clips/*` - Clip handlers
- ✅ `internal/services/clips/*` - Clip service
- ✅ `internal/models/clip.go` - Clip model

## Usage Example

```bash
# Create a clip
curl -X POST http://localhost:9000/api/v1/clips \
  -H "Content-Type: application/json" \
  -d '{
    "source_episode_url": "https://example.com/episode.mp3",
    "start_time": 30,
    "end_time": 45,
    "label": "advertisement"
  }'

# Check status
curl http://localhost:9000/api/v1/clips/{uuid}

# Export dataset
curl -o dataset.zip http://localhost:9000/api/v1/clips/export
```

## ML Training Compatibility

The system produces clips compatible with:
- **OpenAI Whisper**: 16kHz required ✅
- **Facebook Wav2Vec2**: 16kHz required ✅
- **Google Speech-to-Text**: 16kHz supported ✅
- **Custom PyTorch models**: Standard WAV format ✅

## Next Steps

The system is production-ready. Potential enhancements:
- Cloud storage support (S3/GCS) - interface already prepared
- Batch processing API
- Progress webhooks
- Multiple format export options