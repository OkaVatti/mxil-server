// internal/repository/attachment_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// AttachmentRepository handles attachment database operations
type AttachmentRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewAttachmentRepository creates a new attachment repository
func NewAttachmentRepository(db *sqlx.DB, logger *zap.Logger) *AttachmentRepository {
	return &AttachmentRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new attachment
func (r *AttachmentRepository) Create(ctx context.Context, attachment *models.Attachment) error {
	query := `
		INSERT INTO attachments (
			id, email_id, user_id, filename, content_type, size,
			storage_path, storage_backend, checksum, is_inline, created_at
		) VALUES (
			:id, :email_id, :user_id, :filename, :content_type, :size,
			:storage_path, :storage_backend, :checksum, :is_inline, :created_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, attachment)
	return err
}

// GetByID gets an attachment by ID
func (r *AttachmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Attachment, error) {
	var attachment models.Attachment
	query := `SELECT * FROM attachments WHERE id = $1`
	err := r.db.GetContext(ctx, &attachment, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachment: %w", err)
	}
	return &attachment, nil
}

// GetByEmailID gets attachments by email ID
func (r *AttachmentRepository) GetByEmailID(ctx context.Context, emailID uuid.UUID) ([]models.Attachment, error) {
	var attachments []models.Attachment
	query := `SELECT * FROM attachments WHERE email_id = $1 ORDER BY created_at`
	err := r.db.SelectContext(ctx, &attachments, query, emailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}
	return attachments, nil
}

// GetByUserID gets attachments by user ID
func (r *AttachmentRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Attachment, error) {
	var attachments []models.Attachment
	query := `SELECT * FROM attachments WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	err := r.db.SelectContext(ctx, &attachments, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}
	return attachments, nil
}

// Delete deletes an attachment
func (r *AttachmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM attachments WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// DeleteByEmailID deletes attachments by email ID
func (r *AttachmentRepository) DeleteByEmailID(ctx context.Context, emailID uuid.UUID) error {
	query := `DELETE FROM attachments WHERE email_id = $1`
	_, err := r.db.ExecContext(ctx, query, emailID)
	return err
}

// CountByUser counts attachments by user
func (r *AttachmentRepository) CountByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM attachments WHERE user_id = $1`
	err := r.db.GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to count attachments: %w", err)
	}
	return count, nil
}

// GetStorageUsage gets storage usage by user
func (r *AttachmentRepository) GetStorageUsage(ctx context.Context, userID uuid.UUID) (int64, error) {
	var totalSize int64
	query := `SELECT COALESCE(SUM(size), 0) FROM attachments WHERE user_id = $1`
	err := r.db.GetContext(ctx, &totalSize, query, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get storage usage: %w", err)
	}
	return totalSize, nil
}
