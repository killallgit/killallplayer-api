# Services Package

This package contains all business logic and service layers for the Podcast Player API.

## Structure
- `podcast/` - Podcast-related business logic
- `episode/` - Episode management services
- `processing/` - Audio processing services
- `streaming/` - Audio streaming services
- `transcription/` - Transcription services
- `waveform/` - Waveform generation services

## Responsibilities
- Implement business logic
- Orchestrate between different layers
- Handle external API integrations
- Manage processing workflows
- Enforce business rules

## Design Principles
- Services should be stateless
- Dependencies injected through interfaces
- Clear separation of concerns
- Testable with mocked dependencies