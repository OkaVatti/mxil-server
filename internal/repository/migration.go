// internal/repository/migration_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// MigrationRepository handles migration tracking database operations
type MigrationRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewMigrationRepository creates a new migration repository
func NewMigrationRepository(db *sqlx.DB, logger *zap.Logger) *MigrationRepository {
	return &MigrationRepository{
		db:     db,
		logger: logger,
	}
}

// CreateMigrationTable creates the migrations table if it doesn't exist
func (r *MigrationRepository) CreateMigrationTable(ctx context.Context) error {
	query := `
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			version VARCHAR(255) NOT NULL UNIQUE,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := r.db.ExecContext(ctx, query)
	return err
}

// RecordMigration records that a migration was applied
func (r *MigrationRepository) RecordMigration(ctx context.Context, version, name string) error {
	query := `INSERT INTO migrations (version, name) VALUES ($1, $2)`
	_, err := r.db.ExecContext(ctx, query, version, name)
	return err
}

// RemoveMigration removes a migration record
func (r *MigrationRepository) RemoveMigration(ctx context.Context, version string) error {
	query := `DELETE FROM migrations WHERE version = $1`
	_, err := r.db.ExecContext(ctx, query, version)
	return err
}

// GetAppliedMigrations gets all applied migrations
func (r *MigrationRepository) GetAppliedMigrations(ctx context.Context) ([]string, error) {
	var versions []string
	query := `SELECT version FROM migrations ORDER BY version`
	err := r.db.SelectContext(ctx, &versions, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}
	return versions, nil
}

// IsMigrationApplied checks if a migration has been applied
func (r *MigrationRepository) IsMigrationApplied(ctx context.Context, version string) (bool, error) {
	var applied bool
	query := `SELECT EXISTS(SELECT 1 FROM migrations WHERE version = $1)`
	err := r.db.GetContext(ctx, &applied, query, version)
	if err != nil {
		return false, fmt.Errorf("failed to check if migration is applied: %w", err)
	}
	return applied, nil
}

// GetLastMigration gets the last applied migration
func (r *MigrationRepository) GetLastMigration(ctx context.Context) (string, error) {
	var version string
	query := `SELECT version FROM migrations ORDER BY version DESC LIMIT 1`
	err := r.db.GetContext(ctx, &version, query)
	if err != nil {
		return "", fmt.Errorf("failed to get last migration: %w", err)
	}
	return version, nil
}
