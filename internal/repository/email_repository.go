package repository

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"gorm.io/gorm"
)

// EmailFilter defines email filtering options
type EmailFilter struct {
	UserID     uuid.UUID
	Limit      int
	Offset     int
	Search     string
	FolderID   *uuid.UUID
	LabelIDs   []uuid.UUID
	IsRead     *bool
	IsStarred  *bool
	IsArchived *bool
	IsSpam     *bool
	Network    *models.NetworkType
	FromDate   *time.Time
	ToDate     *time.Time
}

// EmailRepository handles email database operations
type EmailRepository struct {
	db *gorm.DB
}

// NewEmailRepository creates a new email repository
func NewEmailRepository(db *gorm.DB) *EmailRepository {
	return &EmailRepository{db: db}
}

// Create creates a new email
func (r *EmailRepository) Create(ctx context.Context, email *models.Email) error {
	return r.db.WithContext(ctx).Create(email).Error
}

// GetByID gets an email by ID
func (r *EmailRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Email, error) {
	var email models.Email
	if err := r.db.WithContext(ctx).
		Preload("Attachments").
		Preload("Labels").
		First(&email, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &email, nil
}

// List lists emails with filtering
func (r *EmailRepository) List(ctx context.Context, filter EmailFilter) ([]models.Email, int64, error) {
	var emails []models.Email
	query := r.db.WithContext(ctx).Model(&models.Email{}).
		Where("user_id = ?", filter.UserID)

	// Apply filters
	if filter.Search != "" {
		searchTerm := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where("LOWER(subject) LIKE ? OR LOWER(body_plain) LIKE ? OR LOWER(sender) LIKE ? OR LOWER(recipients::text) LIKE ?",
			searchTerm, searchTerm, searchTerm, searchTerm)
	}

	if filter.FolderID != nil {
		query = query.Where("folder_id = ?", *filter.FolderID)
	}

	if len(filter.LabelIDs) > 0 {
		query = query.Joins("JOIN email_labels ON email_labels.email_id = emails.id").
			Where("email_labels.label_id IN ?", filter.LabelIDs)
	}

	if filter.IsRead != nil {
		query = query.Where("is_read = ?", *filter.IsRead)
	}

	if filter.IsStarred != nil {
		query = query.Where("is_starred = ?", *filter.IsStarred)
	}

	if filter.IsArchived != nil {
		query = query.Where("is_archived = ?", *filter.IsArchived)
	}

	if filter.IsSpam != nil {
		query = query.Where("is_spam = ?", *filter.IsSpam)
	}

	if filter.Network != nil {
		query = query.Where("network = ?", *filter.Network)
	}

	if filter.FromDate != nil {
		query = query.Where("created_at >= ?", filter.FromDate)
	}

	if filter.ToDate != nil {
		query = query.Where("created_at <= ?", filter.ToDate)
	}

	// Get total count
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply limit and offset
	if err := query.
		Preload("Attachments").
		Preload("Labels").
		Order("created_at DESC").
		Offset(filter.Offset).
		Limit(filter.Limit).
		Find(&emails).Error; err != nil {
		return nil, 0, err
	}

	return emails, total, nil
}

// MarkAsRead marks an email as read or unread
func (r *EmailRepository) MarkAsRead(ctx context.Context, id uuid.UUID, read bool) error {
	return r.db.WithContext(ctx).
		Model(&models.Email{}).
		Where("id = ?", id).
		Update("is_read", read).Error
}

// MarkAsStarred marks an email as starred or unstarred
func (r *EmailRepository) MarkAsStarred(ctx context.Context, id uuid.UUID, starred bool) error {
	return r.db.WithContext(ctx).
		Model(&models.Email{}).
		Where("id = ?", id).
		Update("is_starred", starred).Error
}

// MoveToFolder moves an email to a folder
func (r *EmailRepository) MoveToFolder(ctx context.Context, id, folderID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Email{}).
		Where("id = ?", id).
		Update("folder_id", folderID).Error
}

// Delete soft deletes an email
func (r *EmailRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Email{}).
		Where("id = ?", id).
		Update("deleted_at", time.Now()).Error
}

// CountUnread counts unread emails for a user
func (r *EmailRepository) CountUnread(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Email{}).
		Where("user_id = ? AND is_read = ? AND deleted_at IS NULL", userID, false).
		Count(&count).Error
	return count, err
}

// CountByUser counts emails for a user
func (r *EmailRepository) CountByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&models.Email{}).
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Count(&count).Error
	return count, err
}

// DeleteOld deletes emails older than specified date
func (r *EmailRepository) DeleteOld(ctx context.Context, olderThan time.Time) (int64, error) {
	result := r.db.WithContext(ctx).
		Where("created_at < ?", olderThan).
		Delete(&models.Email{})
	return result.RowsAffected, result.Error
}

// AddLabel adds a label to an email
func (r *EmailRepository) AddLabel(ctx context.Context, emailID, labelID uuid.UUID) error {
	// Check if association already exists
	var count int64
	r.db.WithContext(ctx).
		Table("email_labels").
		Where("email_id = ? AND label_id = ?", emailID, labelID).
		Count(&count)

	if count > 0 {
		return nil // Already exists
	}

	return r.db.WithContext(ctx).
		Exec("INSERT INTO email_labels (email_id, label_id) VALUES (?, ?)", emailID, labelID).Error
}

// RemoveLabel removes a label from an email
func (r *EmailRepository) RemoveLabel(ctx context.Context, emailID, labelID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Exec("DELETE FROM email_labels WHERE email_id = ? AND label_id = ?", emailID, labelID).Error
}

// RemoveLabelFromAll removes a label from all emails
func (r *EmailRepository) RemoveLabelFromAll(ctx context.Context, labelID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Exec("DELETE FROM email_labels WHERE label_id = ?", labelID).Error
}

// CountByLabel counts emails with a specific label
func (r *EmailRepository) CountByLabel(ctx context.Context, labelID uuid.UUID) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Table("email_labels").
		Where("label_id = ?", labelID).
		Count(&count).Error
	return count, err
}

// MoveAllToFolder moves all emails from one folder to another
func (r *EmailRepository) MoveAllToFolder(ctx context.Context, fromFolderID, toFolderID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Email{}).
		Where("folder_id = ?", fromFolderID).
		Update("folder_id", toFolderID).Error
}

// Update updates an email
func (r *EmailRepository) Update(ctx context.Context, email *models.Email) error {
	return r.db.WithContext(ctx).Save(email).Error
}
