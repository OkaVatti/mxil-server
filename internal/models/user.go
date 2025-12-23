package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents a system user
type User struct {
	ID                           uuid.UUID `gorm:"type:uuid;primaryKey"`
	MasterUsername               string    `gorm:"uniqueIndex;not null"`
	Email                        string    `gorm:"uniqueIndex;not null"`
	PasswordHash                 string    `gorm:"not null"`
	DisplayName                  string
	EmailVerified                bool `gorm:"default:false"`
	EmailVerificationToken       string
	EmailVerificationTokenExpiry time.Time
	PasswordResetToken           string
	PasswordResetExpiry          time.Time
	MFAEnabled                   bool `gorm:"default:false"`
	MFASecret                    string
	MFARecoveryCodes             []string `gorm:"type:text[]"`
	IsActive                     bool     `gorm:"default:true"`
	CreatedAt                    time.Time
	UpdatedAt                    time.Time
	LastLogin                    *time.Time
	StorageQuotaTotal            int64             `gorm:"default:1073741824"` // 1GB in bytes
	StorageQuotaUsed             int64             `gorm:"default:0"`
	SessionTimeout               int               `gorm:"default:86400"` // 24 hours in seconds
	NetworkIdentities            []NetworkIdentity `gorm:"foreignKey:UserID"`
	TrustedDevices               []TrustedDevice   `gorm:"foreignKey:UserID"`
}

// NetworkIdentity represents a user's identity on a specific network
type NetworkIdentity struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID    uuid.UUID `gorm:"not null"`
	Network   string    `gorm:"not null"`
	Address   string    `gorm:"not null"`
	IsPrimary bool      `gorm:"default:false"`
	ForwardTo string
	IsActive  bool `gorm:"default:true"`
	CreatedAt time.Time
	LastUsed  time.Time
	Config    string `gorm:"type:jsonb"`
}

// TrustedDevice represents a trusted device for a user
type TrustedDevice struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID      uuid.UUID `gorm:"not null"`
	Fingerprint string    `gorm:"not null"`
	Name        string
	UserAgent   string
	IPAddress   string
	LastUsed    time.Time
	CreatedAt   time.Time
}

// Session represents a user session
type Session struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID `gorm:"not null;index"`
	DeviceID     uuid.UUID
	UserAgent    string
	IPAddress    string
	DeviceName   string
	CreatedAt    time.Time
	LastActivity time.Time
	ExpiresAt    time.Time
	RevokedAt    *time.Time
	TokenHash    string
}

// Email represents an email message
type Email struct {
	ID               uuid.UUID   `gorm:"type:uuid;primaryKey"`
	UserID           uuid.UUID   `gorm:"not null;index"`
	ThreadID         uuid.UUID   `gorm:"not null;index"`
	From             string      `gorm:"not null"`
	To               StringArray `gorm:"type:text[]"`
	Cc               StringArray `gorm:"type:text[]"`
	Bcc              StringArray `gorm:"type:text[]"`
	Subject          string
	BodyPlain        string
	BodyHTML         string
	BodyMarkdown     string
	Network          string      `gorm:"not null"`
	Direction        string      `gorm:"not null"`
	Status           string      `gorm:"not null"`
	IsEncrypted      bool        `gorm:"default:false"`
	EncryptionKeyIDs StringArray `gorm:"type:text[]"`
	EncryptedContent []byte
	IsSpam           bool `gorm:"default:false"`
	Size             int64
	AttachmentIDs    UUIDArray `gorm:"type:uuid[]"`
	MessageID        string    `gorm:"uniqueIndex"`
	InReplyTo        *string
	References       StringArray `gorm:"type:text[]"`
	SentAt           *time.Time
	ReceivedAt       *time.Time
	ReadAt           *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// StringArray is a custom type for string arrays
type StringArray []string

// UUIDArray is a custom type for UUID arrays
type UUIDArray []uuid.UUID

// NetworkType represents network types
type NetworkType string

const (
	NetworkClearnet NetworkType = "clearnet"
	NetworkI2P      NetworkType = "i2p"
	NetworkTor      NetworkType = "tor"
	NetworkLAN      NetworkType = "lan"
	NetworkIPFS     NetworkType = "ipfs"
)

// EmailDirection represents email directions
type EmailDirection string

const (
	EmailDirectionIncoming EmailDirection = "incoming"
	EmailDirectionOutgoing EmailDirection = "outgoing"
)

// EmailStatus represents email statuses
type EmailStatus string

const (
	EmailStatusPending  EmailStatus = "pending"
	EmailStatusSent     EmailStatus = "sent"
	EmailStatusFailed   EmailStatus = "failed"
	EmailStatusReceived EmailStatus = "received"
)

// BeforeCreate hook
func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}

	// Create default folders
	defaultFolders := []Folder{
		{ID: uuid.New(), UserID: u.ID, Name: "Inbox", Path: "Inbox", IsSystem: true},
		{ID: uuid.New(), UserID: u.ID, Name: "Sent", Path: "Sent", IsSystem: true},
		{ID: uuid.New(), UserID: u.ID, Name: "Drafts", Path: "Drafts", IsSystem: true},
		{ID: uuid.New(), UserID: u.ID, Name: "Spam", Path: "Spam", IsSystem: true},
		{ID: uuid.New(), UserID: u.ID, Name: "Trash", Path: "Trash", IsSystem: true},
		{ID: uuid.New(), UserID: u.ID, Name: "Archive", Path: "Archive", IsSystem: true},
	}

	for _, folder := range defaultFolders {
		if err := tx.Create(&folder).Error; err != nil {
			return err
		}
	}

	// Create default labels
	defaultLabels := []Label{
		{ID: uuid.New(), UserID: u.ID, Name: "Important", Color: "#ff0000", IsSystem: true},
		{ID: uuid.New(), UserID: u.ID, Name: "Work", Color: "#0000ff", IsSystem: false},
		{ID: uuid.New(), UserID: u.ID, Name: "Personal", Color: "#00ff00", IsSystem: false},
	}

	for _, label := range defaultLabels {
		if err := tx.Create(&label).Error; err != nil {
			return err
		}
	}

	return nil
}

// AfterCreate hook
func (u *User) AfterCreate(tx *gorm.DB) error {
	// Create primary network identity for clearnet
	identity := NetworkIdentity{
		ID:        uuid.New(),
		UserID:    u.ID,
		Network:   NetworkClearnet,
		Address:   u.Email,
		IsPrimary: true,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	return tx.Create(&identity).Error
}

// UpdateStorageUsed updates storage usage
func (u *User) UpdateStorageUsed(tx *gorm.DB, delta int64) error {
	return tx.Model(u).Update("storage_quota_used", gorm.Expr("storage_quota_used + ?", delta)).Error
}

// CheckStorageQuota checks if user has enough storage
func (u *User) CheckStorageQuota(size int64) bool {
	return u.StorageQuotaUsed+size <= u.StorageQuotaTotal
}

// IsLocked checks if account is locked
func (u *User) IsLocked() bool {
	return u.LockedUntil != nil && u.LockedUntil.After(time.Now())
}

// NeedsMFAVerification checks if MFA verification is needed
func (u *User) NeedsMFAVerification() bool {
	return u.MFAEnabled && (u.MFALastUsed == nil || time.Since(*u.MFALastUsed) > 30*24*time.Hour)
}
