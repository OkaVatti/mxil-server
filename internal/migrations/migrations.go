// internal/migrations/migrations.go
package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// Migration represents a database migration
type Migration struct {
	Version   string
	Name      string
	UpSQL     string
	DownSQL   string
	AppliedAt time.Time
}

// MigrationManager manages database migrations
type MigrationManager struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewMigrationManager creates a new migration manager
func NewMigrationManager(db *sqlx.DB, logger *zap.Logger) *MigrationManager {
	return &MigrationManager{
		db:     db,
		logger: logger,
	}
}

// Run runs all pending migrations
func (m *MigrationManager) Run() error {
	m.logger.Info("Running database migrations")

	// Create migrations table if it doesn't exist
	if err := m.createMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := m.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Get available migrations
	available, err := m.getAvailableMigrations()
	if err != nil {
		return fmt.Errorf("failed to get available migrations: %w", err)
	}

	// Run pending migrations
	for _, migration := range available {
		if _, ok := applied[migration.Version]; !ok {
			if err := m.applyMigration(migration); err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", migration.Version, err)
			}
		}
	}

	m.logger.Info("Database migrations completed")
	return nil
}

// Rollback rolls back the last migration
func (m *MigrationManager) Rollback() error {
	m.logger.Info("Rolling back last migration")

	// Get last applied migration
	lastMigration, err := m.getLastAppliedMigration()
	if err != nil {
		return fmt.Errorf("failed to get last migration: %w", err)
	}

	if lastMigration == nil {
		m.logger.Info("No migrations to rollback")
		return nil
	}

	// Get migration files
	migrations, err := m.getAvailableMigrations()
	if err != nil {
		return err
	}

	// Find the migration to rollback
	var migrationToRollback *Migration
	for _, migration := range migrations {
		if migration.Version == lastMigration.Version {
			migrationToRollback = migration
			break
		}
	}

	if migrationToRollback == nil {
		return fmt.Errorf("migration file not found for version %s", lastMigration.Version)
	}

	// Execute down SQL
	if migrationToRollback.DownSQL != "" {
		if _, err := m.db.Exec(migrationToRollback.DownSQL); err != nil {
			return fmt.Errorf("failed to execute down migration: %w", err)
		}
	}

	// Remove from migrations table
	query := `DELETE FROM migrations WHERE version = $1`
	if _, err := m.db.Exec(query, lastMigration.Version); err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}

	m.logger.Info("Migration rolled back", zap.String("version", lastMigration.Version))
	return nil
}

// Helper methods
func (m *MigrationManager) createMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS migrations (
			version VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			applied_at TIMESTAMP NOT NULL DEFAULT NOW()
		)
	`
	_, err := m.db.Exec(query)
	return err
}

func (m *MigrationManager) getAppliedMigrations() (map[string]*Migration, error) {
	query := `SELECT version, name, applied_at FROM migrations ORDER BY version`
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	migrations := make(map[string]*Migration)
	for rows.Next() {
		var migration Migration
		if err := rows.Scan(&migration.Version, &migration.Name, &migration.AppliedAt); err != nil {
			return nil, err
		}
		migrations[migration.Version] = &migration
	}

	return migrations, nil
}

func (m *MigrationManager) getAvailableMigrations() ([]*Migration, error) {
	migrationsDir := "./migrations"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		// Create migrations directory
		if err := os.MkdirAll(migrationsDir, 0755); err != nil {
			return nil, err
		}
		return []*Migration{}, nil
	}

	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		return nil, err
	}

	var migrations []*Migration
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".up.sql") {
			version := strings.TrimSuffix(file.Name(), ".up.sql")
			name := strings.TrimPrefix(version, strings.Split(version, "_")[0]+"_")

			upSQL, err := os.ReadFile(filepath.Join(migrationsDir, file.Name()))
			if err != nil {
				return nil, err
			}

			downFile := strings.Replace(file.Name(), ".up.sql", ".down.sql", 1)
			var downSQL []byte
			if _, err := os.Stat(filepath.Join(migrationsDir, downFile)); err == nil {
				downSQL, err = os.ReadFile(filepath.Join(migrationsDir, downFile))
				if err != nil {
					return nil, err
				}
			}

			migrations = append(migrations, &Migration{
				Version: version,
				Name:    name,
				UpSQL:   string(upSQL),
				DownSQL: string(downSQL),
			})
		}
	}

	// Sort migrations by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func (m *MigrationManager) applyMigration(migration *Migration) error {
	m.logger.Info("Applying migration", zap.String("version", migration.Version), zap.String("name", migration.Name))

	// Begin transaction
	tx, err := m.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute up SQL
	if _, err := tx.Exec(migration.UpSQL); err != nil {
		return fmt.Errorf("failed to execute migration: %w", err)
	}

	// Record migration
	query := `INSERT INTO migrations (version, name) VALUES ($1, $2)`
	if _, err := tx.Exec(query, migration.Version, migration.Name); err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return err
	}

	m.logger.Info("Migration applied", zap.String("version", migration.Version))
	return nil
}

func (m *MigrationManager) getLastAppliedMigration() (*Migration, error) {
	query := `SELECT version, name, applied_at FROM migrations ORDER BY version DESC LIMIT 1`
	row := m.db.QueryRow(query)

	var migration Migration
	if err := row.Scan(&migration.Version, &migration.Name, &migration.AppliedAt); err != nil {
		// No rows is not an error
		return nil, nil
	}

	return &migration, nil
}
