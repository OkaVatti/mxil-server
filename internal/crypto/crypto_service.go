package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base32"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"

	"github.com/okavatti/mxil-server/m/internal/service"
	"golang.org/x/crypto/bcrypt"
)

// ImportKey imports a key
func (c *CryptoService) ImportKey(publicKey string, privateKeyEncrypted string) (string, error) {
	// TODO: Implement key import
	return "", service.NewServiceError("not_implemented", "Key import not implemented")
}

// ExportKey exports a key
func (c *CryptoService) ExportKey(keyID string) (string, string, error) {
	// TODO: Implement key export
	return "", "", service.NewServiceError("not_implemented", "Key export not implemented")
}

// DeleteKey deletes a key
func (c *CryptoService) DeleteKey(keyID string) error {
	// TODO: Implement key deletion
	return service.NewServiceError("not_implemented", "Key deletion not implemented")
}

// HashPassword hashes a password using Argon2id
func (c *CryptoService) HashPassword(password string) (string, error) {
	return HashPassword(password)
}

// VerifyPassword verifies a password
func (c *CryptoService) VerifyPassword(password, hash string) (bool, error) {
	return VerifyPassword(password, hash)
}

// GenerateToken generates a secure random token
func (c *CryptoService) GenerateToken(length int) (string, error) {
	return GenerateToken(length)
}

// HashToken hashes a token
func (c *CryptoService) HashToken(token string) string {
	return HashToken(token)
}

func (c *CryptoService) encryptPGP(plaintext string, recipientKeys []string) ([]byte, []string, error) {
	// TODO: Implement PGP encryption
	// This would use golang.org/x/crypto/openpgp
	return nil, nil, service.NewServiceError("not_implemented", "PGP encryption not implemented")
}

func (c *CryptoService) generateRSAKeyPair() (string, string, string, error) {
	// Generate RSA key
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate RSA key: %w", err)
	}

	// Extract public key
	publicKey := &privateKey.PublicKey

	// Encode private key
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	})

	// Encode public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	publicKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})

	// Generate fingerprint (SHA256 of public key)
	hash := sha256.Sum256(publicKeyBytes)
	fingerprint := fmt.Sprintf("%x", hash)

	return string(publicKeyPEM), string(privateKeyPEM), fingerprint, nil
}

// GeneratePasswordHash generates a password hash (legacy, use HashPassword instead)
func (c *CryptoService) GeneratePasswordHash(password string) (string, error) {
	// Use bcrypt for compatibility with older systems
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(hash), nil
}

// VerifyPasswordHash verifies a password hash (legacy)
func (c *CryptoService) VerifyPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateAPIKey generates an API key
func (c *CryptoService) GenerateAPIKey() (string, string, error) {
	// Generate random bytes
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", fmt.Errorf("failed to generate key: %w", err)
	}

	// Create API key
	apiKey := base64.URLEncoding.EncodeToString(keyBytes)

	// Create prefix for identification
	prefix := "mxil_"

	// Format: prefix + key
	formattedKey := prefix + apiKey

	// Hash for storage
	hash := c.HashToken(formattedKey)

	return formattedKey, hash, nil
}

// ValidateAPIKey validates an API key
func (c *CryptoService) ValidateAPIKey(apiKey, hash string) bool {
	return c.HashToken(apiKey) == hash
}

// GenerateTOTPSecret generates a TOTP secret
func (c *CryptoService) GenerateTOTPSecret() (string, error) {
	secretBytes := make([]byte, 20)
	if _, err := rand.Read(secretBytes); err != nil {
		return "", fmt.Errorf("failed to generate secret: %w", err)
	}

	// Encode in base32 (standard for TOTP)
	secret := base32.StdEncoding.EncodeToString(secretBytes)
	// Remove padding
	secret = strings.TrimRight(secret, "=")

	return secret, nil
}

// GenerateRecoveryCode generates a recovery code
func (c *CryptoService) GenerateRecoveryCode() (string, error) {
	// Generate 16 random bytes
	codeBytes := make([]byte, 16)
	if _, err := rand.Read(codeBytes); err != nil {
		return "", fmt.Errorf("failed to generate recovery code: %w", err)
	}

	// Encode in base32 without padding
	code := base32.StdEncoding.EncodeToString(codeBytes)
	code = strings.TrimRight(code, "=")

	// Format: XXXX-XXXX-XXXX-XXXX
	formatted := ""
	for i := 0; i < len(code); i += 4 {
		if i > 0 {
			formatted += "-"
		}
		end := i + 4
		if end > len(code) {
			end = len(code)
		}
		formatted += code[i:end]
	}

	return formatted, nil
}
