package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"gorm.io/gorm"
)

// StatsRepository handles statistics database operations
type StatsRepository struct {
	db *gorm.DB
}

// NewStatsRepository creates a new stats repository
func NewStatsRepository(db *gorm.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

// GetUserStats gets user statistics
func (r *StatsRepository) GetUserStats(ctx context.Context) (*UserStats, error) {
	stats := &UserStats{}

	// Total users
	if err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Count(&stats.TotalUsers).Error; err != nil {
		return nil, err
	}

	// Active users (logged in last 30 days)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	if err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("last_login >= ?", thirtyDaysAgo).
		Count(&stats.ActiveUsers).Error; err != nil {
		return nil, err
	}

	// New users today
	today := time.Now().Truncate(24 * time.Hour)
	if err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("created_at >= ?", today).
		Count(&stats.NewUsersToday).Error; err != nil {
		return nil, err
	}

	// New users in last 7 days
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	if err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("created_at >= ?", sevenDaysAgo).
		Count(&stats.NewUsersLast7Days).Error; err != nil {
		return nil, err
	}

	// Users by timezone (example)
	var timezoneResults []struct {
		Timezone string
		Count    int64
	}

	if err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Select("timezone, COUNT(*) as count").
		Group("timezone").
		Scan(&timezoneResults).Error; err != nil {
		// Continue without timezone data
	} else {
		stats.UsersByTimezone = make(map[string]int64)
		for _, result := range timezoneResults {
			stats.UsersByTimezone[result.Timezone] = result.Count
		}
	}

	return stats, nil
}

// GetEmailStats gets email statistics
func (r *StatsRepository) GetEmailStats(ctx context.Context, startDate time.Time) (*EmailStats, error) {
	stats := &EmailStats{}

	// Total emails
	if err := r.db.WithContext(ctx).
		Model(&models.Email{}).
		Count(&stats.TotalEmails).Error; err != nil {
		return nil, err
	}

	// Sent today
	today := time.Now().Truncate(24 * time.Hour)
	if err := r.db.WithContext(ctx).
		Model(&models.Email{}).
		Where("created_at >= ? AND direction = ?", today, "outgoing").
		Count(&stats.SentToday).Error; err != nil {
		return nil, err
	}

	// Received today
	if err := r.db.WithContext(ctx).
		Model(&models.Email{}).
		Where("created_at >= ? AND direction = ?", today, "incoming").
		Count(&stats.ReceivedToday).Error; err != nil {
		return nil, err
	}

	// Emails in last 7 days
	if err := r.db.WithContext(ctx).
		Model(&models.Email{}).
		Where("created_at >= ?", startDate).
		Count(&stats.EmailsLast7Days).Error; err != nil {
		return nil, err
	}

	// Emails by network
	var networkResults []struct {
		Network models.NetworkType
		Count   int64
	}

	if err := r.db.WithContext(ctx).
		Model(&models.Email{}).
		Select("network, COUNT(*) as count").
		Group("network").
		Scan(&networkResults).Error; err != nil {
		// Continue without network data
	} else {
		stats.EmailsByNetwork = make(map[string]int64)
		for _, result := range networkResults {
			stats.EmailsByNetwork[string(result.Network)] = result.Count
		}
	}

	// Emails by hour (last 24 hours)
	var hourResults []struct {
		Hour  int
		Count int64
	}

	last24Hours := time.Now().Add(-24 * time.Hour)
	if err := r.db.WithContext(ctx).
		Model(&models.Email{}).
		Select("EXTRACT(HOUR FROM created_at) as hour, COUNT(*) as count").
		Where("created_at >= ?", last24Hours).
		Group("hour").
		Scan(&hourResults).Error; err != nil {
		// Continue without hour data
	} else {
		stats.EmailsByHour = make(map[int]int64)
		for _, result := range hourResults {
			stats.EmailsByHour[result.Hour] = result.Count
		}
	}

	return stats, nil
}

// GetNetworkStats gets network statistics
func (r *NetworkIdentityRepository) GetNetworkStats(ctx context.Context) (*NetworkStats, error) {
	stats := &NetworkStats{}

	// Total identities
	if err := r.db.WithContext(ctx).
		Model(&models.NetworkIdentity{}).
		Count(&stats.TotalIdentities).Error; err != nil {
		return nil, err
	}

	// Identities by network
	var networkResults []struct {
		Network models.NetworkType
		Count   int64
	}

	if err := r.db.WithContext(ctx).
		Model(&models.NetworkIdentity{}).
		Select("network, COUNT(*) as count").
		Group("network").
		Scan(&networkResults).Error; err != nil {
		return nil, err
	}

	stats.IdentitiesByNetwork = make(map[string]int64)
	for _, result := range networkResults {
		stats.IdentitiesByNetwork[string(result.Network)] = result.Count
	}

	// Network status (check if any active identities per network)
	stats.NetworkStatus = make(map[string]bool)
	for network := range stats.IdentitiesByNetwork {
		var activeCount int64
		r.db.WithContext(ctx).
			Model(&models.NetworkIdentity{}).
			Where("network = ? AND is_active = ?", network, true).
			Count(&activeCount)
		stats.NetworkStatus[network] = activeCount > 0
	}

	return stats, nil
}

// GetStorageStats gets storage statistics
func (r *StatsRepository) GetStorageStats(ctx context.Context) (*StorageStats, error) {
	stats := &StorageStats{}

	// Total storage (sum of all quotas)
	if err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Select("COALESCE(SUM(storage_quota_total), 0)").
		Scan(&stats.TotalStorage).Error; err != nil {
		return nil, err
	}

	// Used storage (sum of all used)
	if err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Select("COALESCE(SUM(storage_quota_used), 0)").
		Scan(&stats.UsedStorage).Error; err != nil {
		return nil, err
	}

	stats.AvailableStorage = stats.TotalStorage - stats.UsedStorage

	// Storage by user
	var userResults []struct {
		UserID uuid.UUID
		Used   int64
		Total  int64
	}

	if err := r.db.WithContext(ctx).
		Model(&models.User{}).
		Select("id as user_id, storage_quota_used as used, storage_quota_total as total").
		Scan(&userResults).Error; err != nil {
		return nil, err
	}

	stats.StorageByUser = make(map[uuid.UUID]int64)
	for _, result := range userResults {
		stats.StorageByUser[result.UserID] = result.Used
	}

	return stats, nil
}

// GetRecentActivity gets recent activity logs
func (r *StatsRepository) GetRecentActivity(ctx context.Context, limit int) ([]ActivityLog, error) {
	// This would typically query a separate activity log table
	// For now, we'll create sample data based on recent emails
	var recentEmails []models.Email
	if err := r.db.WithContext(ctx).
		Order("created_at DESC").
		Limit(limit).
		Find(&recentEmails).Error; err != nil {
		return nil, err
	}

	var activities []ActivityLog
	for _, email := range recentEmails {
		action := "received"
		if email.Direction == "outgoing" {
			action = "sent"
		}

		activities = append(activities, ActivityLog{
			Timestamp: email.CreatedAt,
			UserID:    email.UserID,
			Action:    action + "_email",
			Details:   email.Subject,
		})
	}

	return activities, nil
}

// UserStats contains user statistics
type UserStats struct {
	TotalUsers        int64            `json:"total_users"`
	ActiveUsers       int64            `json:"active_users"`
	NewUsersToday     int64            `json:"new_users_today"`
	NewUsersLast7Days int64            `json:"new_users_last_7_days"`
	UsersByTimezone   map[string]int64 `json:"users_by_timezone"`
}

// EmailStats contains email statistics
type EmailStats struct {
	TotalEmails     int64            `json:"total_emails"`
	SentToday       int64            `json:"sent_today"`
	ReceivedToday   int64            `json:"received_today"`
	EmailsLast7Days int64            `json:"emails_last_7_days"`
	EmailsByNetwork map[string]int64 `json:"emails_by_network"`
	EmailsByHour    map[int]int64    `json:"emails_by_hour"`
}

// NetworkStats contains network statistics
type NetworkStats struct {
	TotalIdentities     int64            `json:"total_identities"`
	IdentitiesByNetwork map[string]int64 `json:"identities_by_network"`
	NetworkStatus       map[string]bool  `json:"network_status"`
}

// StorageStats contains storage statistics
type StorageStats struct {
	TotalStorage     int64               `json:"total_storage"`
	UsedStorage      int64               `json:"used_storage"`
	AvailableStorage int64               `json:"available_storage"`
	StorageByUser    map[uuid.UUID]int64 `json:"storage_by_user"`
}

// ActivityLog contains activity log entry
type ActivityLog struct {
	Timestamp time.Time `json:"timestamp"`
	UserID    uuid.UUID `json:"user_id"`
	Action    string    `json:"action"`
	Details   string    `json:"details"`
}
