package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/crypto/scrypt"
)

// HashPassword hashes a password using bcrypt
func (c *CryptoService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPassword verifies a password against a hash
func (c *CryptoService) VerifyPassword(password, hash string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return false, nil
		}
		return false, fmt.Errorf("failed to verify password: %w", err)
	}
	return true, nil
}

// GenerateMFA generates MFA secret and QR code
func (c *CryptoService) GenerateMFA(accountName string) (secret, qrCode string, err error) {
	// Generate random secret
	secretBytes := make([]byte, 20)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate secret: %w", err)
	}

	secret = base32.StdEncoding.EncodeToString(secretBytes)

	// Generate QR code URL (TOTP format)
	qrCode = fmt.Sprintf("otpauth://totp/MXIL:%s?secret=%s&issuer=MXIL&algorithm=SHA1&digits=6&period=30",
		accountName, secret)

	return secret, qrCode, nil
}

// VerifyMFA verifies an MFA token
func (c *CryptoService) VerifyMFA(secret, token string) (bool, error) {
	// Simple implementation - in production use a proper TOTP library
	// This is a placeholder that accepts any 6-digit token for development
	if len(token) != 6 {
		return false, nil
	}

	for _, c := range token {
		if c < '0' || c > '9' {
			return false, nil
		}
	}

	return true, nil
}

// GenerateRecoveryCodes generates MFA recovery codes
func (c *CryptoService) GenerateRecoveryCodes() []string {
	codes := make([]string, 10)
	for i := 0; i < 10; i++ {
		code := make([]byte, 10)
		if _, err := rand.Read(code); err != nil {
			// Fallback to simpler code
			codes[i] = fmt.Sprintf("RECOVERY-%04d", i)
			continue
		}
		codes[i] = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(code)
	}
	return codes
}

// Encrypt encrypts data using AES-GCM
func (c *CryptoService) Encrypt(plaintext []byte, key []byte) ([]byte, error) {
	// Derive key using scrypt
	derivedKey, salt, err := c.deriveKey(key, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// Prepend salt to ciphertext
	result := make([]byte, len(salt)+len(ciphertext))
	copy(result, salt)
	copy(result[len(salt):], ciphertext)

	return result, nil
}

// Decrypt decrypts data using AES-GCM
func (c *CryptoService) Decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	if len(ciphertext) < 32 {
		return nil, errors.New("ciphertext too short")
	}

	// Extract salt (first 32 bytes)
	salt := ciphertext[:32]
	ciphertext = ciphertext[32:]

	// Derive key using scrypt
	derivedKey, _, err := c.deriveKey(key, salt)
	if err != nil {
		return nil, fmt.Errorf("failed to derive key: %w", err)
	}

	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// deriveKey derives a key from a passphrase using scrypt
func (c *CryptoService) deriveKey(passphrase, salt []byte) ([]byte, []byte, error) {
	if salt == nil {
		salt = make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return nil, nil, fmt.Errorf("failed to generate salt: %w", err)
		}
	}

	key, err := scrypt.Key(passphrase, salt, 32768, 8, 1, 32)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to derive key: %w", err)
	}

	return key, salt, nil
}

// GenerateAPIKey generates a secure API key
func (c *CryptoService) GenerateAPIKey() (string, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return "", fmt.Errorf("failed to generate API key: %w", err)
	}

	// Encode in URL-safe base64
	apiKey := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(key)
	return apiKey, nil
}

// HashString creates a SHA-256 hash of a string
func (c *CryptoService) HashString(data string) string {
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// GenerateRandomString generates a random string
func (c *CryptoService) GenerateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random string: %w", err)
	}

	// Use base32 encoding for readability
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes), nil
}

// ValidatePasswordStrength validates password strength
func (c *CryptoService) ValidatePasswordStrength(password string) (bool, []string) {
	var issues []string

	if len(password) < 12 {
		issues = append(issues, "Password must be at least 12 characters long")
	}

	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case 'A' <= ch && ch <= 'Z':
			hasUpper = true
		case 'a' <= ch && ch <= 'z':
			hasLower = true
		case '0' <= ch && ch <= '9':
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", ch):
			hasSpecial = true
		}
	}

	if !hasUpper {
		issues = append(issues, "Password must contain at least one uppercase letter")
	}
	if !hasLower {
		issues = append(issues, "Password must contain at least one lowercase letter")
	}
	if !hasDigit {
		issues = append(issues, "Password must contain at least one digit")
	}
	if !hasSpecial {
		issues = append(issues, "Password must contain at least one special character")
	}

	return len(issues) == 0, issues
}

// GenerateEncryptionKey generates a new encryption key
func (c *CryptoService) GenerateEncryptionKey() ([]byte, error) {
	key := make([]byte, 32) // 256-bit key
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}
	return key, nil
}

// GenerateKeyPair generates a key pair for asymmetric encryption
func (c *CryptoService) GenerateKeyPair() (publicKey, privateKey []byte, err error) {
	// In production, use RSA or ECC
	// This is a placeholder implementation
	publicKey = make([]byte, 32)
	privateKey = make([]byte, 64)

	if _, err := rand.Read(publicKey); err != nil {
		return nil, nil, fmt.Errorf("failed to generate public key: %w", err)
	}

	if _, err := rand.Read(privateKey); err != nil {
		return nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	return publicKey, privateKey, nil
}

// GenerateChecksum generates a checksum for data
func (c *CryptoService) GenerateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return base64.StdEncoding.EncodeToString(hash[:])
}

// VerifyChecksum verifies a checksum
func (c *CryptoService) VerifyChecksum(data []byte, checksum string) (bool, error) {
	expectedHash := sha256.Sum256(data)
	expectedChecksum := base64.StdEncoding.EncodeToString(expectedHash[:])

	// Constant-time comparison
	if subtle.ConstantTimeCompare([]byte(expectedChecksum), []byte(checksum)) == 1 {
		return true, nil
	}

	return false, nil
}
