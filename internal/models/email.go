package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// BeforeCreate hook
func (e *Email) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.MessageID == "" {
		e.MessageID = generateMessageID()
	}
	// Calculate thread ID if reply
	if e.InReplyTo != "" && e.ThreadID == nil {
		var parentEmail Email
		if err := tx.Where("message_id = ?", e.InReplyTo).First(&parentEmail).Error; err == nil {
			if parentEmail.ThreadID != nil {
				e.ThreadID = parentEmail.ThreadID
			} else {
				e.ThreadID = &parentEmail.ID
			}
		}
	}
	// Set thread ID to self if new thread
	if e.ThreadID == nil {
		e.ThreadID = &e.ID
	}
	return nil
}

// TableName specifies the table name
func (Email) TableName() string {
	return "emails"
}

// Attachment with complete fields
type Attachment struct {
	ID             uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	EmailID        uuid.UUID      `gorm:"type:uuid;not null;index;constraint:OnDelete:CASCADE"`
	Filename       string         `gorm:"not null"`
	ContentType    string         `gorm:"not null"`
	ContentID      string         `gorm:"index"`
	Size           int64          `gorm:"not null"`
	StoragePath    string         `gorm:"not null"`
	StorageBackend string         `gorm:"not null;default:'local'"`
	IsInline       bool           `gorm:"default:false"`
	Hash           string         `gorm:"index"` // SHA256 for deduplication
	CreatedAt      time.Time      `gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt      gorm.DeletedAt `gorm:"index"`

	Email Email `gorm:"foreignKey:EmailID"`
}

// EmailEncryption stores encryption details for an email
type EmailEncryption struct {
	ID        uuid.UUID   `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	EmailID   uuid.UUID   `gorm:"type:uuid;not null;uniqueIndex;constraint:OnDelete:CASCADE"`
	Algorithm string      `gorm:"not null"`
	KeyIDs    StringArray `gorm:"type:text[]"`
	CreatedAt time.Time   `gorm:"default:CURRENT_TIMESTAMP"`

	Email Email `gorm:"foreignKey:EmailID"`
}

func generateMessageID() string {
	return "<" + uuid.New().String() + "@mxil>"
}
