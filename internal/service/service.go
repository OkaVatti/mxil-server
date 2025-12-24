// internal/service/service.go
package service

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
)

// AuthService interface
type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	RefreshToken(ctx context.Context, refreshToken string) (string, error)
	ValidateSession(ctx context.Context, token string) (*models.Session, error)
	GetUserSessions(ctx context.Context, userID uuid.UUID) ([]models.Session, error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error
	RevokeAllSessions(ctx context.Context, userID uuid.UUID) error
	IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error)
	VerifyEmail(ctx context.Context, token string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, password string) error
	ValidateResetToken(ctx context.Context, token string) (bool, error)
	GetMFASetup(ctx context.Context, userID uuid.UUID) (*MFASetup, error)
	VerifyMFASetup(ctx context.Context, userID uuid.UUID, token string) error
	DisableMFA(ctx context.Context, userID uuid.UUID, token string) error
	GenerateRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]string, error)
}

// EmailService interface
type EmailService interface {
	SendEmail(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error)
	SearchEmails(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.Email, int, error)
	GetEmailThread(ctx context.Context, userID, threadID uuid.UUID) ([]models.Email, error)
}

// NetworkService interface
type NetworkService interface {
	GetNetworkStatus(ctx context.Context) (map[models.NetworkType]NetworkStatus, error)
	TestNetwork(ctx context.Context, network models.NetworkType) (bool, error)
}

// CryptoService interface
type CryptoService interface {
	GenerateKeyPair(userID, algorithm string) (publicKey, privateKey, fingerprint string, err error)
	RotateKey(userID, algorithm string) (keyID string, err error)
	EncryptString(data string) (string, error)
	EncryptEmail(plaintext string, keyIDs []string, algorithm string) ([]byte, []string, error)
	GetKeyInfo(publicKey string) (map[string]interface{}, error)
}

// Request/Response types
type RegisterRequest struct {
	Username    string
	Password    string
	Email       string
	DisplayName string
}

type RegisterResponse struct {
	User      *models.User
	Token     string
	SessionID uuid.UUID
}

type LoginRequest struct {
	Username   string
	Password   string
	DeviceInfo DeviceInfo
	MFAToken   string
}

type LoginResponse struct {
	User          *models.User
	Token         string
	SessionID     uuid.UUID
	TrustedDevice bool
}

type DeviceInfo struct {
	Fingerprint string
	UserAgent   string
	IPAddress   string
	DeviceName  string
}

type SendEmailRequest struct {
	To           []string
	Cc           []string
	Bcc          []string
	Subject      string
	BodyPlain    string
	BodyHTML     string
	BodyMarkdown string
	Network      models.NetworkType
	Attachments  []AttachmentInfo
	Encryption   EncryptionConfig
	InReplyTo    string
	References   []string
	DraftID      *uuid.UUID
}

type AttachmentInfo struct {
	Filename    string
	ContentType string
	Size        int64
	Data        string
	IsInline    bool
}

type EncryptionConfig struct {
	Algorithm string
	KeyIDs    []string
	Sign      bool
}

type NetworkStatus struct {
	IsHealthy    bool
	LastError    string
	LastChecked  time.Time
	MessageCount int64
	Latency      time.Duration
}

type MFASetup struct {
	Secret    string
	QRCodeURL string
}

// Error definitions
var (
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account locked")
	ErrMFARequired        = errors.New("mfa required")
	ErrInvalidMFAToken    = errors.New("invalid mfa token")
	ErrInvalidToken       = errors.New("invalid token")
	ErrQuotaExceeded      = errors.New("quota exceeded")
	ErrNetworkUnavailable = errors.New("network unavailable")
	ErrRecipientNotFound  = errors.New("recipient not found")
)
