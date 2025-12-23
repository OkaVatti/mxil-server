package auth

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	// ErrInvalidToken indicates an invalid token
	ErrInvalidToken = errors.New("invalid token")
	// ErrExpiredToken indicates an expired token
	ErrExpiredToken = errors.New("expired token")
)

// JWTService handles JWT token creation and validation
type JWTService struct {
	secret     []byte
	expiration time.Duration
}

// NewJWTService creates a new JWT service
func NewJWTService(secret string, expiration time.Duration) *JWTService {
	return &JWTService{
		secret:     []byte(secret),
		expiration: expiration,
	}
}

// Claims represents JWT claims
type Claims struct {
	UserID    uuid.UUID `json:"user_id"`
	Username  string    `json:"username"`
	SessionID uuid.UUID `json:"session_id"`
	jwt.RegisteredClaims
}

// GenerateToken generates a new JWT token for a user
func (s *JWTService) GenerateToken(userID uuid.UUID, username string, sessionID uuid.UUID) (string, error) {
	expirationTime := time.Now().Add(s.expiration)

	claims := &Claims{
		UserID:    userID,
		Username:  username,
		SessionID: sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "mxil-server",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

// ValidateToken validates a JWT token
func (s *JWTService) ValidateToken(tokenString string) (*Claims, error) {
	// Parse the token
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate the signing method
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return s.secret, nil
	})

	if err != nil {
		return nil, err
	}

	// Validate claims
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// RefreshToken refreshes a JWT token
func (s *JWTService) RefreshToken(tokenString string) (string, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	return s.GenerateToken(claims.UserID, claims.Username, claims.SessionID)
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPasswordHash checks if a password matches a hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateSecureToken generates a secure random token
func GenerateSecureToken(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(bytes), nil
}

// GenerateMFASecret generates a TOTP secret
func GenerateMFASecret() (string, error) {
	secret, err := GenerateSecureToken(20)
	if err != nil {
		return "", err
	}
	return strings.ToUpper(secret), nil
}

// GenerateRecoveryCodes generates MFA recovery codes
func GenerateRecoveryCodes(count int) ([]string, error) {
	var codes []string
	for i := 0; i < count; i++ {
		code, err := GenerateSecureToken(10)
		if err != nil {
			return nil, err
		}
		codes = append(codes, strings.ToUpper(code))
	}
	return codes, nil
}

// GeneratePasswordResetToken generates a password reset token
func GeneratePasswordResetToken() (string, error) {
	return GenerateSecureToken(32)
}

// GenerateEmailVerificationToken generates an email verification token
func GenerateEmailVerificationToken() (string, error) {
	return GenerateSecureToken(32)
}

// GenerateSessionToken generates a session token
func GenerateSessionToken() (string, error) {
	return GenerateSecureToken(64)
}

// HashToken hashes a token for storage
func HashToken(token string) string {
	hashed, _ := bcrypt.GenerateFromPassword([]byte(token), bcrypt.DefaultCost)
	return string(hashed)
}

// ValidatePasswordStrength validates password strength
func ValidatePasswordStrength(password string, minLength int, requireUpper, requireNumber, requireSymbol bool) error {
	if len(password) < minLength {
		return fmt.Errorf("password must be at least %d characters", minLength)
	}

	if requireUpper && !containsUpper(password) {
		return errors.New("password must contain at least one uppercase letter")
	}

	if requireNumber && !containsNumber(password) {
		return errors.New("password must contain at least one number")
	}

	if requireSymbol && !containsSymbol(password) {
		return errors.New("password must contain at least one symbol")
	}

	return nil
}

// Helper functions
func containsUpper(s string) bool {
	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			return true
		}
	}
	return false
}

func containsNumber(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func containsSymbol(s string) bool {
	symbols := "!@#$%^&*()_+-=[]{}|;:,.<>?"
	for _, r := range s {
		if strings.ContainsRune(symbols, r) {
			return true
		}
	}
	return false
}
