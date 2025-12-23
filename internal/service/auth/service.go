package auth

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/auth"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service/crypto"
)

// authService implements AuthService
type authService struct {
	userRepo           repository.UserRepository
	sessionRepo        repository.SessionRepository
	passwordResetRepo  repository.PasswordResetRepository
	jwtService         *auth.JWTService
	cryptoService      crypto.CryptoService
	lockoutDuration    time.Duration
	maxLoginAttempts   int
	maxSessionsPerUser int
	logger             *zap.Logger
	loginAttempts      sync.Map // username -> attempt count
}

// NewAuthService creates a new auth service
func NewAuthService(
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	passwordResetRepo repository.PasswordResetRepository,
	jwtService *auth.JWTService,
	cryptoService crypto.CryptoService,
	lockoutDuration time.Duration,
	maxLoginAttempts int,
	logger *zap.Logger,
) AuthService {
	return &authService{
		userRepo:           userRepo,
		sessionRepo:        sessionRepo,
		passwordResetRepo:  passwordResetRepo,
		jwtService:         jwtService,
		cryptoService:      cryptoService,
		lockoutDuration:    lockoutDuration,
		maxLoginAttempts:   maxLoginAttempts,
		maxSessionsPerUser: 10,
		logger:             logger,
	}
}

// Register creates a new user account
func (s *authService) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	// Validate password strength
	if err := s.CheckPasswordStrength(req.Password); err != nil {
		return nil, err
	}

	// Check if username or email already exists
	existingUser, err := s.userRepo.GetByUsernameOrEmail(ctx, req.Username, req.Email)
	if err == nil && existingUser != nil {
		if existingUser.MasterUsername == req.Username {
			return nil, errors.New("username already exists")
		}
		if existingUser.Email == req.Email {
			return nil, errors.New("email already registered")
		}
	}

	// Hash password
	hashedPassword, err := s.cryptoService.HashPassword(req.Password)
	if err != nil {
		s.logger.Error("Failed to hash password", zap.Error(err))
		return nil, fmt.Errorf("failed to create account")
	}

	// Generate verification token
	verificationToken := s.cryptoService.GenerateSecureToken(32)

	// Create user
	user := &models.User{
		ID:                           uuid.New(),
		MasterUsername:               req.Username,
		Email:                        req.Email,
		PasswordHash:                 hashedPassword,
		DisplayName:                  req.DisplayName,
		CreatedAt:                    time.Now(),
		IsActive:                     true,
		EmailVerified:                false,
		EmailVerificationToken:       verificationToken,
		EmailVerificationTokenExpiry: time.Now().Add(24 * time.Hour),
		StorageQuotaTotal:            1 << 30,      // 1GB default
		SessionTimeout:               24 * 60 * 60, // 24 hours
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("Failed to create user", zap.Error(err))
		return nil, fmt.Errorf("failed to create account")
	}

	// Create session
	deviceInfo := DeviceInfo{
		Fingerprint: s.cryptoService.GenerateSecureToken(16),
		UserAgent:   "",
		IPAddress:   "",
		DeviceName:  "Default",
	}

	session, token, err := s.createSession(ctx, user, deviceInfo)
	if err != nil {
		s.logger.Error("Failed to create session", zap.Error(err))
		// Clean up user if session creation fails
		s.userRepo.Delete(ctx, user.ID)
		return nil, fmt.Errorf("failed to create session")
	}

	// TODO: Send verification email

	return &RegisterResponse{
		User:      user,
		Token:     token,
		SessionID: session.ID,
	}, nil
}

// Login authenticates a user
func (s *authService) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	// Check login attempts
	if attempts, ok := s.loginAttempts.Load(req.Username); ok {
		if attempts.(int) >= s.maxLoginAttempts {
			s.logger.Warn("Account locked due to too many attempts",
				zap.String("username", req.Username))
			return nil, errors.New("account is locked")
		}
	}

	// Get user
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		s.recordFailedAttempt(req.Username)
		return nil, errors.New("invalid credentials")
	}

	// Verify password
	if !s.cryptoService.VerifyPassword(user.PasswordHash, req.Password) {
		s.recordFailedAttempt(req.Username)
		return nil, errors.New("invalid credentials")
	}

	// Check if email is verified
	if !user.EmailVerified {
		return nil, errors.New("email not verified")
	}

	// Check MFA
	if user.MFAEnabled {
		if req.MFAToken == "" {
			// MFA required but not provided
			return &LoginResponse{
				RequiresMFA: true,
			}, nil
		}

		// Verify MFA token
		// This would use the TOTP verification from mfa.go
		// For now, we'll simulate it
		if !s.verifyMFAToken(user, req.MFAToken) {
			s.recordFailedAttempt(req.Username)
			return nil, errors.New("invalid MFA token")
		}
	}

	// Reset failed attempts on successful login
	s.loginAttempts.Delete(req.Username)

	// Create session
	session, token, err := s.createSession(ctx, user, req.DeviceInfo)
	if err != nil {
		s.logger.Error("Failed to create session", zap.Error(err))
		return nil, fmt.Errorf("failed to create session")
	}

	// Update last login
	now := time.Now()
	user.LastLogin = &now
	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("Failed to update user last login", zap.Error(err))
	}

	return &LoginResponse{
		User:          user,
		Token:         token,
		SessionID:     session.ID,
		TrustedDevice: s.isTrustedDevice(user.ID, req.DeviceInfo.Fingerprint),
		RequiresMFA:   false,
	}, nil
}

// Logout terminates a session
func (s *authService) Logout(ctx context.Context, sessionID uuid.UUID) error {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return errors.New("session not found")
	}

	now := time.Now()
	session.ExpiresAt = now
	session.RevokedAt = &now

	if err := s.sessionRepo.Update(ctx, session); err != nil {
		s.logger.Error("Failed to logout session", zap.Error(err))
		return fmt.Errorf("failed to logout")
	}

	return nil
}

// Helper method to verify MFA token
func (s *authService) verifyMFAToken(user *models.User, token string) bool {
	// This would use the TOTP library
	// For now, return true for testing
	return true
}

// Helper method to check if device is trusted
func (s *authService) isTrustedDevice(userID uuid.UUID, fingerprint string) bool {
	// Check if this device fingerprint exists in user's trusted devices
	// For now, return true
	return true
}
