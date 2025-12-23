// internal/crypto/crypto_service.go
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidKey          = errors.New("invalid encryption key")
	ErrInvalidCiphertext   = errors.New("invalid ciphertext")
	ErrDecryptionFailed    = errors.New("decryption failed")
	ErrKeyGenerationFailed = errors.New("key generation failed")
)

// CryptoService provides cryptographic operations
type CryptoService struct {
	encryptionKey []byte
	aead          cipher.AEAD
}

// NewCryptoService creates a new crypto service
func NewCryptoService(encryptionKey string) (*CryptoService, error) {
	if len(encryptionKey) != 32 && len(encryptionKey) != 64 {
		return nil, fmt.Errorf("%w: key must be 32 or 64 bytes, got %d", ErrInvalidKey, len(encryptionKey))
	}

	key := []byte(encryptionKey)
	if len(key) == 64 {
		// Convert hex string to bytes
		var err error
		key, err = hex.DecodeString(encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("%w: invalid hex key: %v", ErrInvalidKey, err)
		}
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create AEAD: %w", err)
	}

	return &CryptoService{
		encryptionKey: key,
		aead:          aead,
	}, nil
}

// Encrypt encrypts data using AES-GCM
func (cs *CryptoService) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, cs.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := cs.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts data using AES-GCM
func (cs *CryptoService) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < cs.aead.NonceSize() {
		return nil, ErrInvalidCiphertext
	}

	nonce := ciphertext[:cs.aead.NonceSize()]
	ciphertext = ciphertext[cs.aead.NonceSize():]

	plaintext, err := cs.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// EncryptString encrypts a string and returns base64 encoded result
func (cs *CryptoService) EncryptString(plaintext string) (string, error) {
	ciphertext, err := cs.Encrypt([]byte(plaintext))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// DecryptString decrypts a base64 encoded string
func (cs *CryptoService) DecryptString(encrypted string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", ErrInvalidCiphertext
	}

	plaintext, err := cs.Decrypt(ciphertext)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// HashPassword hashes a password using Argon2
func (cs *CryptoService) HashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("failed to generate salt: %w", err)
	}

	hash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Combine salt and hash
	result := make([]byte, len(salt)+len(hash))
	copy(result, salt)
	copy(result[len(salt):], hash)

	return base64.StdEncoding.EncodeToString(result), nil
}

// VerifyPassword verifies a password against a hash
func (cs *CryptoService) VerifyPassword(password, hash string) bool {
	decoded, err := base64.StdEncoding.DecodeString(hash)
	if err != nil || len(decoded) < 16 {
		return false
	}

	salt := decoded[:16]
	storedHash := decoded[16:]

	computedHash := argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)

	// Constant time comparison
	return subtleCompare(computedHash, storedHash)
}

// GenerateKeyPair generates RSA key pair
func (cs *CryptoService) GenerateKeyPair(userID, algorithm string) (publicKey, privateKey, fingerprint string, err error) {
	var keySize int
	switch algorithm {
	case "rsa-2048":
		keySize = 2048
	case "rsa-4096":
		keySize = 4096
	default:
		return "", "", "", fmt.Errorf("unsupported algorithm: %s", algorithm)
	}

	privateKeyObj, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return "", "", "", fmt.Errorf("%w: %v", ErrKeyGenerationFailed, err)
	}

	// Encode private key
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKeyObj)
	privateKeyBlock := &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privateKeyBytes,
	}
	privateKey = string(pem.EncodeToMemory(privateKeyBlock))

	// Encode public key
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKeyObj.PublicKey)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to marshal public key: %w", err)
	}

	publicKeyBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	}
	publicKey = string(pem.EncodeToMemory(publicKeyBlock))

	// Generate fingerprint
	fingerprint = cs.generateFingerprint(publicKey, userID)

	return publicKey, privateKey, fingerprint, nil
}

// EncryptEmail encrypts email content for recipients
func (cs *CryptoService) EncryptEmail(plaintext string, keyIDs []string, algorithm string) ([]byte, []string, error) {
	// For now, use AES-GCM for all encryption
	// In production, you'd use each recipient's public key
	ciphertext, err := cs.Encrypt([]byte(plaintext))
	if err != nil {
		return nil, nil, err
	}

	return ciphertext, keyIDs, nil
}

// GetKeyInfo extracts information from a public key
func (cs *CryptoService) GetKeyInfo(publicKeyPEM string) (map[string]interface{}, error) {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse public key: %w", err)
	}

	info := make(map[string]interface{})

	switch key := pub.(type) {
	case *rsa.PublicKey:
		info["type"] = "rsa"
		info["size"] = key.N.BitLen()
		info["exponent"] = key.E
		info["modulus"] = key.N.String()[:50] + "..."
	default:
		info["type"] = "unknown"
	}

	return info, nil
}

// RotateKey rotates encryption keys
func (cs *CryptoService) RotateKey(userID, algorithm string) (string, error) {
	publicKey, _, fingerprint, err := cs.GenerateKeyPair(userID, algorithm)
	if err != nil {
		return "", err
	}

	// Store new key in database (caller's responsibility)
	// Return key ID for reference
	keyID := fmt.Sprintf("%s-%s-%d", algorithm, fingerprint[:8], time.Now().Unix())

	return keyID, nil
}

// HashToken creates a hash of a token for device fingerprinting
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// Helper functions
func (cs *CryptoService) generateFingerprint(publicKey, userID string) string {
	data := publicKey + userID + time.Now().Format("20060102150405")
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func subtleCompare(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}

	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}

	return result == 0
}

// GenerateSecureToken generates a cryptographically secure token
func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", err
	}

	// Use URL-safe base64 encoding
	token := base64.RawURLEncoding.EncodeToString(bytes)
	return token[:length], nil
}

// DeriveKeyFromPassword derives a key from a password using Argon2
func DeriveKeyFromPassword(password string, salt []byte) []byte {
	return argon2.IDKey([]byte(password), salt, 3, 64*1024, 4, 32)
}
