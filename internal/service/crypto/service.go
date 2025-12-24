// internal/service/crypto_service.go
package crypto

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

// CryptoService interface
type CryptoService interface {
	HashPassword(password string) (string, error)
	VerifyPassword(password, hash string) (bool, error)
	GenerateMFA(accountName string) (secret, qrCode string, err error)
	VerifyMFA(secret, token string) (bool, error)
	GenerateRecoveryCodes() []string
	EncryptString(data string) (string, error)
	DecryptString(encrypted string) (string, error)
	GenerateKeyPair(userID, algorithm string) (publicKey, privateKey, fingerprint string, err error)
	RotateKey(userID, algorithm string) (string, error)
	EncryptEmail(content string, keyIDs []string, algorithm string) ([]byte, []string, error)
	GetKeyInfo(publicKey string) (map[string]interface{}, error)
	HashToken(token string) string
}

// CryptoServiceImpl implements CryptoService
type CryptoServiceImpl struct {
	encryptionKey string
}

// NewCryptoService creates a new crypto service
func NewCryptoService(encryptionKey string) CryptoService {
	return &CryptoServiceImpl{
		encryptionKey: encryptionKey,
	}
}

// HashPassword hashes a password
func (s *CryptoServiceImpl) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword verifies a password
func (s *CryptoServiceImpl) VerifyPassword(password, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if err == bcrypt.ErrMismatchedHashAndPassword {
			return false, nil
		}
		return false, fmt.Errorf("failed to verify password: %w", err)
	}
	return true, nil
}

// GenerateMFA generates MFA secret and QR code
func (s *CryptoServiceImpl) GenerateMFA(accountName string) (secret, qrCode string, err error) {
	// Generate random secret
	secretBytes := make([]byte, 20)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate secret: %w", err)
	}
	secret = base32.StdEncoding.EncodeToString(secretBytes)

	// Generate QR code URL (in production, use proper OTP library)
	qrCode = fmt.Sprintf("otpauth://totp/MXIL:%s?secret=%s&issuer=MXIL", accountName, secret)

	return secret, qrCode, nil
}

// VerifyMFA verifies MFA token
func (s *CryptoServiceImpl) VerifyMFA(secret, token string) (bool, error) {
	if secret == "" || token == "" {
		return false, nil
	}

	valid, err := totp.Validate(token, secret)
	if err != nil {
		return false, fmt.Errorf("failed to validate MFA token: %w", err)
	}

	return valid, nil
}

// GenerateRecoveryCodes generates recovery codes
func (s *CryptoServiceImpl) GenerateRecoveryCodes() []string {
	codes := make([]string, 10)
	for i := 0; i < 10; i++ {
		codeBytes := make([]byte, 10)
		rand.Read(codeBytes)
		codes[i] = base32.StdEncoding.EncodeToString(codeBytes)[:8]
	}
	return codes
}

// EncryptString encrypts a string
func (s *CryptoServiceImpl) EncryptString(data string) (string, error) {
	// Simple base64 encoding for now
	// In production, use proper encryption like AES-GCM
	encrypted := base32.StdEncoding.EncodeToString([]byte(data))
	return encrypted, nil
}

// DecryptString decrypts a string
func (s *CryptoServiceImpl) DecryptString(encrypted string) (string, error) {
	// Simple base64 decoding for now
	decoded, err := base32.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}
	return string(decoded), nil
}

// GenerateKeyPair generates a key pair
func (s *CryptoServiceImpl) GenerateKeyPair(userID, algorithm string) (publicKey, privateKey, fingerprint string, err error) {
	// Placeholder implementation
	// In production, generate actual RSA/ECC keys
	publicKey = fmt.Sprintf("public-key-%s-%s", userID, algorithm)
	privateKey = fmt.Sprintf("private-key-%s-%s", userID, algorithm)
	fingerprint = fmt.Sprintf("%x", uuid.New().ID())

	return publicKey, privateKey, fingerprint, nil
}

// RotateKey rotates an encryption key
func (s *CryptoServiceImpl) RotateKey(userID, algorithm string) (string, error) {
	// Placeholder implementation
	newKeyID := fmt.Sprintf("key-%s-%s-%d", userID, algorithm, time.Now().Unix())
	return newKeyID, nil
}

// EncryptEmail encrypts email content
func (s *CryptoServiceImpl) EncryptEmail(content string, keyIDs []string, algorithm string) ([]byte, []string, error) {
	// Placeholder implementation
	encrypted := []byte("encrypted:" + content)
	return encrypted, keyIDs, nil
}

// GetKeyInfo gets key information
func (s *CryptoServiceImpl) GetKeyInfo(publicKey string) (map[string]interface{}, error) {
	// Placeholder implementation
	return map[string]interface{}{
		"type":      "placeholder",
		"algorithm": "rsa-2048",
		"created":   time.Now().Format(time.RFC3339),
		"expires":   time.Now().Add(365 * 24 * time.Hour).Format(time.RFC3339),
	}, nil
}

// HashToken hashes a token
func (s *CryptoServiceImpl) HashToken(token string) string {
	// Simple hash for now
	// In production, use proper cryptographic hash
	return fmt.Sprintf("%x", []byte(token))
}
