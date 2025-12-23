package database

import (
	"context"
	"fmt"
	"time"

	"github.com/okavatti/mxil-server/m/internal/config"
	"github.com/okavatti/mxil-server/m/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB represents the database connection
type DB struct {
	*gorm.DB
}

// NewDatabase creates a new database connection
func NewDatabase(cfg *config.DatabaseConfig) (*DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	// Configure GORM
	gormConfig := &gorm.Config{
		PrepareStmt: true,
		Logger:      logger.Default.LogMode(logger.Silent),
	}

	// Open connection
	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{db}, nil
}

// Migrate runs database migrations
func (db *DB) Migrate() error {
	// Register custom types
	if err := db.SetupGormCustomTypes(); err != nil {
		return err
	}

	// Auto-migrate all models
	return db.AutoMigrate(
		&models.User{},
		&models.Session{},
		&models.NetworkIdentity{},
		&models.Email{},
		&models.Folder{},
		&models.Label{},
		&models.Contact{},
		&models.EncryptionKey{},
		&models.ProviderBridge{},
		&models.Attachment{},
		&models.AuthMethod{},
		&models.TrustedDevice{},
	)
}

// SetupGormCustomTypes sets up custom GORM types
func (db *DB) SetupGormCustomTypes() error {
	// Register custom types
	err := db.Exec(`DO $$ BEGIN
		CREATE TYPE network_type AS ENUM ('clearnet', 'i2p', 'tor', 'lan', 'ipfs');
	EXCEPTION
		WHEN duplicate_object THEN null;
	END $$;`).Error

	if err != nil {
		return fmt.Errorf("failed to create network_type enum: %w", err)
	}

	err = db.Exec(`DO $$ BEGIN
		CREATE TYPE provider_type AS ENUM (
			'gmail', 'outlook', 'yahoo', 'proton', 
			'i2p_bote', 'susimail', 'onion_mail', 'custom_imap'
		);
	EXCEPTION
		WHEN duplicate_object THEN null;
	END $$;`).Error

	if err != nil {
		return fmt.Errorf("failed to create provider_type enum: %w", err)
	}

	return nil
}

// HealthCheck performs a health check on the database
func (db *DB) HealthCheck(ctx context.Context) error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}

	// Create a timeout context for the health check
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Try to ping the database
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	// Check if we can run a simple query
	var result int
	if err := db.WithContext(ctx).Raw("SELECT 1").Scan(&result).Error; err != nil {
		return fmt.Errorf("database query failed: %w", err)
	}

	return nil
}

// Close closes the database connection
func (db *DB) Close() error {
	sqlDB, err := db.DB.DB()
	if err != nil {
		return fmt.Errorf("failed to get sql.DB: %w", err)
	}
	return sqlDB.Close()
}

// Transaction executes a function within a database transaction
func (db *DB) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return db.WithContext(ctx).Transaction(fn)
}

// WithContext returns a new DB instance with context
func (db *DB) WithContext(ctx context.Context) *gorm.DB {
	return db.DB.WithContext(ctx)
}
