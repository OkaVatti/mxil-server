package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// NetworkIdentityRepository handles network identity database operations
type NetworkIdentityRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewNetworkIdentityRepository creates a new network identity repository
func NewNetworkIdentityRepository(db *sqlx.DB, logger *zap.Logger) *NetworkIdentityRepository {
	return &NetworkIdentityRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new network identity
func (r *NetworkIdentityRepository) Create(ctx context.Context, identity *models.NetworkIdentity) error {
	query := `
		INSERT INTO network_identities (
			id, user_id, network, address, is_primary, is_active, forward_to,
			config, last_used, created_at, updated_at
		) VALUES (
			:id, :user_id, :network, :address, :is_primary, :is_active, :forward_to,
			:config, :last_used, :created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, identity)
	return err
}

// GetByID gets a network identity by ID
func (r *NetworkIdentityRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.NetworkIdentity, error) {
	var identity models.NetworkIdentity
	query := `SELECT * FROM network_identities WHERE id = $1`
	err := r.db.GetContext(ctx, &identity, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get network identity: %w", err)
	}
	return &identity, nil
}

// GetByAddress gets a network identity by user ID, network, and address
func (r *NetworkIdentityRepository) GetByAddress(ctx context.Context, userID uuid.UUID, network, address string) (*models.NetworkIdentity, error) {
	var identity models.NetworkIdentity
	query := `SELECT * FROM network_identities WHERE user_id = $1 AND network = $2 AND address = $3`
	err := r.db.GetContext(ctx, &identity, query, userID, network, address)
	if err != nil {
		return nil, fmt.Errorf("failed to get network identity: %w", err)
	}
	return &identity, nil
}

// Update updates a network identity
func (r *NetworkIdentityRepository) Update(ctx context.Context, identity *models.NetworkIdentity) error {
	query := `
		UPDATE network_identities SET
			network = :network,
			address = :address,
			is_primary = :is_primary,
			is_active = :is_active,
			forward_to = :forward_to,
			config = :config,
			last_used = :last_used,
			updated_at = :updated_at
		WHERE id = :id
	`

	_, err := r.db.NamedExecContext(ctx, query, identity)
	return err
}

// Delete deletes a network identity
func (r *NetworkIdentityRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM network_identities WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// ListByUser lists network identities for a user
func (r *NetworkIdentityRepository) ListByUser(ctx context.Context, userID uuid.UUID, network *string, activeOnly bool) ([]models.NetworkIdentity, error) {
	var identities []models.NetworkIdentity

	query := `SELECT * FROM network_identities WHERE user_id = $1`
	args := []interface{}{userID}
	argCount := 2

	if network != nil {
		query += fmt.Sprintf(" AND network = $%d", argCount)
		args = append(args, *network)
		argCount++
	}

	if activeOnly {
		query += fmt.Sprintf(" AND is_active = $%d", argCount)
		args = append(args, true)
		argCount++
	}

	query += ` ORDER BY is_primary DESC, network ASC, address ASC`

	err := r.db.SelectContext(ctx, &identities, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list network identities: %w", err)
	}
	return identities, nil
}

// SetPrimary sets an identity as primary for its network
func (r *NetworkIdentityRepository) SetPrimary(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get the identity to know its network and user
	var identity models.NetworkIdentity
	err = tx.GetContext(ctx, &identity, "SELECT * FROM network_identities WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to get identity: %w", err)
	}

	// Clear primary flag for all identities of this user on the same network
	clearQuery := `UPDATE network_identities SET is_primary = false, updated_at = NOW() WHERE user_id = $1 AND network = $2`
	_, err = tx.ExecContext(ctx, clearQuery, identity.UserID, identity.Network)
	if err != nil {
		return fmt.Errorf("failed to clear primary flags: %w", err)
	}

	// Set this identity as primary
	setQuery := `UPDATE network_identities SET is_primary = true, updated_at = NOW() WHERE id = $1`
	_, err = tx.ExecContext(ctx, setQuery, id)
	if err != nil {
		return fmt.Errorf("failed to set primary: %w", err)
	}

	return tx.Commit()
}

// UpdateLastUsed updates last used timestamp
func (r *NetworkIdentityRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE network_identities SET last_used = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// GetPrimary gets the primary identity for a user on a specific network
func (r *NetworkIdentityRepository) GetPrimary(ctx context.Context, userID uuid.UUID, network string) (*models.NetworkIdentity, error) {
	var identity models.NetworkIdentity
	query := `SELECT * FROM network_identities WHERE user_id = $1 AND network = $2 AND is_primary = true`
	err := r.db.GetContext(ctx, &identity, query, userID, network)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary identity: %w", err)
	}
	return &identity, nil
}

// CountByUser counts network identities for a user
func (r *NetworkIdentityRepository) CountByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM network_identities WHERE user_id = $1`
	err := r.db.GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to count network identities: %w", err)
	}
	return count, nil
}

// GetActiveNetworks gets list of active networks for a user
func (r *NetworkIdentityRepository) GetActiveNetworks(ctx context.Context, userID uuid.UUID) ([]string, error) {
	var networks []string
	query := `SELECT DISTINCT network FROM network_identities WHERE user_id = $1 AND is_active = true`
	err := r.db.SelectContext(ctx, &networks, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active networks: %w", err)
	}
	return networks, nil
}
