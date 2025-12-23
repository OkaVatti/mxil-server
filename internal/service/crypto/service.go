package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// cryptoService implements CryptoService
type cryptoService struct {
	bcryptCost int
}

// NewCryptoService creates a new crypto service
func NewCryptoService() CryptoService {
	return &cryptoService{
		bcryptCost: bcrypt.DefaultCost,
	}
}

// HashPassword hashes a password using bcrypt
func (s *cryptoService) HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}

	// Use bcrypt for password hashing
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), s.bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}

	return string(hashedBytes), nil
}

// VerifyPassword verifies a password against a hash using constant-time comparison
func (s *cryptoService) VerifyPassword(hashedPassword, password string) bool {
	if hashedPassword == "" || password == "" {
		return false
	}

	// Use bcrypt for password verification
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// GenerateSecureToken generates a secure random token
func (s *cryptoService) GenerateSecureToken(length int) string {
	if length <= 0 {
		length = 32 // Default length
	}

	// Calculate bytes needed (hex encoding doubles length)
	bytesNeeded := (length + 1) / 2
	bytes := make([]byte, bytesNeeded)

	if _, err := rand.Read(bytes); err != nil {
		// Fallback to UUID if crypto/rand fails
		return strings.ReplaceAll(uuid.New().String(), "-", "")[:length]
	}

	return hex.EncodeToString(bytes)[:length]
}

// HashToken creates a hash of a token
func (s *cryptoService) HashToken(token string) string {
	if token == "" {
		return ""
	}

	// Use SHA-256 for token hashing
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
