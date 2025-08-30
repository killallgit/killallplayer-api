# Database Schema Design

## Overview

The Podcast Player API uses SQLite as its primary database, chosen for its simplicity, zero operational overhead, and sufficient performance for single-user scenarios. The schema is designed to balance normalization with practical performance considerations.

## Database Configuration

### SQLite Settings
```sql
PRAGMA foreign_keys = ON;
PRAGMA journal_mode = WAL;
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = -64000;  -- 64MB cache
PRAGMA temp_store = MEMORY;
```

### Connection Pool Settings
- Max connections: 10
- Max idle connections: 5
- Connection max lifetime: 30 minutes

## Core Tables

### podcasts
Stores podcast metadata from Podcast Index API.

```sql
CREATE TABLE podcasts (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    author TEXT,
    description TEXT,
    feed_url TEXT,
    image_url TEXT,
    categories TEXT,        -- JSON array ["Technology", "Science"]
    language TEXT,
    website TEXT,
    explicit BOOLEAN DEFAULT FALSE,
    episode_count INTEGER DEFAULT 0,
    metadata TEXT,          -- JSON for additional/future fields
    cached_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_podcasts_title ON podcasts(title);
CREATE INDEX idx_podcasts_cached_at ON podcasts(cached_at);
```

### episodes
Stores individual episode information.

```sql
CREATE TABLE episodes (
    id TEXT PRIMARY KEY,
    podcast_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    audio_url TEXT NOT NULL,
    duration INTEGER,           -- Duration in seconds
    file_size INTEGER,          -- Size in bytes
    pub_date TIMESTAMP,
    episode_number INTEGER,
    season_number INTEGER,
    guid TEXT,                  -- RSS GUID
    explicit BOOLEAN DEFAULT FALSE,
    processed BOOLEAN DEFAULT FALSE,
    processing_error TEXT,      -- Error message if processing failed
    metadata TEXT,              -- JSON for additional fields
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (podcast_id) REFERENCES podcasts(id) ON DELETE CASCADE
);

CREATE INDEX idx_episodes_podcast_id ON episodes(podcast_id);
CREATE INDEX idx_episodes_processed ON episodes(processed);
CREATE INDEX idx_episodes_pub_date ON episodes(pub_date DESC);
CREATE UNIQUE INDEX idx_episodes_guid ON episodes(guid);
```

### audio_tags
Stores user-created time-based annotations.

```sql
CREATE TABLE audio_tags (
    id TEXT PRIMARY KEY,
    episode_id TEXT NOT NULL,
    start_time REAL NOT NULL CHECK (start_time >= 0),
    end_time REAL NOT NULL CHECK (end_time > start_time),
    label TEXT NOT NULL,
    notes TEXT,
    color TEXT DEFAULT '#FF5733',  -- Hex color for UI
    category TEXT,                  -- Optional categorization
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (episode_id) REFERENCES episodes(id) ON DELETE CASCADE,
    CHECK (end_time <= (SELECT duration FROM episodes WHERE id = episode_id))
);

CREATE INDEX idx_tags_episode_id ON audio_tags(episode_id);
CREATE INDEX idx_tags_start_time ON audio_tags(episode_id, start_time);
CREATE INDEX idx_tags_time_range ON audio_tags(episode_id, start_time, end_time);
```

### processing_jobs
Tracks async processing job status.

```sql
CREATE TABLE processing_jobs (
    id TEXT PRIMARY KEY,
    episode_id TEXT NOT NULL,
    job_type TEXT NOT NULL CHECK (job_type IN ('metadata', 'waveform', 'transcription')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    priority INTEGER DEFAULT 0,     -- Higher priority processed first
    progress INTEGER DEFAULT 0 CHECK (progress >= 0 AND progress <= 100),
    result TEXT,                    -- JSON result data
    error TEXT,                     -- Error message if failed
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 3,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (episode_id) REFERENCES episodes(id) ON DELETE CASCADE
);

CREATE INDEX idx_jobs_episode_id ON processing_jobs(episode_id);
CREATE INDEX idx_jobs_status ON processing_jobs(status);
CREATE INDEX idx_jobs_priority_status ON processing_jobs(status, priority DESC, created_at);
CREATE UNIQUE INDEX idx_jobs_episode_type ON processing_jobs(episode_id, job_type);
```

### waveforms
Stores generated waveform data.

```sql
CREATE TABLE waveforms (
    id TEXT PRIMARY KEY,
    episode_id TEXT NOT NULL UNIQUE,
    data TEXT NOT NULL,             -- JSON array of peak values
    data_url TEXT,                  -- Optional: URL if stored externally
    sample_rate INTEGER,
    samples_per_pixel INTEGER,
    bits INTEGER DEFAULT 8,
    length INTEGER,                 -- Number of samples
    version INTEGER DEFAULT 1,      -- Schema version for migrations
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (episode_id) REFERENCES episodes(id) ON DELETE CASCADE
);

CREATE INDEX idx_waveforms_episode_id ON waveforms(episode_id);
```

### transcriptions
Stores Whisper API transcription results.

```sql
CREATE TABLE transcriptions (
    id TEXT PRIMARY KEY,
    episode_id TEXT NOT NULL UNIQUE,
    full_text TEXT NOT NULL,
    segments TEXT NOT NULL,         -- JSON array with timestamps
    language TEXT,
    confidence REAL CHECK (confidence >= 0 AND confidence <= 1),
    model_version TEXT,
    word_count INTEGER,
    processing_time REAL,           -- Time taken to transcribe
    cost REAL,                      -- API cost in dollars
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (episode_id) REFERENCES episodes(id) ON DELETE CASCADE
);

CREATE INDEX idx_transcriptions_episode_id ON transcriptions(episode_id);
CREATE INDEX idx_transcriptions_language ON transcriptions(language);
-- Full-text search index
CREATE VIRTUAL TABLE transcriptions_fts USING fts5(
    episode_id,
    full_text,
    content=transcriptions,
    content_rowid=rowid
);
```

### api_cache
Generic cache for external API responses.

```sql
CREATE TABLE api_cache (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    value_type TEXT DEFAULT 'json',  -- json, text, binary
    size INTEGER,                    -- Size in bytes
    hits INTEGER DEFAULT 0,          -- Cache hit counter
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_accessed TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_cache_expires ON api_cache(expires_at);
CREATE INDEX idx_cache_accessed ON api_cache(last_accessed);
```

### system_settings
Stores system configuration and state.

```sql
CREATE TABLE system_settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    value_type TEXT DEFAULT 'string',  -- string, integer, boolean, json
    description TEXT,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Insert default settings
INSERT INTO system_settings (key, value, description) VALUES
    ('schema_version', '1', 'Database schema version'),
    ('processing_enabled', 'true', 'Enable/disable processing jobs'),
    ('max_concurrent_jobs', '2', 'Maximum concurrent processing jobs'),
    ('cache_ttl_hours', '24', 'Default cache TTL in hours');
```

## Relationship Diagram

```
┌──────────────┐
│   podcasts   │
└──────┬───────┘
       │ 1:N
       │
┌──────▼───────┐         ┌──────────────┐
│   episodes   │◀────────│ audio_tags   │
└──────┬───────┘   1:N   └──────────────┘
       │
       │ 1:1              ┌──────────────┐
       ├─────────────────▶│  waveforms   │
       │                  └──────────────┘
       │
       │ 1:1              ┌──────────────┐
       ├─────────────────▶│transcriptions│
       │                  └──────────────┘
       │
       │ 1:N              ┌──────────────┐
       └─────────────────▶│processing_jobs│
                          └──────────────┘
```

## Migration Strategy

### Version Management
```sql
CREATE TABLE schema_migrations (
    version INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

### Migration Files
```
migrations/
├── 001_initial_schema.sql
├── 002_add_transcriptions.sql
├── 003_add_full_text_search.sql
└── ...
```

### GORM Auto-Migration (Development)
```go
type Episode struct {
    ID          string `gorm:"primaryKey"`
    PodcastID   string `gorm:"not null;index"`
    Title       string `gorm:"not null"`
    // ... other fields
}

// Auto-migrate in development
db.AutoMigrate(&Podcast{}, &Episode{}, &AudioTag{}, ...)
```

## Optimization Strategies

### Index Strategy
1. **Primary lookups**: ID-based access (automatic with PRIMARY KEY)
2. **Foreign key lookups**: All foreign keys are indexed
3. **Query patterns**: Indexes on commonly filtered columns
4. **Composite indexes**: For multi-column queries
5. **Full-text search**: FTS5 for transcript search

### Query Optimization
```sql
-- Efficient episode listing with processing status
SELECT e.*, 
       EXISTS(SELECT 1 FROM waveforms w WHERE w.episode_id = e.id) as has_waveform,
       EXISTS(SELECT 1 FROM transcriptions t WHERE t.episode_id = e.id) as has_transcript
FROM episodes e
WHERE e.podcast_id = ?
ORDER BY e.pub_date DESC
LIMIT 20;

-- Get episode with all related data (single query)
SELECT e.*, 
       w.data as waveform_data,
       t.full_text as transcript
FROM episodes e
LEFT JOIN waveforms w ON w.episode_id = e.id
LEFT JOIN transcriptions t ON t.episode_id = e.id
WHERE e.id = ?;
```

### Performance Tuning
1. **WAL Mode**: Enables concurrent reads during writes
2. **Prepared Statements**: Reduces parsing overhead
3. **Batch Operations**: Group inserts/updates in transactions
4. **Vacuum Schedule**: Regular maintenance for optimal performance
5. **Connection Pooling**: Reuse connections to reduce overhead

## Data Integrity

### Constraints
1. **Foreign Keys**: Cascade deletes maintain referential integrity
2. **Check Constraints**: Validate data at database level
3. **Unique Constraints**: Prevent duplicate entries
4. **NOT NULL**: Required fields enforced

### Triggers
```sql
-- Auto-update timestamps
CREATE TRIGGER update_episodes_timestamp 
AFTER UPDATE ON episodes
BEGIN
    UPDATE episodes SET updated_at = CURRENT_TIMESTAMP 
    WHERE id = NEW.id;
END;

-- Validate tag time ranges
CREATE TRIGGER validate_tag_overlap
BEFORE INSERT ON audio_tags
BEGIN
    SELECT CASE
        WHEN EXISTS (
            SELECT 1 FROM audio_tags 
            WHERE episode_id = NEW.episode_id
            AND id != NEW.id
            AND ((NEW.start_time BETWEEN start_time AND end_time)
                OR (NEW.end_time BETWEEN start_time AND end_time)
                OR (start_time BETWEEN NEW.start_time AND NEW.end_time))
        )
        THEN RAISE(ABORT, 'Tag time range overlaps with existing tag')
    END;
END;
```

## Backup Strategy

### Automatic Backups
```bash
# Daily backup with rotation
sqlite3 podcast.db ".backup backup/podcast_$(date +%Y%m%d).db"

# Keep last 7 days
find backup/ -name "podcast_*.db" -mtime +7 -delete
```

### Point-in-Time Recovery
```sql
-- Enable backup API in application
PRAGMA wal_checkpoint(TRUNCATE);
-- Copy database file while app is running
```

## Monitoring Queries

### Database Health
```sql
-- Database size
SELECT page_count * page_size / 1024 / 1024 as size_mb 
FROM pragma_page_count(), pragma_page_size();

-- Table sizes
SELECT name, 
       SUM(pgsize) / 1024 / 1024 as size_mb
FROM dbstat
GROUP BY name
ORDER BY size_mb DESC;

-- Cache performance
SELECT key, hits, 
       julianday(expires_at) - julianday('now') as ttl_days
FROM api_cache
ORDER BY hits DESC
LIMIT 10;

-- Processing queue status
SELECT job_type, status, COUNT(*) as count
FROM processing_jobs
GROUP BY job_type, status;
```

## Future Considerations

### Multi-User Support
When scaling to multiple users:
1. Add `users` table with authentication
2. Add `user_id` to `audio_tags` table
3. Implement row-level security
4. Consider PostgreSQL migration

### Performance Scaling
If performance becomes an issue:
1. Migrate to PostgreSQL for better concurrency
2. Implement Redis for caching layer
3. Use dedicated search engine (Elasticsearch)
4. Partition large tables by date

### Data Archival
For long-term storage:
1. Archive old episodes to cold storage
2. Implement data retention policies
3. Compress rarely accessed waveforms
4. Move transcripts to full-text search engine