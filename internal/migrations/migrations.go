// internal/migrations/migrations.go
package migrations

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jmoiron/sqlx"
)

type MigrationManager struct {
	db *sqlx.DB
}

func NewMigrationManager(db *sqlx.DB) *MigrationManager {
	return &MigrationManager{db: db}
}

func (m *MigrationManager) Run() error {
	// Create migrations table if it doesn't exist
	if err := m.createMigrationsTable(); err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	// Get applied migrations
	applied, err := m.getAppliedMigrations()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	// Find migration files
	migrationsDir := "./migrations"
	files, err := m.getMigrationFiles(migrationsDir)
	if err != nil {
		return fmt.Errorf("failed to get migration files: %w", err)
	}

	// Apply new migrations
	for _, file := range files {
		if _, exists := applied[file.Name]; !exists {
			if err := m.applyMigration(file); err != nil {
				return fmt.Errorf("failed to apply migration %s: %w", file.Name, err)
			}
		}
	}

	return nil
}

func (m *MigrationManager) createMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS migrations (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL UNIQUE,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`
	_, err := m.db.Exec(query)
	return err
}

func (m *MigrationManager) getAppliedMigrations() (map[string]bool, error) {
	query := `SELECT name FROM migrations ORDER BY applied_at`
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}

	return applied, nil
}

type MigrationFile struct {
	Name string
	Path string
}

func (m *MigrationManager) getMigrationFiles(dir string) ([]MigrationFile, error) {
	var files []MigrationFile

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && strings.HasSuffix(d.Name(), ".up.sql") {
			name := strings.TrimSuffix(d.Name(), ".up.sql")
			files = append(files, MigrationFile{
				Name: name,
				Path: path,
			})
		}

		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// Sort by name (timestamp)
	sort.Slice(files, func(i, j int) bool {
		return files[i].Name < files[j].Name
	})

	return files, nil
}

func (m *MigrationManager) applyMigration(file MigrationFile) error {
	// Read migration SQL
	sqlBytes, err := os.ReadFile(file.Path)
	if err != nil {
		return err
	}

	// Begin transaction
	tx, err := m.db.Beginx()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Execute migration
	if _, err := tx.Exec(string(sqlBytes)); err != nil {
		return fmt.Errorf("migration SQL error: %w", err)
	}

	// Record migration
	query := `INSERT INTO migrations (name) VALUES ($1)`
	if _, err := tx.Exec(query, file.Name); err != nil {
		return err
	}

	// Commit transaction
	return tx.Commit()
}
