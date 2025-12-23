package service

import (
	"context"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
)

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Username    string `json:"username" validate:"required,min=3,max=50"`
	Password    string `json:"password" validate:"required,min=12"`
	Email       string `json:"email" validate:"required,email"`
	DisplayName string `json:"display_name"`
	InviteCode  string `json:"invite_code,omitempty"`
}

// RegisterResponse represents a registration response
type RegisterResponse struct {
	User      *models.User `json:"user"`
	Token     string       `json:"token"`
	SessionID uuid.UUID    `json:"session_id"`
}

// LoginRequest represents a login request
type LoginRequest struct {
	Username   string     `json:"username" validate:"required"`
	Password   string     `json:"password" validate:"required"`
	MFAToken   string     `json:"mfa_token,omitempty"`
	DeviceInfo DeviceInfo `json:"device_info"`
}

// LoginResponse represents a login response
type LoginResponse struct {
	User          *models.User `json:"user"`
	Token         string       `json:"token"`
	SessionID     uuid.UUID    `json:"session_id"`
	TrustedDevice bool         `json:"trusted_device"`
	ExpiresIn     int          `json:"expires_in"`
	MFA           *MFAResponse `json:"mfa,omitempty"`
}

// DeviceInfo represents device information
type DeviceInfo struct {
	Fingerprint string `json:"fingerprint"`
	UserAgent   string `json:"user_agent"`
	IPAddress   string `json:"ip_address"`
	DeviceName  string `json:"device_name"`
	Location    string `json:"location,omitempty"`
}

// MFAResponse represents MFA response
type MFAResponse struct {
	Required  bool   `json:"required"`
	Type      string `json:"type,omitempty"` // "totp", "webauthn", "sms"
	Challenge string `json:"challenge,omitempty"`
	ExpiresAt int64  `json:"expires_at,omitempty"`
}

// MFASetup represents MFA setup information
type MFASetup struct {
	Enabled       bool     `json:"enabled"`
	Type          string   `json:"type,omitempty"`
	Secret        string   `json:"secret,omitempty"`
	QRCode        string   `json:"qr_code,omitempty"`
	RecoveryCodes []string `json:"recovery_codes,omitempty"`
}

// SendEmailRequest represents an email send request
type SendEmailRequest struct {
	To           []string           `json:"to"`
	Cc           []string           `json:"cc"`
	Bcc          []string           `json:"bcc"`
	Subject      string             `json:"subject"`
	BodyPlain    string             `json:"body_plain"`
	BodyHTML     string             `json:"body_html"`
	BodyMarkdown string             `json:"body_markdown"`
	Network      models.NetworkType `json:"network"`
	Attachments  []AttachmentInfo   `json:"attachments"`
	Encryption   EncryptionConfig   `json:"encryption"`
	InReplyTo    string             `json:"in_reply_to,omitempty"`
	References   []string           `json:"references,omitempty"`
	DraftID      *uuid.UUID         `json:"draft_id,omitempty"`
}

// AttachmentInfo represents attachment metadata
type AttachmentInfo struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Data        string `json:"data"` // base64 encoded
	IsInline    bool   `json:"is_inline"`
}

// EncryptionConfig represents email encryption configuration
type EncryptionConfig struct {
	Enabled   bool     `json:"enabled"`
	Algorithm string   `json:"algorithm"` // "pgp", "smime", "age"
	KeyIDs    []string `json:"key_ids"`
	Sign      bool     `json:"sign"`
}

// StorageService interface for storage operations
type StorageService interface {
	// Attachment operations
	StoreAttachment(ctx context.Context, data []byte, filename string) (string, error)
	RetrieveAttachment(ctx context.Context, path string) ([]byte, error)
	DeleteAttachment(ctx context.Context, path string) error
	GetAttachmentSize(ctx context.Context, path string) (int64, error)

	// Email storage operations
	StoreEmail(ctx context.Context, emailData []byte) (string, error)
	RetrieveEmail(ctx context.Context, path string) ([]byte, error)
	DeleteEmail(ctx context.Context, path string) error

	// Quota operations
	GetUsage(ctx context.Context, userID uuid.UUID) (int64, error)
	GetQuota(ctx context.Context, userID uuid.UUID) (int64, error)
	CheckQuota(ctx context.Context, userID uuid.UUID, size int64) (bool, error)
}

// CryptoService interface for cryptographic operations
type CryptoService interface {
	// Email encryption
	EncryptEmail(plaintext string, recipientKeys []string, algorithm string) ([]byte, []string, error)
	DecryptEmail(ciphertext []byte, keyID string, privateKey string) (string, error)

	// Key management
	GenerateKeyPair(userID uuid.UUID, algorithm string) (string, string, string, error) // public, private, fingerprint
	ImportKey(publicKey string, privateKeyEncrypted string) (string, error)
	ExportKey(keyID string) (string, string, error) // public, private
	DeleteKey(keyID string) error
	RotateKey(userID uuid.UUID, algorithm string) (string, error)

	// Password operations
	HashPassword(password string) (string, error)
	VerifyPassword(password, hash string) (bool, error)

	// Token operations
	GenerateToken(length int) (string, error)
	HashToken(token string) string
}

// EmailService interface for email operations
type EmailService interface {
	// Email operations
	SendEmail(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error)
	GetEmail(ctx context.Context, userID, emailID uuid.UUID) (*models.Email, error)
	ListEmails(ctx context.Context, userID uuid.UUID, filter repository.EmailFilter) ([]models.Email, int64, error)
	DeleteEmail(ctx context.Context, userID, emailID uuid.UUID) error
	MoveEmail(ctx context.Context, userID, emailID, folderID uuid.UUID) error
	MarkEmailRead(ctx context.Context, userID, emailID uuid.UUID, read bool) error
	MarkEmailStarred(ctx context.Context, userID, emailID uuid.UUID, starred bool) error

	// Thread operations
	GetEmailThread(ctx context.Context, userID, threadID uuid.UUID) ([]models.Email, error)

	// Search operations
	SearchEmails(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.Email, int64, error)

	// Processing
	ProcessIncomingEmail(ctx context.Context, rawEmail []byte, network models.NetworkType) (*models.Email, error)

	// Draft operations
	SaveDraft(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error)
	UpdateDraft(ctx context.Context, userID, draftID uuid.UUID, req SendEmailRequest) (*models.Email, error)
	DeleteDraft(ctx context.Context, userID, draftID uuid.UUID) error
	ListDrafts(ctx context.Context, userID uuid.UUID) ([]models.Email, error)
}

// AuthService interface for authentication operations
type AuthService interface {
	// User operations
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	RefreshToken(ctx context.Context, token string) (string, error)

	// Email verification
	VerifyEmail(ctx context.Context, token string) error
	ResendVerification(ctx context.Context, email string) error

	// Password reset
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	ValidateResetToken(ctx context.Context, token string) (bool, error)

	// Session management
	ValidateSession(ctx context.Context, tokenHash string) (*models.Session, error)
	GetUserSessions(ctx context.Context, userID uuid.UUID) ([]models.Session, error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error
	RevokeAllSessions(ctx context.Context, userID uuid.UUID) error

	// MFA operations
	GetMFASetup(ctx context.Context, userID uuid.UUID) (*MFASetup, error)
	VerifyMFASetup(ctx context.Context, userID uuid.UUID, token string) error
	DisableMFA(ctx context.Context, userID uuid.UUID, token string) error
	GenerateRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]string, error)

	// Device management
	GetTrustedDevices(ctx context.Context, userID uuid.UUID) ([]models.TrustedDevice, error)
	TrustDevice(ctx context.Context, userID uuid.UUID, deviceFingerprint string) error
	UntrustDevice(ctx context.Context, userID uuid.UUID, deviceID uuid.UUID) error
}

// Service errors
var (
	ErrInvalidCredentials = &ServiceError{"invalid_credentials", "Invalid username or password"}
	ErrUserNotFound       = &ServiceError{"user_not_found", "User not found"}
	ErrUserExists         = &ServiceError{"user_exists", "User already exists"}
	ErrSessionExpired     = &ServiceError{"session_expired", "Session expired"}
	ErrInvalidToken       = &ServiceError{"invalid_token", "Invalid token"}
	ErrAccountLocked      = &ServiceError{"account_locked", "Account is locked"}
	ErrMFARequired        = &ServiceError{"mfa_required", "MFA token required"}
	ErrInvalidMFAToken    = &ServiceError{"invalid_mfa_token", "Invalid MFA token"}
	ErrRecipientNotFound  = &ServiceError{"recipient_not_found", "Recipient not found"}
	ErrNetworkUnavailable = &ServiceError{"network_unavailable", "Network unavailable"}
	ErrEncryptionFailed   = &ServiceError{"encryption_failed", "Encryption failed"}
	ErrQuotaExceeded      = &ServiceError{"quota_exceeded", "Storage quota exceeded"}
	ErrEmailNotFound      = &ServiceError{"email_not_found", "Email not found"}
	ErrAccessDenied       = &ServiceError{"access_denied", "Access denied"}
	ErrValidationFailed   = &ServiceError{"validation_failed", "Validation failed"}
)

// ServiceError represents a service error
type ServiceError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e *ServiceError) Error() string {
	return e.Message
}

// Service errors wrapper
func NewServiceError(code, message string) *ServiceError {
	return &ServiceError{Code: code, Message: message}
}
