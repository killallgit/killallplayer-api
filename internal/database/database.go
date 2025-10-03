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

func Initialize(dbPath string, verbose bool) (*DB, error) {
	dir := filepath.Dir(dbPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create database directory: %w", err)
		}
	}

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

	db, err := gorm.Open(sqlite.Open(dbPath), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying SQL database: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return &DB{DB: db}, nil
}

func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying SQL database: %w", err)
	}
	return sqlDB.Close()
}

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

func (db *DB) AutoMigrate(models ...any) error {
	if err := db.DB.AutoMigrate(models...); err != nil {
		return fmt.Errorf("auto migration failed: %w", err)
	}
	return nil
}

func InitializeWithMigrations() (*DB, error) {
	dbPath := config.GetString("database.path")
	if dbPath == "" {
		return nil, fmt.Errorf("database path is not configured")
	}

	dbVerbose := config.GetBool("database.verbose")

	db, err := Initialize(dbPath, dbVerbose)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := db.AutoMigrate(
		&models.Podcast{},
		&models.Episode{},
		&models.Subscription{},
		&models.Waveform{},
		&models.Transcription{},
		&models.Job{},
		&models.AudioCache{},
		&models.Dataset{},
		&models.Clip{},
	); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	return db, nil
}
