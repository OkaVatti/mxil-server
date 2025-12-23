package auth

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidToken indicates an invalid token
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken indicates an expired token
	ErrExpiredToken = errors.New("expired token")
)

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash checks if a password matches a hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateSecureToken generates a secure random token
func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes), nil
}

// GenerateMFASecret generates a TOTP secret
func GenerateMFASecret() (string, error) {
	secret, err := GenerateSecureToken(20)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(secret), nil
}

// GenerateRecoveryCodes generates MFA recovery codes
func GenerateRecoveryCodes(count int) ([]string, error) {
	var codes []string
	for i := 0; i < count; i++ {
		code, err := GenerateSecureToken(10)
		if err != nil {
			return nil, err
		}
		codes = append(codes, strings.ToUpper(code))
	}
	return codes, nil
}

// GeneratePasswordResetToken generates a password reset token
func GeneratePasswordResetToken() (string, error) {
	return GenerateSecureToken(32)
}

// GenerateEmailVerificationToken generates an email verification token
func GenerateEmailVerificationToken() (string, error) {
	return GenerateSecureToken(32)
}

// GenerateSessionToken generates a session token
func GenerateSessionToken() (string, error) {
	return GenerateSecureToken(64)
}

// HashToken hashes a token for storage
func HashToken(token string) string {
	hashed, _ := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	return string(hashed)
}

// ValidatePasswordStrength validates password strength
func ValidatePasswordStrength(password string, minLength int, requireUpper, requireNumber, requireSymbol bool) error {
	if len(password) < minLength {
		return fmt.Errorf("password must be at least %d characters", minLength)
	}

	if requireUpper && !containsUpper(password) {
		return errors.New("password must contain at least one uppercase letter")
	}

	if requireNumber && !containsNumber(password) {
		return errors.New("password must contain at least one number")
	}

	if requireSymbol && !containsSymbol(password) {
		return errors.New("password must contain at least one symbol")
	}

	return nil
}

// Helper functions
func containsUpper(s string) bool {
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}

func containsNumber(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func containsSymbol(s string) bool {
	symbols := "!@#$%^&*()_+-=[]{}|;:,.<>?"
	for _, r := range s {
		if strings.ContainsRune(symbols, r) {
			return true
		}
	}
	return false
}
