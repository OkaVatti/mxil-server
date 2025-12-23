package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"gorm.io/gorm"
)

// LabelRepository handles label database operations
type LabelRepository struct {
	db *gorm.DB
}

// NewLabelRepository creates a new label repository
func NewLabelRepository(db *gorm.DB) *LabelRepository {
	return &LabelRepository{db: db}
}

// Create creates a new label
func (r *LabelRepository) Create(ctx context.Context, label *models.Label) error {
	return r.db.WithContext(ctx).Create(label).Error
}

// GetByID gets a label by ID
func (r *LabelRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Label, error) {
	var label models.Label
	if err := r.db.WithContext(ctx).
		First(&label, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &label, nil
}

// GetByName gets a label by name for a user
func (r *LabelRepository) GetByName(ctx context.Context, userID uuid.UUID, name string) (*models.Label, error) {
	var label models.Label
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND name = ?", userID, name).
		First(&label).Error; err != nil {
		return nil, err
	}
	return &label, nil
}

// Update updates a label
func (r *LabelRepository) Update(ctx context.Context, label *models.Label) error {
	return r.db.WithContext(ctx).Save(label).Error
}

// Delete deletes a label
func (r *LabelRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Delete(&models.Label{}, "id = ?", id).Error
}

// ListByUser lists labels for a user
func (r *LabelRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Label, error) {
	var labels []models.Label
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("name ASC").
		Find(&labels).Error; err != nil {
		return nil, err
	}
	return labels, nil
}
