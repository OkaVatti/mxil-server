// internal/service/email/storage_interface.go
package email

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
)

// StorageInterface defines the interface for email storage operations
type StorageInterface interface {
	// StoreEmail stores an email with attachments
	StoreEmail(ctx context.Context, email *models.Email, attachments []*Attachment) (string, error)

	// GetEmail retrieves an email and its attachments
	GetEmail(ctx context.Context, emailID uuid.UUID) (*models.Email, []*Attachment, error)

	// DeleteEmail permanently removes an email
	DeleteEmail(ctx context.Context, emailID uuid.UUID) error

	// StoreAttachment stores an email attachment
	StoreAttachment(ctx context.Context, emailID uuid.UUID, attachment *Attachment) (string, error)

	// GetAttachment retrieves an email attachment
	GetAttachment(ctx context.Context, attachmentID uuid.UUID) (*Attachment, error)

	// DeleteAttachment removes an attachment
	DeleteAttachment(ctx context.Context, attachmentID uuid.UUID) error

	// GetEmailSize calculates total storage used by an email
	GetEmailSize(ctx context.Context, emailID uuid.UUID) (int64, error)

	// GetUserStorageUsage gets total storage used by a user
	GetUserStorageUsage(ctx context.Context, userID uuid.UUID) (int64, error)

	// CleanupOrphanedAttachments removes attachments not linked to emails
	CleanupOrphanedAttachments(ctx context.Context) (int64, error)

	// BackupEmail creates a backup of an email
	BackupEmail(ctx context.Context, emailID uuid.UUID) (string, error)

	// RestoreEmail restores an email from backup
	RestoreEmail(ctx context.Context, backupID string) (*models.Email, error)

	// ListAttachments lists all attachments for an email
	ListAttachments(ctx context.Context, emailID uuid.UUID) ([]*Attachment, error)

	// HealthCheck checks storage backend health
	HealthCheck(ctx context.Context) error

	// GetStatistics returns storage statistics
	GetStatistics(ctx context.Context) (*StorageStats, error)
}

// Attachment represents an email attachment
type Attachment struct {
	ID          uuid.UUID `json:"id"`
	EmailID     uuid.UUID `json:"email_id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	Size        int64     `json:"size"`
	Data        []byte    `json:"-"` // Raw data, not serialized to JSON
	Checksum    string    `json:"checksum"`
	IsInline    bool      `json:"is_inline"`
	CreatedAt   time.Time `json:"created_at"`
}

// StorageStats contains storage statistics
type StorageStats struct {
	TotalEmails      int64   `json:"total_emails"`
	TotalAttachments int64   `json:"total_attachments"`
	TotalSize        int64   `json:"total_size"`
	AverageEmailSize float64 `json:"average_email_size"`
	UsersCount       int     `json:"users_count"`
}

// StorageConfig contains configuration for storage backend
type StorageConfig struct {
	Backend           string `json:"backend"` // local, s3, ipfs, etc.
	LocalPath         string `json:"local_path,omitempty"`
	S3Bucket          string `json:"s3_bucket,omitempty"`
	S3Region          string `json:"s3_region,omitempty"`
	S3AccessKey       string `json:"s3_access_key,omitempty"`
	S3SecretKey       string `json:"s3_secret_key,omitempty"`
	IPFSGateway       string `json:"ipfs_gateway,omitempty"`
	IPFSAPI           string `json:"ipfs_api,omitempty"`
	MaxFileSize       int64  `json:"max_file_size"`
	EncryptionEnabled bool   `json:"encryption_enabled"`
	CompressionLevel  int    `json:"compression_level"`
}

// StorageError represents a storage-related error
type StorageError struct {
	Operation string `json:"operation"`
	Message   string `json:"message"`
	Code      string `json:"code"`
}

func (e *StorageError) Error() string {
	return e.Message
}

// Common storage error codes
const (
	ErrStorageFull      = "storage_full"
	ErrFileTooLarge     = "file_too_large"
	ErrNotFound         = "not_found"
	ErrPermissionDenied = "permission_denied"
	ErrCorruptedData    = "corrupted_data"
	ErrEncryptionFailed = "encryption_failed"
	ErrDecryptionFailed = "decryption_failed"
)

// NewStorageError creates a new storage error
func NewStorageError(operation, message, code string) *StorageError {
	return &StorageError{
		Operation: operation,
		Message:   message,
		Code:      code,
	}
}

// ReaderSeekerCloser combines io.Reader, io.Seeker, and io.Closer
type ReaderSeekerCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

// StorageBackend is the interface that all storage backends must implement
type StorageBackend interface {
	// Store stores data and returns a unique identifier
	Store(ctx context.Context, data []byte, metadata map[string]string) (string, error)

	// Retrieve retrieves data by identifier
	Retrieve(ctx context.Context, id string) ([]byte, map[string]string, error)

	// Delete removes data by identifier
	Delete(ctx context.Context, id string) error

	// Exists checks if data exists
	Exists(ctx context.Context, id string) (bool, error)

	// List lists all stored items with optional prefix
	List(ctx context.Context, prefix string) ([]string, error)

	// GetSize gets the size of stored data
	GetSize(ctx context.Context, id string) (int64, error)

	// Cleanup removes old/unused data
	Cleanup(ctx context.Context, olderThan time.Time) (int64, error)

	// HealthCheck checks backend health
	HealthCheck(ctx context.Context) error

	// Close releases resources
	Close() error
}

// StorageFactory creates storage backends
type StorageFactory interface {
	CreateBackend(config StorageConfig) (StorageBackend, error)
	SupportedBackends() []string
}
