// internal/service/auth/auth_service.go
package auth

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/auth"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service"
)

// AuthService implements the auth service interface
type AuthService struct {
	userRepo          *repository.UserRepository
	sessionRepo       *repository.SessionRepository
	passwordResetRepo *repository.PasswordResetRepository
	jwtService        *auth.JWTService
	cryptoService     service.CryptoService
	lockoutDuration   time.Duration
	maxLoginAttempts  int
	logger            *zap.Logger
}

// NewAuthService creates a new auth service
func NewAuthService(
	userRepo *repository.UserRepository,
	sessionRepo *repository.SessionRepository,
	passwordResetRepo *repository.PasswordResetRepository,
	jwtService *auth.JWTService,
	cryptoService service.CryptoService,
	lockoutDuration time.Duration,
	maxLoginAttempts int,
	logger *zap.Logger,
) *AuthService {
	return &AuthService{
		userRepo:          userRepo,
		sessionRepo:       sessionRepo,
		passwordResetRepo: passwordResetRepo,
		jwtService:        jwtService,
		cryptoService:     cryptoService,
		lockoutDuration:   lockoutDuration,
		maxLoginAttempts:  maxLoginAttempts,
		logger:            logger,
	}
}

// Register registers a new user
func (s *AuthService) Register(ctx context.Context, req service.RegisterRequest) (*service.RegisterResponse, error) {
	// Check if user exists
	existing, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err == nil && existing != nil {
		return nil, service.ErrUserExists
	}

	// Check email
	existing, err = s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return nil, service.ErrUserExists
	}

	// Hash password
	hashedPassword, err := s.cryptoService.HashPassword(req.Password)
	if err != nil {
		s.logger.Error("Failed to hash password", zap.Error(err))
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &models.User{
		ID:                uuid.New(),
		MasterUsername:    req.Username,
		Email:             req.Email,
		DisplayName:       req.DisplayName,
		PasswordHash:      hashedPassword,
		IsActive:          true,
		MFAEnabled:        false,
		CreatedAt:         time.Now(),
		SecurityScore:     50,         // Default security score
		StorageQuotaTotal: 1073741824, // 1GB default
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("Failed to create user", zap.Error(err))
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Generate JWT token
	token, err := s.jwtService.GenerateToken(user.ID, user.MasterUsername)
	if err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create session
	session := &models.Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		Token:        token,
		UserAgent:    "", // Will be set by handler
		IPAddress:    "", // Will be set by handler
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		s.logger.Error("Failed to create session", zap.Error(err))
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &service.RegisterResponse{
		User:      user,
		Token:     token,
		SessionID: session.ID,
	}, nil
}

// Login authenticates a user
func (s *AuthService) Login(ctx context.Context, req service.LoginRequest) (*service.LoginResponse, error) {
	// Get user
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		// Also try by email
		user, err = s.userRepo.GetByEmail(ctx, req.Username)
		if err != nil {
			s.logger.Debug("User not found", zap.String("username", req.Username))
			return nil, service.ErrInvalidCredentials
		}
	}

	// Check if account is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		s.logger.Warn("Account locked", zap.String("user_id", user.ID.String()))
		return nil, service.ErrAccountLocked
	}

	// Verify password
	valid, err := s.cryptoService.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil {
		s.logger.Error("Failed to verify password", zap.Error(err))
		return nil, fmt.Errorf("failed to verify password: %w", err)
	}

	if !valid {
		// Increment failed login attempts
		user.FailedLoginAttempts++
		if user.FailedLoginAttempts >= s.maxLoginAttempts {
			lockedUntil := time.Now().Add(s.lockoutDuration)
			user.LockedUntil = &lockedUntil
			s.logger.Warn("Account locked due to too many failed attempts",
				zap.String("user_id", user.ID.String()))
		}

		if err := s.userRepo.Update(ctx, user); err != nil {
			s.logger.Error("Failed to update user login attempts", zap.Error(err))
		}

		return nil, service.ErrInvalidCredentials
	}

	// Reset failed login attempts
	user.FailedLoginAttempts = 0
	user.LockedUntil = nil
	user.LastLogin = time.Now()

	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to update user login info", zap.Error(err))
	}

	// Check MFA
	if user.MFAEnabled && req.MFAToken == "" {
		return nil, service.ErrMFARequired
	}

	if user.MFAEnabled && req.MFAToken != "" {
		// TODO: Implement MFA verification
		// For now, just check if token is valid
		if !s.verifyMFAToken(user.MFASecret, req.MFAToken) {
			return nil, service.ErrInvalidMFAToken
		}
	}

	// Generate JWT token
	token, err := s.jwtService.GenerateToken(user.ID, user.MasterUsername)
	if err != nil {
		s.logger.Error("Failed to generate token", zap.Error(err))
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create session
	session := &models.Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		Token:        token,
		UserAgent:    req.DeviceInfo.UserAgent,
		IPAddress:    req.DeviceInfo.IPAddress,
		ExpiresAt:    time.Now().Add(24 * time.Hour),
		CreatedAt:    time.Now(),
		LastActivity: time.Now(),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		s.logger.Error("Failed to create session", zap.Error(err))
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Check if device is trusted
	trustedDevice := s.isTrustedDevice(user, req.DeviceInfo.Fingerprint)

	return &service.LoginResponse{
		User:          user,
		Token:         token,
		SessionID:     session.ID,
		TrustedDevice: trustedDevice,
	}, nil
}

// ValidateSession validates a session token
func (s *AuthService) ValidateSession(ctx context.Context, token string) (*models.Session, error) {
	// Validate JWT token
	claims, err := s.jwtService.ValidateToken(token)
	if err != nil {
		return nil, err
	}

	// Get session from database
	session, err := s.sessionRepo.GetByToken(ctx, token)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// Check if session is expired
	if session.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("session expired")
	}

	// Update last activity
	session.LastActivity = time.Now()
	if err := s.sessionRepo.Update(ctx, session); err != nil {
		s.logger.Error("Failed to update session activity", zap.Error(err))
	}

	// Verify user matches
	if session.UserID != claims.UserID {
		return nil, fmt.Errorf("session user mismatch")
	}

	return session, nil
}

// IsAdmin checks if user has admin privileges
func (s *AuthService) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}

	// For now, check if user has admin role
	// In a real implementation, you might have a roles table
	return strings.Contains(strings.ToLower(user.MasterUsername), "admin"), nil
}

// Logout logs out a user session
func (s *AuthService) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return s.sessionRepo.Delete(ctx, sessionID)
}

// RefreshToken refreshes a JWT token
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	// For now, just validate the token and generate a new one
	claims, err := s.jwtService.ValidateToken(refreshToken)
	if err != nil {
		return "", err
	}

	// Generate new token
	newToken, err := s.jwtService.GenerateToken(claims.UserID, claims.Username)
	if err != nil {
		return "", err
	}

	// Update session token
	session, err := s.sessionRepo.GetByToken(ctx, refreshToken)
	if err != nil {
		return "", err
	}

	session.Token = newToken
	session.LastActivity = time.Now()

	if err := s.sessionRepo.Update(ctx, session); err != nil {
		s.logger.Error("Failed to update session token", zap.Error(err))
	}

	return newToken, nil
}

// GetUserSessions gets all active sessions for a user
func (s *AuthService) GetUserSessions(ctx context.Context, userID uuid.UUID) ([]models.Session, error) {
	return s.sessionRepo.ListByUser(ctx, userID, 50, 0)
}

// RevokeSession revokes a specific session
func (s *AuthService) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}

	if session.UserID != userID {
		return fmt.Errorf("session does not belong to user")
	}

	return s.sessionRepo.Delete(ctx, sessionID)
}

// RevokeAllSessions revokes all sessions except current one
func (s *AuthService) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	sessions, err := s.sessionRepo.ListByUser(ctx, userID, 100, 0)
	if err != nil {
		return err
	}

	for _, session := range sessions {
		// Skip current session if we have its ID
		// This would require passing current session ID
		if err := s.sessionRepo.Delete(ctx, session.ID); err != nil {
			s.logger.Error("Failed to delete session", zap.Error(err))
		}
	}

	return nil
}

// Helper methods
func (s *AuthService) verifyMFAToken(secret, token string) bool {
	if secret == "" || token == "" {
		return false
	}

	valid, err := totp.Validate(token, secret)
	if err != nil {
		s.logger.Error("MFA validation error", zap.Error(err))
		return false
	}

	return valid
}

func (s *AuthService) isTrustedDevice(user *models.User, fingerprint string) bool {
	// Check if device is in user's trusted devices
	for _, device := range user.TrustedDevices {
		if device.Fingerprint == fingerprint {
			return true
		}
	}
	return false
}

func (s *AuthService) generateMFASecret() (string, error) {
	// Generate random secret
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return "", err
	}

	return base32.StdEncoding.EncodeToString(secret), nil
}

// Implement other required methods...
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	// TODO: Implement email verification
	return nil
}

func (s *AuthService) ResendVerification(ctx context.Context, email string) error {
	// TODO: Implement resend verification
	return nil
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	// TODO: Implement password reset request
	return nil
}

func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// TODO: Implement password reset
	return nil
}

func (s *AuthService) ValidateResetToken(ctx context.Context, token string) (bool, error) {
	// TODO: Implement token validation
	return false, nil
}

func (s *AuthService) GetMFASetup(ctx context.Context, userID uuid.UUID) (*service.MFASetup, error) {
	// Generate MFA secret
	secret, err := s.generateMFASecret()
	if err != nil {
		return nil, err
	}

	// Generate QR code
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "MXIL Server",
		AccountName: userID.String(),
		Secret:      secret,
	})
	if err != nil {
		return nil, err
	}

	return &service.MFASetup{
		Secret: secret,
		QRCode: key.URL(),
	}, nil
}

func (s *AuthService) VerifyMFASetup(ctx context.Context, userID uuid.UUID, token string) error {
	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify token
	if !s.verifyMFAToken(user.MFASecret, token) {
		return service.ErrInvalidMFAToken
	}

	// Enable MFA
	user.MFAEnabled = true
	return s.userRepo.Update(ctx, user)
}

func (s *AuthService) DisableMFA(ctx context.Context, userID uuid.UUID, token string) error {
	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Verify token
	if !s.verifyMFAToken(user.MFASecret, token) {
		return service.ErrInvalidMFAToken
	}

	// Disable MFA
	user.MFAEnabled = false
	user.MFASecret = ""
	return s.userRepo.Update(ctx, user)
}

func (s *AuthService) GenerateRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	// Generate 10 recovery codes
	codes := make([]string, 10)
	for i := 0; i < 10; i++ {
		code := make([]byte, 8)
		if _, err := rand.Read(code); err != nil {
			return nil, err
		}
		codes[i] = base32.StdEncoding.EncodeToString(code)
	}

	// TODO: Store recovery codes in database
	return codes, nil
}
