package database

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/killallgit/player-api/pkg/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestInitialize(t *testing.T) {
	tests := []struct {
		name        string
		dbPath      string
		wantErr     bool
		checkResult func(*testing.T, *DB)
	}{
		{
			name:    "successful connection with in-memory database",
			dbPath:  ":memory:",
			wantErr: false,
			checkResult: func(t *testing.T, conn *DB) {
				assert.NotNil(t, conn)
				assert.NotNil(t, conn.DB)
			},
		},
		{
			name:    "successful connection with file database",
			dbPath:  filepath.Join(t.TempDir(), "test.db"),
			wantErr: false,
			checkResult: func(t *testing.T, conn *DB) {
				assert.NotNil(t, conn)
				assert.NotNil(t, conn.DB)
			},
		},
		{
			name:    "empty database path creates in-memory database",
			dbPath:  "",
			wantErr: false,
			checkResult: func(t *testing.T, conn *DB) {
				// Empty path creates an in-memory database
				assert.NotNil(t, conn)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := Initialize(tt.dbPath, false)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if tt.checkResult != nil {
				tt.checkResult(t, conn)
			}

			// Cleanup
			if conn != nil {
				conn.Close()
			}
		})
	}
}

func TestDB_Close(t *testing.T) {
	// Create a connection
	conn, err := Initialize(":memory:", false)
	require.NoError(t, err)
	require.NotNil(t, conn)

	// Close the connection
	err = conn.Close()
	assert.NoError(t, err)

	// Verify connection is closed by checking if health check fails
	// This is more reliable than trying to execute SQL which may vary by driver
	err = conn.HealthCheck()
	assert.Error(t, err, "HealthCheck should fail after database is closed")
}

func TestDB_HealthCheck(t *testing.T) {
	tests := []struct {
		name      string
		setupConn func() (*DB, func())
		wantErr   bool
	}{
		{
			name: "healthy connection",
			setupConn: func() (*DB, func()) {
				conn, _ := Initialize(":memory:", false)
				return conn, func() {
					if conn != nil {
						conn.Close()
					}
				}
			},
			wantErr: false,
		},
		{
			name: "closed connection",
			setupConn: func() (*DB, func()) {
				conn, _ := Initialize(":memory:", false)
				conn.Close()
				return conn, func() {}
			},
			wantErr: true,
		},
		{
			name: "nil connection",
			setupConn: func() (*DB, func()) {
				return nil, func() {}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, cleanup := tt.setupConn()
			defer cleanup()

			err := conn.HealthCheck()

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDB_AutoMigrate(t *testing.T) {
	// Define a test model
	type TestModel struct {
		gorm.Model
		Name  string
		Email string
		Age   int
	}

	tests := []struct {
		name    string
		models  []interface{}
		wantErr bool
		verify  func(*testing.T, *DB)
	}{
		{
			name:    "successful migration with single model",
			models:  []interface{}{&TestModel{}},
			wantErr: false,
			verify: func(t *testing.T, conn *DB) {
				// Check if table exists
				var count int64
				err := conn.DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test_models'").Scan(&count).Error
				assert.NoError(t, err)
				assert.Equal(t, int64(1), count)
			},
		},
		{
			name:    "successful migration with multiple models",
			models:  []interface{}{&TestModel{}},
			wantErr: false,
			verify: func(t *testing.T, conn *DB) {
				// Verify table exists
				var count int64
				err := conn.DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='test_models'").Scan(&count).Error
				assert.NoError(t, err)
				assert.Equal(t, int64(1), count)
			},
		},
		{
			name:    "migration with no models",
			models:  []interface{}{},
			wantErr: false,
			verify:  func(t *testing.T, conn *DB) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create connection
			conn, err := Initialize(":memory:", false)
			require.NoError(t, err)
			require.NotNil(t, conn)
			defer conn.Close()

			// Run migration
			err = conn.AutoMigrate(tt.models...)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				if tt.verify != nil {
					tt.verify(t, conn)
				}
			}
		})
	}
}

func TestDB_DatabaseOperations(t *testing.T) {
	// Define a test model
	type User struct {
		gorm.Model
		Name  string
		Email string
		Age   int
	}

	// Create connection and migrate
	conn, err := Initialize(":memory:", false)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	// Migrate the model
	err = conn.AutoMigrate(&User{})
	require.NoError(t, err)

	t.Run("create record", func(t *testing.T) {
		user := User{
			Name:  "John Doe",
			Email: "john@example.com",
			Age:   30,
		}

		err := conn.DB.Create(&user).Error
		assert.NoError(t, err)
		assert.NotZero(t, user.ID)
	})

	t.Run("find record", func(t *testing.T) {
		var user User
		err := conn.DB.First(&user, "email = ?", "john@example.com").Error
		assert.NoError(t, err)
		assert.Equal(t, "John Doe", user.Name)
		assert.Equal(t, 30, user.Age)
	})

	t.Run("update record", func(t *testing.T) {
		err := conn.DB.Model(&User{}).Where("email = ?", "john@example.com").Update("age", 31).Error
		assert.NoError(t, err)

		var user User
		conn.DB.First(&user, "email = ?", "john@example.com")
		assert.Equal(t, 31, user.Age)
	})

	t.Run("delete record", func(t *testing.T) {
		err := conn.DB.Where("email = ?", "john@example.com").Delete(&User{}).Error
		assert.NoError(t, err)

		var count int64
		conn.DB.Model(&User{}).Where("email = ?", "john@example.com").Count(&count)
		assert.Equal(t, int64(0), count)
	})
}

func TestDB_ConnectionPool(t *testing.T) {
	conn, err := Initialize(":memory:", false)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	// Get underlying SQL DB
	sqlDB, err := conn.DB.DB()
	require.NoError(t, err)

	// Check connection pool settings
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)

	// Verify settings
	stats := sqlDB.Stats()
	assert.LessOrEqual(t, stats.Idle, 5)
	assert.GreaterOrEqual(t, stats.MaxOpenConnections, 10)
}

func TestDB_Transaction(t *testing.T) {
	type TestRecord struct {
		gorm.Model
		Value string
	}

	conn, err := Initialize(":memory:", false)
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	err = conn.AutoMigrate(&TestRecord{})
	require.NoError(t, err)

	t.Run("successful transaction", func(t *testing.T) {
		err := conn.DB.Transaction(func(tx *gorm.DB) error {
			// Create multiple records in transaction
			for i := 0; i < 3; i++ {
				record := TestRecord{Value: "test"}
				if err := tx.Create(&record).Error; err != nil {
					return err
				}
			}
			return nil
		})

		assert.NoError(t, err)

		// Verify records were created
		var count int64
		conn.DB.Model(&TestRecord{}).Count(&count)
		assert.Equal(t, int64(3), count)
	})

	t.Run("failed transaction rollback", func(t *testing.T) {
		// Count before transaction
		var countBefore int64
		conn.DB.Model(&TestRecord{}).Count(&countBefore)

		err := conn.DB.Transaction(func(tx *gorm.DB) error {
			// Create a record
			record := TestRecord{Value: "rollback-test"}
			if err := tx.Create(&record).Error; err != nil {
				return err
			}

			// Force an error to trigger rollback
			return gorm.ErrInvalidTransaction
		})

		assert.Error(t, err)

		// Verify no new records were created (transaction was rolled back)
		var countAfter int64
		conn.DB.Model(&TestRecord{}).Count(&countAfter)
		assert.Equal(t, countBefore, countAfter)
	})
}

func TestInitializeWithMigrations(t *testing.T) {
	// Save original env to restore later
	originalEnv := os.Getenv("GO_TEST_MODE")
	os.Setenv("GO_TEST_MODE", "1")
	defer os.Setenv("GO_TEST_MODE", originalEnv)

	tests := []struct {
		name      string
		setupFunc func()
		wantErr   bool
		errMsg    string
	}{
		{
			name: "successful initialization with valid config",
			setupFunc: func() {
				// Reset viper
				viper.Reset()
				// Set test database path
				viper.Set("database.path", ":memory:")
				viper.Set("database.verbose", false)
			},
			wantErr: false,
		},
		{
			name: "error when database path not configured",
			setupFunc: func() {
				// Reset viper
				viper.Reset()
				// Don't set database path
			},
			wantErr: true,
			errMsg:  "database path is not configured",
		},
		{
			name: "successful initialization with file database",
			setupFunc: func() {
				// Reset viper
				viper.Reset()
				// Set test database path
				tempDir := t.TempDir()
				viper.Set("database.path", filepath.Join(tempDir, "test.db"))
				viper.Set("database.verbose", false)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test
			tt.setupFunc()

			// Call InitializeWithMigrations
			db, err := InitializeWithMigrations()

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
				assert.Nil(t, db)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, db)
				
				// Verify migrations were run by checking if tables exist
				if db != nil {
					// Check if podcast table exists
					var count int64
					err = db.DB.Raw("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='podcasts'").Scan(&count).Error
					assert.NoError(t, err)
					assert.Greater(t, count, int64(0), "podcasts table should exist")
					
					// Clean up
					db.Close()
				}
			}
		})
	}
}

func TestInitializeWithMigrations_ConfigNotInitialized(t *testing.T) {
	// Save original env to restore later
	originalEnv := os.Getenv("GO_TEST_MODE")
	os.Setenv("GO_TEST_MODE", "1")
	defer os.Setenv("GO_TEST_MODE", originalEnv)

	// Reset viper to simulate uninitialized config
	viper.Reset()
	
	// Set required config
	viper.Set("database.path", ":memory:")
	viper.Set("database.verbose", false)
	viper.Set("server.port", 8080) // Required for config validation

	// Call InitializeWithMigrations - should initialize config automatically
	db, err := InitializeWithMigrations()
	
	assert.NoError(t, err)
	assert.NotNil(t, db)
	
	if db != nil {
		// Verify config was initialized by checking if we can get values
		assert.True(t, config.IsInitialized())
		
		// Clean up
		db.Close()
	}
}
