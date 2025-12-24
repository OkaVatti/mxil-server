package auth

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"go.uber.org/zap"
)

// AuthService provides authentication and authorization services
type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	ValidateSession(ctx context.Context, token string) (*models.Session, error)
	IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	RefreshToken(ctx context.Context, refreshToken string) (string, error)
	GetUserSessions(ctx context.Context, userID uuid.UUID) ([]models.Session, error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error
	RevokeAllSessions(ctx context.Context, userID uuid.UUID) error
	VerifyEmail(ctx context.Context, token string) error
	ResendVerification(ctx context.Context, email string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	ValidateResetToken(ctx context.Context, token string) (bool, error)
}

// JWTService provides JWT token operations
type JWTService interface {
	GenerateToken(userID uuid.UUID, username string) (string, error)
	ValidateToken(tokenString string) (*JWTClaims, error)
}

// CryptoService provides cryptographic operations
type CryptoService interface {
	HashPassword(password string) (string, error)
	VerifyPassword(password, hash string) (bool, error)
	GenerateMFA(accountName string) (secret, qrCode string, err error)
	VerifyMFA(secret, token string) (bool, error)
	GenerateRecoveryCodes() []string
	Encrypt(plaintext []byte, key []byte) ([]byte, error)
	Decrypt(ciphertext []byte, key []byte) ([]byte, error)
	ValidatePasswordStrength(password string) (bool, []string)
	GenerateEncryptionKey() ([]byte, error)
	GenerateChecksum(data []byte) string
	VerifyChecksum(data []byte, checksum string) (bool, error)
}

// NetworkService provides network operations
type NetworkService interface {
	TestConnection(ctx context.Context, networkType string) (bool, error)
	SendMessage(ctx context.Context, networkType string, message interface{}) error
	ReceiveMessages(ctx context.Context, networkType string) ([]interface{}, error)
	GetStatus(ctx context.Context, networkType string) NetworkStatus
}

// NetworkAdapter interface for different network types
type NetworkAdapter interface {
	TestConnection(ctx context.Context) (bool, error)
	SendMessage(ctx context.Context, message interface{}) error
	ReceiveMessages(ctx context.Context) ([]interface{}, error)
	GetStatus(ctx context.Context) NetworkStatus
}

// NetworkStatus represents network status
type NetworkStatus struct {
	IsHealthy    bool      `json:"is_healthy"`
	LastError    string    `json:"last_error,omitempty"`
	LastChecked  time.Time `json:"last_checked"`
	MessageCount int64     `json:"message_count"`
	Latency      int64     `json:"latency"` // in milliseconds
}

// JWTClaims represents JWT token claims
type JWTClaims struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
}

// RegisterRequest represents user registration request
type RegisterRequest struct {
	Username    string `json:"username"`
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// RegisterResponse represents user registration response
type RegisterResponse struct {
	User      *models.User `json:"user"`
	Token     string       `json:"token"`
	SessionID uuid.UUID    `json:"session_id"`
}

// LoginRequest represents user login request
type LoginRequest struct {
	Username   string     `json:"username"`
	Password   string     `json:"password"`
	DeviceInfo DeviceInfo `json:"device_info"`
	MFAToken   string     `json:"mfa_token"`
}

// LoginResponse represents user login response
type LoginResponse struct {
	User          *models.User `json:"user"`
	Token         string       `json:"token"`
	SessionID     uuid.UUID    `json:"session_id"`
	TrustedDevice bool         `json:"trusted_device"`
}

// DeviceInfo represents device information
type DeviceInfo struct {
	Fingerprint string `json:"fingerprint"`
	UserAgent   string `json:"user_agent"`
	IPAddress   string `json:"ip_address"`
	DeviceName  string `json:"device_name"`
}

// MFASetup represents MFA setup information
type MFASetup struct {
	Secret string `json:"secret"`
	QRCode string `json:"qr_code"`
}

// EmailService provides email operations
type EmailService interface {
	SendEmail(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error)
	SearchEmails(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.Email, int, error)
	GetEmail(ctx context.Context, emailID uuid.UUID) (*models.Email, error)
	MarkAsRead(ctx context.Context, emailID uuid.UUID, read bool) error
	MarkAsStarred(ctx context.Context, emailID uuid.UUID, starred bool) error
	MoveToFolder(ctx context.Context, emailID, folderID uuid.UUID) error
	DeleteEmail(ctx context.Context, emailID uuid.UUID) error
	GetThread(ctx context.Context, threadID uuid.UUID) ([]models.Email, error)
}

// SendEmailRequest represents email sending request
type SendEmailRequest struct {
	To           []string           `json:"to"`
	Cc           []string           `json:"cc,omitempty"`
	Bcc          []string           `json:"bcc,omitempty"`
	Subject      string             `json:"subject"`
	BodyPlain    string             `json:"body_plain,omitempty"`
	BodyHTML     string             `json:"body_html,omitempty"`
	BodyMarkdown string             `json:"body_markdown,omitempty"`
	Network      models.NetworkType `json:"network"`
	Attachments  []AttachmentInfo   `json:"attachments,omitempty"`
	Encryption   EncryptionConfig   `json:"encryption,omitempty"`
	InReplyTo    string             `json:"in_reply_to,omitempty"`
	References   []string           `json:"references,omitempty"`
	DraftID      *uuid.UUID         `json:"draft_id,omitempty"`
}

// AttachmentInfo represents email attachment
type AttachmentInfo struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Data        string `json:"data"`
	IsInline    bool   `json:"is_inline"`
}

// EncryptionConfig represents email encryption configuration
type EncryptionConfig struct {
	Algorithm string   `json:"algorithm"`
	KeyIDs    []string `json:"key_ids"`
}

// StorageService provides storage operations
type StorageService interface {
	UploadFile(ctx context.Context, data []byte, filename, contentType string) (string, error)
	DownloadFile(ctx context.Context, path string) ([]byte, error)
	DeleteFile(ctx context.Context, path string) error
	GetFileInfo(ctx context.Context, path string) (map[string]interface{}, error)
	ListFiles(ctx context.Context, prefix string) ([]string, error)
}

// Common errors
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrMFARequired        = errors.New("mfa required")
	ErrInvalidMFAToken    = errors.New("invalid mfa token")
	ErrAccountLocked      = errors.New("account locked")
	ErrUserExists         = errors.New("user already exists")
	ErrNotFound           = errors.New("not found")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrValidationFailed   = errors.New("validation failed")
)

// NewAuthService creates a new auth service
func NewAuthService(
	userRepo *repository.UserRepository,
	sessionRepo *repository.SessionRepository,
	passwordResetRepo *repository.PasswordResetRepository,
	jwtService JWTService,
	cryptoService CryptoService,
	lockoutDuration time.Duration,
	maxLoginAttempts int,
	logger *zap.Logger,
) AuthService {
	return &authServiceImpl{
		userRepo:          userRepo,
		sessionRepo:       sessionRepo,
		passwordResetRepo: passwordResetRepo,
		jwtService:        jwtService,
		cryptoService:     cryptoService,
		lockoutDuration:   lockoutDuration,
		maxLoginAttempts:  maxLoginAttempts,
		logger:            logger,
	}
}

// authServiceImpl implements AuthService
type authServiceImpl struct {
	userRepo          *repository.UserRepository
	sessionRepo       *repository.SessionRepository
	passwordResetRepo *repository.PasswordResetRepository
	jwtService        JWTService
	cryptoService     CryptoService
	lockoutDuration   time.Duration
	maxLoginAttempts  int
	logger            *zap.Logger
}

func (s *authServiceImpl) Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error) {
	// Implementation from internal/auth/auth.go
	return nil, errors.New("not implemented")
}

func (s *authServiceImpl) Login(ctx context.Context, req LoginRequest) (*LoginResponse, error) {
	// Implementation from internal/auth/auth.go
	return nil, errors.New("not implemented")
}

func (s *authServiceImpl) ValidateSession(ctx context.Context, token string) (*models.Session, error) {
	// Implementation from internal/auth/auth.go
	return nil, errors.New("not implemented")
}

func (s *authServiceImpl) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	// Implementation from internal/auth/auth.go
	return false, errors.New("not implemented")
}

func (s *authServiceImpl) Logout(ctx context.Context, sessionID uuid.UUID) error {
	return s.sessionRepo.Delete(ctx, sessionID)
}

func (s *authServiceImpl) RefreshToken(ctx context.Context, refreshToken string) (string, error) {
	claims, err := s.jwtService.ValidateToken(refreshToken)
	if err != nil {
		return "", err
	}
	return s.jwtService.GenerateToken(claims.UserID, claims.Username)
}

func (s *authServiceImpl) GetUserSessions(ctx context.Context, userID uuid.UUID) ([]models.Session, error) {
	return s.sessionRepo.ListByUser(ctx, userID, 50, 0)
}

func (s *authServiceImpl) RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error {
	session, err := s.sessionRepo.GetByID(ctx, sessionID)
	if err != nil {
		return err
	}
	if session.UserID != userID {
		return ErrPermissionDenied
	}
	return s.sessionRepo.Delete(ctx, sessionID)
}

func (s *authServiceImpl) RevokeAllSessions(ctx context.Context, userID uuid.UUID) error {
	sessions, err := s.sessionRepo.ListByUser(ctx, userID, 100, 0)
	if err != nil {
		return err
	}
	for _, session := range sessions {
		if err := s.sessionRepo.Delete(ctx, session.ID); err != nil {
			s.logger.Error("Failed to delete session", zap.Error(err))
		}
	}
	return nil
}

func (s *authServiceImpl) VerifyEmail(ctx context.Context, token string) error {
	return errors.New("not implemented")
}

func (s *authServiceImpl) ResendVerification(ctx context.Context, email string) error {
	return errors.New("not implemented")
}

func (s *authServiceImpl) RequestPasswordReset(ctx context.Context, email string) error {
	return errors.New("not implemented")
}

func (s *authServiceImpl) ResetPassword(ctx context.Context, token, newPassword string) error {
	return errors.New("not implemented")
}

func (s *authServiceImpl) ValidateResetToken(ctx context.Context, token string) (bool, error) {
	return false, errors.New("not implemented")
}

// NewJWTService creates a new JWT service
func NewJWTService(secret string, expiration time.Duration) JWTService {
	return &jwtServiceImpl{
		secret:     secret,
		expiration: expiration,
	}
}

type jwtServiceImpl struct {
	secret     string
	expiration time.Duration
}

func (s *jwtServiceImpl) GenerateToken(userID uuid.UUID, username string) (string, error) {
	// Implementation from internal/auth/jwt.go
	return "", errors.New("not implemented")
}

func (s *jwtServiceImpl) ValidateToken(tokenString string) (*JWTClaims, error) {
	// Implementation from internal/auth/jwt.go
	return nil, errors.New("not implemented")
}

// NewCryptoService creates a new crypto service
func NewCryptoService(encryptionKey string) CryptoService {
	return &cryptoServiceImpl{
		encryptionKey: encryptionKey,
	}
}

type cryptoServiceImpl struct {
	encryptionKey string
}

func (s *cryptoServiceImpl) HashPassword(password string) (string, error) {
	return "", errors.New("not implemented")
}

func (s *cryptoServiceImpl) VerifyPassword(password, hash string) (bool, error) {
	return false, errors.New("not implemented")
}

func (s *cryptoServiceImpl) GenerateMFA(accountName string) (secret, qrCode string, err error) {
	return "", "", errors.New("not implemented")
}

func (s *cryptoServiceImpl) VerifyMFA(secret, token string) (bool, error) {
	return false, errors.New("not implemented")
}

func (s *cryptoServiceImpl) GenerateRecoveryCodes() []string {
	return []string{}
}

func (s *cryptoServiceImpl) Encrypt(plaintext []byte, key []byte) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (s *cryptoServiceImpl) Decrypt(ciphertext []byte, key []byte) ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (s *cryptoServiceImpl) ValidatePasswordStrength(password string) (bool, []string) {
	return false, []string{"not implemented"}
}

func (s *cryptoServiceImpl) GenerateEncryptionKey() ([]byte, error) {
	return nil, errors.New("not implemented")
}

func (s *cryptoServiceImpl) GenerateChecksum(data []byte) string {
	return ""
}

func (s *cryptoServiceImpl) VerifyChecksum(data []byte, checksum string) (bool, error) {
	return false, errors.New("not implemented")
}
