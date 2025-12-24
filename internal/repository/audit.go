// internal/repository/audit_repository.go
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

// AuditRepository handles audit log database operations
type AuditRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewAuditRepository creates a new audit repository
func NewAuditRepository(db *sqlx.DB, logger *zap.Logger) *AuditRepository {
	return &AuditRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new audit log entry
func (r *AuditRepository) Create(ctx context.Context, audit *models.AuditLog) error {
	query := `
		INSERT INTO audit_logs (
			id, user_id, action, resource_type, resource_id,
			details, ip_address, user_agent, created_at
		) VALUES (
			:id, :user_id, :action, :resource_type, :resource_id,
			:details, :ip_address, :user_agent, :created_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, audit)
	return err
}

// List lists audit logs with filtering
func (r *AuditRepository) List(ctx context.Context, userID *uuid.UUID, action, resourceType string, fromDate, toDate *time.Time, limit, offset int) ([]models.AuditLog, int64, error) {
	var logs []models.AuditLog
	var total int64

	// Build query
	query := `SELECT * FROM audit_logs WHERE 1=1`
	countQuery := `SELECT COUNT(*) FROM audit_logs WHERE 1=1`

	args := []interface{}{}
	argCount := 1

	if userID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argCount)
		countQuery += fmt.Sprintf(" AND user_id = $%d", argCount)
		args = append(args, userID)
		argCount++
	}

	if action != "" {
		query += fmt.Sprintf(" AND action = $%d", argCount)
		countQuery += fmt.Sprintf(" AND action = $%d", argCount)
		args = append(args, action)
		argCount++
	}

	if resourceType != "" {
		query += fmt.Sprintf(" AND resource_type = $%d", argCount)
		countQuery += fmt.Sprintf(" AND resource_type = $%d", argCount)
		args = append(args, resourceType)
		argCount++
	}

	if fromDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argCount)
		countQuery += fmt.Sprintf(" AND created_at >= $%d", argCount)
		args = append(args, fromDate)
		argCount++
	}

	if toDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argCount)
		countQuery += fmt.Sprintf(" AND created_at <= $%d", argCount)
		args = append(args, toDate)
		argCount++
	}

	query += " ORDER BY created_at DESC"

	// Add limit/offset
	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
		argCount++

		if offset > 0 {
			query += fmt.Sprintf(" OFFSET $%d", argCount)
			args = append(args, offset)
		}
	}

	// Get total count
	err := r.db.GetContext(ctx, &total, countQuery, args[:len(args)-2]...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count audit logs: %w", err)
	}

	// Get logs
	err = r.db.SelectContext(ctx, &logs, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get audit logs: %w", err)
	}

	return logs, total, nil
}

// GetByID gets an audit log by ID
func (r *AuditRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.AuditLog, error) {
	var audit models.AuditLog
	query := `SELECT * FROM audit_logs WHERE id = $1`
	err := r.db.GetContext(ctx, &audit, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get audit log: %w", err)
	}
	return &audit, nil
}

// DeleteOld deletes old audit logs
func (r *AuditRepository) DeleteOld(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM audit_logs WHERE created_at < $1`
	result, err := r.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old audit logs: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows, nil
}
