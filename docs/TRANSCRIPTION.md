# Transcription Feature

The Podcast Player API includes automatic transcription generation for podcast episodes using OpenAI's Whisper model running locally via whisper.cpp.

## Features

- **Automatic Triggering**: Transcriptions are automatically generated when an episode is fetched for the first time
- **Offline Processing**: Uses whisper.cpp for 100% local, private transcription
- **Background Processing**: Transcriptions are generated asynchronously in the background
- **Status Tracking**: Real-time status updates (processing, completed, error)
- **Model Support**: Supports various Whisper models (base, small, medium, large)

## How It Works

1. When an episode is fetched via the API, the system checks if a transcription exists
2. If no transcription is found, a background job is automatically created
3. The worker pool picks up the job and:
   - Downloads the episode audio
   - Processes it with whisper.cpp
   - Stores the transcription in the database
4. Future requests for the same episode include the transcription

## API Response

When fetching an episode, the transcription status is included:

```json
{
  "episode": {
    "id": 42087790098,
    "title": "Episode Title",
    // ... other episode fields
    "transcription": {
      "status": "ok",
      "message": "Transcription ready",
      "data": {
        "text": "This is the full transcription text...",
        "language": "en",
        "duration": 300.5,
        "model": "ggml-base.en.bin"
      }
    }
  }
}
```

## Status Values

- `ok` - Transcription is complete and available
- `processing` - Transcription is being generated
- `downloading` - Audio file is being downloaded
- `error` - Transcription generation failed

## Docker Setup

The Docker image includes whisper.cpp and a base English model pre-configured:

```bash
# Build and run with Docker Compose
docker-compose up -d

# Or build manually
docker build -t podcast-player-api .
docker run -p 8080:8080 podcast-player-api
```

## Configuration

### Environment Variables

```bash
# Enable/disable transcription
KILLALL_TRANSCRIPTION_ENABLED=true

# Path to whisper.cpp binary
KILLALL_TRANSCRIPTION_WHISPER_PATH=/app/bin/main

# Path to Whisper model
KILLALL_TRANSCRIPTION_MODEL_PATH=/app/models/ggml-base.en.bin

# Language code (en, es, fr, etc.)
KILLALL_TRANSCRIPTION_LANGUAGE=en
```

### Using Different Models

The Docker image includes the base English model by default. To use a different model:

1. Download the model:
```bash
# Small model (~39MB, faster)
wget https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-small.en.bin

# Medium model (~1.5GB, more accurate)
wget https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-medium.en.bin

# Large model (~3GB, best accuracy)
wget https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-large.bin
```

2. Mount it in the container:
```yaml
volumes:
  - ./models:/app/models
```

3. Update the environment variable:
```bash
KILLALL_TRANSCRIPTION_MODEL_PATH=/app/models/ggml-medium.en.bin
```

## Local Development

For local development without Docker:

1. Install whisper.cpp:
```bash
git clone https://github.com/ggerganov/whisper.cpp
cd whisper.cpp
make
```

2. Download a model:
```bash
bash ./models/download-ggml-model.sh base.en
```

3. Configure your `.env`:
```env
KILLALL_TRANSCRIPTION_WHISPER_PATH=/path/to/whisper.cpp/main
KILLALL_TRANSCRIPTION_MODEL_PATH=/path/to/whisper.cpp/models/ggml-base.en.bin
```

4. Run the server:
```bash
task serve
```

## Performance Considerations

- **Model Size**: Larger models provide better accuracy but require more memory and processing time
- **Concurrent Jobs**: Limit concurrent transcription jobs based on available CPU/memory
- **Audio Quality**: Better audio quality results in more accurate transcriptions
- **Language**: Using language-specific models improves accuracy

## Troubleshooting

### Transcription stays in "processing" state
- Check worker logs for errors
- Verify whisper.cpp binary is executable
- Ensure model file exists and is readable

### Out of memory errors
- Use a smaller model
- Reduce number of concurrent workers
- Increase container memory limits

### Poor transcription quality
- Try a larger model
- Ensure audio quality is good
- Use language-specific models for non-English content

## API Examples

### Get Episode with Transcription
```bash
curl http://localhost:8080/api/v1/episodes/123
```

### Check Transcription Status
The transcription field in the response will show current status:
- If transcription exists: Full transcription data
- If processing: Status and progress
- If not started: Will automatically trigger generation