package repository

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"gorm.io/gorm"
)

// UserRepository handles user database operations
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// GetByID gets a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).
		Preload("NetworkIdentities").
		Preload("AuthMethods").
		Preload("TrustedDevices").
		First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByUsername gets a user by username
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).
		Where("master_username = ?", username).
		First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// GetByEmail gets a user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	if err := r.db.WithContext(ctx).
		Where("email = ?", email).
		First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// Update updates a user
func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// Delete soft deletes a user
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Update("deleted_at", time.Now()).Error
}

// ListWithFilter lists users with filters
func (r *UserRepository) ListWithFilter(ctx context.Context, limit, offset int, search string, activeOnly, verifiedOnly bool) ([]models.User, int64, error) {
	var users []models.User
	query := r.db.WithContext(ctx).Model(&models.User{})

	if search != "" {
		searchTerm := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(master_username) LIKE ? OR LOWER(display_name) LIKE ? OR LOWER(email) LIKE ?",
			searchTerm, searchTerm, searchTerm)
	}

	if activeOnly {
		query = query.Where("deleted_at IS NULL")
	}

	if verifiedOnly {
		query = query.Where("email_verified = ?", true)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply limit and offset
	if err := query.
		Order("created_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&users).Error; err != nil {
		return nil, 0, err
	}

	return users, total, nil
}

// GetStorageUsage gets user storage usage
func (r *UserRepository) GetStorageUsage(ctx context.Context, userID uuid.UUID) (used int64, total int64, err error) {
	var user models.User
	if err := r.db.WithContext(ctx).
		Select("storage_quota_used", "storage_quota_total").
		First(&user, "id = ?", userID).Error; err != nil {
		return 0, 0, err
	}
	return user.StorageQuotaUsed, user.StorageQuotaTotal, nil
}

// UpdateStorageUsage updates user storage usage
func (r *UserRepository) UpdateStorageUsage(ctx context.Context, userID uuid.UUID, used int64) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("storage_quota_used", used).Error
}

// UpdateLastLogin updates user's last login time
func (r *UserRepository) UpdateLastLogin(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("last_login", time.Now()).Error
}

// IncrementLoginAttempts increments failed login attempts
func (r *UserRepository) IncrementLoginAttempts(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("failed_login_attempts", gorm.Expr("failed_login_attempts + ?", 1)).Error
}

// ResetLoginAttempts resets failed login attempts
func (r *UserRepository) ResetLoginAttempts(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Updates(map[string]interface{}{
			"failed_login_attempts": 0,
			"locked_until":          nil,
		}).Error
}

// LockAccount locks an account
func (r *UserRepository) LockAccount(ctx context.Context, userID uuid.UUID, until time.Time) error {
	return r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", userID).
		Update("locked_until", until).Error
}
