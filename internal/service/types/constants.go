// internal/service/types/constants.go
package types

const (
	// Default limits
	DefaultMaxEmailSize      = 50 * 1024 * 1024 // 50MB
	DefaultMaxAttachments    = 10
	DefaultMaxAttachmentSize = 25 * 1024 * 1024 // 25MB per attachment

	// Timeouts
	DefaultSessionTimeout   = 24 * 60 * 60 // 24 hours
	DefaultLockoutDuration  = 15 * 60      // 15 minutes
	DefaultMaxLoginAttempts = 5

	// Storage quotas
	DefaultStorageQuota = 1 << 30 // 1GB
)

// Service configuration
type ServiceConfig struct {
	MaxEmailSize      int64
	MaxAttachments    int
	MaxAttachmentSize int64
	SessionTimeout    int64
	LockoutDuration   int64
	MaxLoginAttempts  int
	StorageQuota      int64
	NetworkAdapters   map[string]interface{}
}
