# Models Package

This package contains all data models and domain entities for the Podcast Player API.

## Structure
- Database models with GORM tags
- Domain entities
- Data transfer objects (DTOs)
- Model validations

## Models
- `Podcast` - Podcast information
- `Episode` - Episode details and metadata
- `AudioTag` - User-created audio tags
- `ProcessingJob` - Audio processing job queue
- `Waveform` - Waveform data for episodes
- `Transcription` - Episode transcriptions

## Responsibilities
- Define database schema through GORM models
- Implement model validations
- Handle model relationships
- Provide data transformation methods