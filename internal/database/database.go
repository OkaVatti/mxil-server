// internal/database/database.go
package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/config"
)

// DB wraps sqlx.DB with additional methods
type DB struct {
	*sqlx.DB
	logger *zap.Logger
}

// NewDB creates a new database connection
func NewDB(cfg config.DatabaseConfig, logger *zap.Logger) (*DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database, cfg.SSLMode,
	)

	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Database connection established")
	return &DB{DB: db, logger: logger}, nil
}

// HealthCheck checks database health
func (db *DB) HealthCheck(ctx context.Context) error {
	return db.PingContext(ctx)
}

// Close closes the database connection
func (db *DB) Close() error {
	db.logger.Info("Closing database connection")
	return db.DB.Close()
}

// ExecuteMigration executes a single migration
func (db *DB) ExecuteMigration(ctx context.Context, sql string) error {
	_, err := db.ExecContext(ctx, sql)
	return err
}

// BeginTx begins a transaction
func (db *DB) BeginTx(ctx context.Context) (*sqlx.Tx, error) {
	return db.BeginTxx(ctx, nil)
}
