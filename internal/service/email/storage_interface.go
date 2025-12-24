// internal/service/email/storage_interface.go
package email

import (
	"context"
	"io"

	"github.com/google/uuid"
)

// StorageService interface for email service
type StorageService interface {
	Store(ctx context.Context, key string, data io.Reader) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	FindByHash(ctx context.Context, hash string) (uuid.UUID, error)
	SetMetadata(ctx context.Context, key string, metadata map[string]interface{}) error
	GetMetadata(ctx context.Context, key string) (map[string]interface{}, error)
}
