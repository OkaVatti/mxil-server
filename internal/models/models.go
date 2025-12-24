// internal/models/models.go
package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// JSONB type for PostgreSQL JSONB columns
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
	return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
	if value == nil {
		*j = JSONB{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	default:
		return json.Unmarshal([]byte{}, j)
	}
}

// StringArray type for PostgreSQL text arrays
type StringArray []string

func (s StringArray) Value() (driver.Value, error) {
	if len(s) == 0 {
		return "{}", nil
	}
	bytes, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return string(bytes), nil
}

func (s *StringArray) Scan(value interface{}) error {
	if value == nil {
		*s = StringArray{}
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, s)
	case string:
		return json.Unmarshal([]byte(v), s)
	default:
		return json.Unmarshal([]byte{}, s)
	}
}

// NetworkType represents different email networks
type NetworkType string

const (
	NetworkClearnet NetworkType = "clearnet"
	NetworkI2P      NetworkType = "i2p"
	NetworkTor      NetworkType = "tor"
	NetworkLAN      NetworkType = "lan"
	NetworkIPFS     NetworkType = "ipfs"
)

// ProviderType represents different email providers
type ProviderType string

const (
	ProviderGmail      ProviderType = "gmail"
	ProviderOutlook    ProviderType = "outlook"
	ProviderYahoo      ProviderType = "yahoo"
	ProviderProton     ProviderType = "proton"
	ProviderI2PBote    ProviderType = "i2p_bote"
	ProviderSusimail   ProviderType = "susimail"
	ProviderOnionMail  ProviderType = "onion_mail"
	ProviderCustomIMAP ProviderType = "custom_imap"
)

// User represents a system user
type User struct {
	ID                    uuid.UUID         `json:"id" db:"id"`
	MasterUsername        string            `json:"master_username" db:"master_username"`
	Email                 string            `json:"email" db:"email"`
	DisplayName           string            `json:"display_name" db:"display_name"`
	Bio                   string            `json:"bio" db:"bio"`
	PasswordHash          string            `json:"-" db:"password_hash"`
	MFAEnabled            bool              `json:"mfa_enabled" db:"mfa_enabled"`
	MFASecret             string            `json:"-" db:"mfa_secret"`
	RecoveryCodes         string            `json:"-" db:"recovery_codes"`
	IsActive              bool              `json:"is_active" db:"is_active"`
	IsVerified            bool              `json:"is_verified" db:"is_verified"`
	VerificationToken     string            `json:"-" db:"verification_token"`
	VerificationExpiresAt *time.Time        `json:"-" db:"verification_expires_at"`
	FailedLoginAttempts   int               `json:"-" db:"failed_login_attempts"`
	LockedUntil           *time.Time        `json:"-" db:"locked_until"`
	LastLogin             *time.Time        `json:"last_login" db:"last_login"`
	SecurityScore         int               `json:"security_score" db:"security_score"`
	StorageQuotaUsed      int64             `json:"storage_quota_used" db:"storage_quota_used"`
	StorageQuotaTotal     int64             `json:"storage_quota_total" db:"storage_quota_total"`
	TrustedDevices        JSONB             `json:"trusted_devices" db:"trusted_devices"`
	AuthMethods           JSONB             `json:"auth_methods" db:"auth_methods"`
	MetadataMinimization  bool              `json:"metadata_minimization" db:"metadata_minimization"`
	LoggingConsent        bool              `json:"logging_consent" db:"logging_consent"`
	AnalyticsOptOut       bool              `json:"analytics_opt_out" db:"analytics_opt_out"`
	AutoDeleteOldMessages bool              `json:"auto_delete_old_messages" db:"auto_delete_old_messages"`
	RetentionDays         int               `json:"retention_days" db:"retention_days"`
	SessionTimeout        int               `json:"session_timeout" db:"session_timeout"`
	LastSecurityScan      *time.Time        `json:"last_security_scan" db:"last_security_scan"`
	UITheme               string            `json:"ui_theme" db:"ui_theme"`
	AccentColor           string            `json:"accent_color" db:"accent_color"`
	Density               string            `json:"density" db:"density"`
	NetworkIdentities     []NetworkIdentity `json:"network_identities" db:"-"`
	CreatedAt             time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time         `json:"updated_at" db:"updated_at"`
	DeletedAt             gorm.DeletedAt    `json:"deleted_at" db:"deleted_at"`
}

// Session represents a user session
type Session struct {
	ID                uuid.UUID `json:"id" db:"id"`
	UserID            uuid.UUID `json:"user_id" db:"user_id"`
	Token             string    `json:"token" db:"token"`
	UserAgent         string    `json:"user_agent" db:"user_agent"`
	IPAddress         string    `json:"ip_address" db:"ip_address"`
	DeviceFingerprint string    `json:"-" db:"device_fingerprint"`
	ExpiresAt         time.Time `json:"expires_at" db:"expires_at"`
	LastActivity      time.Time `json:"last_activity" db:"last_activity"`
	CreatedAt         time.Time `json:"created_at" db:"created_at"`
}

// Folder represents an email folder
type Folder struct {
	ID          uuid.UUID  `json:"id" db:"id"`
	UserID      uuid.UUID  `json:"user_id" db:"user_id"`
	ParentID    *uuid.UUID `json:"parent_id" db:"parent_id"`
	Name        string     `json:"name" db:"name"`
	Path        string     `json:"path" db:"path"`
	IsSystem    bool       `json:"is_system" db:"is_system"`
	EmailCount  int        `json:"email_count" db:"email_count"`
	UnreadCount int        `json:"unread_count" db:"unread_count"`
	CreatedAt   time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at" db:"updated_at"`
}

// Label represents an email label
type Label struct {
	ID         uuid.UUID `json:"id" db:"id"`
	UserID     uuid.UUID `json:"user_id" db:"user_id"`
	Name       string    `json:"name" db:"name"`
	Color      string    `json:"color" db:"color"`
	Icon       string    `json:"icon" db:"icon"`
	IsSystem   bool      `json:"is_system" db:"is_system"`
	EmailCount int       `json:"email_count" db:"email_count"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// Contact represents a user contact
type Contact struct {
	ID                uuid.UUID   `json:"id" db:"id"`
	UserID            uuid.UUID   `json:"user_id" db:"user_id"`
	Name              string      `json:"name" db:"name"`
	EmailAddress      string      `json:"email_address" db:"email_address"`
	PublicKeys        StringArray `json:"public_keys" db:"public_keys"`
	NetworkIdentities JSONB       `json:"network_identities" db:"network_identities"`
	Notes             string      `json:"notes" db:"notes"`
	IsTrusted         bool        `json:"is_trusted" db:"is_trusted"`
	LastContacted     *time.Time  `json:"last_contacted" db:"last_contacted"`
	CreatedAt         time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at" db:"updated_at"`
}

// NetworkIdentity represents a network address
type NetworkIdentity struct {
	ID        uuid.UUID   `json:"id" db:"id"`
	UserID    uuid.UUID   `json:"user_id" db:"user_id"`
	Network   NetworkType `json:"network" db:"network"`
	Address   string      `json:"address" db:"address"`
	IsPrimary bool        `json:"is_primary" db:"is_primary"`
	IsActive  bool        `json:"is_active" db:"is_active"`
	ForwardTo string      `json:"forward_to" db:"forward_to"`
	Config    JSONB       `json:"config" db:"config"`
	LastUsed  *time.Time  `json:"last_used" db:"last_used"`
	CreatedAt time.Time   `json:"created_at" db:"created_at"`
	UpdatedAt time.Time   `json:"updated_at" db:"updated_at"`
}

// ProviderBridge represents an external email provider
type ProviderBridge struct {
	ID                   uuid.UUID    `json:"id" db:"id"`
	UserID               uuid.UUID    `json:"user_id" db:"user_id"`
	ProviderType         ProviderType `json:"provider_type" db:"provider_type"`
	AccountIdentifier    string       `json:"account_identifier" db:"account_identifier"`
	EncryptedCredentials string       `json:"-" db:"encrypted_credentials"`
	IsActive             bool         `json:"is_active" db:"is_active"`
	SyncInterval         int          `json:"sync_interval" db:"sync_interval"`
	LastSync             *time.Time   `json:"last_sync" db:"last_sync"`
	SyncStatus           string       `json:"sync_status" db:"sync_status"`
	Config               JSONB        `json:"config" db:"config"`
	CreatedAt            time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time    `json:"updated_at" db:"updated_at"`
}

// EncryptionKey represents a user's encryption key
type EncryptionKey struct {
	ID                  uuid.UUID      `json:"id" db:"id"`
	UserID              uuid.UUID      `json:"user_id" db:"user_id"`
	KeyType             string         `json:"key_type" db:"key_type"`
	KeyID               string         `json:"key_id" db:"key_id"`
	PublicKey           string         `json:"public_key" db:"public_key"`
	PrivateKeyEncrypted string         `json:"-" db:"private_key_encrypted"`
	Fingerprint         string         `json:"fingerprint" db:"fingerprint"`
	IsPrimary           bool           `json:"is_primary" db:"is_primary"`
	IsActive            bool           `json:"is_active" db:"is_active"`
	ExpiresAt           *time.Time     `json:"expires_at" db:"expires_at"`
	LastUsed            *time.Time     `json:"last_used" db:"last_used"`
	CreatedAt           time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt           time.Time      `json:"updated_at" db:"updated_at"`
	DeletedAt           gorm.DeletedAt `json:"deleted_at" db:"deleted_at"`
}

// Email represents an email message
type Email struct {
	ID             uuid.UUID      `json:"id" db:"id"`
	UserID         uuid.UUID      `json:"user_id" db:"user_id"`
	ThreadID       uuid.UUID      `json:"thread_id" db:"thread_id"`
	FolderID       *uuid.UUID     `json:"folder_id" db:"folder_id"`
	MessageID      string         `json:"message_id" db:"message_id"`
	From           string         `json:"from" db:"from"`
	To             StringArray    `json:"to" db:"to"`
	Cc             StringArray    `json:"cc" db:"cc"`
	Bcc            StringArray    `json:"bcc" db:"bcc"`
	Subject        string         `json:"subject" db:"subject"`
	BodyPlain      string         `json:"body_plain" db:"body_plain"`
	BodyHTML       string         `json:"body_html" db:"body_html"`
	BodyMarkdown   string         `json:"body_markdown" db:"body_markdown"`
	Network        NetworkType    `json:"network" db:"network"`
	Priority       int            `json:"priority" db:"priority"`
	IsRead         bool           `json:"is_read" db:"is_read"`
	IsStarred      bool           `json:"is_starred" db:"is_starred"`
	IsArchived     bool           `json:"is_archived" db:"is_archived"`
	IsSpam         bool           `json:"is_spam" db:"is_spam"`
	IsEncrypted    bool           `json:"is_encrypted" db:"is_encrypted"`
	EncryptionKeys StringArray    `json:"encryption_keys" db:"encryption_keys"`
	Attachments    JSONB          `json:"attachments" db:"attachments"`
	Headers        JSONB          `json:"headers" db:"headers"`
	InReplyTo      string         `json:"in_reply_to" db:"in_reply_to"`
	References     StringArray    `json:"references" db:"references"`
	SentAt         *time.Time     `json:"sent_at" db:"sent_at"`
	ReceivedAt     time.Time      `json:"received_at" db:"received_at"`
	CreatedAt      time.Time      `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at" db:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"deleted_at" db:"deleted_at"`
	Labels         []Label        `json:"labels" db:"-"`
}

// Attachment represents an email attachment
type Attachment struct {
	ID             uuid.UUID `json:"id" db:"id"`
	EmailID        uuid.UUID `json:"email_id" db:"email_id"`
	UserID         uuid.UUID `json:"user_id" db:"user_id"`
	Filename       string    `json:"filename" db:"filename"`
	ContentType    string    `json:"content_type" db:"content_type"`
	Size           int64     `json:"size" db:"size"`
	StoragePath    string    `json:"storage_path" db:"storage_path"`
	StorageBackend string    `json:"storage_backend" db:"storage_backend"`
	Checksum       string    `json:"checksum" db:"checksum"`
	IsInline       bool      `json:"is_inline" db:"is_inline"`
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
}

// AuditLog represents an audit trail entry
type AuditLog struct {
	ID           uuid.UUID  `json:"id" db:"id"`
	UserID       *uuid.UUID `json:"user_id" db:"user_id"`
	Action       string     `json:"action" db:"action"`
	ResourceType string     `json:"resource_type" db:"resource_type"`
	ResourceID   *uuid.UUID `json:"resource_id" db:"resource_id"`
	Details      JSONB      `json:"details" db:"details"`
	IPAddress    string     `json:"ip_address" db:"ip_address"`
	UserAgent    string     `json:"user_agent" db:"user_agent"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
}

// Statistics represents system statistics
type Statistics struct {
	ID          uuid.UUID `json:"id" db:"id"`
	MetricName  string    `json:"metric_name" db:"metric_name"`
	MetricValue JSONB     `json:"metric_value" db:"metric_value"`
	RecordedAt  time.Time `json:"recorded_at" db:"recorded_at"`
}

// PasswordResetToken represents a password reset token
type PasswordResetToken struct {
	ID        uuid.UUID  `json:"id" db:"id"`
	UserID    uuid.UUID  `json:"user_id" db:"user_id"`
	Token     string     `json:"token" db:"token"`
	ExpiresAt time.Time  `json:"expires_at" db:"expires_at"`
	UsedAt    *time.Time `json:"used_at" db:"used_at"`
	CreatedAt time.Time  `json:"created_at" db:"created_at"`
}
