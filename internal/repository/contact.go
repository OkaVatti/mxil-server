// internal/repository/contact_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// ContactRepository handles contact database operations
type ContactRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewContactRepository creates a new contact repository
func NewContactRepository(db *sqlx.DB, logger *zap.Logger) *ContactRepository {
	return &ContactRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new contact
func (r *ContactRepository) Create(ctx context.Context, contact *models.Contact) error {
	query := `
		INSERT INTO contacts (
			id, user_id, name, email_address, public_keys, network_identities,
			notes, is_trusted, last_contacted, created_at, updated_at
		) VALUES (
			:id, :user_id, :name, :email_address, :public_keys, :network_identities,
			:notes, :is_trusted, :last_contacted, :created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, contact)
	return err
}

// GetByID gets a contact by ID
func (r *ContactRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Contact, error) {
	var contact models.Contact
	query := `SELECT * FROM contacts WHERE id = $1`
	err := r.db.GetContext(ctx, &contact, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get contact: %w", err)
	}
	return &contact, nil
}

// GetByEmail gets a contact by user ID and email
func (r *ContactRepository) GetByEmail(ctx context.Context, userID uuid.UUID, email string) (*models.Contact, error) {
	var contact models.Contact
	query := `SELECT * FROM contacts WHERE user_id = $1 AND email_address = $2`
	err := r.db.GetContext(ctx, &contact, query, userID, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get contact: %w", err)
	}
	return &contact, nil
}

// Update updates a contact
func (r *ContactRepository) Update(ctx context.Context, contact *models.Contact) error {
	query := `
		UPDATE contacts SET
			name = :name,
			email_address = :email_address,
			public_keys = :public_keys,
			network_identities = :network_identities,
			notes = :notes,
			is_trusted = :is_trusted,
			last_contacted = :last_contacted,
			updated_at = :updated_at
		WHERE id = :id
	`

	_, err := r.db.NamedExecContext(ctx, query, contact)
	return err
}

// Delete deletes a contact
func (r *ContactRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM contacts WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// ListByUser lists contacts for a user
func (r *ContactRepository) ListByUser(ctx context.Context, userID uuid.UUID, trustedOnly bool, limit, offset int) ([]models.Contact, int64, error) {
	var contacts []models.Contact
	var total int64

	// Count query
	countQuery := `SELECT COUNT(*) FROM contacts WHERE user_id = $1`
	if trustedOnly {
		countQuery += ` AND is_trusted = true`
	}
	err := r.db.GetContext(ctx, &total, countQuery, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count contacts: %w", err)
	}

	// List query
	listQuery := `SELECT * FROM contacts WHERE user_id = $1`
	if trustedOnly {
		listQuery += ` AND is_trusted = true`
	}
	listQuery += ` ORDER BY name ASC, email_address ASC LIMIT $2 OFFSET $3`

	err = r.db.SelectContext(ctx, &contacts, listQuery, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list contacts: %w", err)
	}

	return contacts, total, nil
}

// Search searches contacts by query
func (r *ContactRepository) Search(ctx context.Context, userID uuid.UUID, queryStr string, limit, offset int) ([]models.Contact, int64, error) {
	var contacts []models.Contact
	var total int64

	// Count query
	countQuery := `
		SELECT COUNT(*) FROM contacts 
		WHERE user_id = $1 AND (
			name ILIKE $2 OR 
			email_address ILIKE $2 OR 
			notes ILIKE $2
		)
	`
	err := r.db.GetContext(ctx, &total, countQuery, userID, "%"+queryStr+"%")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Search query
	searchQuery := `
		SELECT * FROM contacts 
		WHERE user_id = $1 AND (
			name ILIKE $2 OR 
			email_address ILIKE $2 OR 
			notes ILIKE $2
		)
		ORDER BY 
			CASE 
				WHEN name ILIKE $2 THEN 1
				WHEN email_address ILIKE $2 THEN 2
				ELSE 3
			END,
			name ASC
		LIMIT $3 OFFSET $4
	`
	err = r.db.SelectContext(ctx, &contacts, searchQuery, userID, "%"+queryStr+"%", limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search contacts: %w", err)
	}

	return contacts, total, nil
}

// UpdateLastContacted updates last contacted time
func (r *ContactRepository) UpdateLastContacted(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE contacts SET last_contacted = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// MarkAsTrusted marks contact as trusted
func (r *ContactRepository) MarkAsTrusted(ctx context.Context, id uuid.UUID, trusted bool) error {
	query := `UPDATE contacts SET is_trusted = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, trusted, id)
	return err
}

// GetRecent gets recently contacted contacts
func (r *ContactRepository) GetRecent(ctx context.Context, userID uuid.UUID, limit int) ([]models.Contact, error) {
	var contacts []models.Contact
	query := `
		SELECT * FROM contacts 
		WHERE user_id = $1 AND last_contacted IS NOT NULL 
		ORDER BY last_contacted DESC 
		LIMIT $2
	`
	err := r.db.SelectContext(ctx, &contacts, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent contacts: %w", err)
	}
	return contacts, nil
}

// GetFrequent gets frequently contacted contacts
func (r *ContactRepository) GetFrequent(ctx context.Context, userID uuid.UUID, limit int) ([]models.Contact, error) {
	var contacts []models.Contact
	query := `
		SELECT c.*, COUNT(e.id) as email_count
		FROM contacts c
		LEFT JOIN emails e ON (
			e.user_id = c.user_id AND 
			(e."from" = c.email_address OR c.email_address = ANY(e."to"))
		)
		WHERE c.user_id = $1
		GROUP BY c.id
		ORDER BY email_count DESC, c.name ASC
		LIMIT $2
	`
	err := r.db.SelectContext(ctx, &contacts, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get frequent contacts: %w", err)
	}
	return contacts, nil
}

// Import imports multiple contacts
func (r *ContactRepository) Import(ctx context.Context, contacts []models.Contact) (int, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	imported := 0
	for _, contact := range contacts {
		// Check if contact already exists
		var exists bool
		checkQuery := `SELECT EXISTS(SELECT 1 FROM contacts WHERE user_id = $1 AND email_address = $2)`
		err := tx.GetContext(ctx, &exists, checkQuery, contact.UserID, contact.EmailAddress)
		if err != nil {
			continue
		}

		if exists {
			// Update existing
			updateQuery := `
				UPDATE contacts SET
					name = COALESCE(NULLIF($1, ''), name),
					public_keys = $2,
					network_identities = $3,
					notes = COALESCE(NULLIF($4, ''), notes),
					is_trusted = $5,
					updated_at = NOW()
				WHERE user_id = $6 AND email_address = $7
			`
			_, err = tx.ExecContext(ctx, updateQuery,
				contact.Name, contact.PublicKeys, contact.NetworkIdentities,
				contact.Notes, contact.IsTrusted, contact.UserID, contact.EmailAddress)
		} else {
			// Insert new
			insertQuery := `
				INSERT INTO contacts (
					id, user_id, name, email_address, public_keys, network_identities,
					notes, is_trusted, created_at, updated_at
				) VALUES (
					$1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW()
				)
			`
			_, err = tx.ExecContext(ctx, insertQuery,
				contact.ID, contact.UserID, contact.Name, contact.EmailAddress,
				contact.PublicKeys, contact.NetworkIdentities, contact.Notes, contact.IsTrusted)
		}

		if err == nil {
			imported++
		}
	}

	err = tx.Commit()
	if err != nil {
		return 0, fmt.Errorf("failed to commit import transaction: %w", err)
	}

	return imported, nil
}

// Export exports contacts for a user
func (r *ContactRepository) Export(ctx context.Context, userID uuid.UUID) ([]models.Contact, error) {
	var contacts []models.Contact
	query := `SELECT * FROM contacts WHERE user_id = $1 ORDER BY name ASC, email_address ASC`
	err := r.db.SelectContext(ctx, &contacts, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to export contacts: %w", err)
	}
	return contacts, nil
}
