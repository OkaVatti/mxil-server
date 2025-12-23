// internal/crypto/hash.go
package crypto

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/argon2"
)

// HashPassword hashes a password using Argon2id
func HashPassword(password string) (string, error) {
	// Generate salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Hash parameters
	time := uint32(1)
	memory := uint32(64 * 1024)
	threads := uint8(4)
	keyLen := uint32(32)

	hash := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)

	// Encode salt and hash
	encodedSalt := base64.RawStdEncoding.EncodeToString(salt)
	encodedHash := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, memory, time, threads, encodedSalt, encodedHash), nil
}

// VerifyPassword verifies a password against a hash
func VerifyPassword(password, encodedHash string) (bool, error) {
	var version int
	var memory, time uint32
	var threads uint8
	var encodedSalt, encodedHashPart string

	_, err := fmt.Sscanf(encodedHash, "$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		&version, &memory, &time, &threads, &encodedSalt, &encodedHashPart)
	if err != nil {
		return false, err
	}

	salt, err := base64.RawStdEncoding.DecodeString(encodedSalt)
	if err != nil {
		return false, err
	}

	hashPart, err := base64.RawStdEncoding.DecodeString(encodedHashPart)
	if err != nil {
		return false, err
	}

	keyLen := uint32(len(hashPart))
	hash := argon2.IDKey([]byte(password), salt, time, memory, threads, keyLen)

	// Constant time comparison
	if len(hash) != len(hashPart) {
		return false, nil
	}

	var diff byte
	for i := range hash {
		diff |= hash[i] ^ hashPart[i]
	}

	return diff == 0, nil
}

// GenerateToken generates a secure random token
func GenerateToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
