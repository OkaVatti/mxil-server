package interfaces

import (
	"context"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/internal/models"
)

// AuthService interface
type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	ValidateToken(ctx context.Context, token string) (*models.Session, error)
	// ... other methods
}

// EmailService interface
type EmailService interface {
	SendEmail(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error)
	SearchEmails(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.Email, int, error)
	// ... other methods
}

// Types for interfaces
type RegisterRequest struct {
	Username    string `json:"username"`
	Password    string `json:"password"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
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
	MFAToken   string     `json:"mfa_token"`
}

type LoginResponse struct {
	User          *models.User `json:"user"`
	Token         string       `json:"token"`
	SessionID     uuid.UUID    `json:"session_id"`
	TrustedDevice bool         `json:"trusted_device"`
}

type DeviceInfo struct {
	Fingerprint string `json:"fingerprint"`
	UserAgent   string `json:"user_agent"`
	IPAddress   string `json:"ip_address"`
	DeviceName  string `json:"device_name"`
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
	Data        string `json:"data"`
	IsInline    bool   `json:"is_inline"`
}

type EncryptionConfig struct {
	Algorithm string   `json:"algorithm"`
	KeyIDs    []string `json:"key_ids"`
}
