// internal/service/jwt_service.go
package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// JWTService interface
type JWTService interface {
	GenerateToken(userID uuid.UUID, username string) (string, error)
	ValidateToken(tokenString string) (*JWTClaims, error)
}

// JWTClaims custom claims
type JWTClaims struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	jwt.RegisteredClaims
}

// jwtServiceImpl implements JWTService
type jwtServiceImpl struct {
	secret     string
	expiration time.Duration
}

// NewJWTService creates a new JWT service
func NewJWTService(secret string, expiration time.Duration) JWTService {
	return &jwtServiceImpl{
		secret:     secret,
		expiration: expiration,
	}
}

// GenerateToken generates a JWT token
func (s *jwtServiceImpl) GenerateToken(userID uuid.UUID, username string) (string, error) {
	claims := JWTClaims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			ID:        uuid.New().String(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.secret))
}

// ValidateToken validates a JWT token
func (s *jwtServiceImpl) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.secret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}
