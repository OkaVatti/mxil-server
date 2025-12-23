package service

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// LocalStorage implements StorageService using local filesystem
type LocalStorage struct {
	basePath    string
	maxFileSize int64
	logger      *zap.Logger
}

// NewLocalStorage creates a new local storage service
func NewLocalStorage(basePath string, maxFileSize int64, logger *zap.Logger) (*LocalStorage, error) {
	// Create base directory if it doesn't exist
	if err := os.MkdirAll(basePath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create storage directory: %w", err)
	}

	// Create subdirectories
	subdirs := []string{"attachments", "tmp", "backup"}
	for _, dir := range subdirs {
		path := filepath.Join(basePath, dir)
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, fmt.Errorf("failed to create subdirectory %s: %w", dir, err)
		}
	}

	return &LocalStorage{
		basePath:    basePath,
		maxFileSize: maxFileSize,
		logger:      logger,
	}, nil
}

// SaveAttachment saves an attachment to local storage
func (s *LocalStorage) SaveAttachment(ctx context.Context, data []byte, filename string) (string, error) {
	// Check file size
	if int64(len(data)) > s.maxFileSize {
		return "", NewServiceError("file_too_large", fmt.Sprintf("File exceeds maximum size of %d bytes", s.maxFileSize))
	}

	// Generate unique filename
	hash := sha256.Sum256(data)
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".bin"
	}
	uniqueFilename := fmt.Sprintf("%x%s", hash[:8], ext)
	filePath := filepath.Join(s.basePath, "attachments", uniqueFilename)

	// Check if file already exists (deduplication)
	if _, err := os.Stat(filePath); err == nil {
		return uniqueFilename, nil
	}

	// Write file
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	s.logger.Debug("Attachment saved", zap.String("filename", filename), zap.String("path", uniqueFilename))
	return uniqueFilename, nil
}

// GetAttachment retrieves an attachment from local storage
func (s *LocalStorage) GetAttachment(ctx context.Context, path string) ([]byte, error) {
	fullPath := filepath.Join(s.basePath, "attachments", path)

	// Security check: ensure path is within base directory
	if !isSafePath(fullPath, s.basePath) {
		return nil, NewServiceError("invalid_path", "Invalid file path")
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	return data, nil
}

// DeleteAttachment deletes an attachment from local storage
func (s *LocalStorage) DeleteAttachment(ctx context.Context, path string) error {
	fullPath := filepath.Join(s.basePath, "attachments", path)

	// Security check
	if !isSafePath(fullPath, s.basePath) {
		return NewServiceError("invalid_path", "Invalid file path")
	}

	if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	s.logger.Debug("Attachment deleted", zap.String("path", path))
	return nil
}

// CleanupTempFiles removes temporary files older than specified duration
func (s *LocalStorage) CleanupTempFiles(ctx context.Context, olderThan time.Duration) error {
	tempDir := filepath.Join(s.basePath, "tmp")
	files, err := os.ReadDir(tempDir)
	if err != nil {
		return fmt.Errorf("failed to read temp directory: %w", err)
	}

	cutoffTime := time.Now().Add(-olderThan)
	deletedCount := 0

	for _, file := range files {
		filePath := filepath.Join(tempDir, file.Name())
		info, err := file.Info()
		if err != nil {
			s.logger.Warn("Failed to get file info", zap.String("path", filePath), zap.Error(err))
			continue
		}

		if info.ModTime().Before(cutoffTime) {
			if err := os.Remove(filePath); err != nil {
				s.logger.Warn("Failed to delete temp file", zap.String("path", filePath), zap.Error(err))
			} else {
				deletedCount++
			}
		}
	}

	s.logger.Info("Temp files cleaned up", zap.Int("deleted", deletedCount))
	return nil
}

// GetStats returns storage statistics
func (s *LocalStorage) GetStats(ctx context.Context) (*StorageStats, error) {
	var totalSize int64

	// Walk through attachments directory
	err := filepath.Walk(filepath.Join(s.basePath, "attachments"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})

	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to calculate storage usage: %w", err)
	}

	// Get disk usage
	var stat syscall.Statfs_t
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	if err := syscall.Statfs(wd, &stat); err != nil {
		return nil, fmt.Errorf("failed to get disk stats: %w", err)
	}

	// Calculate in bytes
	allBytes := stat.Blocks * uint64(stat.Bsize)
	freeBytes := stat.Bfree * uint64(stat.Bsize)
	usedBytes := allBytes - freeBytes

	return &StorageStats{
		Total:     int64(allBytes),
		Used:      int64(usedBytes),
		Available: int64(freeBytes),
		ByType: map[string]int64{
			"attachments": totalSize,
		},
	}, nil
}

// GetBackend returns the storage backend type
func (s *LocalStorage) GetBackend() string {
	return "local"
}

// Helper function to check if path is safe
func isSafePath(path, baseDir string) bool {
	rel, err := filepath.Rel(baseDir, path)
	if err != nil {
		return false
	}

	// Check if the relative path tries to escape the base directory
	if rel == ".." || len(rel) >= 3 && rel[0:3] == "../" {
		return false
	}

	return true
}
