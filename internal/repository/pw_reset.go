// internal/repository/password_reset_repository.go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// PasswordResetRepository handles password reset token database operations
type PasswordResetRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewPasswordResetRepository creates a new password reset repository
func NewPasswordResetRepository(db *sqlx.DB, logger *zap.Logger) *PasswordResetRepository {
	return &PasswordResetRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new password reset token
func (r *PasswordResetRepository) Create(ctx context.Context, token *models.PasswordResetToken) error {
	query := `
		INSERT INTO password_reset_tokens (
			id, user_id, token, expires_at, used_at, created_at
		) VALUES (
			:id, :user_id, :token, :expires_at, :used_at, :created_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, token)
	return err
}

// GetByToken gets a password reset token by token string
func (r *PasswordResetRepository) GetByToken(ctx context.Context, token string) (*models.PasswordResetToken, error) {
	var resetToken models.PasswordResetToken
	query := `SELECT * FROM password_reset_tokens WHERE token = $1`
	err := r.db.GetContext(ctx, &resetToken, query, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get password reset token: %w", err)
	}
	return &resetToken, nil
}

// GetByUserID gets password reset tokens for a user
func (r *PasswordResetRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]models.PasswordResetToken, error) {
	var tokens []models.PasswordResetToken
	query := `SELECT * FROM password_reset_tokens WHERE user_id = $1 ORDER BY created_at DESC`
	err := r.db.SelectContext(ctx, &tokens, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get password reset tokens: %w", err)
	}
	return tokens, nil
}

// MarkAsUsed marks a token as used
func (r *PasswordResetRepository) MarkAsUsed(ctx context.Context, token string) error {
	query := `UPDATE password_reset_tokens SET used_at = NOW() WHERE token = $1`
	_, err := r.db.ExecContext(ctx, query, token)
	return err
}

// Delete deletes a password reset token
func (r *PasswordResetRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM password_reset_tokens WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// DeleteByUserID deletes all tokens for a user
func (r *PasswordResetRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM password_reset_tokens WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

// DeleteExpired deletes expired tokens
func (r *PasswordResetRepository) DeleteExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM password_reset_tokens WHERE expires_at < NOW() OR used_at IS NOT NULL`
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired tokens: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows, nil
}

// IsValid checks if a token is valid (not used and not expired)
func (r *PasswordResetRepository) IsValid(ctx context.Context, token string) (bool, error) {
	var valid bool
	query := `SELECT EXISTS(SELECT 1 FROM password_reset_tokens WHERE token = $1 AND used_at IS NULL AND expires_at > NOW())`
	err := r.db.GetContext(ctx, &valid, query, token)
	if err != nil {
		return false, fmt.Errorf("failed to check token validity: %w", err)
	}
	return valid, nil
}

// GetRecentAttempts gets recent password reset attempts for a user
func (r *PasswordResetRepository) GetRecentAttempts(ctx context.Context, userID uuid.UUID, within time.Duration) ([]models.PasswordResetToken, error) {
	var tokens []models.PasswordResetToken
	query := `
		SELECT * FROM password_reset_tokens 
		WHERE user_id = $1 AND created_at > $2 
		ORDER BY created_at DESC
	`
	cutoff := time.Now().Add(-within)
	err := r.db.SelectContext(ctx, &tokens, query, userID, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent attempts: %w", err)
	}
	return tokens, nil
}

// CleanupOldTokens cleans up old password reset tokens
func (r *PasswordResetRepository) CleanupOldTokens(ctx context.Context, olderThan time.Duration) (int64, error) {
	query := `DELETE FROM password_reset_tokens WHERE created_at < $1`
	cutoff := time.Now().Add(-olderThan)
	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old tokens: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows, nil
}
