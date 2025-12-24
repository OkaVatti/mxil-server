// cmd/seed/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/config"
	"github.com/okavatti/mxil-server/m/internal/crypto"
	"github.com/okavatti/mxil-server/m/internal/database"
)

func main() {
	// Setup logger
	logger, err := zap.NewDevelopment()
	if err != nil {
		log.Fatal("Failed to create logger:", err)
	}
	defer logger.Sync()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load config", zap.Error(err))
	}

	// Connect to database
	db, err := database.NewDB(cfg.Database, logger)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	// Seed data
	if err := seedDatabase(db.DB, logger); err != nil {
		logger.Fatal("Failed to seed database", zap.Error(err))
	}

	logger.Info("Database seeded successfully")
}

func seedDatabase(db *sqlx.DB, logger *zap.Logger) error {
	ctx := context.Background()

	// Create crypto service for password hashing
	cryptoService := crypto.NewCryptoService()

	// Seed test users
	testUsers := []struct {
		username string
		email    string
		password string
		role     string
	}{
		{"alice", "alice@example.com", "AlicePassword123!", "user"},
		{"bob", "bob@example.com", "BobPassword123!", "user"},
		{"charlie", "charlie@example.com", "CharliePassword123!", "user"},
		{"david", "david@example.com", "DavidPassword123!", "admin"},
	}

	for _, user := range testUsers {
		// Hash password
		hashedPassword, err := cryptoService.HashPassword(user.password)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}

		// Check if user exists
		var count int
		err = db.GetContext(ctx, &count, "SELECT COUNT(*) FROM users WHERE email = $1", user.email)
		if err != nil {
			return fmt.Errorf("failed to check user: %w", err)
		}

		if count > 0 {
			logger.Info("User already exists", zap.String("email", user.email))
			continue
		}

		// Insert user
		userID := uuid.New()
		query := `
			INSERT INTO users (
				id, master_username, email, display_name, password_hash, 
				is_active, security_score, created_at, updated_at
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		`

		_, err = db.ExecContext(ctx, query,
			userID,
			user.username,
			user.email,
			user.username, // display name
			hashedPassword,
			true,
			75,
			time.Now(),
			time.Now(),
		)

		if err != nil {
			return fmt.Errorf("failed to insert user: %w", err)
		}

		logger.Info("Created user", zap.String("username", user.username))

		// Create system folders for user
		folders := []struct {
			name string
			path string
		}{
			{"Inbox", "Inbox"},
			{"Sent", "Sent"},
			{"Drafts", "Drafts"},
			{"Spam", "Spam"},
			{"Trash", "Trash"},
			{"Archive", "Archive"},
		}

		for _, folder := range folders {
			folderID := uuid.New()
			query = `
				INSERT INTO folders (id, user_id, name, path, is_system, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`

			_, err = db.ExecContext(ctx, query,
				folderID,
				userID,
				folder.name,
				folder.path,
				true,
				time.Now(),
				time.Now(),
			)

			if err != nil {
				return fmt.Errorf("failed to insert folder: %w", err)
			}
		}

		// Create test labels
		labels := []struct {
			name  string
			color string
		}{
			{"Work", "#4ecdc4"},
			{"Personal", "#45b7d1"},
			{"Important", "#ff6b6b"},
			{"Travel", "#96ceb4"},
			{"Finance", "#feca57"},
		}

		for _, label := range labels {
			labelID := uuid.New()
			query = `
				INSERT INTO labels (id, user_id, name, color, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6)
			`

			_, err = db.ExecContext(ctx, query,
				labelID,
				userID,
				label.name,
				label.color,
				time.Now(),
				time.Now(),
			)

			if err != nil {
				return fmt.Errorf("failed to insert label: %w", err)
			}
		}

		// Create test contacts
		contacts := []struct {
			name      string
			email     string
			isTrusted bool
		}{
			{"John Doe", "john@example.com", true},
			{"Jane Smith", "jane@example.com", true},
			{"Support", "support@example.com", false},
		}

		for _, contact := range contacts {
			contactID := uuid.New()
			query = `
				INSERT INTO contacts (id, user_id, name, email_address, is_trusted, created_at, updated_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
			`

			_, err = db.ExecContext(ctx, query,
				contactID,
				userID,
				contact.name,
				contact.email,
				contact.isTrusted,
				time.Now(),
				time.Now(),
			)

			if err != nil {
				return fmt.Errorf("failed to insert contact: %w", err)
			}
		}

		// Create test emails
		for i := 1; i <= 5; i++ {
			emailID := uuid.New()
			threadID := uuid.New()
			receivedAt := time.Now().Add(-time.Duration(i) * 24 * time.Hour)

			query = `
				INSERT INTO emails (
					id, user_id, thread_id, "from", "to", subject, 
					body_plain, network, is_read, received_at, created_at, updated_at
				) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
			`

			_, err = db.ExecContext(ctx, query,
				emailID,
				userID,
				threadID,
				"sender@example.com",
				`["`+user.email+`"]`,
				fmt.Sprintf("Test Email %d", i),
				fmt.Sprintf("This is test email %d content.", i),
				"clearnet",
				i%2 == 0, // Alternate read status
				receivedAt,
				time.Now(),
				time.Now(),
			)

			if err != nil {
				return fmt.Errorf("failed to insert email: %w", err)
			}
		}
	}

	return nil
}
