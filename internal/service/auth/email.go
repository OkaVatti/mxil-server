package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// VerifyEmail verifies email address
func (s *authService) VerifyEmail(ctx context.Context, token string) error {
	// Look up verification token
	user, err := s.userRepo.GetByVerificationToken(ctx, token)
	if err != nil {
		return errors.New("invalid or expired verification token")
	}

	// Check if token is expired
	if user.EmailVerificationTokenExpiry.Before(time.Now()) {
		return errors.New("verification token has expired")
	}

	// Verify email
	user.EmailVerified = true
	user.EmailVerificationToken = ""
	user.EmailVerificationTokenExpiry = time.Time{}

	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to verify email", zap.Error(err))
		return fmt.Errorf("failed to verify email")
	}

	s.logger.Info("Email verified successfully", zap.String("email", user.Email))
	return nil
}

// ResendVerification resends verification email
func (s *authService) ResendVerification(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// Don't reveal if user exists
		return nil
	}

	if user.EmailVerified {
		return nil // Already verified
	}

	// Generate new verification token
	token := generateSecureToken(32)
	user.EmailVerificationToken = token
	user.EmailVerificationTokenExpiry = time.Now().Add(24 * time.Hour)

	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to update verification token", zap.Error(err))
		return fmt.Errorf("failed to resend verification")
	}

	// TODO: Actually send verification email via email service
	s.logger.Info("Verification email resent", zap.String("email", email))

	return nil
}

// RequestPasswordReset initiates password reset
func (s *authService) RequestPasswordReset(ctx context.Context, email string) error {
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// Don't reveal if user exists
		s.logger.Info("Password reset requested", zap.String("email", email))
		return nil
	}

	// Generate secure reset token
	resetToken := generateSecureToken(64)
	user.PasswordResetToken = s.cryptoService.HashToken(resetToken)
	user.PasswordResetExpiry = time.Now().Add(1 * time.Hour)

	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to set reset token", zap.Error(err))
		return fmt.Errorf("failed to request password reset")
	}

	// TODO: Send reset email via email service
	s.logger.Info("Password reset requested",
		zap.String("email", email),
		zap.Time("expiry", user.PasswordResetExpiry))

	return nil
}

// ResetPassword resets user password
func (s *authService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Validate password strength
	if err := s.CheckPasswordStrength(newPassword); err != nil {
		return err
	}

	// Hash the token for comparison
	hashedToken := s.cryptoService.HashToken(token)

	// Find user by reset token
	user, err := s.userRepo.GetByResetToken(ctx, hashedToken)
	if err != nil {
		return errors.New("invalid or expired reset token")
	}

	// Check if token is expired
	if user.PasswordResetExpiry.Before(time.Now()) {
		return errors.New("reset token has expired")
	}

	// Hash new password
	hashedPassword, err := s.cryptoService.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Update password and clear reset token
	user.PasswordHash = hashedPassword
	user.PasswordResetToken = ""
	user.PasswordResetExpiry = time.Time{}

	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to reset password", zap.Error(err))
		return fmt.Errorf("failed to reset password")
	}

	// Revoke all existing sessions for security
	s.RevokeAllSessions(ctx, user.ID)

	s.logger.Info("Password reset successfully",
		zap.String("user_id", user.ID.String()))

	return nil
}

// ValidateResetToken checks if a reset token is valid
func (s *authService) ValidateResetToken(ctx context.Context, token string) (bool, error) {
	hashedToken := s.cryptoService.HashToken(token)

	user, err := s.userRepo.GetByResetToken(ctx, hashedToken)
	if err != nil {
		return false, nil
	}

	// Check if token is expired
	if user.PasswordResetExpiry.Before(time.Now()) {
		// Clean up expired token
		user.PasswordResetToken = ""
		user.PasswordResetExpiry = time.Time{}
		s.userRepo.Update(ctx, user)
		return false, nil
	}

	return true, nil
}

// Helper function to generate secure token
func generateSecureToken(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to UUID if crypto/rand fails
		return strings.ReplaceAll(uuid.New().String(), "-", "")[:length]
	}
	return hex.EncodeToString(bytes)[:length]
}
