# Annotation Updates & Dataset Traceability

## Overview

This document outlines the approach for handling annotation updates and maintaining traceability between annotations and generated ML training datasets.

## Current State

- Annotations use auto-increment IDs (not stable across systems)
- No versioning on annotation edits
- No way to track which annotation version was used in a dataset
- Cannot determine if a dataset is "stale" after annotations are edited

## Proposed Solution: UUID-based Annotations with Versioning

### Core Principles

1. **Immutable Datasets**: Generated datasets are snapshots and never auto-update
2. **Full Traceability**: Track exactly which annotation version was used in each dataset
3. **Change Detection**: Identify which datasets need regeneration after annotation updates
4. **Audit Trail**: Maintain history of annotation changes over time

## Implementation Approach

### Phase 1: Add UUIDs to Annotations (MVP)

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

### Phase 2: Add Version Tracking (Future Enhancement)

#### Enhanced Database Schema

```sql
-- Add versioning to annotations
ALTER TABLE annotations 
  ADD COLUMN version INTEGER DEFAULT 1,
  ADD COLUMN updated_by VARCHAR(255);

-- Create annotation history table
CREATE TABLE annotation_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  annotation_uuid VARCHAR(36) NOT NULL,
  version INTEGER NOT NULL,
  label VARCHAR(255),
  start_time REAL,
  end_time REAL,
  episode_id INTEGER,
  changed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
  changed_by VARCHAR(255),
  change_reason TEXT,
  UNIQUE(annotation_uuid, version),
  INDEX idx_uuid_version (annotation_uuid, version)
);
```

#### Versioned Annotation Model

```go
type Annotation struct {
    gorm.Model
    UUID      string  `json:"uuid" gorm:"uniqueIndex;not null"`
    Version   int     `json:"version" gorm:"default:1"`
    EpisodeID uint    `json:"episode_id" gorm:"not null;index"`
    Label     string  `json:"label" gorm:"not null"`
    StartTime float64 `json:"start_time" gorm:"not null"`
    EndTime   float64 `json:"end_time" gorm:"not null"`
    UpdatedBy string  `json:"updated_by,omitempty"`
    
    // Relationship
    Episode Episode `json:"episode,omitempty" gorm:"foreignKey:EpisodeID"`
}

type AnnotationHistory struct {
    ID             uint      `gorm:"primaryKey"`
    AnnotationUUID string    `gorm:"not null;index"`
    Version        int       `gorm:"not null"`
    Label          string    
    StartTime      float64   
    EndTime        float64   
    EpisodeID      uint      
    ChangedAt      time.Time 
    ChangedBy      string    
    ChangeReason   string    
}
```

#### Dataset Entry with Version Tracking

```json
{
  "audio_path": "./clips/clip_001.mp3",
  "annotation_uuid": "a3f4d5e6-1234-5678-9abc-def012345678",
  "annotation_version": 2,
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
2. Set version to 1
3. Store in database

### Updating an Annotation

**MVP Approach:**
- Simply update the annotation record
- UUID remains the same
- Datasets can track back to the annotation (though won't know if it changed)

**Full Version Approach:**
1. Load current annotation
2. Save current state to `annotation_history` table
3. Increment version number
4. Update annotation with new values
5. Set `updated_by` field

### Generating a Dataset

1. Collect annotations based on filters
2. For each annotation:
   - Extract audio clip
   - Create dataset entry with `annotation_uuid`
   - Include `annotation_version` (if versioning implemented)
   - Store original timestamps and metadata
3. Save dataset with full traceability

### Checking Dataset Staleness (Future)

```go
func CheckDatasetStaleness(datasetID string) []StaleAnnotation {
    dataset := LoadDataset(datasetID)
    stale := []StaleAnnotation{}
    
    for entry := range dataset.Entries {
        current := GetAnnotation(entry.AnnotationUUID)
        if current.Version > entry.AnnotationVersion {
            stale = append(stale, StaleAnnotation{
                UUID: entry.AnnotationUUID,
                DatasetVersion: entry.AnnotationVersion,
                CurrentVersion: current.Version,
            })
        }
    }
    return stale
}
```

## Benefits

1. **Stable References**: UUIDs provide stable identifiers across database migrations
2. **Dataset Reproducibility**: Can trace exactly what data was used for training
3. **Change Management**: Know when datasets need regeneration
4. **Audit Compliance**: Full history of what changed and when
5. **Multi-user Support**: Track who made changes (with authentication)

## Migration Strategy

### Step 1: MVP Implementation
- Add UUID column to annotations table
- Generate UUIDs for existing records
- Update API to return UUIDs
- Include annotation_uuid in dataset exports

### Step 2: Version Tracking (Optional)
- Add version column
- Create history table
- Update annotation service to track versions
- Add staleness detection API

### Step 3: UI Integration
- Show annotation UUID in UI
- Display version information
- Highlight stale datasets
- Provide regeneration suggestions

## API Endpoint Changes

### Current Endpoints (No Changes for MVP)
- `POST /api/v1/episodes/{id}/annotations` - Create annotation (generates UUID)
- `GET /api/v1/episodes/{id}/annotations` - List annotations (includes UUIDs)
- `PUT /api/v1/episodes/annotations/{id}` - Update annotation (preserves UUID)
- `DELETE /api/v1/episodes/annotations/{id}` - Delete annotation

### Future Endpoints
- `GET /api/v1/annotations/{uuid}/history` - Get annotation version history
- `GET /api/v1/datasets/{id}/staleness` - Check dataset staleness
- `POST /api/v1/datasets/{id}/regenerate` - Regenerate dataset with latest annotations

## Implementation Priority

1. **High Priority (MVP)**:
   - Add UUID to annotation model
   - Include UUID in dataset exports
   - Basic traceability

2. **Medium Priority**:
   - Version tracking
   - History table
   - Update tracking

3. **Low Priority**:
   - Staleness detection
   - Automated regeneration suggestions
   - Diff visualization

## Notes

- UUIDs should be generated server-side for consistency
- Consider using UUID v4 for randomness
- Version numbers should be monotonically increasing
- History table can grow large - consider retention policies
- Dataset immutability is crucial for reproducible ML training