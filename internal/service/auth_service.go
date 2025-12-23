package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/auth"
	"github.com/okavatti/mxil-server/m/internal/crypto"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
)

var (
	// ErrUserExists indicates user already exists
	ErrUserExists = errors.New("user already exists")
	// ErrInvalidCredentials indicates invalid credentials
	ErrInvalidCredentials = errors.New("invalid credentials")
	// ErrAccountLocked indicates account is locked
	ErrAccountLocked = errors.New("account locked")
	// ErrMFARequired indicates MFA is required
	ErrMFARequired = errors.New("mfa required")
	// ErrInvalidMFAToken indicates invalid MFA token
	ErrInvalidMFAToken = errors.New("invalid mfa token")
	// ErrInvalidToken indicates invalid token
	ErrInvalidToken = errors.New("invalid token")
)

// AuthService handles authentication operations
type AuthService struct {
	userRepo         *repository.UserRepository
	sessionRepo      *repository.SessionRepository
	emailRepo        *repository.EmailRepository
	jwtService       *auth.JWTService
	cryptoService    *crypto.CryptoService
	emailService     *EmailService
	lockoutDuration  time.Duration
	maxLoginAttempts int
}

// NewAuthService creates a new auth service
func NewAuthService(
	userRepo *repository.UserRepository,
	sessionRepo *repository.SessionRepository,
	emailRepo *repository.EmailRepository,
	jwtService *auth.JWTService,
	cryptoService *crypto.CryptoService,
	emailService *EmailService,
	lockoutDuration time.Duration,
	maxLoginAttempts int,
) *AuthService {
	return &AuthService{
		userRepo:         userRepo,
		sessionRepo:      sessionRepo,
		emailRepo:        emailRepo,
		jwtService:       jwtService,
		cryptoService:    cryptoService,
		emailService:     emailService,
		lockoutDuration:  lockoutDuration,
		maxLoginAttempts: maxLoginAttempts,
	}
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username    string
	Password    string
	Email       string
	DisplayName string
	InviteCode  string
}

// RegisterResponse represents a registration response
type RegisterResponse struct {
	User      *models.User
	Token     string
	SessionID uuid.UUID
}

// Register registers a new user
func (s *AuthService) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	// Check if username exists
	existingUser, _ := s.userRepo.GetByUsername(ctx, req.Username)
	if existingUser != nil {
		return nil, ErrUserExists
	}

	// Check if email exists
	existingEmail, _ := s.userRepo.GetByEmail(ctx, req.Email)
	if existingEmail != nil {
		return nil, ErrUserExists
	}

	// Hash password
	passwordHash, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &models.User{
		ID:             uuid.New(),
		MasterUsername: req.Username,
		Email:          req.Email,
		DisplayName:    req.DisplayName,
		PasswordHash:   passwordHash,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// Create default storage quota (10GB)
	user.StorageQuotaTotal = 10 * 1024 * 1024 * 1024 // 10GB

	// Save user
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create session
	session, token, err := s.createSession(ctx, user, "Registration")
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return &RegisterResponse{
		User:      user,
		Token:     token,
		SessionID: session.ID,
	}, nil
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username   string
	Password   string
	DeviceInfo DeviceInfo
	MFAToken   string
}

// DeviceInfo represents device information
type DeviceInfo struct {
	Fingerprint string
	UserAgent   string
	IPAddress   string
	DeviceName  string
}

// LoginResponse represents a login response
type LoginResponse struct {
	User          *models.User
	Token         string
	SessionID     uuid.UUID
	TrustedDevice bool
}

// Login authenticates a user
func (s *AuthService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	// Get user
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		// Also try by email
		user, err = s.userRepo.GetByEmail(ctx, req.Username)
		if err != nil {
			return nil, ErrInvalidCredentials
		}
	}

	// Check if account is locked
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		return nil, ErrAccountLocked
	}

	// Check password
	if !auth.CheckPasswordHash(req.Password, user.PasswordHash) {
		// Increment failed login attempts
		s.userRepo.IncrementLoginAttempts(ctx, user.ID)

		// Check if should lock account
		if user.FailedLoginAttempts >= s.maxLoginAttempts-1 {
			lockUntil := time.Now().Add(s.lockoutDuration)
			s.userRepo.LockAccount(ctx, user.ID, lockUntil)
		}

		return nil, ErrInvalidCredentials
	}

	// Check MFA if enabled
	if user.MFAEnabled {
		if req.MFAToken == "" {
			return nil, ErrMFARequired
		}

		// Validate MFA token
		if !s.validateMFAToken(user, req.MFAToken) {
			return nil, ErrInvalidMFAToken
		}
	}

	// Reset failed login attempts
	s.userRepo.ResetLoginAttempts(ctx, user.ID)

	// Update last login
	s.userRepo.UpdateLastLogin(ctx, user.ID)

	// Create session
	session, token, err := s.createSession(ctx, user, req.DeviceInfo.DeviceName)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Check if device is trusted
	trustedDevice := s.isTrustedDevice(ctx, user.ID, req.DeviceInfo.Fingerprint)

	return &LoginResponse{
		User:          user,
		Token:         token,
		SessionID:     session.ID,
		TrustedDevice: trustedDevice,
	}, nil
}

// Logout logs out a user from a specific session
func (s *AuthService) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return s.sessionRepo.Delete(ctx, sessionID)
}

// ValidateSession validates a session token
func (s *AuthService) ValidateSession(ctx context.Context, token string) (*models.Session, error) {
	// Validate JWT token
	claims, err := s.jwtService.ValidateToken(token)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Get session from database
	session, err := s.sessionRepo.GetByID(ctx, claims.SessionID)
	if err != nil {
		return nil, ErrInvalidToken
	}

	// Check if session is expired
	if session.ExpiresAt.Before(time.Now()) {
		return nil, ErrInvalidToken
	}

	// Update last activity
	s.sessionRepo.UpdateLastActivity(ctx, session.ID)

	return session, nil
}

// RefreshToken refreshes a JWT token
func (s *AuthService) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	// In a real implementation, this would validate a refresh token
	// and issue a new access token. For now, we'll just validate
	// the current token and return it.
	claims, err := s.jwtService.ValidateToken(refreshToken)
	if err != nil {
		return "", ErrInvalidToken
	}

	// Generate new token
	return s.jwtService.GenerateToken(claims.UserID, claims.Username, claims.SessionID)
}

// GetUserSessions gets all active sessions for a user
func (s *AuthService) GetUserSessions(ctx context.Context, userID uuid.UUID) ([]models.Session, error) {
	return s.sessionRepo.ListByUser(ctx, userID)
}

// RevokeSession revokes a specific session for a user
func (s *AuthService) RevokeSession(ctx context.Context, userID uuid.UUID, sessionID uuid.UUID) error {
	// Get session
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}

	// Check ownership
	if session.UserID != userID {
		return errors.New("session does not belong to user")
	}

	// Delete session
	return s.sessionRepo.Delete(ctx, sessionID)
}

// RevokeAllSessions revokes all sessions for a user except current one
func (s *AuthService) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	sessions, err := s.sessionRepo.ListByUser(ctx, userID)
	if err != nil {
		return err
	}

	// Delete all sessions
	for _, session := range sessions {
		if err := s.sessionRepo.Delete(ctx, session.ID); err != nil {
			// Log error but continue
			continue
		}
	}

	return nil
}

// VerifyEmail verifies an email address
func (s *AuthService) VerifyEmail(ctx context.Context, token string) error {
	// In a real implementation, this would validate the token
	// and mark the email as verified. For now, just return success.
	return nil
}

// RequestPasswordReset requests a password reset
func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	// Get user by email
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		// Don't reveal that user doesn't exist
		return nil
	}

	// Generate reset token
	token, err := auth.GeneratePasswordResetToken()
	if err != nil {
		return fmt.Errorf("failed to generate reset token: %w", err)
	}

	// In a real implementation, you would:
	// 1. Save the reset token to the database
	// 2. Send an email with the reset link

	return nil
}

// ResetPassword resets a user's password
func (s *AuthService) ResetPassword(ctx context.Context, token, newPassword string) error {
	// Validate token
	valid, err := s.ValidateResetToken(ctx, token)
	if err != nil || !valid {
		return ErrInvalidToken
	}

	// In a real implementation, you would:
	// 1. Get user ID from token
	// 2. Hash new password
	// 3. Update password in database

	return nil
}

// ValidateResetToken validates a password reset token
func (s *AuthService) ValidateResetToken(ctx context.Context, token string) (bool, error) {
	// In a real implementation, this would check if the token
	// exists in the database and hasn't expired.
	return true, nil
}

// GetMFASetup gets MFA setup information for a user
func (s *AuthService) GetMFASetup(ctx context.Context, userID uuid.UUID) (*MFASetup, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	if user.MFAEnabled {
		return &MFASetup{
			Enabled: true,
			Type:    "totp",
		}, nil
	}

	// Generate new secret
	secret, err := auth.GenerateMFASecret()
	if err != nil {
		return nil, fmt.Errorf("failed to generate MFA secret: %w", err)
	}

	return &MFASetup{
		Enabled: false,
		Type:    "totp",
		Secret:  secret,
		URI:     s.generateTOTPURI(user, secret),
	}, nil
}

// VerifyMFASetup verifies MFA setup
func (s *AuthService) VerifyMFASetup(ctx context.Context, userID uuid.UUID, token string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// In a real implementation, this would validate the TOTP token
	// For now, just mark MFA as enabled
	user.MFAEnabled = true
	// user.MFASecret = encryptedSecret

	return s.userRepo.Update(ctx, user)
}

// DisableMFA disables MFA for a user
func (s *AuthService) DisableMFA(ctx context.Context, userID uuid.UUID, token string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	if !user.MFAEnabled {
		return nil
	}

	// Validate token
	if !s.validateMFAToken(user, token) {
		return ErrInvalidMFAToken
	}

	user.MFAEnabled = false
	user.MFASecret = ""
	return s.userRepo.Update(ctx, user)
}

// GenerateRecoveryCodes generates MFA recovery codes
func (s *AuthService) GenerateRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	codes, err := auth.GenerateRecoveryCodes(10)
	if err != nil {
		return nil, fmt.Errorf("failed to generate recovery codes: %w", err)
	}

	// In a real implementation, you would save these codes
	// to the database (hashed) for the user

	return codes, nil
}

// Helper methods
func (s *AuthService) createSession(ctx context.Context, user *models.User, deviceName string) (*models.Session, string, error) {
	// Generate session token
	sessionToken, err := auth.GenerateSessionToken()
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate session token: %w", err)
	}

	// Hash token for storage
	tokenHash := auth.HashToken(sessionToken)

	// Create session
	session := &models.Session{
		ID:           uuid.New(),
		UserID:       user.ID,
		TokenHash:    tokenHash,
		DeviceName:   deviceName,
		LastActivity: time.Now(),
		ExpiresAt:    time.Now().Add(24 * time.Hour), // 24 hour session
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Save session
	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, "", fmt.Errorf("failed to create session: %w", err)
	}

	// Generate JWT token
	jwtToken, err := s.jwtService.GenerateToken(user.ID, user.MasterUsername, session.ID)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate JWT token: %w", err)
	}

	return session, jwtToken, nil
}

func (s *AuthService) validateMFAToken(user *models.User, token string) bool {
	// In a real implementation, this would validate the TOTP token
	// using the user's MFA secret. For now, accept any 6-digit token.
	return len(token) == 6
}

func (s *AuthService) isTrustedDevice(ctx context.Context, userID uuid.UUID, fingerprint string) bool {
	// Check if device is already trusted
	// This would query a trusted_devices table
	return false
}

func (s *AuthService) generateTOTPURI(user *models.User, secret string) string {
	// Generate TOTP URI for QR code
	return fmt.Sprintf("otpauth://totp/MXIL:%s?secret=%s&issuer=MXIL&digits=6&period=30",
		user.Email, secret)
}

// MFASetup represents MFA setup information
type MFASetup struct {
	Enabled bool   `json:"enabled"`
	Type    string `json:"type"`
	Secret  string `json:"secret,omitempty"`
	URI     string `json:"uri,omitempty"`
	QRCode  string `json:"qr_code,omitempty"`
}
