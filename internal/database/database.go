package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/killallgit/player-api/internal/models"
	"github.com/killallgit/player-api/pkg/config"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DB struct {
	*gorm.DB
}

// Initialize creates a new database connection with the provided configuration
func Initialize(dbPath string, verbose bool) (*DB, error) {
	// Ensure the database directory exists
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

	// Configure GORM logger
	logLevel := logger.Silent
	if verbose {
		logLevel = logger.Info
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
		NowFunc: func() time.Time {
			return time.Now().UTC()
		},
	}

	// Open database connection
	db, err := gorm.Open(sqlite.Open(dbPath), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL database to configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying SQL database: %w", err)
	}

	// Set connection pool settings
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &DB{DB: db}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL database: %w", err)
	}
	return sqlDB.Close()
}

// HealthCheck verifies the database connection is working
func (db *DB) HealthCheck() error {
	if db == nil || db.DB == nil {
		return fmt.Errorf("database not initialized")
	}

	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL database: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// AutoMigrate runs GORM auto migration for the provided models
func (db *DB) AutoMigrate(models ...any) error {
	if err := db.DB.AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto migration failed: %w", err)
	}
	// Only log migration success if verbose is enabled
	// to avoid confusion with SQL query logs
	return nil
}

// InitializeWithMigrations initializes the database and runs all migrations
// This is the primary entry point for database initialization in the application
func InitializeWithMigrations() (*DB, error) {

	dbPath := config.GetString("database.path")
	if dbPath == "" {
		return nil, fmt.Errorf("database path is not configured")
	}

	// Get verbose logging setting from config
	dbVerbose := config.GetBool("database.verbose")

	// Initialize database connection
	db, err := Initialize(dbPath, dbVerbose)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	// Run auto-migration for all models
	if err := db.AutoMigrate(
		&models.Podcast{},
		&models.Episode{},
		&models.User{},
		&models.Subscription{},
		&models.Waveform{},
		&models.Job{},
		&models.Annotation{},
	); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}
