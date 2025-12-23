package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"gorm.io/gorm"
)

// EncryptionKeyRepository handles encryption key database operations
type EncryptionKeyRepository struct {
	db *gorm.DB
}

// NewEncryptionKeyRepository creates a new encryption key repository
func NewEncryptionKeyRepository(db *gorm.DB) *EncryptionKeyRepository {
	return &EncryptionKeyRepository{db: db}
}

// Create creates a new encryption key
func (r *EncryptionKeyRepository) Create(ctx context.Context, key *models.EncryptionKey) error {
	return r.db.WithContext(ctx).Create(key).Error
}

// GetByID gets an encryption key by ID
func (r *EncryptionKeyRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.EncryptionKey, error) {
	var key models.EncryptionKey
	if err := r.db.WithContext(ctx).
		First(&key, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// GetByKeyID gets an encryption key by key ID
func (r *EncryptionKeyRepository) GetByKeyID(ctx context.Context, keyID string) (*models.EncryptionKey, error) {
	var key models.EncryptionKey
	if err := r.db.WithContext(ctx).
		Where("key_id = ?", keyID).
		First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// Update updates an encryption key
func (r *EncryptionKeyRepository) Update(ctx context.Context, key *models.EncryptionKey) error {
	return r.db.WithContext(ctx).Save(key).Error
}

// Delete deletes an encryption key
func (r *EncryptionKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Delete(&models.EncryptionKey{}, "id = ?", id).Error
}

// ListByUser lists encryption keys for a user
func (r *EncryptionKeyRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.EncryptionKey, error) {
	var keys []models.EncryptionKey
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("is_primary DESC, created_at DESC").
		Find(&keys).Error; err != nil {
		return nil, err
	}
	return keys, nil
}

// GetPrimaryKey gets the primary encryption key for a user
func (r *EncryptionKeyRepository) GetPrimaryKey(ctx context.Context, userID uuid.UUID) (*models.EncryptionKey, error) {
	var key models.EncryptionKey
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_primary = ?", userID, true).
		First(&key).Error; err != nil {
		return nil, err
	}
	return &key, nil
}

// ClearPrimary clears primary flag for all keys of a user
func (r *EncryptionKeyRepository) ClearPrimary(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.EncryptionKey{}).
		Where("user_id = ?", userID).
		Update("is_primary", false).Error
}

// CountActive counts active keys for a user
func (r *EncryptionKeyRepository) CountActive(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.EncryptionKey{}).
		Where("user_id = ? AND is_active = ?", userID, true).
		Count(&count).Error
	return count, err
}

// UpdateLastUsed updates the last used time of a key
func (r *EncryptionKeyRepository) UpdateLastUsed(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.EncryptionKey{}).
		Where("id = ?", id).
		Update("last_used", time.Now()).Error
}

// GetKeyUsage gets key usage statistics
func (r *EncryptionKeyRepository) GetKeyUsage(ctx context.Context, keyID uuid.UUID) (*KeyUsage, error) {
	// This would typically query a separate usage table
	// For now, we'll return basic information
	var key models.EncryptionKey
	if err := r.db.WithContext(ctx).
		First(&key, "id = ?", keyID).Error; err != nil {
		return nil, err
	}

	return &KeyUsage{
		TotalEncrypted: 0, // Would need separate tracking
		LastUsed:       key.LastUsed,
		FirstUsed:      &key.CreatedAt,
	}, nil
}

// KeyUsage contains key usage statistics
type KeyUsage struct {
	TotalEncrypted int64      `json:"total_encrypted"`
	LastUsed       *time.Time `json:"last_used"`
	FirstUsed      *time.Time `json:"first_used"`
}
