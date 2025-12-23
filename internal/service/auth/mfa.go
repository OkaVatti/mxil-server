package auth

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
)

// GetMFASetup returns MFA setup information
func (s *authService) GetMFASetup(ctx context.Context, userID uuid.UUID) (*MFASetupResponse, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if user.MFAEnabled {
		return nil, fmt.Errorf("MFA already enabled")
	}

	// Generate secure MFA secret
	secretBytes := make([]byte, 20)
	if _, err := rand.Read(secretBytes); err != nil {
		return nil, fmt.Errorf("failed to generate MFA secret: %w", err)
	}
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secretBytes)

	// Generate QR code URL
	qrCode, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "MXIL",
		AccountName: user.MasterUsername,
		Secret:      secret,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate QR code: %w", err)
	}

	// Generate secure recovery codes
	recoveryCodes := make([]string, 8)
	for i := 0; i < 8; i++ {
		recoveryCodes[i] = s.cryptoService.GenerateSecureToken(10)
	}

	// Store recovery codes (hashed) temporarily
	hashedRecoveryCodes := make([]string, len(recoveryCodes))
	for i, code := range recoveryCodes {
		hashedRecoveryCodes[i] = s.cryptoService.HashToken(code)
	}
	user.MFARecoveryCodes = hashedRecoveryCodes
	user.MFASecret = secret

	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to store MFA setup: %w", err)
	}

	return &MFASetupResponse{
		Secret:        secret,
		QRCode:        qrCode.URL(),
		RecoveryCodes: recoveryCodes,
	}, nil
}

// VerifyMFASetup verifies MFA setup
func (s *authService) VerifyMFASetup(ctx context.Context, userID uuid.UUID, token string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if user.MFAEnabled {
		return fmt.Errorf("MFA already enabled")
	}

	// Validate TOTP token
	token = strings.TrimSpace(token)
	valid := totp.Validate(token, user.MFASecret)
	if !valid {
		// Check if it's a recovery code
		isRecoveryCode := false
		hashedToken := s.cryptoService.HashToken(token)
		for i, code := range user.MFARecoveryCodes {
			if code == hashedToken {
				// Remove used recovery code
				user.MFARecoveryCodes = append(user.MFARecoveryCodes[:i], user.MFARecoveryCodes[i+1:]...)
				isRecoveryCode = true
				break
			}
		}
		if !isRecoveryCode {
			return errors.New("invalid MFA token")
		}
	}

	user.MFAEnabled = true
	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to enable MFA: %w", err)
	}

	return nil
}

// DisableMFA disables MFA for a user
func (s *authService) DisableMFA(ctx context.Context, userID uuid.UUID, token string) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if !user.MFAEnabled {
		return nil // Already disabled
	}

	// Verify token before disabling
	token = strings.TrimSpace(token)
	valid := totp.Validate(token, user.MFASecret)
	if !valid {
		// Check recovery code
		isRecoveryCode := false
		hashedToken := s.cryptoService.HashToken(token)
		for i, code := range user.MFARecoveryCodes {
			if code == hashedToken {
				isRecoveryCode = true
				// Remove used recovery code
				user.MFARecoveryCodes = append(user.MFARecoveryCodes[:i], user.MFARecoveryCodes[i+1:]...)
				break
			}
		}
		if !isRecoveryCode {
			return errors.New("invalid MFA token")
		}
	}

	user.MFAEnabled = false
	user.MFASecret = ""
	user.MFARecoveryCodes = nil

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to disable MFA: %w", err)
	}

	return nil
}

// GenerateRecoveryCodes generates new recovery codes
func (s *authService) GenerateRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]string, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	if !user.MFAEnabled {
		return nil, fmt.Errorf("MFA not enabled")
	}

	// Generate new recovery codes
	recoveryCodes := make([]string, 8)
	hashedRecoveryCodes := make([]string, 8)

	for i := 0; i < 8; i++ {
		recoveryCodes[i] = s.cryptoService.GenerateSecureToken(10)
		hashedRecoveryCodes[i] = s.cryptoService.HashToken(recoveryCodes[i])
	}

	user.MFARecoveryCodes = hashedRecoveryCodes
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update recovery codes: %w", err)
	}

	return recoveryCodes, nil
}
