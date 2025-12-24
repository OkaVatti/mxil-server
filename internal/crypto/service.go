package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"
)

// Service provides an interface for cryptographic operations
type Service interface {
	HashPassword(password string) (string, error)
	VerifyPassword(password, hash string) (bool, error)
	GenerateMFA(accountName string) (secret, qrCode string, err error)
	VerifyMFA(secret, token string) (bool, error)
	GenerateRecoveryCodes() []string
	Encrypt(plaintext []byte, key []byte) ([]byte, error)
	Decrypt(ciphertext []byte, key []byte) ([]byte, error)
	ValidatePasswordStrength(password string) (bool, []string)
	GenerateEncryptionKey() ([]byte, error)
	GenerateChecksum(data []byte) string
	VerifyChecksum(data []byte, checksum string) (bool, error)
}

// NewService creates a new crypto service
func NewService(encryptionKey string) (Service, error) {
	service := &CryptoService{}

	// If encryption key is provided, set it
	if encryptionKey != "" {
		// Convert string key to bytes
		key := []byte(encryptionKey)
		if len(key) != 32 {
			return nil, fmt.Errorf("encryption key must be 32 bytes, got %d", len(key))
		}
		service.encryptionKey = key
	}

	return service, nil
}

// TokenManager handles token generation and validation
type TokenManager struct {
	secretKey []byte
}

// NewTokenManager creates a new token manager
func NewTokenManager(secretKey string) *TokenManager {
	return &TokenManager{
		secretKey: []byte(secretKey),
	}
}

// GenerateToken generates a secure token
func (tm *TokenManager) GenerateToken(data string, expiry time.Duration) (string, error) {
	// Create token data with expiry
	tokenData := fmt.Sprintf("%s:%d", data, time.Now().Add(expiry).Unix())

	// Encrypt token data
	crypto := &CryptoService{}
	encrypted, err := crypto.Encrypt([]byte(tokenData), tm.secretKey)
	if err != nil {
		return "", fmt.Errorf("failed to encrypt token: %w", err)
	}

	// Encode to base64
	return base64.URLEncoding.EncodeToString(encrypted), nil
}

// ValidateToken validates and decrypts a token
func (tm *TokenManager) ValidateToken(token string) (string, bool, error) {
	// Decode from base64
	encrypted, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", false, fmt.Errorf("failed to decode token: %w", err)
	}

	// Decrypt token data
	crypto := &CryptoService{}
	decrypted, err := crypto.Decrypt(encrypted, tm.secretKey)
	if err != nil {
		return "", false, fmt.Errorf("failed to decrypt token: %w", err)
	}

	// Parse token data
	var data string
	var expiry int64
	fmt.Sscanf(string(decrypted), "%s:%d", &data, &expiry)

	// Check expiry
	if time.Unix(expiry, 0).Before(time.Now()) {
		return "", false, fmt.Errorf("token expired")
	}

	return data, true, nil
}

// PasswordManager handles password operations
type PasswordManager struct {
	crypto Service
}

// NewPasswordManager creates a new password manager
func NewPasswordManager(crypto Service) *PasswordManager {
	return &PasswordManager{
		crypto: crypto,
	}
}

// CreatePasswordHash creates a password hash with salt
func (pm *PasswordManager) CreatePasswordHash(password string) (string, error) {
	// Validate password strength
	if ok, issues := pm.crypto.ValidatePasswordStrength(password); !ok {
		return "", fmt.Errorf("weak password: %v", issues)
	}

	// Hash the password
	return pm.crypto.HashPassword(password)
}

// VerifyPasswordWithHash verifies a password against a hash
func (pm *PasswordManager) VerifyPasswordWithHash(password, hash string) (bool, error) {
	return pm.crypto.VerifyPassword(password, hash)
}

// GeneratePassword generates a strong random password
func (pm *PasswordManager) GeneratePassword(length int) (string, error) {
	if length < 12 {
		length = 12
	}

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*()_+-=[]{}|;:,.<>?"

	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate password: %w", err)
	}

	for i := 0; i < length; i++ {
		bytes[i] = charset[bytes[i]%byte(len(charset))]
	}

	// Ensure password meets strength requirements
	password := string(bytes)
	if ok, _ := pm.crypto.ValidatePasswordStrength(password); !ok {
		// If generated password doesn't meet requirements, regenerate
		return pm.GeneratePassword(length)
	}

	return password, nil
}
