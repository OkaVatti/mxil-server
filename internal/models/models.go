package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// NetworkType defines supported network types
type NetworkType string

const (
	NetworkClearnet NetworkType = "clearnet"
	NetworkI2P      NetworkType = "i2p"
	NetworkTor      NetworkType = "tor"
	NetworkLAN      NetworkType = "lan"
	NetworkIPFS     NetworkType = "ipfs"
)

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

// StringArray is a custom type for storing string arrays in PostgreSQL
type StringArray []string

// JSONB is a custom type for storing JSONB data in PostgreSQL
type JSONB map[string]interface{}

// User represents a user in the system
type User struct {
	ID                  uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	MasterUsername      string    `gorm:"uniqueIndex;not null"`
	Email               string    `gorm:"uniqueIndex;not null"`
	EmailVerified       bool      `gorm:"default:false"`
	DisplayName         string    `gorm:"not null"`
	Bio                 string
	PasswordHash        string `gorm:"not null"`
	MFAEnabled          bool   `gorm:"default:false"`
	MFASecret           string
	StorageQuotaUsed    int64 `gorm:"default:0"`
	StorageQuotaTotal   int64 `gorm:"default:10737418240"` // 10GB default
	SecurityScore       int   `gorm:"default:0"`
	FailedLoginAttempts int   `gorm:"default:0"`
	LockedUntil         *time.Time
	LastLogin           *time.Time
	Timezone            string `gorm:"default:'UTC'"`

	// Privacy settings
	MetadataMinimization  bool `gorm:"default:true"`
	LoggingConsent        bool `gorm:"default:true"`
	AnalyticsOptOut       bool `gorm:"default:false"`
	AutoDeleteOldMessages bool `gorm:"default:false"`
	RetentionDays         int  `gorm:"default:365"`

	// Security settings
	SessionTimeout   int `gorm:"default:86400"` // 24 hours in seconds
	LastSecurityScan *time.Time

	// UI settings
	UITheme     string `gorm:"default:'light'"`
	AccentColor string `gorm:"default:'#6d4aff'"`
	Density     string `gorm:"default:'comfortable'"`

	// Timestamps
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Associations
	NetworkIdentities []NetworkIdentity `gorm:"foreignKey:UserID"`
	AuthMethods       []AuthMethod      `gorm:"foreignKey:UserID"`
	TrustedDevices    []TrustedDevice   `gorm:"foreignKey:UserID"`
}

// Email represents an email message
type Email struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index"`
	ThreadID    uuid.UUID `gorm:"type:uuid;index"`
	MessageID   string    `gorm:"uniqueIndex;not null"`
	Subject     string
	BodyPlain   string      `gorm:"type:text"`
	BodyHTML    string      `gorm:"type:text"`
	Sender      string      `gorm:"not null"`
	Recipients  StringArray `gorm:"type:jsonb"`
	CC          StringArray `gorm:"type:jsonb"`
	BCC         StringArray `gorm:"type:jsonb"`
	Network     NetworkType `gorm:"not null;index"`
	Direction   string      `gorm:"not null;index"` // "incoming" or "outgoing"
	IsRead      bool        `gorm:"default:false;index"`
	IsStarred   bool        `gorm:"default:false;index"`
	IsArchived  bool        `gorm:"default:false;index"`
	IsSpam      bool        `gorm:"default:false;index"`
	IsEncrypted bool        `gorm:"default:false"`
	IsSigned    bool        `gorm:"default:false"`
	Size        int64       `gorm:"default:0"`
	FolderID    uuid.UUID   `gorm:"type:uuid;index"`

	// Timestamps
	CreatedAt time.Time `gorm:"index"`
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Associations
	User        User         `gorm:"foreignKey:UserID"`
	Folder      Folder       `gorm:"foreignKey:FolderID"`
	Labels      []Label      `gorm:"many2many:email_labels;"`
	Attachments []Attachment `gorm:"foreignKey:EmailID"`
}

// Session represents a user session
type Session struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index"`
	TokenHash    string    `gorm:"not null;index"`
	UserAgent    string
	IPAddress    string
	Location     string
	DeviceName   string
	LastActivity time.Time
	ExpiresAt    time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Association
	User User `gorm:"foreignKey:UserID"`
}

// NetworkIdentity represents a network identity/address
type NetworkIdentity struct {
	ID        uuid.UUID   `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID    uuid.UUID   `gorm:"type:uuid;not null;index"`
	Network   NetworkType `gorm:"not null;index"`
	Address   string      `gorm:"not null;index"`
	IsPrimary bool        `gorm:"default:false;index"`
	ForwardTo string
	Config    JSONB `gorm:"type:jsonb"`
	IsActive  bool  `gorm:"default:true;index"`
	LastUsed  *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`

	// Association
	User User `gorm:"foreignKey:UserID"`
}

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

// Attachment represents an email attachment
type Attachment struct {
	ID          uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	EmailID     uuid.UUID `gorm:"type:uuid;not null;index"`
	Filename    string    `gorm:"not null"`
	ContentType string    `gorm:"not null"`
	Size        int64     `gorm:"not null"`
	StoragePath string    `gorm:"not null"`
	IsInline    bool      `gorm:"default:false"`
	CreatedAt   time.Time
	DeletedAt   gorm.DeletedAt `gorm:"index"`

	// Association
	Email Email `gorm:"foreignKey:EmailID"`
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

// TrustedDevice represents a trusted device for a user
type TrustedDevice struct {
	ID         uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index"`
	DeviceHash string    `gorm:"not null;index"`
	DeviceName string
	LastUsed   time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  gorm.DeletedAt `gorm:"index"`

	// Association
	User User `gorm:"foreignKey:UserID"`
}
