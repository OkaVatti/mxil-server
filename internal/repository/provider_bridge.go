// internal/repository/provider_bridge_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// ProviderBridgeRepository handles provider bridge database operations
type ProviderBridgeRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewProviderBridgeRepository creates a new provider bridge repository
func NewProviderBridgeRepository(db *sqlx.DB, logger *zap.Logger) *ProviderBridgeRepository {
	return &ProviderBridgeRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new provider bridge
func (r *ProviderBridgeRepository) Create(ctx context.Context, bridge *models.ProviderBridge) error {
	query := `
		INSERT INTO provider_bridges (
			id, user_id, provider_type, account_identifier, encrypted_credentials,
			is_active, sync_interval, last_sync, sync_status, config,
			created_at, updated_at
		) VALUES (
			:id, :user_id, :provider_type, :account_identifier, :encrypted_credentials,
			:is_active, :sync_interval, :last_sync, :sync_status, :config,
			:created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, bridge)
	return err
}

// GetByID gets a provider bridge by ID
func (r *ProviderBridgeRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ProviderBridge, error) {
	var bridge models.ProviderBridge
	query := `SELECT * FROM provider_bridges WHERE id = $1`
	err := r.db.GetContext(ctx, &bridge, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider bridge: %w", err)
	}
	return &bridge, nil
}

// GetByProvider gets a provider bridge by user ID, provider type, and account identifier
func (r *ProviderBridgeRepository) GetByProvider(ctx context.Context, userID uuid.UUID, providerType, accountIdentifier string) (*models.ProviderBridge, error) {
	var bridge models.ProviderBridge
	query := `SELECT * FROM provider_bridges WHERE user_id = $1 AND provider_type = $2 AND account_identifier = $3`
	err := r.db.GetContext(ctx, &bridge, query, userID, providerType, accountIdentifier)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider bridge: %w", err)
	}
	return &bridge, nil
}

// Update updates a provider bridge
func (r *ProviderBridgeRepository) Update(ctx context.Context, bridge *models.ProviderBridge) error {
	query := `
		UPDATE provider_bridges SET
			provider_type = :provider_type,
			account_identifier = :account_identifier,
			encrypted_credentials = :encrypted_credentials,
			is_active = :is_active,
			sync_interval = :sync_interval,
			last_sync = :last_sync,
			sync_status = :sync_status,
			config = :config,
			updated_at = :updated_at
		WHERE id = :id
	`

	_, err := r.db.NamedExecContext(ctx, query, bridge)
	return err
}

// Delete deletes a provider bridge
func (r *ProviderBridgeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM provider_bridges WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// ListByUser lists provider bridges for a user
func (r *ProviderBridgeRepository) ListByUser(ctx context.Context, userID uuid.UUID, activeOnly bool) ([]models.ProviderBridge, error) {
	var bridges []models.ProviderBridge

	query := `SELECT * FROM provider_bridges WHERE user_id = $1`
	if activeOnly {
		query += ` AND is_active = true`
	}
	query += ` ORDER BY provider_type ASC, account_identifier ASC`

	err := r.db.SelectContext(ctx, &bridges, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list provider bridges: %w", err)
	}
	return bridges, nil
}

// UpdateSyncStatus updates sync status and last sync time
func (r *ProviderBridgeRepository) UpdateSyncStatus(ctx context.Context, id uuid.UUID, status string, lastSync string) error {
	query := `UPDATE provider_bridges SET sync_status = $1, last_sync = $2, updated_at = NOW() WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, status, lastSync, id)
	return err
}

// SetActive sets provider bridge active status
func (r *ProviderBridgeRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) error {
	query := `UPDATE provider_bridges SET is_active = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, active, id)
	return err
}

// UpdateCredentials updates encrypted credentials
func (r *ProviderBridgeRepository) UpdateCredentials(ctx context.Context, id uuid.UUID, credentials string) error {
	query := `UPDATE provider_bridges SET encrypted_credentials = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, credentials, id)
	return err
}

// GetByProviderType gets provider bridges by provider type
func (r *ProviderBridgeRepository) GetByProviderType(ctx context.Context, userID uuid.UUID, providerType string) ([]models.ProviderBridge, error) {
	var bridges []models.ProviderBridge
	query := `SELECT * FROM provider_bridges WHERE user_id = $1 AND provider_type = $2 ORDER BY account_identifier ASC`
	err := r.db.SelectContext(ctx, &bridges, query, userID, providerType)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider bridges by type: %w", err)
	}
	return bridges, nil
}

// GetActiveSyncBridges gets all bridges that need syncing
func (r *ProviderBridgeRepository) GetActiveSyncBridges(ctx context.Context) ([]models.ProviderBridge, error) {
	var bridges []models.ProviderBridge
	query := `
		SELECT * FROM provider_bridges 
		WHERE is_active = true 
		AND (
			last_sync IS NULL OR 
			last_sync < NOW() - INTERVAL '1 second' * sync_interval OR
			sync_status = 'failed'
		)
		ORDER BY last_sync ASC NULLS FIRST
	`
	err := r.db.SelectContext(ctx, &bridges, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sync bridges: %w", err)
	}
	return bridges, nil
}

// GetSyncStats gets sync statistics for a user
func (r *ProviderBridgeRepository) GetSyncStats(ctx context.Context, userID uuid.UUID) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total bridges
	var total int
	err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM provider_bridges WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total bridges: %w", err)
	}
	stats["total_bridges"] = total

	// Active bridges
	var active int
	err = r.db.GetContext(ctx, &active, `SELECT COUNT(*) FROM provider_bridges WHERE user_id = $1 AND is_active = true`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active bridges: %w", err)
	}
	stats["active_bridges"] = active

	// Last successful sync
	var lastSync string
	err = r.db.GetContext(ctx, &lastSync, `
		SELECT MAX(last_sync) FROM provider_bridges 
		WHERE user_id = $1 AND sync_status = 'success' AND last_sync IS NOT NULL
	`, userID)
	if err == nil && lastSync != "" {
		stats["last_successful_sync"] = lastSync
	}

	// Bridges by provider type
	type ProviderCount struct {
		ProviderType string `db:"provider_type"`
		Count        int    `db:"count"`
	}
	var providerCounts []ProviderCount
	err = r.db.SelectContext(ctx, &providerCounts, `
		SELECT provider_type, COUNT(*) as count 
		FROM provider_bridges 
		WHERE user_id = $1 
		GROUP BY provider_type
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider counts: %w", err)
	}

	providerStats := make(map[string]int)
	for _, pc := range providerCounts {
		providerStats[pc.ProviderType] = pc.Count
	}
	stats["bridges_by_provider"] = providerStats

	return stats, nil
}

// CheckProviderExists checks if a provider bridge already exists
func (r *ProviderBridgeRepository) CheckProviderExists(ctx context.Context, userID uuid.UUID, providerType, accountIdentifier string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM provider_bridges WHERE user_id = $1 AND provider_type = $2 AND account_identifier = $3)`
	err := r.db.GetContext(ctx, &exists, query, userID, providerType, accountIdentifier)
	if err != nil {
		return false, fmt.Errorf("failed to check provider exists: %w", err)
	}
	return exists, nil
}
