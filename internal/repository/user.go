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

// UserRepository handles user database operations
type UserRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewUserRepository creates a new user repository
func NewUserRepository(db *sqlx.DB, logger *zap.Logger) *UserRepository {
	return &UserRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new user
func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	query := `
		INSERT INTO users (
			id, master_username, email, display_name, bio, password_hash,
			mfa_enabled, mfa_secret, recovery_codes, is_active, is_verified,
			verification_token, verification_expires_at, failed_login_attempts,
			locked_until, last_login, security_score, storage_quota_used,
			storage_quota_total, trusted_devices, auth_methods, metadata_minimization,
			logging_consent, analytics_opt_out, auto_delete_old_messages,
			retention_days, session_timeout, last_security_scan, ui_theme,
			accent_color, density, created_at, updated_at
		) VALUES (
			:id, :master_username, :email, :display_name, :bio, :password_hash,
			:mfa_enabled, :mfa_secret, :recovery_codes, :is_active, :is_verified,
			:verification_token, :verification_expires_at, :failed_login_attempts,
			:locked_until, :last_login, :security_score, :storage_quota_used,
			:storage_quota_total, :trusted_devices, :auth_methods, :metadata_minimization,
			:logging_consent, :analytics_opt_out, :auto_delete_old_messages,
			:retention_days, :session_timeout, :last_security_scan, :ui_theme,
			:accent_color, :density, :created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, user)
	return err
}

// GetByID gets a user by ID
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE id = $1`
	err := r.db.GetContext(ctx, &user, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	return &user, nil
}

// GetByUsername gets a user by username
func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE master_username = $1`
	err := r.db.GetContext(ctx, &user, query, username)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}
	return &user, nil
}

// GetByEmail gets a user by email
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	query := `SELECT * FROM users WHERE email = $1`
	err := r.db.GetContext(ctx, &user, query, email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

// Update updates a user
func (r *UserRepository) Update(ctx context.Context, user *models.User) error {
	query := `
		UPDATE users SET
			master_username = :master_username,
			email = :email,
			display_name = :display_name,
			bio = :bio,
			password_hash = :password_hash,
			mfa_enabled = :mfa_enabled,
			mfa_secret = :mfa_secret,
			recovery_codes = :recovery_codes,
			is_active = :is_active,
			is_verified = :is_verified,
			verification_token = :verification_token,
			verification_expires_at = :verification_expires_at,
			failed_login_attempts = :failed_login_attempts,
			locked_until = :locked_until,
			last_login = :last_login,
			security_score = :security_score,
			storage_quota_used = :storage_quota_used,
			storage_quota_total = :storage_quota_total,
			trusted_devices = :trusted_devices,
			auth_methods = :auth_methods,
			metadata_minimization = :metadata_minimization,
			logging_consent = :logging_consent,
			analytics_opt_out = :analytics_opt_out,
			auto_delete_old_messages = :auto_delete_old_messages,
			retention_days = :retention_days,
			session_timeout = :session_timeout,
			last_security_scan = :last_security_scan,
			ui_theme = :ui_theme,
			accent_color = :accent_color,
			density = :density,
			updated_at = :updated_at
		WHERE id = :id
	`

	_, err := r.db.NamedExecContext(ctx, query, user)
	return err
}

// Delete soft deletes a user
func (r *UserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET deleted_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// List lists users with filtering
func (r *UserRepository) List(ctx context.Context, activeOnly, verifiedOnly bool, limit, offset int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	// Count query
	countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	if activeOnly {
		countQuery += ` AND is_active = true`
	}
	if verifiedOnly {
		countQuery += ` AND is_verified = true`
	}
	err := r.db.GetContext(ctx, &total, countQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count users: %w", err)
	}

	// List query
	listQuery := `SELECT * FROM users WHERE deleted_at IS NULL`
	if activeOnly {
		listQuery += ` AND is_active = true`
	}
	if verifiedOnly {
		listQuery += ` AND is_verified = true`
	}
	listQuery += ` ORDER BY created_at DESC LIMIT $1 OFFSET $2`

	err = r.db.SelectContext(ctx, &users, listQuery, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list users: %w", err)
	}

	return users, total, nil
}

// Search searches users by query
func (r *UserRepository) Search(ctx context.Context, query string, limit, offset int) ([]models.User, int64, error) {
	var users []models.User
	var total int64

	searchQuery := `%` + query + `%`

	// Count query
	countQuery := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL AND (master_username ILIKE $1 OR email ILIKE $1 OR display_name ILIKE $1)`
	err := r.db.GetContext(ctx, &total, countQuery, searchQuery)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Search query
	listQuery := `SELECT * FROM users WHERE deleted_at IS NULL AND (master_username ILIKE $1 OR email ILIKE $1 OR display_name ILIKE $1) ORDER BY master_username ASC LIMIT $2 OFFSET $3`
	err = r.db.SelectContext(ctx, &users, listQuery, searchQuery, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search users: %w", err)
	}

	return users, total, nil
}

// UpdateLastLogin updates user's last login time
func (r *UserRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET last_login = NOW(), updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// UpdateSecurityScore updates user's security score
func (r *UserRepository) UpdateSecurityScore(ctx context.Context, id uuid.UUID, score int) error {
	query := `UPDATE users SET security_score = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, score, id)
	return err
}

// UpdateStorageUsage updates user's storage usage
func (r *UserRepository) UpdateStorageUsage(ctx context.Context, id uuid.UUID, used int64) error {
	query := `UPDATE users SET storage_quota_used = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, used, id)
	return err
}

// SetActive sets user active status
func (r *UserRepository) SetActive(ctx context.Context, id uuid.UUID, active bool) error {
	query := `UPDATE users SET is_active = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, active, id)
	return err
}

// SetVerified sets user verified status
func (r *UserRepository) SetVerified(ctx context.Context, id uuid.UUID, verified bool) error {
	query := `UPDATE users SET is_verified = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, verified, id)
	return err
}

// LockUser locks a user account
func (r *UserRepository) LockUser(ctx context.Context, id uuid.UUID, until time.Time) error {
	query := `UPDATE users SET locked_until = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, until, id)
	return err
}

// UnlockUser unlocks a user account
func (r *UserRepository) UnlockUser(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET locked_until = NULL, failed_login_attempts = 0, updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// IncrementFailedLogin increments failed login attempts
func (r *UserRepository) IncrementFailedLogin(ctx context.Context, id uuid.UUID) (int, error) {
	// Get current count
	var currentCount int
	err := r.db.GetContext(ctx, &currentCount, `SELECT failed_login_attempts FROM users WHERE id = $1`, id)
	if err != nil {
		return 0, fmt.Errorf("failed to get failed login attempts: %w", err)
	}

	// Increment
	newCount := currentCount + 1
	query := `UPDATE users SET failed_login_attempts = $1, updated_at = NOW() WHERE id = $2`
	_, err = r.db.ExecContext(ctx, query, newCount, id)
	if err != nil {
		return 0, fmt.Errorf("failed to increment failed login attempts: %w", err)
	}

	return newCount, nil
}

// ResetFailedLogin resets failed login attempts
func (r *UserRepository) ResetFailedLogin(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE users SET failed_login_attempts = 0, updated_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// UpdatePassword updates user password
func (r *UserRepository) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) error {
	query := `UPDATE users SET password_hash = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, passwordHash, id)
	return err
}

// UpdateMFA updates user MFA settings
func (r *UserRepository) UpdateMFA(ctx context.Context, id uuid.UUID, enabled bool, secret string) error {
	query := `UPDATE users SET mfa_enabled = $1, mfa_secret = $2, updated_at = NOW() WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, enabled, secret, id)
	return err
}

// GetUserStats gets user statistics
func (r *UserRepository) GetUserStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total users
	var total int64
	err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("failed to get total users: %w", err)
	}
	stats["total"] = total

	// Active users
	var active int64
	err = r.db.GetContext(ctx, &active, `SELECT COUNT(*) FROM users WHERE is_active = true AND deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users: %w", err)
	}
	stats["active"] = active

	// Verified users
	var verified int64
	err = r.db.GetContext(ctx, &verified, `SELECT COUNT(*) FROM users WHERE is_verified = true AND deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("failed to get verified users: %w", err)
	}
	stats["verified"] = verified

	// Users with MFA
	var mfaEnabled int64
	err = r.db.GetContext(ctx, &mfaEnabled, `SELECT COUNT(*) FROM users WHERE mfa_enabled = true AND deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("failed to get MFA enabled users: %w", err)
	}
	stats["mfa_enabled"] = mfaEnabled

	// New users today
	var newToday int64
	today := time.Now().Truncate(24 * time.Hour)
	err = r.db.GetContext(ctx, &newToday, `SELECT COUNT(*) FROM users WHERE created_at >= $1 AND deleted_at IS NULL`, today)
	if err != nil {
		return nil, fmt.Errorf("failed to get new users today: %w", err)
	}
	stats["new_today"] = newToday

	// New users this week
	var newWeek int64
	weekAgo := time.Now().AddDate(0, 0, -7)
	err = r.db.GetContext(ctx, &newWeek, `SELECT COUNT(*) FROM users WHERE created_at >= $1 AND deleted_at IS NULL`, weekAgo)
	if err != nil {
		return nil, fmt.Errorf("failed to get new users this week: %w", err)
	}
	stats["new_week"] = newWeek

	// Average security score
	var avgScore float64
	err = r.db.GetContext(ctx, &avgScore, `SELECT AVG(security_score) FROM users WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("failed to get average security score: %w", err)
	}
	stats["avg_security_score"] = avgScore

	// Storage usage
	var totalStorage, usedStorage int64
	err = r.db.GetContext(ctx, &totalStorage, `SELECT COALESCE(SUM(storage_quota_total), 0) FROM users WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("failed to get total storage: %w", err)
	}
	stats["total_storage"] = totalStorage

	err = r.db.GetContext(ctx, &usedStorage, `SELECT COALESCE(SUM(storage_quota_used), 0) FROM users WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("failed to get used storage: %w", err)
	}
	stats["used_storage"] = usedStorage

	if totalStorage > 0 {
		stats["storage_percentage"] = float64(usedStorage) / float64(totalStorage) * 100
	}

	// User growth over time
	type GrowthData struct {
		Date  string `db:"date"`
		Count int64  `db:"count"`
	}
	var growthData []GrowthData
	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as count
		FROM users
		WHERE created_at >= NOW() - INTERVAL '30 days'
		AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date
	`
	err = r.db.SelectContext(ctx, &growthData, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get growth data: %w", err)
	}
	stats["growth_data"] = growthData

	return stats, nil
}

// CheckUsernameExists checks if username exists
func (r *UserRepository) CheckUsernameExists(ctx context.Context, username string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE master_username = $1 AND deleted_at IS NULL)`
	err := r.db.GetContext(ctx, &exists, query, username)
	if err != nil {
		return false, fmt.Errorf("failed to check username exists: %w", err)
	}
	return exists, nil
}

// CheckEmailExists checks if email exists
func (r *UserRepository) CheckEmailExists(ctx context.Context, email string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE email = $1 AND deleted_at IS NULL)`
	err := r.db.GetContext(ctx, &exists, query, email)
	if err != nil {
		return false, fmt.Errorf("failed to check email exists: %w", err)
	}
	return exists, nil
}
