// internal/repository/attachment_repository.go
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// AttachmentRepository handles attachment database operations
type AttachmentRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewAttachmentRepository creates a new attachment repository
func NewAttachmentRepository(db *sqlx.DB, logger *zap.Logger) *AttachmentRepository {
	return &AttachmentRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new attachment
func (r *AttachmentRepository) Create(ctx context.Context, attachment *models.Attachment) error {
	query := `
		INSERT INTO attachments (
			id, email_id, user_id, filename, content_type, size,
			storage_path, storage_backend, checksum, is_inline, created_at
		) VALUES (
			:id, :email_id, :user_id, :filename, :content_type, :size,
			:storage_path, :storage_backend, :checksum, :is_inline, :created_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, attachment)
	return err
}

// GetByID gets an attachment by ID
func (r *AttachmentRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Attachment, error) {
	var attachment models.Attachment
	query := `SELECT * FROM attachments WHERE id = $1`
	err := r.db.GetContext(ctx, &attachment, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachment: %w", err)
	}
	return &attachment, nil
}

// GetByEmailID gets attachments by email ID
func (r *AttachmentRepository) GetByEmailID(ctx context.Context, emailID uuid.UUID) ([]models.Attachment, error) {
	var attachments []models.Attachment
	query := `SELECT * FROM attachments WHERE email_id = $1 ORDER BY created_at`
	err := r.db.SelectContext(ctx, &attachments, query, emailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments: %w", err)
	}
	return attachments, nil
}

// GetByUserID gets attachments by user ID
func (r *AttachmentRepository) GetByUserID(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Attachment, int64, error) {
	var attachments []models.Attachment
	var total int64

	// Count query
	countQuery := `SELECT COUNT(*) FROM attachments WHERE user_id = $1`
	err := r.db.GetContext(ctx, &total, countQuery, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count attachments: %w", err)
	}

	// List query
	listQuery := `SELECT * FROM attachments WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	err = r.db.SelectContext(ctx, &attachments, listQuery, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get attachments: %w", err)
	}

	return attachments, total, nil
}

// Update updates an attachment
func (r *AttachmentRepository) Update(ctx context.Context, attachment *models.Attachment) error {
	query := `
		UPDATE attachments SET
			filename = :filename,
			content_type = :content_type,
			size = :size,
			storage_path = :storage_path,
			storage_backend = :storage_backend,
			checksum = :checksum,
			is_inline = :is_inline
		WHERE id = :id
	`

	_, err := r.db.NamedExecContext(ctx, query, attachment)
	return err
}

// Delete deletes an attachment
func (r *AttachmentRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM attachments WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// DeleteByEmailID deletes attachments by email ID
func (r *AttachmentRepository) DeleteByEmailID(ctx context.Context, emailID uuid.UUID) error {
	query := `DELETE FROM attachments WHERE email_id = $1`
	_, err := r.db.ExecContext(ctx, query, emailID)
	return err
}

// DeleteByUserID deletes attachments by user ID
func (r *AttachmentRepository) DeleteByUserID(ctx context.Context, userID uuid.UUID) error {
	query := `DELETE FROM attachments WHERE user_id = $1`
	_, err := r.db.ExecContext(ctx, query, userID)
	return err
}

// CountByUser counts attachments by user
func (r *AttachmentRepository) CountByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM attachments WHERE user_id = $1`
	err := r.db.GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to count attachments: %w", err)
	}
	return count, nil
}

// GetStorageUsage gets storage usage by user
func (r *AttachmentRepository) GetStorageUsage(ctx context.Context, userID uuid.UUID) (int64, error) {
	var totalSize int64
	query := `SELECT COALESCE(SUM(size), 0) FROM attachments WHERE user_id = $1`
	err := r.db.GetContext(ctx, &totalSize, query, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get storage usage: %w", err)
	}
	return totalSize, nil
}

// GetSystemStorageUsage gets total system storage usage
func (r *AttachmentRepository) GetSystemStorageUsage(ctx context.Context) (int64, error) {
	var totalSize int64
	query := `SELECT COALESCE(SUM(size), 0) FROM attachments`
	err := r.db.GetContext(ctx, &totalSize, query)
	if err != nil {
		return 0, fmt.Errorf("failed to get system storage usage: %w", err)
	}
	return totalSize, nil
}

// GetByChecksum gets an attachment by checksum for a user
func (r *AttachmentRepository) GetByChecksum(ctx context.Context, userID uuid.UUID, checksum string) (*models.Attachment, error) {
	var attachment models.Attachment
	query := `SELECT * FROM attachments WHERE user_id = $1 AND checksum = $2`
	err := r.db.GetContext(ctx, &attachment, query, userID, checksum)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachment by checksum: %w", err)
	}
	return &attachment, nil
}

// GetByStoragePath gets an attachment by storage path
func (r *AttachmentRepository) GetByStoragePath(ctx context.Context, storagePath string) (*models.Attachment, error) {
	var attachment models.Attachment
	query := `SELECT * FROM attachments WHERE storage_path = $1`
	err := r.db.GetContext(ctx, &attachment, query, storagePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachment by storage path: %w", err)
	}
	return &attachment, nil
}

// GetByBackend gets attachments by storage backend
func (r *AttachmentRepository) GetByBackend(ctx context.Context, backend string) ([]models.Attachment, error) {
	var attachments []models.Attachment
	query := `SELECT * FROM attachments WHERE storage_backend = $1 ORDER BY created_at`
	err := r.db.SelectContext(ctx, &attachments, query, backend)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachments by backend: %w", err)
	}
	return attachments, nil
}

// GetLargeAttachments gets attachments larger than specified size
func (r *AttachmentRepository) GetLargeAttachments(ctx context.Context, minSize int64) ([]models.Attachment, error) {
	var attachments []models.Attachment
	query := `SELECT * FROM attachments WHERE size > $1 ORDER BY size DESC`
	err := r.db.SelectContext(ctx, &attachments, query, minSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get large attachments: %w", err)
	}
	return attachments, nil
}

// GetOldAttachments gets attachments older than specified date
func (r *AttachmentRepository) GetOldAttachments(ctx context.Context, olderThan time.Time) ([]models.Attachment, error) {
	var attachments []models.Attachment
	query := `SELECT * FROM attachments WHERE created_at < $1 ORDER BY created_at`
	err := r.db.SelectContext(ctx, &attachments, query, olderThan)
	if err != nil {
		return nil, fmt.Errorf("failed to get old attachments: %w", err)
	}
	return attachments, nil
}

// GetOrphanedAttachments gets attachments without associated emails
func (r *AttachmentRepository) GetOrphanedAttachments(ctx context.Context) ([]models.Attachment, error) {
	var attachments []models.Attachment
	query := `
		SELECT a.* FROM attachments a
		LEFT JOIN emails e ON a.email_id = e.id
		WHERE e.id IS NULL OR e.deleted_at IS NOT NULL
		ORDER BY a.created_at
	`
	err := r.db.SelectContext(ctx, &attachments, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get orphaned attachments: %w", err)
	}
	return attachments, nil
}

// DeleteOrphanedAttachments deletes attachments without associated emails
func (r *AttachmentRepository) DeleteOrphanedAttachments(ctx context.Context) (int64, error) {
	query := `
		DELETE FROM attachments a
		WHERE a.email_id IN (
			SELECT e.id FROM emails e WHERE e.deleted_at IS NOT NULL
		) OR a.email_id NOT IN (
			SELECT e.id FROM emails e
		)
	`
	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to delete orphaned attachments: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows, nil
}

// CleanupOldAttachments deletes attachments older than specified date
func (r *AttachmentRepository) CleanupOldAttachments(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM attachments WHERE created_at < $1`
	result, err := r.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old attachments: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows, nil
}

// GetAttachmentStats gets attachment statistics
func (r *AttachmentRepository) GetAttachmentStats(ctx context.Context, userID *uuid.UUID) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Base query
	baseQuery := `FROM attachments`
	args := []interface{}{}
	argCount := 1

	if userID != nil {
		baseQuery += ` WHERE user_id = $1`
		args = append(args, userID)
		argCount++
	}

	// Total count
	var total int64
	countQuery := `SELECT COUNT(*) ` + baseQuery
	err := r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get total count: %w", err)
	}
	stats["total_count"] = total

	// Total size
	var totalSize int64
	sizeQuery := `SELECT COALESCE(SUM(size), 0) ` + baseQuery
	err = r.db.GetContext(ctx, &totalSize, sizeQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get total size: %w", err)
	}
	stats["total_size"] = totalSize

	// Average size
	var avgSize float64
	if total > 0 {
		avgSize = float64(totalSize) / float64(total)
	}
	stats["average_size"] = avgSize

	// By content type
	type ContentTypeStats struct {
		ContentType string `db:"content_type"`
		Count       int64  `db:"count"`
		TotalSize   int64  `db:"total_size"`
	}
	var contentTypeStats []ContentTypeStats
	contentTypeQuery := `
		SELECT 
			COALESCE(content_type, 'unknown') as content_type,
			COUNT(*) as count,
			COALESCE(SUM(size), 0) as total_size
		` + baseQuery + ` 
		GROUP BY content_type 
		ORDER BY total_size DESC
	`
	err = r.db.SelectContext(ctx, &contentTypeStats, contentTypeQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get content type stats: %w", err)
	}
	stats["by_content_type"] = contentTypeStats

	// By storage backend
	type BackendStats struct {
		Backend   string `db:"storage_backend"`
		Count     int64  `db:"count"`
		TotalSize int64  `db:"total_size"`
	}
	var backendStats []BackendStats
	backendQuery := `
		SELECT 
			COALESCE(storage_backend, 'unknown') as storage_backend,
			COUNT(*) as count,
			COALESCE(SUM(size), 0) as total_size
		` + baseQuery + ` 
		GROUP BY storage_backend 
		ORDER BY total_size DESC
	`
	err = r.db.SelectContext(ctx, &backendStats, backendQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get backend stats: %w", err)
	}
	stats["by_backend"] = backendStats

	// Recent activity
	var recentActivity struct {
		Today int64 `db:"today"`
		Week  int64 `db:"week"`
		Month int64 `db:"month"`
	}
	recentQuery := `
		SELECT 
			COUNT(CASE WHEN created_at >= CURRENT_DATE THEN 1 END) as today,
			COUNT(CASE WHEN created_at >= CURRENT_DATE - INTERVAL '7 days' THEN 1 END) as week,
			COUNT(CASE WHEN created_at >= CURRENT_DATE - INTERVAL '30 days' THEN 1 END) as month
		` + baseQuery
	err = r.db.GetContext(ctx, &recentActivity, recentQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent activity: %w", err)
	}
	stats["recent_activity"] = recentActivity

	// Size distribution
	type SizeRangeStats struct {
		SizeRange string `db:"size_range"`
		Count     int64  `db:"count"`
	}
	var sizeRangeStats []SizeRangeStats
	sizeRangeQuery := `
		SELECT 
			CASE
				WHEN size < 1024 THEN '< 1KB'
				WHEN size < 10240 THEN '1-10KB'
				WHEN size < 102400 THEN '10-100KB'
				WHEN size < 1048576 THEN '100KB-1MB'
				WHEN size < 10485760 THEN '1-10MB'
				WHEN size < 104857600 THEN '10-100MB'
				ELSE '> 100MB'
			END as size_range,
			COUNT(*) as count
		` + baseQuery + ` 
		GROUP BY size_range 
		ORDER BY MIN(size)
	`
	err = r.db.SelectContext(ctx, &sizeRangeStats, sizeRangeQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get size range stats: %w", err)
	}
	stats["size_distribution"] = sizeRangeStats

	return stats, nil
}

// BulkCreate creates multiple attachments in a transaction
func (r *AttachmentRepository) BulkCreate(ctx context.Context, attachments []models.Attachment) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO attachments (
			id, email_id, user_id, filename, content_type, size,
			storage_path, storage_backend, checksum, is_inline, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
		)
	`

	for _, attachment := range attachments {
		_, err := tx.ExecContext(ctx, query,
			attachment.ID,
			attachment.EmailID,
			attachment.UserID,
			attachment.Filename,
			attachment.ContentType,
			attachment.Size,
			attachment.StoragePath,
			attachment.StorageBackend,
			attachment.Checksum,
			attachment.IsInline,
			attachment.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("failed to insert attachment: %w", err)
		}
	}

	return tx.Commit()
}

// UpdateStoragePath updates storage path for an attachment
func (r *AttachmentRepository) UpdateStoragePath(ctx context.Context, id uuid.UUID, newPath string) error {
	query := `UPDATE attachments SET storage_path = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, newPath, id)
	return err
}

// UpdateStorageBackend updates storage backend for an attachment
func (r *AttachmentRepository) UpdateStorageBackend(ctx context.Context, id uuid.UUID, newBackend string) error {
	query := `UPDATE attachments SET storage_backend = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, newBackend, id)
	return err
}

// MigrateAttachments migrates attachments from one backend to another
func (r *AttachmentRepository) MigrateAttachments(ctx context.Context, fromBackend, toBackend string) (int64, error) {
	query := `UPDATE attachments SET storage_backend = $1 WHERE storage_backend = $2`
	result, err := r.db.ExecContext(ctx, query, toBackend, fromBackend)
	if err != nil {
		return 0, fmt.Errorf("failed to migrate attachments: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows, nil
}

// FindDuplicates finds duplicate attachments by checksum
func (r *AttachmentRepository) FindDuplicates(ctx context.Context, userID *uuid.UUID) ([]models.Attachment, error) {
	var attachments []models.Attachment

	query := `
		SELECT a1.* FROM attachments a1
		INNER JOIN (
			SELECT checksum, COUNT(*) as count
			FROM attachments
			WHERE checksum IS NOT NULL AND checksum != ''
			GROUP BY checksum
			HAVING COUNT(*) > 1
		) a2 ON a1.checksum = a2.checksum
	`
	args := []interface{}{}
	if userID != nil {
		query += ` WHERE a1.user_id = $1`
		args = append(args, userID)
	}

	query += ` ORDER BY a1.checksum, a1.created_at`

	err := r.db.SelectContext(ctx, &attachments, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to find duplicates: %w", err)
	}
	return attachments, nil
}

// GetUnusedAttachments gets attachments not linked to any active email
func (r *AttachmentRepository) GetUnusedAttachments(ctx context.Context, olderThan time.Time) ([]models.Attachment, error) {
	var attachments []models.Attachment
	query := `
		SELECT a.* FROM attachments a
		LEFT JOIN emails e ON a.email_id = e.id AND e.deleted_at IS NULL
		WHERE e.id IS NULL AND a.created_at < $1
		ORDER BY a.created_at
	`
	err := r.db.SelectContext(ctx, &attachments, query, olderThan)
	if err != nil {
		return nil, fmt.Errorf("failed to get unused attachments: %w", err)
	}
	return attachments, nil
}

// HealthCheck performs a health check on the attachments table
func (r *AttachmentRepository) HealthCheck(ctx context.Context) error {
	query := `SELECT 1 FROM attachments LIMIT 1`
	var result int
	err := r.db.GetContext(ctx, &result, query)
	return err
}
