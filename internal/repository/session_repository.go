package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"gorm.io/gorm"
)

// SessionRepository handles session database operations
type SessionRepository struct {
	db *gorm.DB
}

// NewSessionRepository creates a new session repository
func NewSessionRepository(db *gorm.DB) *SessionRepository {
	return &SessionRepository{db: db}
}

// Create creates a new session
func (r *SessionRepository) Create(ctx context.Context, session *models.Session) error {
	return r.db.WithContext(ctx).Create(session).Error
}

// GetByID gets a session by ID
func (r *SessionRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Session, error) {
	var session models.Session
	if err := r.db.WithContext(ctx).
		First(&session, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// GetByToken gets a session by token
func (r *SessionRepository) GetByToken(ctx context.Context, tokenHash string) (*models.Session, error) {
	var session models.Session
	if err := r.db.WithContext(ctx).
		Where("token_hash = ? AND expires_at > ?", tokenHash, time.Now()).
		First(&session).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

// Update updates a session
func (r *SessionRepository) Update(ctx context.Context, session *models.Session) error {
	return r.db.WithContext(ctx).Save(session).Error
}

// Delete deletes a session
func (r *SessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Delete(&models.Session{}, "id = ?", id).Error
}

// DeleteByUserID deletes all sessions for a user
func (r *SessionRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Delete(&models.Session{}, "user_id = ?", userID).Error
}

// DeleteExpired deletes expired sessions
func (r *SessionRepository) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).
		Where("expires_at <= ?", time.Now()).
		Delete(&models.Session{}).Error
}

// ListByUser lists all sessions for a user
func (r *SessionRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Session, error) {
	var sessions []models.Session
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND expires_at > ?", userID, time.Now()).
		Order("last_activity DESC").
		Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

// UpdateLastActivity updates the last activity time of a session
func (r *SessionRepository) UpdateLastActivity(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Model(&models.Session{}).
		Where("id = ?", id).
		Update("last_activity", time.Now()).Error
}
