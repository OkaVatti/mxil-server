package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"gorm.io/gorm"
)

// ProviderBridgeRepository handles provider bridge database operations
type ProviderBridgeRepository struct {
	db *gorm.DB
}

// NewProviderBridgeRepository creates a new provider bridge repository
func NewProviderBridgeRepository(db *gorm.DB) *ProviderBridgeRepository {
	return &ProviderBridgeRepository{db: db}
}

// Create creates a new provider bridge
func (r *ProviderBridgeRepository) Create(ctx context.Context, provider *models.ProviderBridge) error {
	return r.db.WithContext(ctx).Create(provider).Error
}

// GetByID gets a provider bridge by ID
func (r *ProviderBridgeRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.ProviderBridge, error) {
	var provider models.ProviderBridge
	if err := r.db.WithContext(ctx).
		First(&provider, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

// GetByAccount gets a provider bridge by account identifier
func (r *ProviderBridgeRepository) GetByAccount(ctx context.Context, userID uuid.UUID, providerType models.ProviderType, accountIdentifier string) (*models.ProviderBridge, error) {
	var provider models.ProviderBridge
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND provider_type = ? AND account_identifier = ?",
			userID, providerType, accountIdentifier).
		First(&provider).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

// Update updates a provider bridge
func (r *ProviderBridgeRepository) Update(ctx context.Context, provider *models.ProviderBridge) error {
	return r.db.WithContext(ctx).Save(provider).Error
}

// Delete deletes a provider bridge
func (r *ProviderBridgeRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Delete(&models.ProviderBridge{}, "id = ?", id).Error
}

// ListByUser lists provider bridges for a user
func (r *ProviderBridgeRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.ProviderBridge, error) {
	var providers []models.ProviderBridge
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&providers).Error; err != nil {
		return nil, err
	}
	return providers, nil
}

// UpdateLastSync updates the last sync time of a provider bridge
func (r *ProviderBridgeRepository) UpdateLastSync(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).
		Model(&models.ProviderBridge{}).
		Where("id = ?", id).
		Update("last_sync", &now).Error
}

// GetSyncStats gets sync statistics for a provider bridge
func (r *ProviderBridgeRepository) GetSyncStats(ctx context.Context, providerID uuid.UUID) (*SyncStats, error) {
	// This would typically query a separate sync log table
	// For now, we'll return basic information
	var provider models.ProviderBridge
	if err := r.db.WithContext(ctx).
		First(&provider, "id = ?", providerID).Error; err != nil {
		return nil, err
	}

	return &SyncStats{
		TotalEmails:  0, // Would need separate tracking
		LastSync:     provider.LastSync,
		SyncDuration: 0,
		SuccessRate:  0,
		ErrorCount:   0,
	}, nil
}

// SyncStats contains sync statistics
type SyncStats struct {
	TotalEmails  int64      `json:"total_emails"`
	LastSync     *time.Time `json:"last_sync"`
	SyncDuration int64      `json:"sync_duration"` // in seconds
	SuccessRate  float64    `json:"success_rate"`
	ErrorCount   int64      `json:"error_count"`
}
