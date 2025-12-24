package storage

import (
	"context"
	"fmt"
	"io"
	"path/filepath"

	"go.uber.org/zap"
)

// StorageBackend defines the interface for storage backends
type StorageBackend interface {
	Store(ctx context.Context, key string, data io.Reader) error
	Retrieve(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
	Exists(ctx context.Context, key string) (bool, error)
}

// LocalStorage implements StorageBackend for local filesystem
type LocalStorage struct {
	basePath string
	logger   *zap.Logger
}

// NewLocalStorage creates a new local storage instance
func NewLocalStorage(basePath string, logger *zap.Logger) (*LocalStorage, error) {
	return &LocalStorage{
		basePath: basePath,
		logger:   logger,
	}, nil
}

func (s *LocalStorage) Store(ctx context.Context, key string, data io.Reader) error {
	path := filepath.Join(s.basePath, key)
	// Implementation would write data to file
	return nil
}

func (s *LocalStorage) Retrieve(ctx context.Context, key string) (io.ReadCloser, error) {
	// Implementation would read file
	return nil, fmt.Errorf("not implemented")
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	return nil
}

func (s *LocalStorage) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}
