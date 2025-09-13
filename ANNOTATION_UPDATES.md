# Annotation Updates & Dataset Traceability

## Overview

This document outlines the approach for adding UUID-based identifiers to annotations for stable references in ML training datasets.

## Current State

- Annotations use auto-increment IDs (not stable across systems)
- No stable identifiers for tracking annotations in exported datasets
- IDs can differ between development, staging, and production databases

## Proposed Solution: UUID-based Annotations

### Core Principles

1. **Stable Identifiers**: UUIDs provide consistent references across all systems
2. **Dataset Traceability**: Track exactly which annotation was used in each dataset
3. **Immutable Datasets**: Generated datasets are snapshots with stable annotation references

## Implementation Approach

#### Database Schema Changes

```sql
-- Add UUID column to existing annotations table
ALTER TABLE annotations 
  ADD COLUMN uuid VARCHAR(36) NOT NULL DEFAULT (lower(hex(randomblob(16)))),
  ADD UNIQUE INDEX idx_uuid (uuid);

-- Populate UUIDs for existing records
UPDATE annotations SET uuid = lower(hex(randomblob(16))) WHERE uuid IS NULL;
```

#### Updated Annotation Model

```go
type Annotation struct {
    gorm.Model
    UUID      string  `json:"uuid" gorm:"uniqueIndex;not null"`
    EpisodeID uint    `json:"episode_id" gorm:"not null;index"`
    Label     string  `json:"label" gorm:"not null"`
    StartTime float64 `json:"start_time" gorm:"not null"`
    EndTime   float64 `json:"end_time" gorm:"not null"`
    
    // Relationship
    Episode Episode `json:"episode,omitempty" gorm:"foreignKey:EpisodeID"`
}
```

#### Dataset Entry Structure

```json
{
  "audio_path": "./clips/clip_001.mp3",
  "annotation_uuid": "a3f4d5e6-1234-5678-9abc-def012345678",
  "label": "advertisement",
  "duration": 29.8,
  "episode_id": 123,
  "original_stream_url": "https://podcast.com/episode123.mp3",
  "original_start_time": 45.5,
  "original_end_time": 75.3,
  "exported_at": "2024-01-15T10:30:00Z"
}
```


## Workflow Examples

### Creating an Annotation

1. Generate UUID on creation
2. Store in database with all annotation data
3. Return annotation with UUID in response

### Updating an Annotation

- Update the annotation record fields (label, start_time, end_time)
- UUID remains the same for stable reference
- Datasets maintain their reference to the original annotation UUID

### Generating a Dataset

1. Collect annotations based on filters
2. For each annotation:
   - Extract audio clip
   - Create dataset entry with `annotation_uuid`
   - Store original timestamps and metadata
3. Save dataset with full traceability to source annotations

## Benefits

1. **Stable References**: UUIDs provide stable identifiers across database migrations
2. **Dataset Reproducibility**: Can trace exactly what annotations were used for training
3. **Cross-system Compatibility**: Same UUID works across dev, staging, and production
4. **Simple Implementation**: No complex versioning or history tracking needed

## Migration Strategy

### Implementation Steps
- Add UUID column to annotations table
- Generate UUIDs for any existing records
- Update API to return UUIDs in responses
- Include annotation_uuid in dataset exports

### UI Integration
- Show annotation UUID in UI for reference
- Use UUID for dataset traceability

## API Endpoint Changes

### Endpoints (No Breaking Changes)
- `POST /api/v1/episodes/{id}/annotations` - Create annotation (generates UUID automatically)
- `GET /api/v1/episodes/{id}/annotations` - List annotations (includes UUIDs in response)
- `PUT /api/v1/episodes/annotations/{id}` - Update annotation (preserves UUID)
- `DELETE /api/v1/episodes/annotations/{id}` - Delete annotation

## Implementation Notes

- UUIDs are generated server-side for consistency
- Use UUID v4 for randomness
- Dataset immutability is crucial for reproducible ML training
- SQLite-compatible UUID generation using hex(randomblob(16))