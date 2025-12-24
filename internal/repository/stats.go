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

// StatsRepository handles statistics database operations
type StatsRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewStatsRepository creates a new stats repository
func NewStatsRepository(db *sqlx.DB, logger *zap.Logger) *StatsRepository {
	return &StatsRepository{
		db:     db,
		logger: logger,
	}
}

// RecordStat records a statistic
func (r *StatsRepository) RecordStat(ctx context.Context, stat *models.Statistics) error {
	query := `
		INSERT INTO statistics (
			id, metric_name, metric_value, recorded_at
		) VALUES (
			:id, :metric_name, :metric_value, :recorded_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, stat)
	return err
}

// GetStatsByMetric gets statistics by metric name
func (r *StatsRepository) GetStatsByMetric(ctx context.Context, metricName string, fromDate, toDate *time.Time, limit int) ([]models.Statistics, error) {
	var stats []models.Statistics

	query := `SELECT * FROM statistics WHERE metric_name = $1`
	args := []interface{}{metricName}
	argCount := 2

	if fromDate != nil {
		query += fmt.Sprintf(" AND recorded_at >= $%d", argCount)
		args = append(args, fromDate)
		argCount++
	}

	if toDate != nil {
		query += fmt.Sprintf(" AND recorded_at <= $%d", argCount)
		args = append(args, toDate)
		argCount++
	}

	query += " ORDER BY recorded_at DESC"

	if limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argCount)
		args = append(args, limit)
	}

	err := r.db.SelectContext(ctx, &stats, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats by metric: %w", err)
	}
	return stats, nil
}

// GetLatestStat gets the latest statistic for a metric
func (r *StatsRepository) GetLatestStat(ctx context.Context, metricName string) (*models.Statistics, error) {
	var stat models.Statistics
	query := `SELECT * FROM statistics WHERE metric_name = $1 ORDER BY recorded_at DESC LIMIT 1`
	err := r.db.GetContext(ctx, &stat, query, metricName)
	if err != nil {
		return nil, fmt.Errorf("failed to get latest stat: %w", err)
	}
	return &stat, nil
}

// GetSystemStats gets system-wide statistics
func (r *StatsRepository) GetSystemStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// User statistics
	var userCount int64
	err := r.db.GetContext(ctx, &userCount, `SELECT COUNT(*) FROM users`)
	if err != nil {
		return nil, fmt.Errorf("failed to get user count: %w", err)
	}
	stats["user_count"] = userCount

	// Active user count
	var activeUserCount int64
	err = r.db.GetContext(ctx, &activeUserCount, `SELECT COUNT(*) FROM users WHERE is_active = true`)
	if err != nil {
		return nil, fmt.Errorf("failed to get active user count: %w", err)
	}
	stats["active_user_count"] = activeUserCount

	// Email statistics
	var emailCount int64
	err = r.db.GetContext(ctx, &emailCount, `SELECT COUNT(*) FROM emails WHERE deleted_at IS NULL`)
	if err != nil {
		return nil, fmt.Errorf("failed to get email count: %w", err)
	}
	stats["email_count"] = emailCount

	// Today's emails
	var todayEmailCount int64
	today := time.Now().Truncate(24 * time.Hour)
	err = r.db.GetContext(ctx, &todayEmailCount, `SELECT COUNT(*) FROM emails WHERE created_at >= $1 AND deleted_at IS NULL`, today)
	if err != nil {
		return nil, fmt.Errorf("failed to get today's email count: %w", err)
	}
	stats["today_email_count"] = todayEmailCount

	// Storage usage
	var storageUsed int64
	err = r.db.GetContext(ctx, &storageUsed, `SELECT COALESCE(SUM(storage_quota_used), 0) FROM users`)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage used: %w", err)
	}
	stats["storage_used"] = storageUsed

	// Total storage quota
	var storageTotal int64
	err = r.db.GetContext(ctx, &storageTotal, `SELECT COALESCE(SUM(storage_quota_total), 0) FROM users`)
	if err != nil {
		return nil, fmt.Errorf("failed to get total storage: %w", err)
	}
	stats["storage_total"] = storageTotal

	// Attachment statistics
	var attachmentCount int64
	err = r.db.GetContext(ctx, &attachmentCount, `SELECT COUNT(*) FROM attachments`)
	if err != nil {
		return nil, fmt.Errorf("failed to get attachment count: %w", err)
	}
	stats["attachment_count"] = attachmentCount

	// Total attachment size - THIS WAS THE SPECIFIC ERROR
	var attachmentSize int64
	err = r.db.GetContext(ctx, &attachmentSize, `SELECT COALESCE(SUM(size), 0) FROM attachments`) // FIXED HERE
	if err != nil {
		return nil, fmt.Errorf("failed to get attachment size: %w", err)
	}
	stats["attachment_size"] = attachmentSize

	// Network statistics
	networkStats := make(map[string]int64)
	query := `
		SELECT network, COUNT(*) as count
		FROM emails
		WHERE deleted_at IS NULL
		GROUP BY network
	`
	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get network stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var network string
		var count int64
		if err := rows.Scan(&network, &count); err == nil {
			networkStats[network] = count
		}
	}
	stats["network_stats"] = networkStats

	// Daily activity (last 7 days)
	type DailyActivity struct {
		Date  string `db:"date"`
		Count int64  `db:"count"`
	}
	var dailyActivity []DailyActivity
	query = `
		SELECT DATE(created_at) as date, COUNT(*) as count
		FROM emails
		WHERE created_at >= NOW() - INTERVAL '7 days'
		AND deleted_at IS NULL
		GROUP BY DATE(created_at)
		ORDER BY date
	`
	err = r.db.SelectContext(ctx, &dailyActivity, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get daily activity: %w", err)
	}
	stats["daily_activity"] = dailyActivity

	// User growth (last 30 days)
	var userGrowth int64
	monthAgo := time.Now().AddDate(0, 0, -30)
	err = r.db.GetContext(ctx, &userGrowth, `SELECT COUNT(*) FROM users WHERE created_at >= $1`, monthAgo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user growth: %w", err)
	}
	stats["user_growth_30d"] = userGrowth

	// Email growth (last 30 days)
	var emailGrowth int64
	err = r.db.GetContext(ctx, &emailGrowth, `SELECT COUNT(*) FROM emails WHERE created_at >= $1 AND deleted_at IS NULL`, monthAgo)
	if err != nil {
		return nil, fmt.Errorf("failed to get email growth: %w", err)
	}
	stats["email_growth_30d"] = emailGrowth

	return stats, nil
}

// GetUserStats gets statistics for a specific user
func (r *StatsRepository) GetUserStats(ctx context.Context, userID uuid.UUID) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Email counts
	var totalEmails, unreadEmails, starredEmails, spamEmails int64
	err := r.db.GetContext(ctx, &totalEmails, `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND deleted_at IS NULL`, userID) // Fixed
	if err != nil {
		return nil, fmt.Errorf("failed to get total emails: %w", err)
	}
	stats["total_emails"] = totalEmails

	err = r.db.GetContext(ctx, &unreadEmails, `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND is_read = false AND deleted_at IS NULL`, userID) // Fixed
	if err != nil {
		return nil, fmt.Errorf("failed to get unread emails: %w", err)
	}
	stats["unread_emails"] = unreadEmails

	err = r.db.GetContext(ctx, &starredEmails, `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND is_starred = true AND deleted_at IS NULL`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get starred emails: %w", err)
	}
	stats["starred_emails"] = starredEmails

	err = r.db.GetContext(ctx, &spamEmails, `SELECT COUNT(*) FROM emails WHERE user_id = $1 AND is_spam = true AND deleted_at IS NULL`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get spam emails: %w", err)
	}
	stats["spam_emails"] = spamEmails

	// Storage usage
	var storageUsed, storageTotal int64
	err = r.db.GetContext(ctx, &storageUsed, `SELECT storage_quota_used FROM users WHERE id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage used: %w", err)
	}
	stats["storage_used"] = storageUsed

	err = r.db.GetContext(ctx, &storageTotal, `SELECT storage_quota_total FROM users WHERE id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get storage total: %w", err)
	}
	stats["storage_total"] = storageTotal

	// Network usage
	networkStats := make(map[string]int64)
	query := `
		SELECT network, COUNT(*) as count
		FROM emails
		WHERE user_id = $1 AND deleted_at IS NULL
		GROUP BY network
	`
	rows, err := r.db.QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get network stats: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var network string
		var count int64
		if err := rows.Scan(&network, &count); err == nil {
			networkStats[network] = count
		}
	}
	stats["network_stats"] = networkStats

	// Recent activity
	type RecentActivity struct {
		Action    string    `db:"action"`
		Count     int64     `db:"count"`
		LastEvent time.Time `db:"last_event"`
	}
	var recentActivity []RecentActivity
	query = `
		SELECT 
			action,
			COUNT(*) as count,
			MAX(created_at) as last_event
		FROM audit_logs
		WHERE user_id = $1
		AND created_at >= NOW() - INTERVAL '7 days'
		GROUP BY action
		ORDER BY count DESC
		LIMIT 10
	`
	err = r.db.SelectContext(ctx, &recentActivity, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent activity: %w", err)
	}
	stats["recent_activity"] = recentActivity

	// Folder statistics
	type FolderStat struct {
		FolderName string `db:"folder_name"`
		Count      int64  `db:"count"`
		Unread     int64  `db:"unread"`
	}
	var folderStats []FolderStat
	query = `
		SELECT 
			f.name as folder_name,
			COUNT(e.id) as count,
			SUM(CASE WHEN e.is_read = false THEN 1 ELSE 0 END) as unread
		FROM folders f
		LEFT JOIN emails e ON f.id = e.folder_id AND e.deleted_at IS NULL
		WHERE f.user_id = $1
		GROUP BY f.id, f.name
		ORDER BY f.name
	`
	err = r.db.SelectContext(ctx, &folderStats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder stats: %w", err)
	}
	stats["folder_stats"] = folderStats

	// Label statistics
	type LabelStat struct {
		LabelName string `db:"label_name"`
		Count     int64  `db:"count"`
	}
	var labelStats []LabelStat
	query = `
		SELECT 
			l.name as label_name,
			COUNT(el.email_id) as count
		FROM labels l
		LEFT JOIN email_labels el ON l.id = el.label_id
		LEFT JOIN emails e ON el.email_id = e.id AND e.deleted_at IS NULL
		WHERE l.user_id = $1
		GROUP BY l.id, l.name
		ORDER BY l.name
	`
	err = r.db.SelectContext(ctx, &labelStats, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get label stats: %w", err)
	}
	stats["label_stats"] = labelStats

	// Contact statistics
	var contactCount int64
	err = r.db.GetContext(ctx, &contactCount, `SELECT COUNT(*) FROM contacts WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get contact count: %w", err)
	}
	stats["contact_count"] = contactCount

	var trustedContactCount int64
	err = r.db.GetContext(ctx, &trustedContactCount, `SELECT COUNT(*) FROM contacts WHERE user_id = $1 AND is_trusted = true`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get trusted contact count: %w", err)
	}
	stats["trusted_contact_count"] = trustedContactCount

	// Session statistics
	var sessionCount int64
	err = r.db.GetContext(ctx, &sessionCount, `SELECT COUNT(*) FROM sessions WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session count: %w", err)
	}
	stats["session_count"] = sessionCount

	var activeSessionCount int64
	err = r.db.GetContext(ctx, &activeSessionCount, `SELECT COUNT(*) FROM sessions WHERE user_id = $1 AND expires_at > NOW()`, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get active session count: %w", err)
	}
	stats["active_session_count"] = activeSessionCount

	return stats, nil
}

// CleanupOldStats cleans up old statistics
func (r *StatsRepository) CleanupOldStats(ctx context.Context, olderThan time.Time) (int64, error) {
	query := `DELETE FROM statistics WHERE recorded_at < $1`
	result, err := r.db.ExecContext(ctx, query, olderThan)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old stats: %w", err)
	}

	rows, _ := result.RowsAffected()
	return rows, nil
}

// GetTopSenders gets top senders for a user
func (r *StatsRepository) GetTopSenders(ctx context.Context, userID uuid.UUID, limit int) ([]map[string]interface{}, error) {
	var senders []map[string]interface{}
	query := `
		SELECT 
			"from" as sender,
			COUNT(*) as email_count,
			MAX(received_at) as last_received
		FROM emails
		WHERE user_id = $1
		AND "from" != ''
		AND deleted_at IS NULL
		GROUP BY "from"
		ORDER BY email_count DESC
		LIMIT $2
	`

	rows, err := r.db.QueryxContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top senders: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var sender string
		var count int64
		var lastReceived time.Time
		if err := rows.Scan(&sender, &count, &lastReceived); err == nil {
			senders = append(senders, map[string]interface{}{
				"sender":        sender,
				"email_count":   count,
				"last_received": lastReceived,
			})
		}
	}

	return senders, nil
}

// GetTopRecipients gets top recipients for a user
func (r *StatsRepository) GetTopRecipients(ctx context.Context, userID uuid.UUID, limit int) ([]map[string]interface{}, error) {
	// This is more complex because recipients are in an array
	// We'll need to unnest the array
	query := `
		WITH recipient_counts AS (
			SELECT 
				UNNEST("to") as recipient,
				COUNT(*) as email_count,
				MAX(received_at) as last_sent
			FROM emails
			WHERE user_id = $1
			AND deleted_at IS NULL
			AND array_length("to", 1) > 0
			GROUP BY UNNEST("to")
		)
		SELECT 
			recipient,
			email_count,
			last_sent
		FROM recipient_counts
		ORDER BY email_count DESC
		LIMIT $2
	`

	var recipients []map[string]interface{}
	rows, err := r.db.QueryxContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top recipients: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var recipient string
		var count int64
		var lastSent time.Time
		if err := rows.Scan(&recipient, &count, &lastSent); err == nil {
			recipients = append(recipients, map[string]interface{}{
				"recipient":   recipient,
				"email_count": count,
				"last_sent":   lastSent,
			})
		}
	}

	return recipients, nil
}

// GetEmailVolumeOverTime gets email volume over time
func (r *StatsRepository) GetEmailVolumeOverTime(ctx context.Context, userID *uuid.UUID, days int) ([]map[string]interface{}, error) {
	var results []map[string]interface{}

	query := `
		SELECT 
			DATE(created_at) as date,
			COUNT(*) as email_count,
			SUM(CASE WHEN is_read = false THEN 1 ELSE 0 END) as unread_count
		FROM emails
		WHERE created_at >= NOW() - INTERVAL '` + fmt.Sprintf("%d", days) + ` days'
		AND deleted_at IS NULL
	`
	args := []interface{}{}
	argCount := 1

	if userID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argCount)
		args = append(args, userID)
		argCount++
	}

	query += `
		GROUP BY DATE(created_at)
		ORDER BY date
	`

	rows, err := r.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get email volume: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var date string
		var count, unread int64
		if err := rows.Scan(&date, &count, &unread); err == nil {
			results = append(results, map[string]interface{}{
				"date":         date,
				"email_count":  count,
				"unread_count": unread,
			})
		}
	}

	return results, nil
}
