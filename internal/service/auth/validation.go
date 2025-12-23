package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"unicode"
)

// CheckPasswordStrength validates password strength
func (s *authService) CheckPasswordStrength(password string) error {
	// Minimum length
	if len(password) < 12 {
		return errors.New("password must be at least 12 characters long")
	}

	// Check for uppercase
	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSymbol := false
	uniqueChars := make(map[rune]bool)

	for _, char := range password {
		uniqueChars[char] = true
		switch {
		case unicode.IsUpper(char):
			hasUpper = true
		case unicode.IsLower(char):
			hasLower = true
		case unicode.IsNumber(char):
			hasNumber = true
		case unicode.IsPunct(char) || unicode.IsSymbol(char):
			hasSymbol = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasNumber {
		return errors.New("password must contain at least one number")
	}
	if !hasSymbol {
		return errors.New("password must contain at least one special character")
	}

	// Check for character diversity
	if len(uniqueChars) < 8 {
		return errors.New("password must contain at least 8 unique characters")
	}

	// Check for common patterns
	lowerPassword := strings.ToLower(password)
	commonPatterns := []string{
		"password", "123456", "qwerty", "admin", "welcome",
		"password123", "12345678", "123456789", "1234567890",
		"letmein", "monkey", "dragon", "baseball", "football",
		"master", "hello", "secret", "sunshine", "shadow",
	}

	for _, pattern := range commonPatterns {
		if strings.Contains(lowerPassword, pattern) {
			return errors.New("password contains common pattern")
		}
	}

	// Check for sequential characters
	for i := 0; i < len(password)-2; i++ {
		a, b, c := password[i], password[i+1], password[i+2]
		if (a+1 == b && b+1 == c) || (a-1 == b && b-1 == c) {
			return errors.New("password contains sequential characters")
		}
	}

	// Check for keyboard patterns
	keyboardRows := []string{
		"qwertyuiop", "asdfghjkl", "zxcvbnm",
		"1234567890", "!@#$%^&*()",
	}

	for _, row := range keyboardRows {
		for i := 0; i < len(row)-2; i++ {
			pattern := row[i : i+3]
			if strings.Contains(lowerPassword, pattern) {
				return errors.New("password contains keyboard pattern")
			}
		}
	}

	return nil
}

// Helper function to generate fingerprint
func generateFingerprint(input string) string {
	// Use SHA-256 for secure fingerprint generation
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:16]) // Return first 16 bytes
}
