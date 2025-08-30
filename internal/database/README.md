# Database Package

This package provides the database access layer for the Podcast Player API.

## Structure
- `connection.go` - Database connection management
- `migrations/` - Database migration files
- `repositories/` - Repository pattern implementations
- `queries/` - Complex query builders

## Responsibilities
- Manage database connections
- Implement repository pattern
- Handle database transactions
- Execute migrations
- Provide query builders

## Database
- SQLite for development and single-user deployment
- GORM as the ORM
- Support for connection pooling
- WAL mode enabled for better concurrency

## Usage
The database package is initialized at application startup and provides repository interfaces to the service layer.