// internal/models/email.go
package models

import (
	"time"

	"github.com/google/uuid"
)

// TableName specifies the table name
func (Email) TableName() string {
	return "emails"
}

// TableName specifies the table name
func (Attachment) TableName() string {
	return "attachments"
}

// TableName specifies the table name
func (Label) TableName() string {
	return "labels"
}

// TableName specifies the table name
func (Folder) TableName() string {
	return "folders"
}

// EmailRule represents an email filtering rule
type EmailRule struct {
	ID         uuid.UUID `json:"id" gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	UserID     uuid.UUID `json:"user_id" gorm:"type:uuid;not null;index"`
	Name       string    `json:"name" gorm:"not null"`
	Conditions JSONB     `json:"conditions" gorm:"type:jsonb;not null"`
	Actions    JSONB     `json:"actions" gorm:"type:jsonb;not null"`
	Priority   int       `json:"priority" gorm:"default:0;index:,sort:desc"`
	IsActive   bool      `json:"is_active" gorm:"default:true"`
	Timestamps
}

// TableName specifies the table name
func (EmailRule) TableName() string {
	return "email_rules"
}

// TableName specifies the table name
func (EncryptionKey) TableName() string {
	return "encryption_keys"
}

// EmailEncryption represents encryption metadata for an email
type EmailEncryption struct {
	ID              uuid.UUID   `json:"id" gorm:"type:uuid;primaryKey;default:uuid_generate_v4()"`
	EmailID         uuid.UUID   `json:"email_id" gorm:"type:uuid;not null;index"`
	Layers          JSONB       `json:"layers" gorm:"type:jsonb;not null"`
	KeyIDs          StringArray `json:"key_ids" gorm:"type:text[];not null"`
	Algorithm       string      `json:"algorithm" gorm:"not null"`
	KeySize         *int        `json:"key_size,omitempty"`
	IsHybrid        bool        `json:"is_hybrid" gorm:"default:false"`
	IsPQEnabled     bool        `json:"is_pq_enabled" gorm:"default:false"`
	IsForwardSecure bool        `json:"is_forward_secure" gorm:"default:false"`
	CreatedAt       time.Time   `json:"created_at" gorm:"default:CURRENT_TIMESTAMP"`
}

// TableName specifies the table name
func (EmailEncryption) TableName() string {
	return "email_encryption"
}

// TableName specifies the table name
func (Contact) TableName() string {
	return "contacts"
}
