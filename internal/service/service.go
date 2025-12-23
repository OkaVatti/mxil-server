// internal/service/service.go
package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/okavatti/mxil-server/internal/auth"
	"github.com/okavatti/mxil-server/internal/config"
	"github.com/okavatti/mxil-server/internal/repository"
)

type Services struct {
	Auth    AuthService
	Email   EmailService
	Crypto  CryptoService
	Network NetworkService
}

type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error)
	Login(ctx context.Context, req LoginRequest) (*AuthResponse, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	ValidateToken(ctx context.Context, token string) (*SessionInfo, error)
	RefreshToken(ctx context.Context, refreshToken string) (string, error)
	IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error)
}

type EmailService interface {
	SendEmail(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*Email, error)
	ListEmails(ctx context.Context, userID uuid.UUID, filter EmailFilter) ([]Email, int, error)
	GetEmail(ctx context.Context, emailID uuid.UUID) (*Email, error)
	MarkAsRead(ctx context.Context, emailID uuid.UUID, read bool) error
	DeleteEmail(ctx context.Context, emailID uuid.UUID) error
}

type CryptoService interface {
	HashPassword(password string) (string, error)
	VerifyPassword(hash, password string) bool
	Encrypt(data []byte) ([]byte, error)
	Decrypt(data []byte) ([]byte, error)
	GenerateKeyPair(userID, algorithm string) (string, string, string, error)
}

type NetworkService interface {
	GetStatus(ctx context.Context) (map[string]NetworkStatus, error)
	TestNetwork(ctx context.Context, network string) (bool, error)
}

// Implementation structs
type authService struct {
	userRepo    *repository.UserRepository
	sessionRepo *repository.SessionRepository
	jwtService  *auth.JWTService
	crypto      CryptoService
	config      config.SecurityConfig
}

type cryptoService struct {
	encryptionKey []byte
}

// RegisterRequest represents user registration request
type RegisterRequest struct {
	Username    string `json:"username" validate:"required,min=3,max=50"`
	Email       string `json:"email" validate:"required,email"`
	Password    string `json:"password" validate:"required,min=12"`
	DisplayName string `json:"display_name,omitempty"`
}

// LoginRequest represents user login request
type LoginRequest struct {
	Username   string     `json:"username" validate:"required"`
	Password   string     `json:"password" validate:"required"`
	DeviceInfo DeviceInfo `json:"device_info,omitempty"`
}

// DeviceInfo represents device information
type DeviceInfo struct {
	UserAgent  string `json:"user_agent"`
	IPAddress  string `json:"ip_address"`
	DeviceName string `json:"device_name"`
}

// AuthResponse represents authentication response
type AuthResponse struct {
	User      UserInfo  `json:"user"`
	Token     string    `json:"token"`
	SessionID uuid.UUID `json:"session_id"`
	ExpiresIn int       `json:"expires_in"`
}

// SessionInfo represents session information
type SessionInfo struct {
	ID        uuid.UUID `json:"id"`
	UserID    uuid.UUID `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// UserInfo represents safe user information
type UserInfo struct {
	ID           uuid.UUID    `json:"id"`
	Username     string       `json:"username"`
	Email        string       `json:"email"`
	DisplayName  string       `json:"display_name,omitempty"`
	MFAActivated bool         `json:"mfa_activated"`
	StorageQuota StorageQuota `json:"storage_quota"`
	CreatedAt    time.Time    `json:"created_at"`
}

// StorageQuota represents storage quota information
type StorageQuota struct {
	Used    int64   `json:"used"`
	Total   int64   `json:"total"`
	Percent float64 `json:"percent"`
}

// Email represents an email message
type Email struct {
	ID             uuid.UUID `json:"id"`
	ThreadID       uuid.UUID `json:"thread_id,omitempty"`
	From           string    `json:"from"`
	To             []string  `json:"to"`
	Subject        string    `json:"subject"`
	Body           string    `json:"body,omitempty"`
	Network        string    `json:"network"`
	IsRead         bool      `json:"is_read"`
	IsStarred      bool      `json:"is_starred"`
	HasAttachments bool      `json:"has_attachments"`
	SizeBytes      int64     `json:"size_bytes"`
	ReceivedAt     time.Time `json:"received_at"`
}

// EmailFilter represents email filtering options
type EmailFilter struct {
	UserID    uuid.UUID  `json:"user_id"`
	FolderID  *uuid.UUID `json:"folder_id,omitempty"`
	IsRead    *bool      `json:"is_read,omitempty"`
	IsStarred *bool      `json:"is_starred,omitempty"`
	Network   *string    `json:"network,omitempty"`
	Search    string     `json:"search,omitempty"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
}

// SendEmailRequest represents email sending request
type SendEmailRequest struct {
	To          []string     `json:"to" validate:"required"`
	Subject     string       `json:"subject" validate:"required"`
	Body        string       `json:"body" validate:"required"`
	Network     string       `json:"network" validate:"required"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

// Attachment represents email attachment
type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Data        []byte `json:"data"`
	Size        int64  `json:"size"`
}

// NetworkStatus represents network status
type NetworkStatus struct {
	IsHealthy    bool      `json:"is_healthy"`
	LastError    string    `json:"last_error,omitempty"`
	LastChecked  time.Time `json:"last_checked"`
	MessageCount int       `json:"message_count"`
	LatencyMs    int64     `json:"latency_ms,omitempty"`
}

// NewAuthService creates a new authentication service
func NewAuthService(
	userRepo *repository.UserRepository,
	sessionRepo *repository.SessionRepository,
	jwtService *auth.JWTService,
	crypto CryptoService,
	config config.SecurityConfig,
) AuthService {
	return &authService{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		jwtService:  jwtService,
		crypto:      crypto,
		config:      config,
	}
}

// NewCryptoService creates a new crypto service
func NewCryptoService(encryptionKey string) CryptoService {
	key := sha256.Sum256([]byte(encryptionKey))
	return &cryptoService{encryptionKey: key[:]}
}

func (s *authService) Register(ctx context.Context, req RegisterRequest) (*AuthResponse, error) {
	// Check if user already exists
	existing, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("username already exists")
	}

	existing, err = s.userRepo.GetByEmail(ctx, req.Email)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("email already exists")
	}

	// Hash password
	hashedPassword, err := s.crypto.HashPassword(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &repository.User{
		ID:                uuid.New(),
		MasterUsername:    req.Username,
		Email:             req.Email,
		DisplayName:       req.DisplayName,
		PasswordHash:      hashedPassword,
		StorageQuotaTotal: 1 << 30, // 1GB default
		IsActive:          true,
		CreatedAt:         time.Now(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create session
	return s.createSession(ctx, user)
}

func (s *authService) Login(ctx context.Context, req LoginRequest) (*AuthResponse, error) {
	// Get user
	user, err := s.userRepo.GetByUsername(ctx, req.Username)
	if err != nil || user == nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Verify password
	if !s.crypto.VerifyPassword(user.PasswordHash, req.Password) {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if account is locked
	// TODO: Implement login attempt tracking and lockout

	// Create session
	return s.createSession(ctx, user)
}

func (s *authService) createSession(ctx context.Context, user *repository.User) (*AuthResponse, error) {
	// Generate JWT token
	token, err := s.jwtService.GenerateToken(user.ID, user.MasterUsername)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// Create session record
	session := &repository.Session{
		ID:        uuid.New(),
		UserID:    user.ID,
		TokenHash: hashToken(token),
		UserAgent: "",
		IPAddress: "",
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	if err := s.sessionRepo.Create(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Prepare user info
	userInfo := UserInfo{
		ID:           user.ID,
		Username:     user.MasterUsername,
		Email:        user.Email,
		DisplayName:  user.DisplayName,
		MFAActivated: user.MFAEnabled,
		StorageQuota: StorageQuota{
			Used:    user.StorageQuotaUsed,
			Total:   user.StorageQuotaTotal,
			Percent: float64(user.StorageQuotaUsed) / float64(user.StorageQuotaTotal) * 100,
		},
		CreatedAt: user.CreatedAt,
	}

	return &AuthResponse{
		User:      userInfo,
		Token:     token,
		SessionID: session.ID,
		ExpiresIn: 86400, // 24 hours in seconds
	}, nil
}

func (s *authService) ValidateToken(ctx context.Context, token string) (*SessionInfo, error) {
	// Parse and validate JWT
	_, err := s.jwtService.ValidateToken(token)
	if err != nil {
		return nil, fmt.Errorf("invalid token: %w", err)
	}

	// Get session
	session, err := s.sessionRepo.GetByTokenHash(ctx, hashToken(token))
	if err != nil || session == nil {
		return nil, fmt.Errorf("session not found")
	}

	// Check if session expired
	if time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired")
	}

	return &SessionInfo{
		ID:        session.ID,
		UserID:    session.UserID,
		ExpiresAt: session.ExpiresAt,
	}, nil
}

func (s *authService) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return s.sessionRepo.Delete(ctx, sessionID)
}

func (s *authService) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	// TODO: Implement refresh token logic
	return "", fmt.Errorf("not implemented")
}

func (s *authService) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	// TODO: Implement admin check
	// For now, check if user is the first user (assuming first user is admin)
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return false, err
	}

	// Simple check: if user ID is the first created user, they're admin
	// In production, you'd have an is_admin field
	count, err := s.userRepo.Count(ctx)
	if err != nil {
		return false, err
	}

	return count <= 1, nil
}

func (s *cryptoService) HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (s *cryptoService) VerifyPassword(hash, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func (s *cryptoService) Encrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

func (s *cryptoService) Decrypt(data []byte) ([]byte, error) {
	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func (s *cryptoService) GenerateKeyPair(userID, algorithm string) (string, string, string, error) {
	// TODO: Implement key pair generation
	// For now, return placeholder values
	publicKey := "placeholder-public-key"
	privateKey := "placeholder-private-key"
	fingerprint := sha256Hex(userID + algorithm + time.Now().String())

	return publicKey, privateKey, fingerprint, nil
}

// Helper functions
func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func sha256Hex(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// NewNetworkService creates a new network service
func NewNetworkService(config config.NetworkConfig) NetworkService {
	return &networkService{config: config}
}

type networkService struct {
	config config.NetworkConfig
}

func (s *networkService) GetStatus(ctx context.Context) (map[string]NetworkStatus, error) {
	status := make(map[string]NetworkStatus)
	now := time.Now()

	// Clearnet status
	status["clearnet"] = NetworkStatus{
		IsHealthy:    true,
		LastChecked:  now,
		MessageCount: 0,
	}

	// I2P status
	status["i2p"] = NetworkStatus{
		IsHealthy:    s.config.EnableI2P,
		LastChecked:  now,
		MessageCount: 0,
	}

	// Tor status
	status["tor"] = NetworkStatus{
		IsHealthy:    s.config.EnableTor,
		LastChecked:  now,
		MessageCount: 0,
	}

	return status, nil
}

func (s *networkService) TestNetwork(ctx context.Context, network string) (bool, error) {
	// TODO: Implement actual network testing
	switch network {
	case "clearnet":
		return true, nil
	case "i2p":
		return s.config.EnableI2P, nil
	case "tor":
		return s.config.EnableTor, nil
	default:
		return false, fmt.Errorf("unknown network: %s", network)
	}
}
