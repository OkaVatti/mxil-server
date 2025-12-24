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

// SessionRepository handles session database operations
type SessionRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *sqlx.DB, logger *zap.Logger) *SessionRepository {
	return &SessionRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new session
func (r *SessionRepository) Create(ctx context.Context, session *models.Session) error {
	query := `
		INSERT INTO sessions (
			id, user_id, token, user_agent, ip_address, device_fingerprint,
			expires_at, last_activity, created_at
		) VALUES (
			:id, :user_id, :token, :user_agent, :ip_address, :device_fingerprint,
			:expires_at, :last_activity, :created_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, session)
	return err
}

// GetByID gets a session by ID
func (r *SessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Session, error) {
	var session models.Session
	query := `SELECT * FROM sessions WHERE id = $1`
	err := r.db.GetContext(ctx, &session, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	return &session, nil
}

// GetByToken gets a session by token
func (r *SessionRepository) GetByToken(ctx context.Context, token string) (*models.Session, error) {
	var session models.Session
	query := `SELECT * FROM sessions WHERE token = $1`
	err := r.db.GetContext(ctx, &session, query, token)
	if err != nil {
		return nil, fmt.Errorf("failed to get session by token: %w", err)
	}
	return &session, nil
}

// Update updates a session
func (r *SessionRepository) Update(ctx context.Context, session *models.Session) error {
	query := `
		UPDATE sessions SET
			token = :token,
			user_agent = :user_agent,
			ip_address = :ip_address,
			device_fingerprint = :device_fingerprint,
			expires_at = :expires_at,
			last_activity = :last_activity
		WHERE id = :id
	`

	_, err := r.db.NamedExecContext(ctx, query, session)
	return err
}

// Delete deletes a session
func (r *SessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM sessions WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// ListByUser lists sessions for a user
func (r *SessionRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Session, error) {
	var sessions []models.Session
	query := `SELECT * FROM sessions WHERE user_id = $1 ORDER BY last_activity DESC LIMIT $2 OFFSET $3`
	err := r.db.SelectContext(ctx, &sessions, query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}
	return sessions, nil
}

// UpdateLastActivity updates session last activity
func (r *SessionRepository) UpdateLastActivity(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE sessions SET last_activity = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// DeleteExpired deletes expired sessions
func (r *SessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	query := `DELETE FROM sessions WHERE expires_at < NOW()`
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired sessions: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows, nil
}

// DeleteByUser deletes all sessions for a user
func (r *SessionRepository) DeleteByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	query := `DELETE FROM sessions WHERE user_id = $1`
	result, err := r.db.ExecContext(ctx, query, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to delete user sessions: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows, nil
}

// DeleteOldInactive deletes old inactive sessions
func (r *SessionRepository) DeleteOldInactive(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	query := `DELETE FROM sessions WHERE last_activity < $1`
	result, err := r.db.ExecContext(ctx, query, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old inactive sessions: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows, nil
}

// CountByUser counts sessions for a user
func (r *SessionRepository) CountByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM sessions WHERE user_id = $1`
	err := r.db.GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to count sessions: %w", err)
	}
	return count, nil
}

// GetActiveSessions gets active sessions (not expired)
func (r *SessionRepository) GetActiveSessions(ctx context.Context, limit int) ([]models.Session, error) {
	var sessions []models.Session
	query := `SELECT * FROM sessions WHERE expires_at > NOW() ORDER BY last_activity DESC LIMIT $1`
	err := r.db.SelectContext(ctx, &sessions, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}
	return sessions, nil
}

// GetSessionStats gets session statistics
func (r *SessionRepository) GetSessionStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total sessions
	var total int64
	err := r.db.GetContext(ctx, &total, `SELECT COUNT(*) FROM sessions`)
	if err != nil {
		return nil, fmt.Errorf("failed to get total sessions: %w", err)
	}
	stats["total_sessions"] = total

	// Active sessions
	var active int64
	err = r.db.GetContext(ctx, &active, `SELECT COUNT(*) FROM sessions WHERE expires_at > NOW()`)
	if err != nil {
		return nil, fmt.Errorf("failed to get active sessions: %w", err)
	}
	stats["active_sessions"] = active

	// Expired sessions
	var expired int64
	err = r.db.GetContext(ctx, &expired, `SELECT COUNT(*) FROM sessions WHERE expires_at <= NOW()`)
	if err != nil {
		return nil, fmt.Errorf("failed to get expired sessions: %w", err)
	}
	stats["expired_sessions"] = expired

	// Average session duration (in seconds)
	var avgDuration float64
	err = r.db.GetContext(ctx, &avgDuration, `
		SELECT AVG(EXTRACT(EPOCH FROM (expires_at - created_at)))
		FROM sessions
		WHERE expires_at <= NOW()
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to get average session duration: %w", err)
	}
	stats["avg_session_duration_seconds"] = avgDuration

	// Sessions by user agent (top 10)
	type UserAgentStat struct {
		UserAgent string `db:"user_agent"`
		Count     int64  `db:"count"`
	}
	var userAgentStats []UserAgentStat
	query := `
		SELECT 
			COALESCE(user_agent, 'unknown') as user_agent,
			COUNT(*) as count
		FROM sessions
		GROUP BY user_agent
		ORDER BY count DESC
		LIMIT 10
	`
	err = r.db.SelectContext(ctx, &userAgentStats, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get user agent stats: %w", err)
	}
	stats["user_agent_stats"] = userAgentStats

	// Sessions created today
	var today int64
	todayStart := time.Now().Truncate(24 * time.Hour)
	err = r.db.GetContext(ctx, &today, `SELECT COUNT(*) FROM sessions WHERE created_at >= $1`, todayStart)
	if err != nil {
		return nil, fmt.Errorf("failed to get today's sessions: %w", err)
	}
	stats["sessions_today"] = today

	// Sessions created this week
	var week int64
	weekStart := time.Now().AddDate(0, 0, -7)
	err = r.db.GetContext(ctx, &week, `SELECT COUNT(*) FROM sessions WHERE created_at >= $1`, weekStart)
	if err != nil {
		return nil, fmt.Errorf("failed to get this week's sessions: %w", err)
	}
	stats["sessions_week"] = week

	// Inactive sessions (older than 30 days)
	var inactive int64
	inactiveCutoff := time.Now().AddDate(0, 0, -30)
	err = r.db.GetContext(ctx, &inactive, `SELECT COUNT(*) FROM sessions WHERE last_activity < $1`, inactiveCutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to get inactive sessions: %w", err)
	}
	stats["inactive_sessions"] = inactive

	return stats, nil
}
