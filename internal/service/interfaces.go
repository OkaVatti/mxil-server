// internal/service/interfaces.go
package service

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
)

// ServiceError represents a service-level error
type ServiceError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ServiceError) Error() string {
	return e.Code + ": " + e.Message
}

func NewServiceError(code, message string) *ServiceError {
	return &ServiceError{Code: code, Message: message}
}

// Common errors
var (
	ErrUserNotFound       = NewServiceError("user_not_found", "User not found")
	ErrEmailNotFound      = NewServiceError("email_not_found", "Email not found")
	ErrInvalidCredentials = NewServiceError("invalid_credentials", "Invalid credentials")
	ErrUserExists         = NewServiceError("user_exists", "User already exists")
	ErrAccountLocked      = NewServiceError("account_locked", "Account is locked")
	ErrQuotaExceeded      = NewServiceError("quota_exceeded", "Storage quota exceeded")
	ErrAccessDenied       = NewServiceError("access_denied", "Access denied")
	ErrSessionExpired     = NewServiceError("session_expired", "Session expired")
	ErrInvalidToken       = NewServiceError("invalid_token", "Invalid token")
	ErrNetworkUnavailable = NewServiceError("network_unavailable", "Network unavailable")
	ErrRecipientNotFound  = NewServiceError("recipient_not_found", "Recipient not found")
	ErrMFARequired        = NewServiceError("mfa_required", "MFA required")
	ErrInvalidMFAToken    = NewServiceError("invalid_mfa_token", "Invalid MFA token")
)

// AuthService handles authentication and authorization
type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	RefreshToken(ctx context.Context, token string) (string, error)
	VerifyEmail(ctx context.Context, token string) error
	ResendVerification(ctx context.Context, email string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	ValidateResetToken(ctx context.Context, token string) (bool, error)
	ValidateSession(ctx context.Context, tokenHash string) (*models.Session, error)
	GetUserSessions(ctx context.Context, userID uuid.UUID) ([]models.Session, error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error
	RevokeAllSessions(ctx context.Context, userID uuid.UUID) error
	GetMFASetup(ctx context.Context, userID uuid.UUID) (*MFASetup, error)
	VerifyMFASetup(ctx context.Context, userID uuid.UUID, token string) error
	DisableMFA(ctx context.Context, userID uuid.UUID, token string) error
	GenerateRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]string, error)
	GetTrustedDevices(ctx context.Context, userID uuid.UUID) ([]models.TrustedDevice, error)
	TrustDevice(ctx context.Context, userID uuid.UUID, deviceFingerprint string) error
	UntrustDevice(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) error
}

// EmailService handles email operations
type EmailService interface {
	SendEmail(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error)
	GetEmail(ctx context.Context, userID, emailID uuid.UUID) (*models.Email, error)
	ListEmails(ctx context.Context, userID uuid.UUID, filter repository.EmailFilter) ([]models.Email, int64, error)
	DeleteEmail(ctx context.Context, userID, emailID uuid.UUID) error
	MoveEmail(ctx context.Context, userID, emailID, folderID uuid.UUID) error
	MarkEmailRead(ctx context.Context, userID, emailID uuid.UUID, read bool) error
	MarkEmailStarred(ctx context.Context, userID, emailID uuid.UUID, starred bool) error
	GetEmailThread(ctx context.Context, userID, threadID uuid.UUID) ([]models.Email, error)
	SearchEmails(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.Email, int64, error)
	ProcessIncomingEmail(ctx context.Context, rawEmail []byte, network models.NetworkType) (*models.Email, error)
	SaveDraft(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error)
	UpdateDraft(ctx context.Context, userID, draftID uuid.UUID, req SendEmailRequest) (*models.Email, error)
	DeleteDraft(ctx context.Context, userID, draftID uuid.UUID) error
	ListDrafts(ctx context.Context, userID uuid.UUID) ([]models.Email, error)
	GetEmailStatistics(ctx context.Context, userID uuid.UUID) (*EmailStatistics, error)
	ExportEmails(ctx context.Context, userID uuid.UUID, format string, filter repository.EmailFilter) (io.ReadCloser, error)
	ImportEmails(ctx context.Context, userID uuid.UUID, format string, data io.Reader) (int, error)
	CleanupOldEmails(ctx context.Context, olderThan time.Duration) (int64, error)
	GetEmailRules(ctx context.Context, userID uuid.UUID) ([]models.EmailRule, error)
	CreateEmailRule(ctx context.Context, userID uuid.UUID, rule *models.EmailRule) error
	UpdateEmailRule(ctx context.Context, userID, ruleID uuid.UUID, rule *models.EmailRule) error
	DeleteEmailRule(ctx context.Context, userID, ruleID uuid.UUID) error
	TestEmailRule(ctx context.Context, userID uuid.UUID, conditions models.JSONB) ([]models.Email, error)
}

// NetworkService handles multi-network operations
type NetworkService interface {
	SendEmail(ctx context.Context, email *models.Email, network models.NetworkType) error
	ReceiveEmails(ctx context.Context, network models.NetworkType) (chan *models.Email, error)
	GetNetworkStatus(ctx context.Context) (map[models.NetworkType]NetworkStatus, error)
	ConnectToNetwork(ctx context.Context, network models.NetworkType) error
	DisconnectFromNetwork(ctx context.Context, network models.NetworkType) error
	TestNetwork(ctx context.Context, network models.NetworkType) (bool, error)
}

// StorageService handles file storage operations
type StorageService interface {
	SaveAttachment(ctx context.Context, data []byte, filename string) (string, error)
	GetAttachment(ctx context.Context, path string) ([]byte, error)
	DeleteAttachment(ctx context.Context, path string) error
	CleanupTempFiles(ctx context.Context, olderThan time.Duration) error
	GetStats(ctx context.Context) (*StorageStats, error)
	GetBackend() string
}

// CryptoService handles cryptographic operations
type CryptoService interface {
	EncryptEmail(plaintext string, recipientKeys []string, algorithm string) ([]byte, []string, error)
	DecryptEmail(ciphertext []byte, keyID string, privateKey string) (string, error)
	GenerateKeyPair(userID string, algorithm string) (string, string, string, error)
	ImportKey(publicKey string, privateKeyEncrypted string) (string, error)
	ExportKey(keyID string) (string, string, error)
	DeleteKey(keyID string) error
	RotateKey(userID string, algorithm string) (string, error)
	HashPassword(password string) (string, error)
	VerifyPassword(password, hash string) (bool, error)
	GenerateToken(length int) (string, error)
	HashToken(token string) string
	Encrypt(plaintext []byte) (string, error)
	Decrypt(encodedCiphertext string) ([]byte, error)
	EncryptString(plaintext string) (string, error)
	DecryptString(ciphertext string) (string, error)
	GenerateAPIKey() (string, string, error)
	ValidateAPIKey(apiKey, hash string) bool
	GenerateTOTPSecret() (string, error)
	GenerateRecoveryCode() (string, error)
	GetKeyInfo(publicKeyPEM string) (map[string]interface{}, error)
}

// Request/Response types
type RegisterRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
	InviteCode  string `json:"invite_code,omitempty"`
}

type RegisterResponse struct {
	User      *models.User `json:"user"`
	Token     string       `json:"token"`
	SessionID uuid.UUID    `json:"session_id"`
}

type LoginRequest struct {
	Username   string     `json:"username"`
	Password   string     `json:"password"`
	DeviceInfo DeviceInfo `json:"device_info"`
	MFAToken   string     `json:"mfa_token,omitempty"`
}

type LoginResponse struct {
	User          *models.User `json:"user"`
	Token         string       `json:"token"`
	SessionID     uuid.UUID    `json:"session_id"`
	TrustedDevice bool         `json:"trusted_device"`
	ExpiresIn     int          `json:"expires_in"`
	MFA           *MFAResponse `json:"mfa,omitempty"`
}

type MFAResponse struct {
	Required  bool   `json:"required"`
	Type      string `json:"type"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

type DeviceInfo struct {
	Fingerprint string `json:"fingerprint"`
	UserAgent   string `json:"user_agent"`
	IPAddress   string `json:"ip_address"`
	DeviceName  string `json:"device_name,omitempty"`
}

type MFASetup struct {
	Enabled bool   `json:"enabled"`
	Secret  string `json:"secret,omitempty"`
	QRCode  string `json:"qr_code,omitempty"`
}

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

type AttachmentInfo struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Data        string `json:"data"` // base64 encoded
	ContentID   string `json:"content_id,omitempty"`
	IsInline    bool   `json:"is_inline"`
}

type EncryptionConfig struct {
	Enabled   bool   `json:"enabled"`
	Algorithm string `json:"algorithm,omitempty"`
	KeySize   int    `json:"key_size,omitempty"`
}

type NetworkStatus struct {
	IsHealthy    bool          `json:"is_healthy"`
	LastError    string        `json:"last_error,omitempty"`
	LastChecked  time.Time     `json:"last_checked"`
	MessageCount int64         `json:"message_count"`
	Latency      time.Duration `json:"latency,omitempty"`
}

type StorageStats struct {
	Total     int64            `json:"total"`
	Used      int64            `json:"used"`
	Available int64            `json:"available"`
	ByType    map[string]int64 `json:"by_type"`
}

type EmailStatistics struct {
	Total      int64            `json:"total"`
	Unread     int64            `json:"unread"`
	Today      int64            `json:"today"`
	ByNetwork  map[string]int64 `json:"by_network"`
	ByProvider map[string]int64 `json:"by_provider"`
}
