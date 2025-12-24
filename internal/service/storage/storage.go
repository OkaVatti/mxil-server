// internal/service/storage/storage_service.go
package storageservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/config"
)

// StorageService implements the storage interface
type StorageService struct {
	backend StorageBackend
	config  config.StorageConfig
	logger  *zap.Logger
	mu      sync.RWMutex
}

// StorageBackend defines the interface for storage backends
type StorageBackend interface {
	Store(ctx context.Context, data []byte, metadata map[string]string) (string, error)
	Retrieve(ctx context.Context, id string) ([]byte, map[string]string, error)
	Delete(ctx context.Context, id string) error
	Exists(ctx context.Context, id string) (bool, error)
	GetSize(ctx context.Context, id string) (int64, error)
	HealthCheck(ctx context.Context) error
	Close() error
}

// NewStorageService creates a new storage service
func NewStorageService(cfg config.StorageConfig, logger *zap.Logger) (*StorageService, error) {
	var backend StorageBackend
	var err error

	switch cfg.Backend {
	case "local":
		backend, err = NewLocalStorageBackend(cfg.LocalPath, logger)
	case "s3":
		backend, err = NewS3StorageBackend(cfg, logger)
	case "ipfs":
		backend, err = NewIPFSStorageBackend(cfg, logger)
	default:
		return nil, fmt.Errorf("unsupported storage backend: %s", cfg.Backend)
	}

	if err != nil {
		return nil, err
	}

	return &StorageService{
		backend: backend,
		config:  cfg,
		logger:  logger,
	}, nil
}

// StoreEmail stores an email with attachments
func (s *StorageService) StoreEmail(ctx context.Context, emailID uuid.UUID, data []byte) (string, error) {
	metadata := map[string]string{
		"type":       "email",
		"email_id":   emailID.String(),
		"stored_at":  time.Now().Format(time.RFC3339),
		"size_bytes": fmt.Sprintf("%d", len(data)),
	}

	return s.backend.Store(ctx, data, metadata)
}

// RetrieveEmail retrieves an email
func (s *StorageService) RetrieveEmail(ctx context.Context, storageID string) ([]byte, error) {
	data, _, err := s.backend.Retrieve(ctx, storageID)
	return data, err
}

// StoreAttachment stores an email attachment
func (s *StorageService) StoreAttachment(ctx context.Context, emailID uuid.UUID, filename string, contentType string, data []byte) (string, error) {
	// Generate checksum
	hash := sha256.Sum256(data)
	checksum := hex.EncodeToString(hash[:])

	metadata := map[string]string{
		"type":         "attachment",
		"email_id":     emailID.String(),
		"filename":     filename,
		"content_type": contentType,
		"checksum":     checksum,
		"size_bytes":   fmt.Sprintf("%d", len(data)),
		"stored_at":    time.Now().Format(time.RFC3339),
	}

	return s.backend.Store(ctx, data, metadata)
}

// RetrieveAttachment retrieves an email attachment
func (s *StorageService) RetrieveAttachment(ctx context.Context, storageID string) ([]byte, map[string]string, error) {
	return s.backend.Retrieve(ctx, storageID)
}

// DeleteEmail deletes an email from storage
func (s *StorageService) DeleteEmail(ctx context.Context, storageID string) error {
	return s.backend.Delete(ctx, storageID)
}

// DeleteAttachment deletes an attachment from storage
func (s *StorageService) DeleteAttachment(ctx context.Context, storageID string) error {
	return s.backend.Delete(ctx, storageID)
}

// GetStorageUsage gets storage usage statistics
func (s *StorageService) GetStorageUsage(ctx context.Context) (int64, error) {
	// TODO: Implement actual storage usage calculation
	// For now, return 0
	return 0, nil
}

// HealthCheck checks storage backend health
func (s *StorageService) HealthCheck(ctx context.Context) error {
	return s.backend.HealthCheck(ctx)
}

// Close closes the storage service
func (s *StorageService) Close() error {
	return s.backend.Close()
}

// LocalStorageBackend implements local file storage
type LocalStorageBackend struct {
	basePath string
	logger   *zap.Logger
}

// NewLocalStorageBackend creates a new local storage backend
func NewLocalStorageBackend(basePath string, logger *zap.Logger) (*LocalStorageBackend, error) {
	// Create base directory
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	return &LocalStorageBackend{
		basePath: basePath,
		logger:   logger,
	}, nil
}

func (b *LocalStorageBackend) Store(ctx context.Context, data []byte, metadata map[string]string) (string, error) {
	// Generate storage ID
	storageID := uuid.New().String()
	filePath := filepath.Join(b.basePath, storageID)

	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Write data to file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	// Write metadata to separate file
	// TODO: Write metadata as JSON

	b.logger.Debug("Stored file", zap.String("path", filePath), zap.Int("size", len(data)))
	return storageID, nil
}

func (b *LocalStorageBackend) Retrieve(ctx context.Context, id string) ([]byte, map[string]string, error) {
	filePath := filepath.Join(b.basePath, id)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Read metadata
	metadata := make(map[string]string)
	metaPath := filePath + ".meta"
	if _, err := os.Stat(metaPath); err == nil {
		// TODO: Read and parse metadata
	}

	return data, metadata, nil
}

func (b *LocalStorageBackend) Delete(ctx context.Context, id string) error {
	filePath := filepath.Join(b.basePath, id)
	if err := os.Remove(filePath); err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	// Delete metadata file
	metaPath := filePath + ".meta"
	if _, err := os.Stat(metaPath); err == nil {
		os.Remove(metaPath)
	}

	return nil
}

func (b *LocalStorageBackend) Exists(ctx context.Context, id string) (bool, error) {
	filePath := filepath.Join(b.basePath, id)
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (b *LocalStorageBackend) GetSize(ctx context.Context, id string) (int64, error) {
	filePath := filepath.Join(b.basePath, id)
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

func (b *LocalStorageBackend) HealthCheck(ctx context.Context) error {
	// Check if base directory exists and is writable
	testFile := filepath.Join(b.basePath, ".healthcheck")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		return fmt.Errorf("storage directory not writable: %w", err)
	}
	os.Remove(testFile)
	return nil
}

func (b *LocalStorageBackend) Close() error {
	return nil
}

// S3StorageBackend implements S3 storage
type S3StorageBackend struct {
	logger *zap.Logger
}

func NewS3StorageBackend(cfg config.StorageConfig, logger *zap.Logger) (*S3StorageBackend, error) {
	// TODO: Implement S3 storage
	return &S3StorageBackend{logger: logger}, nil
}

func (b *S3StorageBackend) Store(ctx context.Context, data []byte, metadata map[string]string) (string, error) {
	// TODO: Implement S3 storage
	return "", fmt.Errorf("S3 storage not implemented")
}

func (b *S3StorageBackend) Retrieve(ctx context.Context, id string) ([]byte, map[string]string, error) {
	// TODO: Implement S3 retrieval
	return nil, nil, fmt.Errorf("S3 retrieval not implemented")
}

func (b *S3StorageBackend) Delete(ctx context.Context, id string) error {
	// TODO: Implement S3 deletion
	return fmt.Errorf("S3 deletion not implemented")
}

func (b *S3StorageBackend) Exists(ctx context.Context, id string) (bool, error) {
	// TODO: Implement S3 exists check
	return false, fmt.Errorf("S3 exists check not implemented")
}

func (b *S3StorageBackend) GetSize(ctx context.Context, id string) (int64, error) {
	// TODO: Implement S3 size check
	return 0, fmt.Errorf("S3 size check not implemented")
}

func (b *S3StorageBackend) HealthCheck(ctx context.Context) error {
	// TODO: Implement S3 health check
	return fmt.Errorf("S3 health check not implemented")
}

func (b *S3StorageBackend) Close() error {
	return nil
}

// IPFSStorageBackend implements IPFS storage
type IPFSStorageBackend struct {
	logger *zap.Logger
}

func NewIPFSStorageBackend(cfg config.StorageConfig, logger *zap.Logger) (*IPFSStorageBackend, error) {
	// TODO: Implement IPFS storage
	return &IPFSStorageBackend{logger: logger}, nil
}

func (b *IPFSStorageBackend) Store(ctx context.Context, data []byte, metadata map[string]string) (string, error) {
	// TODO: Implement IPFS storage
	return "", fmt.Errorf("IPFS storage not implemented")
}

func (b *IPFSStorageBackend) Retrieve(ctx context.Context, id string) ([]byte, map[string]string, error) {
	// TODO: Implement IPFS retrieval
	return nil, nil, fmt.Errorf("IPFS retrieval not implemented")
}

func (b *IPFSStorageBackend) Delete(ctx context.Context, id string) error {
	// TODO: Implement IPFS deletion (IPFS is immutable, so this would involve unpinning)
	return fmt.Errorf("IPFS deletion not implemented")
}

func (b *IPFSStorageBackend) Exists(ctx context.Context, id string) (bool, error) {
	// TODO: Implement IPFS exists check
	return false, fmt.Errorf("IPFS exists check not implemented")
}

func (b *IPFSStorageBackend) GetSize(ctx context.Context, id string) (int64, error) {
	// TODO: Implement IPFS size check
	return 0, fmt.Errorf("IPFS size check not implemented")
}

func (b *IPFSStorageBackend) HealthCheck(ctx context.Context) error {
	// TODO: Implement IPFS health check
	return fmt.Errorf("IPFS health check not implemented")
}

func (b *IPFSStorageBackend) Close() error {
	return nil
}
