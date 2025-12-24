// internal/repository/label_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// LabelRepository handles label database operations
type LabelRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewLabelRepository creates a new label repository
func NewLabelRepository(db *sqlx.DB, logger *zap.Logger) *LabelRepository {
	return &LabelRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new label
func (r *LabelRepository) Create(ctx context.Context, label *models.Label) error {
	query := `
		INSERT INTO labels (
			id, user_id, name, color, icon, is_system, email_count,
			created_at, updated_at
		) VALUES (
			:id, :user_id, :name, :color, :icon, :is_system, :email_count,
			:created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, label)
	return err
}

// GetByID gets a label by ID
func (r *LabelRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Label, error) {
	var label models.Label
	query := `SELECT * FROM labels WHERE id = $1`
	err := r.db.GetContext(ctx, &label, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get label: %w", err)
	}
	return &label, nil
}

// GetByName gets a label by user ID and name
func (r *LabelRepository) GetByName(ctx context.Context, userID uuid.UUID, name string) (*models.Label, error) {
	var label models.Label
	query := `SELECT * FROM labels WHERE user_id = $1 AND name = $2`
	err := r.db.GetContext(ctx, &label, query, userID, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get label: %w", err)
	}
	return &label, nil
}

// Update updates a label
func (r *LabelRepository) Update(ctx context.Context, label *models.Label) error {
	query := `
		UPDATE labels SET
			name = :name,
			color = :color,
			icon = :icon,
			email_count = :email_count,
			updated_at = :updated_at
		WHERE id = :id
	`

	_, err := r.db.NamedExecContext(ctx, query, label)
	return err
}

// Delete deletes a label (only if not system label)
func (r *LabelRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM labels WHERE id = $1 AND is_system = false`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// ListByUser lists labels for a user
func (r *LabelRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Label, error) {
	var labels []models.Label
	query := `SELECT * FROM labels WHERE user_id = $1 ORDER BY is_system DESC, name ASC`
	err := r.db.SelectContext(ctx, &labels, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list labels: %w", err)
	}
	return labels, nil
}

// ApplyToEmail applies a label to an email
func (r *LabelRepository) ApplyToEmail(ctx context.Context, emailID, labelID uuid.UUID) error {
	query := `INSERT INTO email_labels (email_id, label_id, created_at) VALUES ($1, $2, NOW()) ON CONFLICT (email_id, label_id) DO NOTHING`
	_, err := r.db.ExecContext(ctx, query, emailID, labelID)
	if err != nil {
		return fmt.Errorf("failed to apply label to email: %w", err)
	}

	// Update label count
	updateQuery := `UPDATE labels SET email_count = email_count + 1, updated_at = NOW() WHERE id = $1`
	_, err = r.db.ExecContext(ctx, updateQuery, labelID)
	return err
}

// RemoveFromEmail removes a label from an email
func (r *LabelRepository) RemoveFromEmail(ctx context.Context, emailID, labelID uuid.UUID) error {
	query := `DELETE FROM email_labels WHERE email_id = $1 AND label_id = $2`
	result, err := r.db.ExecContext(ctx, query, emailID, labelID)
	if err != nil {
		return fmt.Errorf("failed to remove label from email: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows > 0 {
		// Update label count
		updateQuery := `UPDATE labels SET email_count = GREATEST(0, email_count - 1), updated_at = NOW() WHERE id = $1`
		_, err = r.db.ExecContext(ctx, updateQuery, labelID)
	}
	return err
}

// GetEmailLabels gets labels for an email
func (r *LabelRepository) GetEmailLabels(ctx context.Context, emailID uuid.UUID) ([]models.Label, error) {
	var labels []models.Label
	query := `
		SELECT l.* FROM labels l
		INNER JOIN email_labels el ON l.id = el.label_id
		WHERE el.email_id = $1
		ORDER BY l.name ASC
	`
	err := r.db.SelectContext(ctx, &labels, query, emailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get email labels: %w", err)
	}
	return labels, nil
}

// GetEmailsByLabel gets emails with a specific label
func (r *LabelRepository) GetEmailsByLabel(ctx context.Context, userID, labelID uuid.UUID, limit, offset int) ([]models.Email, int64, error) {
	var emails []models.Email
	var total int64

	// Count query
	countQuery := `
		SELECT COUNT(DISTINCT e.id) FROM emails e
		INNER JOIN email_labels el ON e.id = el.email_id
		WHERE e.user_id = $1 AND el.label_id = $2 AND e.deleted_at IS NULL
	`
	err := r.db.GetContext(ctx, &total, countQuery, userID, labelID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count labeled emails: %w", err)
	}

	// List query
	listQuery := `
		SELECT DISTINCT e.* FROM emails e
		INNER JOIN email_labels el ON e.id = el.email_id
		WHERE e.user_id = $1 AND el.label_id = $2 AND e.deleted_at IS NULL
		ORDER BY e.received_at DESC
		LIMIT $3 OFFSET $4
	`
	err = r.db.SelectContext(ctx, &emails, listQuery, userID, labelID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get labeled emails: %w", err)
	}

	return emails, total, nil
}

// UpdateCount updates label email count
func (r *LabelRepository) UpdateCount(ctx context.Context, labelID uuid.UUID) error {
	query := `
		UPDATE labels SET 
			email_count = (SELECT COUNT(*) FROM email_labels WHERE label_id = $1),
			updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, labelID)
	return err
}

// GetSystemLabels gets system labels for a user
func (r *LabelRepository) GetSystemLabels(ctx context.Context, userID uuid.UUID) ([]models.Label, error) {
	var labels []models.Label
	query := `SELECT * FROM labels WHERE user_id = $1 AND is_system = true ORDER BY name ASC`
	err := r.db.SelectContext(ctx, &labels, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get system labels: %w", err)
	}
	return labels, nil
}

// CheckNameExists checks if a label name already exists for user
func (r *LabelRepository) CheckNameExists(ctx context.Context, userID uuid.UUID, name string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM labels WHERE user_id = $1 AND name = $2)`
	err := r.db.GetContext(ctx, &exists, query, userID, name)
	if err != nil {
		return false, fmt.Errorf("failed to check name exists: %w", err)
	}
	return exists, nil
}
