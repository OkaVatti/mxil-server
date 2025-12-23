package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// EncryptEmail encrypts email content
func (s *cryptoService) EncryptEmail(content string, keyIDs []string, algorithm string) ([]byte, []string, error) {
	if content == "" {
		return nil, nil, fmt.Errorf("content cannot be empty")
	}

	if len(keyIDs) == 0 {
		return nil, nil, fmt.Errorf("at least one key ID required")
	}

	// Default to AES-GCM if algorithm not specified
	if algorithm == "" {
		algorithm = "aes-gcm-256"
	}

	switch algorithm {
	case "aes-gcm-128", "aes-gcm-256":
		return s.encryptWithAESGCM(content, keyIDs, algorithm)
	default:
		return nil, nil, fmt.Errorf("unsupported encryption algorithm: %s", algorithm)
	}
}

// encryptWithAESGCM implements AES-GCM encryption
func (s *cryptoService) encryptWithAESGCM(content string, keyIDs []string, algorithm string) ([]byte, []string, error) {
	// Determine key size based on algorithm
	keySize := 32 // 256 bits
	if algorithm == "aes-gcm-128" {
		keySize = 16 // 128 bits
	}

	// Generate random key
	key := make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, nil, fmt.Errorf("failed to generate encryption key: %w", err)
	}

	// Create cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, []byte(content), nil)

	// TODO: In production, encrypt the key with each recipient's public key
	// For now, we'll just return the key IDs

	return ciphertext, keyIDs, nil
}

// Helper function for main.go
func GenerateDeviceFingerprint(userAgent, acceptLang, acceptEnc string) string {
	input := userAgent + acceptLang + acceptEnc
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:16])
}
