// internal/models/user_complete.go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Complete User model with all required fields
type User struct {
	// Base fields
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	MasterUsername string    `gorm:"uniqueIndex;not null"`
	Email          string    `gorm:"uniqueIndex;not null"`
	PasswordHash   string    `gorm:"not null"`
	DisplayName    string
	Bio            string

	// Email verification
	EmailVerified                bool `gorm:"default:false"`
	EmailVerificationToken       string
	EmailVerificationTokenExpiry time.Time

	// Password reset
	PasswordResetToken  string
	PasswordResetExpiry time.Time

	// MFA
	MFAEnabled       bool `gorm:"default:false"`
	MFASecret        string
	MFARecoveryCodes []string `gorm:"type:text[]"`
	MFALastUsed      *time.Time

	// Security
	SecurityScore       int `gorm:"default:0"`
	LastSecurityScan    *time.Time
	SessionTimeout      int `gorm:"default:86400"` // 24 hours
	LockedUntil         *time.Time
	FailedLoginAttempts int `gorm:"default:0"`

	// Storage
	StorageQuotaTotal int64 `gorm:"default:1073741824"` // 1GB
	StorageQuotaUsed  int64 `gorm:"default:0"`

	// Privacy settings
	MetadataMinimization  bool `gorm:"default:true"`
	LoggingConsent        bool `gorm:"default:false"`
	AnalyticsOptOut       bool `gorm:"default:true"`
	AutoDeleteOldMessages bool `gorm:"default:false"`
	RetentionDays         int  `gorm:"default:0"`

	// UI preferences
	UITheme     string `gorm:"default:'dark'"`
	AccentColor string `gorm:"default:'#6d4aff'"`
	Density     string `gorm:"default:'comfortable'"`

	// Status
	IsActive bool `gorm:"default:true"`

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
	LastLogin *time.Time

	// Relationships
	NetworkIdentities []NetworkIdentity `gorm:"foreignKey:UserID"`
	TrustedDevices    []TrustedDevice   `gorm:"foreignKey:UserID"`
	AuthMethods       []AuthMethod      `gorm:"foreignKey:UserID"`
}

// TableName specifies the table name
func (User) TableName() string {
	return "users"
}

// Email model with all required fields
type Email struct {
	ID         uuid.UUID   `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID   `gorm:"type:uuid;not null;index"`
	ThreadID   *uuid.UUID  `gorm:"type:uuid;index"`
	MessageID  string      `gorm:"uniqueIndex;not null"`
	InReplyTo  string      `gorm:"index"`
	References StringArray `gorm:"type:text[]"`

	// Addresses
	FromAddress  string      `gorm:"not null;index"`
	ToAddresses  StringArray `gorm:"type:text[];not null"`
	CCAddresses  StringArray `gorm:"type:text[]"`
	BCCAddresses StringArray `gorm:"type:text[]"`
	ReplyTo      string

	// Content
	Subject      string
	BodyPlain    string `gorm:"type:text"`
	BodyHTML     string `gorm:"type:text"`
	BodyMarkdown string `gorm:"type:text"`

	// Metadata
	Network   NetworkType `gorm:"not null;index"`
	FolderID  *uuid.UUID  `gorm:"type:uuid;index"`
	Direction string      `gorm:"not null;default:'incoming'"`
	Status    string      `gorm:"not null;default:'received'"`

	// Flags
	IsRead     bool `gorm:"default:false;index"`
	IsStarred  bool `gorm:"default:false;index"`
	IsArchived bool `gorm:"default:false;index"`
	IsSpam     bool `gorm:"default:false;index"`
	IsDraft    bool `gorm:"default:false"`

	// Encryption
	IsEncrypted      bool        `gorm:"default:false"`
	EncryptionKeyIDs StringArray `gorm:"type:text[]"`

	// Attachments
	HasAttachments bool         `gorm:"default:false"`
	AttachmentIDs  UUIDArray    `gorm:"type:uuid[]"`
	Attachments    []Attachment `gorm:"foreignKey:EmailID"`

	// Size tracking
	Size      int64   `gorm:"not null;default:0"`
	SpamScore float64 `gorm:"default:0"`

	// Timestamps
	SentAt     *time.Time
	ReceivedAt time.Time `gorm:"not null;index"`
	ReadAt     *time.Time
	CreatedAt  time.Time      `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt  time.Time      `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt  gorm.DeletedAt `gorm:"index"`

	// Network-specific
	ReceivedVia NetworkType `gorm:"not null"`

	// Relationships
	User   User    `gorm:"foreignKey:UserID"`
	Folder *Folder `gorm:"foreignKey:FolderID"`
	Labels []Label `gorm:"many2many:email_labels;"`
}

// TableName specifies the table name
func (Email) TableName() string {
	return "emails"
}

// NetworkIdentity model with Location field
type NetworkIdentity struct {
	ID        uuid.UUID   `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID   `gorm:"type:uuid;not null;index"`
	Network   NetworkType `gorm:"not null;index"`
	Address   string      `gorm:"not null;uniqueIndex"`
	IsPrimary bool        `gorm:"default:false;index"`
	ForwardTo string
	Config    JSONB     `gorm:"type:jsonb"`
	IsActive  bool      `gorm:"default:true;index"`
	Location  string    // Added missing field
	CreatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	LastUsed  *time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Relationships
	User User `gorm:"foreignKey:UserID"`
}

// TableName specifies the table name
func (NetworkIdentity) TableName() string {
	return "network_identities"
}

// Session model with Location field
type Session struct {
	ID                uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID            uuid.UUID  `gorm:"type:uuid;not null;index"`
	TokenHash         string     `gorm:"uniqueIndex;not null"`
	DeviceID          *uuid.UUID `gorm:"type:uuid"`
	DeviceFingerprint string     `gorm:"index"`
	UserAgent         string
	IPAddress         string `gorm:"index"`
	DeviceName        string
	Location          string    // Added missing field
	CreatedAt         time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	LastActivity      time.Time `gorm:"default:CURRENT_TIMESTAMP"`
	ExpiresAt         time.Time `gorm:"not null;index"`
	RevokedAt         *time.Time
	DeletedAt         gorm.DeletedAt `gorm:"index"`

	// Relationships
	User User `gorm:"foreignKey:UserID"`
}

// TableName specifies the table name
func (Session) TableName() string {
	return "sessions"
}

// StringArray type with proper Scan/Value methods (already in types.go)
type StringArray []string

// NetworkType enum (already defined but included for completeness)
type NetworkType string

const (
	NetworkClearnetComplete NetworkType = "clearnet"
	NetworkI2PComplete      NetworkType = "i2p"
	NetworkTorComplete      NetworkType = "tor"
	NetworkLANComplete      NetworkType = "lan"
	NetworkIPFSComplete     NetworkType = "ipfs"
)
