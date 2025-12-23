package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// RefreshToken generates a new access token
func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	// Validate refresh token
	claims, err := s.jwtService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return "", errors.New("invalid or expired refresh token")
	}

	// Extract user ID from claims
	userIDStr, ok := claims["user_id"].(string)
	if !ok {
		return "", errors.New("invalid token claims")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return "", errors.New("invalid user ID in token")
	}

	// Verify session is still valid
	sessionIDStr, ok := claims["session_id"].(string)
	if !ok {
		return "", errors.New("invalid session ID in token")
	}

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		return "", errors.New("invalid session ID format")
	}

	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return "", errors.New("session not found")
	}

	if session.RevokedAt != nil || session.ExpiresAt.Before(time.Now()) {
		return "", errors.New("session expired or revoked")
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", errors.New("user not found")
	}

	// Generate new access token
	newToken, err := s.jwtService.GenerateAccessToken(user.ID, session.ID)
	if err != nil {
		s.logger.Error("Failed to generate access token", zap.Error(err))
		return "", fmt.Errorf("failed to generate token")
	}

	return newToken, nil
}

// GetUserSessions returns all active sessions for a user
func (s *authService) GetUserSessions(ctx context.Context, userID uuid.UUID) ([]models.Session, error) {
	return s.sessionRepo.GetActiveSessionsByUser(ctx, userID)
}

// RevokeSession revokes a specific session
func (s *authService) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return errors.New("session not found")
	}

	if session.UserID != userID {
		return errors.New("session does not belong to user")
	}

	now := time.Now()
	session.ExpiresAt = now
	session.RevokedAt = &now

	return s.sessionRepo.Update(ctx, session)
}

// RevokeAllSessions revokes all sessions except current
func (s *authService) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	sessions, err := s.sessionRepo.GetActiveSessionsByUser(ctx, userID)
	if err != nil {
		return fmt.Errorf("failed to get sessions: %w", err)
	}

	now := time.Now()
	for _, session := range sessions {
		session.ExpiresAt = now
		session.RevokedAt = &now
		if err := s.sessionRepo.Update(ctx, &session); err != nil {
			s.logger.Error("Failed to revoke session",
				zap.Error(err),
				zap.String("session_id", session.ID.String()))
		}
	}

	return nil
}

// ValidateSession validates a session token
func (s *authService) ValidateSession(ctx context.Context, token string) (*models.Session, error) {
	// Validate JWT token
	claims, err := s.jwtService.ValidateAccessToken(token)
	if err != nil {
		return nil, errors.New("invalid or expired token")
	}

	// Extract session ID
	sessionIDStr, ok := claims["session_id"].(string)
	if !ok {
		return nil, errors.New("invalid token claims")
	}

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		return nil, errors.New("invalid session ID in token")
	}

	// Get session from repository
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return nil, errors.New("session not found")
	}

	if session.RevokedAt != nil || session.ExpiresAt.Before(time.Now()) {
		return nil, errors.New("invalid or expired token")
	}

	// Update last activity
	session.LastActivity = time.Now()
	if err := s.sessionRepo.Update(ctx, session); err != nil {
		s.logger.Error("Failed to update session activity", zap.Error(err))
	}

	return session, nil
}

// Helper method to create session
func (s *authService) createSession(ctx context.Context, user *models.User, device DeviceInfo) (*models.Session, string, error) {
	// Check session limit
	sessions, err := s.sessionRepo.GetActiveSessionsByUser(ctx, user.ID)
	if err == nil && len(sessions) >= s.maxSessionsPerUser {
		// Revoke oldest session
		oldest := sessions[0]
		now := time.Now()
		oldest.ExpiresAt = now
		oldest.RevokedAt = &now
		if err := s.sessionRepo.Update(ctx, &oldest); err != nil {
			s.logger.Warn("Failed to revoke oldest session", zap.Error(err))
		}
	}

	// Create session
	session := &models.Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		UserAgent:    device.UserAgent,
		IPAddress:    device.IPAddress,
		DeviceName:   device.DeviceName,
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
		ExpiresAt:    time.Now().Add(time.Duration(user.SessionTimeout) * time.Second),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	// Generate JWT token
	token, err := s.jwtService.GenerateAccessToken(user.ID, session.ID)
	if err != nil {
		// Clean up session if token generation fails
		s.sessionRepo.Delete(ctx, session.ID)
		return nil, "", fmt.Errorf("failed to generate token: %w", err)
	}

	return session, token, nil
}

// Helper method to record failed login attempt
func (s *authService) recordFailedAttempt(username string) {
	attempts, _ := s.loginAttempts.LoadOrStore(username, 0)
	count := attempts.(int) + 1
	s.loginAttempts.Store(username, count)

	if count >= s.maxLoginAttempts {
		s.logger.Warn("Account locked due to failed attempts",
			zap.String("username", username),
			zap.Int("attempts", count))
	}
}
