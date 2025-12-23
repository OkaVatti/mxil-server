package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"gorm.io/gorm"
)

// NetworkIdentityRepository handles network identity database operations
type NetworkIdentityRepository struct {
	db *gorm.DB
}

// NewNetworkIdentityRepository creates a new network identity repository
func NewNetworkIdentityRepository(db *gorm.DB) *NetworkIdentityRepository {
	return &NetworkIdentityRepository{db: db}
}

// Create creates a new network identity
func (r *NetworkIdentityRepository) Create(ctx context.Context, identity *models.NetworkIdentity) error {
	return r.db.WithContext(ctx).Create(identity).Error
}

// GetByID gets a network identity by ID
func (r *NetworkIdentityRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.NetworkIdentity, error) {
	var identity models.NetworkIdentity
	if err := r.db.WithContext(ctx).
		First(&identity, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &identity, nil
}

// GetByAddress gets a network identity by address
func (r *NetworkIdentityRepository) GetByAddress(ctx context.Context, address string) (*models.NetworkIdentity, error) {
	var identity models.NetworkIdentity
	if err := r.db.WithContext(ctx).
		Where("address = ?", address).
		First(&identity).Error; err != nil {
		return nil, err
	}
	return &identity, nil
}

// Update updates a network identity
func (r *NetworkIdentityRepository) Update(ctx context.Context, identity *models.NetworkIdentity) error {
	return r.db.WithContext(ctx).Save(identity).Error
}

// Delete deletes a network identity
func (r *NetworkIdentityRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Delete(&models.NetworkIdentity{}, "id = ?", id).Error
}

// ListByUser lists network identities for a user
func (r *NetworkIdentityRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.NetworkIdentity, error) {
	var identities []models.NetworkIdentity
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("is_primary DESC, created_at DESC").
		Find(&identities).Error; err != nil {
		return nil, err
	}
	return identities, nil
}

// ClearPrimary clears primary flag for all identities of a user in a network
func (r *NetworkIdentityRepository) ClearPrimary(ctx context.Context, userID uuid.UUID, network models.NetworkType) error {
	return r.db.WithContext(ctx).
		Model(&models.NetworkIdentity{}).
		Where("user_id = ? AND network = ?", userID, network).
		Update("is_primary", false).Error
}

// CountByUser counts network identities for a user
func (r *NetworkIdentityRepository) CountByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.NetworkIdentity{}).
		Where("user_id = ?", userID).
		Count(&count).Error
	return count, err
}

// CountByNetwork counts network identities by network type
func (r *NetworkIdentityRepository) CountByNetwork(ctx context.Context) (map[models.NetworkType]int64, error) {
	type result struct {
		Network models.NetworkType
		Count   int64
	}

	var results []result
	err := r.db.WithContext(ctx).
		Model(&models.NetworkIdentity{}).
		Select("network, COUNT(*) as count").
		Group("network").
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	counts := make(map[models.NetworkType]int64)
	for _, r := range results {
		counts[r.Network] = r.Count
	}

	return counts, nil
}
