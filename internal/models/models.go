package models

// NetworkType defines supported network types

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NetworkType represents the type of network
const ()

// ProviderType defines supported email provider types
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

// JSONB is a custom type for storing JSONB data in PostgreSQL
type JSONB map[string]interface{}

// Contact represents a contact in the address book
type Contact struct {
	ID                uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID            uuid.UUID `gorm:"type:uuid;not null;index"`
	Name              string
	EmailAddress      string      `gorm:"not null;index"`
	PublicKeys        StringArray `gorm:"type:jsonb"`
	NetworkIdentities JSONB       `gorm:"type:jsonb"`
	Notes             string
	IsTrusted         bool `gorm:"default:false"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         gorm.DeletedAt `gorm:"index"`

	// Association
	User User `gorm:"foreignKey:UserID"`
}

// Folder represents an email folder
type Folder struct {
	ID        uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID    uuid.UUID  `gorm:"type:uuid;not null;index"`
	ParentID  *uuid.UUID `gorm:"type:uuid;index"`
	Name      string     `gorm:"not null"`
	Path      string     `gorm:"not null;index"`
	IsSystem  bool       `gorm:"default:false"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Associations
	User   User    `gorm:"foreignKey:UserID"`
	Parent *Folder `gorm:"foreignKey:ParentID"`
}

// Label represents an email label
type Label struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index"`
	Name       string    `gorm:"not null;index"`
	Color      string
	Icon       string
	IsSystem   bool  `gorm:"default:false"`
	EmailCount int64 `gorm:"-:all"` // Ignored by GORM, used for counting emails
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`

	// Associations
	User   User    `gorm:"foreignKey:UserID"`
	Emails []Email `gorm:"many2many:email_labels;"`
}

// EncryptionKey represents an encryption key pair
type EncryptionKey struct {
	ID                  uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID              uuid.UUID `gorm:"type:uuid;not null;index"`
	KeyType             string    `gorm:"not null;index"` // e.g., "rsa-2048", "ec-p256"
	KeyID               string    `gorm:"not null;uniqueIndex"`
	PublicKey           string    `gorm:"type:text;not null"`
	PrivateKeyEncrypted string    `gorm:"type:text;not null"`
	Fingerprint         string    `gorm:"not null;index"`
	IsPrimary           bool      `gorm:"default:false;index"`
	IsActive            bool      `gorm:"default:true;index"`
	LastUsed            *time.Time
	ExpiresAt           *time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
	DeletedAt           gorm.DeletedAt `gorm:"index"`

	// Association
	User User `gorm:"foreignKey:UserID"`
}

// ProviderBridge represents a bridge to an external email provider
type ProviderBridge struct {
	ID                   uuid.UUID    `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID               uuid.UUID    `gorm:"type:uuid;not null;index"`
	ProviderType         ProviderType `gorm:"not null;index"`
	AccountIdentifier    string       `gorm:"not null;index"`
	EncryptedCredentials string       `gorm:"type:text;not null"`
	IsActive             bool         `gorm:"default:true;index"`
	SyncInterval         int          `gorm:"default:300"` // 5 minutes in seconds
	LastSync             *time.Time
	Config               JSONB `gorm:"type:jsonb"`
	CreatedAt            time.Time
	UpdatedAt            time.Time
	DeletedAt            gorm.DeletedAt `gorm:"index"`

	// Association
	User User `gorm:"foreignKey:UserID"`
}

// AuthMethod represents an authentication method (for MFA)
type AuthMethod struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Type      string    `gorm:"not null"` // "totp", "webauthn", "backup_code"
	Config    JSONB     `gorm:"type:jsonb"`
	IsEnabled bool      `gorm:"default:true"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Association
	User User `gorm:"foreignKey:UserID"`
}
