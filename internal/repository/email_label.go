// internal/repository/email_label_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// EmailLabelRepository handles email-label relationship database operations
type EmailLabelRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewEmailLabelRepository creates a new email-label repository
func NewEmailLabelRepository(db *sqlx.DB, logger *zap.Logger) *EmailLabelRepository {
	return &EmailLabelRepository{
		db:     db,
		logger: logger,
	}
}

// AddLabelToEmail adds a label to an email
func (r *EmailLabelRepository) AddLabelToEmail(ctx context.Context, emailID, labelID uuid.UUID) error {
	query := `INSERT INTO email_labels (email_id, label_id, created_at) VALUES ($1, $2, NOW()) ON CONFLICT (email_id, label_id) DO NOTHING`
	_, err := r.db.ExecContext(ctx, query, emailID, labelID)
	return err
}

// RemoveLabelFromEmail removes a label from an email
func (r *EmailLabelRepository) RemoveLabelFromEmail(ctx context.Context, emailID, labelID uuid.UUID) error {
	query := `DELETE FROM email_labels WHERE email_id = $1 AND label_id = $2`
	_, err := r.db.ExecContext(ctx, query, emailID, labelID)
	return err
}

// GetLabelsForEmail gets all labels for an email
func (r *EmailLabelRepository) GetLabelsForEmail(ctx context.Context, emailID uuid.UUID) ([]uuid.UUID, error) {
	var labelIDs []uuid.UUID
	query := `SELECT label_id FROM email_labels WHERE email_id = $1`
	err := r.db.SelectContext(ctx, &labelIDs, query, emailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get labels for email: %w", err)
	}
	return labelIDs, nil
}

// GetEmailsForLabel gets all emails for a label
func (r *EmailLabelRepository) GetEmailsForLabel(ctx context.Context, labelID uuid.UUID, limit, offset int) ([]uuid.UUID, int64, error) {
	var emailIDs []uuid.UUID
	var total int64

	// Count query
	countQuery := `SELECT COUNT(*) FROM email_labels WHERE label_id = $1`
	err := r.db.GetContext(ctx, &total, countQuery, labelID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count emails for label: %w", err)
	}

	// List query
	listQuery := `SELECT email_id FROM email_labels WHERE label_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	err = r.db.SelectContext(ctx, &emailIDs, listQuery, labelID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get emails for label: %w", err)
	}

	return emailIDs, total, nil
}

// RemoveAllLabelsFromEmail removes all labels from an email
func (r *EmailLabelRepository) RemoveAllLabelsFromEmail(ctx context.Context, emailID uuid.UUID) error {
	query := `DELETE FROM email_labels WHERE email_id = $1`
	_, err := r.db.ExecContext(ctx, query, emailID)
	return err
}

// RemoveLabelFromAllEmails removes a label from all emails
func (r *EmailLabelRepository) RemoveLabelFromAllEmails(ctx context.Context, labelID uuid.UUID) error {
	query := `DELETE FROM email_labels WHERE label_id = $1`
	_, err := r.db.ExecContext(ctx, query, labelID)
	return err
}

// CountEmailsForLabel counts emails with a specific label
func (r *EmailLabelRepository) CountEmailsForLabel(ctx context.Context, labelID uuid.UUID) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM email_labels WHERE label_id = $1`
	err := r.db.GetContext(ctx, &count, query, labelID)
	if err != nil {
		return 0, fmt.Errorf("failed to count emails for label: %w", err)
	}
	return count, nil
}
