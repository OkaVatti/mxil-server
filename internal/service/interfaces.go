// internal/service/interfaces.go - Define missing service interfaces
package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
)

// AuthService defines authentication service interface
type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	RefreshToken(ctx context.Context, refreshToken string) (string, error)
	VerifyEmail(ctx context.Context, token string) error
	ResendVerification(ctx context.Context, email string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	ValidateResetToken(ctx context.Context, token string) (bool, error)
	GetMFASetup(ctx context.Context, userID uuid.UUID) (*MFASetup, error)
	VerifyMFASetup(ctx context.Context, userID uuid.UUID, token string) error
	DisableMFA(ctx context.Context, userID uuid.UUID, token string) error
	GenerateRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]string, error)
	ValidateSession(ctx context.Context, token string) (*models.Session, error)
	IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error)
	GetUserSessions(ctx context.Context, userID uuid.UUID) ([]models.Session, error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error
	RevokeAllSessions(ctx context.Context, userID uuid.UUID) error
}

// EmailService defines email service interface
type EmailService interface {
	SendEmail(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error)
	SearchEmails(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.Email, int64, error)
	GetEmailThread(ctx context.Context, userID uuid.UUID, threadID uuid.UUID) ([]models.Email, error)
}

// NetworkService defines network service interface
type NetworkService interface {
	GetNetworkStatus(ctx context.Context) (map[models.NetworkType]NetworkStatus, error)
	TestNetwork(ctx context.Context, network models.NetworkType) (bool, error)
	SendMessage(ctx context.Context, network models.NetworkType, message []byte) error
	ReceiveMessage(ctx context.Context, network models.NetworkType) ([]byte, error)
}

// CryptoService defines cryptography service interface
type CryptoService interface {
	GenerateKeyPair(userID, algorithm string) (publicKey, privateKey, fingerprint string, err error)
	EncryptEmail(plaintext string, keyIDs []string, algorithm string) (encrypted []byte, usedKeyIDs []string, err error)
	DecryptEmail(encrypted []byte, keyID string) (plaintext string, err error)
	RotateKey(userID, algorithm string) (newKeyID string, err error)
	GetKeyInfo(publicKey string) (map[string]interface{}, error)
	EncryptString(data string) (string, error)
	DecryptString(encrypted string) (string, error)
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

type DeviceInfo struct {
	Fingerprint string
	UserAgent   string
	IPAddress   string
	DeviceName  string
}

type MFASetup struct {
	Secret string
	QRCode string
}

type AttachmentInfo struct {
	Filename    string
	ContentType string
	Size        int64
	Data        string // base64 encoded
	IsInline    bool
}

type EncryptionConfig struct {
	Encrypt      bool
	PublicKeys   []string
	Algorithm    string
	Sign         bool
	PrivateKeyID string
}

type NetworkStatus struct {
	IsHealthy    bool
	LastError    string
	LastChecked  time.Time
	MessageCount int64
	Latency      time.Duration
}

// Error types
var (
	ErrUserExists         = &ServiceError{Code: "user_exists", Message: "Username or email already exists"}
	ErrInvalidCredentials = &ServiceError{Code: "invalid_credentials", Message: "Invalid username or password"}
	ErrAccountLocked      = &ServiceError{Code: "account_locked", Message: "Account is locked due to too many failed attempts"}
	ErrMFARequired        = &ServiceError{Code: "mfa_required", Message: "MFA token required"}
	ErrInvalidMFAToken    = &ServiceError{Code: "invalid_mfa_token", Message: "Invalid MFA token"}
	ErrInvalidToken       = &ServiceError{Code: "invalid_token", Message: "Invalid or expired token"}
	ErrQuotaExceeded      = &ServiceError{Code: "quota_exceeded", Message: "Storage quota exceeded"}
	ErrNetworkUnavailable = &ServiceError{Code: "network_unavailable", Message: "Network is currently unavailable"}
	ErrRecipientNotFound  = &ServiceError{Code: "recipient_not_found", Message: "One or more recipients not found"}
)

type ServiceError struct {
	Code    string
	Message string
}

func (e *ServiceError) Error() string {
	return e.Message
}
