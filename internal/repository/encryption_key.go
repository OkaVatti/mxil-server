// internal/repository/encryption_key_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// EncryptionKeyRepository handles encryption key database operations
type EncryptionKeyRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewEncryptionKeyRepository creates a new encryption key repository
func NewEncryptionKeyRepository(db *sqlx.DB, logger *zap.Logger) *EncryptionKeyRepository {
	return &EncryptionKeyRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new encryption key
func (r *EncryptionKeyRepository) Create(ctx context.Context, key *models.EncryptionKey) error {
	query := `
		INSERT INTO encryption_keys (
			id, user_id, key_type, key_id, public_key, private_key_encrypted,
			fingerprint, is_primary, is_active, expires_at, last_used,
			created_at, updated_at
		) VALUES (
			:id, :user_id, :key_type, :key_id, :public_key, :private_key_encrypted,
			:fingerprint, :is_primary, :is_active, :expires_at, :last_used,
			:created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, key)
	return err
}

// GetByID gets an encryption key by ID
func (r *EncryptionKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.EncryptionKey, error) {
	var key models.EncryptionKey
	query := `SELECT * FROM encryption_keys WHERE id = $1 AND deleted_at IS NULL`
	err := r.db.GetContext(ctx, &key, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}
	return &key, nil
}

// GetByKeyID gets an encryption key by key ID
func (r *EncryptionKeyRepository) GetByKeyID(ctx context.Context, keyID string) (*models.EncryptionKey, error) {
	var key models.EncryptionKey
	query := `SELECT * FROM encryption_keys WHERE key_id = $1 AND deleted_at IS NULL`
	err := r.db.GetContext(ctx, &key, query, keyID)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}
	return &key, nil
}

// GetByFingerprint gets an encryption key by fingerprint
func (r *EncryptionKeyRepository) GetByFingerprint(ctx context.Context, fingerprint string) (*models.EncryptionKey, error) {
	var key models.EncryptionKey
	query := `SELECT * FROM encryption_keys WHERE fingerprint = $1 AND deleted_at IS NULL`
	err := r.db.GetContext(ctx, &key, query, fingerprint)
	if err != nil {
		return nil, fmt.Errorf("failed to get encryption key: %w", err)
	}
	return &key, nil
}

// Update updates an encryption key
func (r *EncryptionKeyRepository) Update(ctx context.Context, key *models.EncryptionKey) error {
	query := `
		UPDATE encryption_keys SET
			key_type = :key_type,
			key_id = :key_id,
			public_key = :public_key,
			private_key_encrypted = :private_key_encrypted,
			fingerprint = :fingerprint,
			is_primary = :is_primary,
			is_active = :is_active,
			expires_at = :expires_at,
			last_used = :last_used,
			updated_at = :updated_at
		WHERE id = :id AND deleted_at IS NULL
	`

	_, err := r.db.NamedExecContext(ctx, query, key)
	return err
}

// Delete soft deletes an encryption key
func (r *EncryptionKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE encryption_keys SET deleted_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// ListByUser lists encryption keys for a user
func (r *EncryptionKeyRepository) ListByUser(ctx context.Context, userID uuid.UUID, keyType *string, activeOnly bool) ([]models.EncryptionKey, error) {
	var keys []models.EncryptionKey

	query := `SELECT * FROM encryption_keys WHERE user_id = $1 AND deleted_at IS NULL`
	args := []interface{}{userID}
	argCount := 2

	if keyType != nil {
		query += fmt.Sprintf(" AND key_type = $%d", argCount)
		args = append(args, *keyType)
		argCount++
	}

	if activeOnly {
		query += ` AND is_active = true AND (expires_at IS NULL OR expires_at > NOW())`
	}

	query += ` ORDER BY is_primary DESC, key_type ASC, created_at DESC`

	err := r.db.SelectContext(ctx, &keys, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list encryption keys: %w", err)
	}
	return keys, nil
}

// SetPrimary sets a key as primary for its type
func (r *EncryptionKeyRepository) SetPrimary(ctx context.Context, id uuid.UUID) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get the key to know its type and user
	var key models.EncryptionKey
	err = tx.GetContext(ctx, &key, "SELECT * FROM encryption_keys WHERE id = $1 AND deleted_at IS NULL", id)
	if err != nil {
		return fmt.Errorf("failed to get key: %w", err)
	}

	// Clear primary flag for all keys of this user of the same type
	clearQuery := `UPDATE encryption_keys SET is_primary = false, updated_at = NOW() WHERE user_id = $1 AND key_type = $2 AND deleted_at IS NULL`
	_, err = tx.ExecContext(ctx, clearQuery, key.UserID, key.KeyType)
	if err != nil {
		return fmt.Errorf("failed to clear primary flags: %w", err)
	}

	// Set this key as primary
	setQuery := `UPDATE encryption_keys SET is_primary = true, updated_at = NOW() WHERE id = $1`
	_, err = tx.ExecContext(ctx, setQuery, id)
	if err != nil {
		return fmt.Errorf("failed to set primary: %w", err)
	}

	return tx.Commit()
}

// UpdateLastUsed updates last used timestamp
func (r *EncryptionKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE encryption_keys SET last_used = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// SetActive sets key active status
func (r *EncryptionKeyRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) error {
	query := `UPDATE encryption_keys SET is_active = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, active, id)
	return err
}

// GetPrimary gets primary key for a user of a specific type
func (r *EncryptionKeyRepository) GetPrimary(ctx context.Context, userID uuid.UUID, keyType string) (*models.EncryptionKey, error) {
	var key models.EncryptionKey
	query := `SELECT * FROM encryption_keys WHERE user_id = $1 AND key_type = $2 AND is_primary = true AND is_active = true AND deleted_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())`
	err := r.db.GetContext(ctx, &key, query, userID, keyType)
	if err != nil {
		return nil, fmt.Errorf("failed to get primary key: %w", err)
	}
	return &key, nil
}

// GetExpired gets expired keys
func (r *EncryptionKeyRepository) GetExpired(ctx context.Context) ([]models.EncryptionKey, error) {
	var keys []models.EncryptionKey
	query := `SELECT * FROM encryption_keys WHERE expires_at < NOW() AND deleted_at IS NULL AND is_active = true`
	err := r.db.SelectContext(ctx, &keys, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired keys: %w", err)
	}
	return keys, nil
}

// GetStats gets key statistics for a user
func (r *EncryptionKeyRepository) GetStats(ctx context.Context, userID uuid.UUID) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total keys
	var total int
	err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM encryption_keys WHERE user_id = $1 AND deleted_at IS NULL`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get total keys: %w", err)
	}
	stats["total_keys"] = total

	// Active keys
	var active int
	err = r.db.GetContext(ctx, &active, `SELECT COUNT(*) FROM encryption_keys WHERE user_id = $1 AND is_active = true AND deleted_at IS NULL`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active keys: %w", err)
	}
	stats["active_keys"] = active

	// Expired keys
	var expired int
	err = r.db.GetContext(ctx, &expired, `SELECT COUNT(*) FROM encryption_keys WHERE user_id = $1 AND expires_at < NOW() AND deleted_at IS NULL`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired keys: %w", err)
	}
	stats["expired_keys"] = expired

	// Keys by type
	type KeyTypeCount struct {
		KeyType string `db:"key_type"`
		Count   int    `db:"count"`
	}
	var typeCounts []KeyTypeCount
	err = r.db.SelectContext(ctx, &typeCounts, `
		SELECT key_type, COUNT(*) as count 
		FROM encryption_keys 
		WHERE user_id = $1 AND deleted_at IS NULL 
		GROUP BY key_type
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get key type counts: %w", err)
	}

	typeStats := make(map[string]int)
	for _, tc := range typeCounts {
		typeStats[tc.KeyType] = tc.Count
	}
	stats["keys_by_type"] = typeStats

	return stats, nil
}

// CheckKeyIDExists checks if a key ID already exists
func (r *EncryptionKeyRepository) CheckKeyIDExists(ctx context.Context, keyID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM encryption_keys WHERE key_id = $1 AND deleted_at IS NULL)`
	err := r.db.GetContext(ctx, &exists, query, keyID)
	if err != nil {
		return false, fmt.Errorf("failed to check key ID exists: %w", err)
	}
	return exists, nil
}

// CheckFingerprintExists checks if a fingerprint already exists
func (r *EncryptionKeyRepository) CheckFingerprintExists(ctx context.Context, fingerprint string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM encryption_keys WHERE fingerprint = $1 AND deleted_at IS NULL)`
	err := r.db.GetContext(ctx, &exists, query, fingerprint)
	if err != nil {
		return false, fmt.Errorf("failed to check fingerprint exists: %w", err)
	}
	return exists, nil
}
