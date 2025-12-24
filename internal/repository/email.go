// internal/repository/email_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// EmailRepository handles email database operations
type EmailRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewEmailRepository creates a new email repository
func NewEmailRepository(db *sqlx.DB, logger *zap.Logger) *EmailRepository {
	return &EmailRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new email
func (r *EmailRepository) Create(ctx context.Context, email *models.Email) error {
	query := `
		INSERT INTO emails (
			id, user_id, thread_id, "from", "to", cc, bcc, subject,
			body_plain, body_html, body_markdown, network, priority,
			is_read, is_starred, is_archived, is_spam, is_encrypted,
			encryption_keys, attachments, headers, in_reply_to, references,
			folder_id, sent_at, received_at, created_at, updated_at
		) VALUES (
			:id, :user_id, :thread_id, :from, :to, :cc, :bcc, :subject,
			:body_plain, :body_html, :body_markdown, :network, :priority,
			:is_read, :is_starred, :is_archived, :is_spam, :is_encrypted,
			:encryption_keys, :attachments, :headers, :in_reply_to, :references,
			:folder_id, :sent_at, :received_at, :created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, email)
	return err
}

// GetByID gets an email by ID
func (r *EmailRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Email, error) {
	var email models.Email
	query := `SELECT * FROM emails WHERE id = $1 AND deleted_at IS NULL`
	err := r.db.GetContext(ctx, &email, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get email: %w", err)
	}
	return &email, nil
}

// GetByMessageID gets an email by message ID
func (r *EmailRepository) GetByMessageID(ctx context.Context, messageID string) (*models.Email, error) {
	var email models.Email
	query := `SELECT * FROM emails WHERE message_id = $1 AND deleted_at IS NULL`
	err := r.db.GetContext(ctx, &email, query, messageID)
	if err != nil {
		return nil, fmt.Errorf("failed to get email: %w", err)
	}
	return &email, nil
}

// Update updates an email
func (r *EmailRepository) Update(ctx context.Context, email *models.Email) error {
	query := `
		UPDATE emails SET
			"from" = :from,
			"to" = :to,
			cc = :cc,
			bcc = :bcc,
			subject = :subject,
			body_plain = :body_plain,
			body_html = :body_html,
			body_markdown = :body_markdown,
			network = :network,
			priority = :priority,
			is_read = :is_read,
			is_starred = :is_starred,
			is_archived = :is_archived,
			is_spam = :is_spam,
			is_encrypted = :is_encrypted,
			encryption_keys = :encryption_keys,
			attachments = :attachments,
			headers = :headers,
			in_reply_to = :in_reply_to,
			references = :references,
			folder_id = :folder_id,
			sent_at = :sent_at,
			received_at = :received_at,
			updated_at = :updated_at
		WHERE id = :id AND deleted_at IS NULL
	`

	_, err := r.db.NamedExecContext(ctx, query, email)
	return err
}

// Delete soft deletes an email
func (r *EmailRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE emails SET deleted_at = NOW() WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// ListByUser lists emails for a user with filtering
func (r *EmailRepository) ListByUser(ctx context.Context, userID uuid.UUID, folderID *uuid.UUID,
	isRead, isStarred, isArchived, isSpam *bool, network *string, limit, offset int) ([]models.Email, int64, error) {

	var emails []models.Email
	var total int64

	// Build count query
	countQuery := `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND deleted_at IS NULL`
	args := []interface{}{userID}
	argCount := 2

	if folderID != nil {
		countQuery += fmt.Sprintf(" AND folder_id = $%d", argCount)
		args = append(args, folderID)
		argCount++
	}

	if isRead != nil {
		countQuery += fmt.Sprintf(" AND is_read = $%d", argCount)
		args = append(args, *isRead)
		argCount++
	}

	if isStarred != nil {
		countQuery += fmt.Sprintf(" AND is_starred = $%d", argCount)
		args = append(args, *isStarred)
		argCount++
	}

	if isArchived != nil {
		countQuery += fmt.Sprintf(" AND is_archived = $%d", argCount)
		args = append(args, *isArchived)
		argCount++
	}

	if isSpam != nil {
		countQuery += fmt.Sprintf(" AND is_spam = $%d", argCount)
		args = append(args, *isSpam)
		argCount++
	}

	if network != nil {
		countQuery += fmt.Sprintf(" AND network = $%d", argCount)
		args = append(args, *network)
		argCount++
	}

	// Get total count
	err := r.db.GetContext(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count emails: %w", err)
	}

	// Build list query
	listQuery := `SELECT * FROM emails WHERE user_id = $1 AND deleted_at IS NULL`
	listArgs := []interface{}{userID}
	argCount = 2

	if folderID != nil {
		listQuery += fmt.Sprintf(" AND folder_id = $%d", argCount)
		listArgs = append(listArgs, folderID)
		argCount++
	}

	if isRead != nil {
		listQuery += fmt.Sprintf(" AND is_read = $%d", argCount)
		listArgs = append(listArgs, *isRead)
		argCount++
	}

	if isStarred != nil {
		listQuery += fmt.Sprintf(" AND is_starred = $%d", argCount)
		listArgs = append(listArgs, *isStarred)
		argCount++
	}

	if isArchived != nil {
		listQuery += fmt.Sprintf(" AND is_archived = $%d", argCount)
		listArgs = append(listArgs, *isArchived)
		argCount++
	}

	if isSpam != nil {
		listQuery += fmt.Sprintf(" AND is_spam = $%d", argCount)
		listArgs = append(listArgs, *isSpam)
		argCount++
	}

	if network != nil {
		listQuery += fmt.Sprintf(" AND network = $%d", argCount)
		listArgs = append(listArgs, *network)
		argCount++
	}

	listQuery += ` ORDER BY received_at DESC LIMIT $` + fmt.Sprintf("%d", argCount) + ` OFFSET $` + fmt.Sprintf("%d", argCount+1)
	listArgs = append(listArgs, limit, offset)

	err = r.db.SelectContext(ctx, &emails, listQuery, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list emails: %w", err)
	}

	return emails, total, nil
}

// Search searches emails by query
func (r *EmailRepository) Search(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.Email, int64, error) {
	var emails []models.Email
	var total int64

	// Count query
	countQuery := `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND deleted_at IS NULL AND search_vector @@ plainto_tsquery('english', $2)`
	err := r.db.GetContext(ctx, &total, countQuery, userID, query)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count search results: %w", err)
	}

	// Search query
	searchQuery := `SELECT * FROM emails WHERE user_id = $1 AND deleted_at IS NULL AND search_vector @@ plainto_tsquery('english', $2) ORDER BY ts_rank(search_vector, plainto_tsquery('english', $2)) DESC LIMIT $3 OFFSET $4`
	err = r.db.SelectContext(ctx, &emails, searchQuery, userID, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search emails: %w", err)
	}

	return emails, total, nil
}

// GetThread gets emails in a thread
func (r *EmailRepository) GetThread(ctx context.Context, userID, threadID uuid.UUID) ([]models.Email, error) {
	var emails []models.Email
	query := `SELECT * FROM emails WHERE user_id = $1 AND thread_id = $2 AND deleted_at IS NULL ORDER BY received_at`
	err := r.db.SelectContext(ctx, &emails, query, userID, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread: %w", err)
	}
	return emails, nil
}

// MoveToFolder moves email to folder
func (r *EmailRepository) MoveToFolder(ctx context.Context, emailID, folderID uuid.UUID) error {
	query := `UPDATE emails SET folder_id = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, folderID, emailID)
	return err
}

// MarkAsRead marks email as read
func (r *EmailRepository) MarkAsRead(ctx context.Context, emailID uuid.UUID, read bool) error {
	query := `UPDATE emails SET is_read = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, read, emailID)
	return err
}

// MarkAsStarred marks email as starred
func (r *EmailRepository) MarkAsStarred(ctx context.Context, emailID uuid.UUID, starred bool) error {
	query := `UPDATE emails SET is_starred = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, starred, emailID)
	return err
}

// MarkAsSpam marks email as spam
func (r *EmailRepository) MarkAsSpam(ctx context.Context, emailID uuid.UUID, spam bool) error {
	query := `UPDATE emails SET is_spam = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, spam, emailID)
	return err
}

// Archive archives email
func (r *EmailRepository) Archive(ctx context.Context, emailID uuid.UUID, archived bool) error {
	query := `UPDATE emails SET is_archived = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, archived, emailID)
	return err
}

// GetUnreadCount gets unread email count for user
func (r *EmailRepository) GetUnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	query := `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND is_read = false AND deleted_at IS NULL`
	err := r.db.GetContext(ctx, &count, query, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to get unread count: %w", err)
	}
	return count, nil
}

// GetStats gets email statistics for user
func (r *EmailRepository) GetStats(ctx context.Context, userID uuid.UUID) (map[string]int64, error) {
	stats := make(map[string]int64)

	queries := []struct {
		key   string
		query string
	}{
		{"total", `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND deleted_at IS NULL`},
		{"unread", `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND is_read = false AND deleted_at IS NULL`},
		{"starred", `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND is_starred = true AND deleted_at IS NULL`},
		{"spam", `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND is_spam = true AND deleted_at IS NULL`},
		{"archived", `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND is_archived = true AND deleted_at IS NULL`},
	}

	for _, q := range queries {
		var count int64
		err := r.db.GetContext(ctx, &count, q.query, userID)
		if err != nil {
			return nil, fmt.Errorf("failed to get stat %s: %w", q.key, err)
		}
		stats[q.key] = count
	}

	return stats, nil
}

// GetByAddress gets emails by address (sender or recipient)
func (r *EmailRepository) GetByAddress(ctx context.Context, userID uuid.UUID, address string, limit, offset int) ([]models.Email, int64, error) {
	var emails []models.Email
	var total int64

	// Count query
	countQuery := `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND deleted_at IS NULL AND ($2 = ANY("to") OR "from" = $2)`
	err := r.db.GetContext(ctx, &total, countQuery, userID, address)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count emails by address: %w", err)
	}

	// List query
	listQuery := `SELECT * FROM emails WHERE user_id = $1 AND deleted_at IS NULL AND ($2 = ANY("to") OR "from" = $2) ORDER BY received_at DESC LIMIT $3 OFFSET $4`
	err = r.db.SelectContext(ctx, &emails, listQuery, userID, address, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get emails by address: %w", err)
	}

	return emails, total, nil
}
