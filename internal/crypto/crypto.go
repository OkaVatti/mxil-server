package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

var (
	// ErrDecryptionFailed indicates decryption failed
	ErrDecryptionFailed = errors.New("decryption failed")
	// ErrInvalidKey indicates an invalid key
	ErrInvalidKey = errors.New("invalid key")
)

// CryptoService handles encryption and decryption operations
type CryptoService struct {
	masterKey []byte
}

// NewCryptoService creates a new crypto service
func NewCryptoService(masterKey string) (*CryptoService, error) {
	if len(masterKey) < 32 {
		return nil, errors.New("master key must be at least 32 bytes")
	}

	// Hash the master key to ensure it's exactly 32 bytes
	hash := sha256.Sum256([]byte(masterKey))

	return &CryptoService{
		masterKey: hash[:],
	}, nil
}

// Encrypt encrypts data using AES-GCM
func (s *CryptoService) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.masterKey)
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
	return ciphertext, nil
}

// Decrypt decrypts data using AES-GCM
func (s *CryptoService) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.masterKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrDecryptionFailed
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns base64 encoded result
func (s *CryptoService) EncryptString(plaintext string) (string, error) {
	encrypted, err := s.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptString decrypts a base64 encoded string
func (s *CryptoService) DecryptString(encrypted string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", ErrDecryptionFailed
	}

	decrypted, err := s.Decrypt(data)
	if err != nil {
		return "", err
	}

	return string(decrypted), nil
}

// GenerateKeyPair generates an RSA key pair
func (s *CryptoService) GenerateKeyPair(userID string, algorithm string) (publicKey, privateKey, fingerprint string, err error) {
	var keySize int
	switch algorithm {
	case "rsa-2048":
		keySize = 2048
	case "rsa-4096":
		keySize = 4096
	default:
		return "", "", "", fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	// Generate RSA key pair
	privKey, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Generate fingerprint (SHA-256 of public key)
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	hash := sha256.Sum256(pubKeyBytes)
	fingerprint = fmt.Sprintf("%x", hash[:8]) // First 8 bytes as hex

	// Encode private key
	privKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privKey),
	})

	// Encode public key
	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: pubKeyBytes,
	})

	return string(pubKeyPEM), string(privKeyPEM), fingerprint, nil
}

// EncryptEmail encrypts an email for multiple recipients
func (s *CryptoService) EncryptEmail(content string, recipientKeyIDs []string, algorithm string) (string, []string, error) {
	// In a real implementation, this would encrypt the email for each recipient
	// using their public keys. For now, we'll just encrypt with the master key.
	encrypted, err := s.EncryptString(content)
	if err != nil {
		return "", nil, err
	}

	return encrypted, recipientKeyIDs, nil
}

// DecryptEmail decrypts an email
func (s *CryptoService) DecryptEmail(encryptedContent string, keyID string) (string, error) {
	// In a real implementation, this would use the user's private key
	// For now, we'll just decrypt with the master key
	return s.DecryptString(encryptedContent)
}

// RotateKey rotates a user's encryption key
func (s *CryptoService) RotateKey(userID string, algorithm string) (string, error) {
	// Generate new key pair
	_, _, fingerprint, err := s.GenerateKeyPair(userID, algorithm)
	if err != nil {
		return "", err
	}

	// In a real implementation, we would:
	// 1. Store the new keys
	// 2. Re-encrypt existing data with the new key
	// 3. Return the new key ID

	// For now, just return the fingerprint as key ID
	return fingerprint, nil
}

// GetKeyInfo extracts information from a public key
func (s *CryptoService) GetKeyInfo(publicKeyPEM string) (map[string]interface{}, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	info := make(map[string]interface{})

	switch key := pubKey.(type) {
	case *rsa.PublicKey:
		info["type"] = "rsa"
		info["bits"] = key.Size() * 8
		info["exponent"] = key.E
		info["modulus"] = fmt.Sprintf("%x", key.N)[:64] + "..."

	default:
		info["type"] = "unknown"
	}

	return info, nil
}

// Private helper methods
func (c *CryptoService) encryptAESGCM(plaintext string) ([]byte, []string, error) {
	block, err := aes.NewCipher(c.masterKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return ciphertext, []string{"local"}, nil
}

// HashToken creates a secure hash of a token for storage
func HashToken(token string) string {
	// Use Argon2 for token hashing
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		// Fallback to SHA-256 if random fails
		hash := sha256.Sum256([]byte(token))
		return fmt.Sprintf("%x", hash)
	}

	hash := argon2.IDKey([]byte(token), salt, 1, 64*1024, 4, 32)
	return base64.StdEncoding.EncodeToString(hash)
}

// GenerateAPIKey generates a secure API key
func GenerateAPIKey() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}

	// Format as base64 without padding
	key := base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(bytes)

	// Add prefix and format for readability
	parts := []string{}
	for i := 0; i < len(key); i += 8 {
		end := i + 8
		if end > len(key) {
			end = len(key)
		}
		parts = append(parts, key[i:end])
	}

	return fmt.Sprintf("mxil_%s", strings.Join(parts, "_")), nil
}

// DeriveKey derives a key from a password using Argon2
func DeriveKey(password, salt string) []byte {
	return argon2.IDKey([]byte(password), []byte(salt), 3, 64*1024, 4, 32)
}
