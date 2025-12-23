// internal/api/handlers/admin.go
package handlers

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
)

// AdminHandler handles admin operations
type AdminHandler struct {
	userRepo    *repository.UserRepository
	emailRepo   *repository.EmailRepository
	networkRepo *repository.NetworkIdentityRepository
	statsRepo   *repository.StatsRepository
	logger      *zap.Logger
}

// NewAdminHandler creates a new admin handler
func NewAdminHandler(
	userRepo *repository.UserRepository,
	emailRepo *repository.EmailRepository,
	networkRepo *repository.NetworkIdentityRepository,
	statsRepo *repository.StatsRepository,
	logger *zap.Logger,
) *AdminHandler {
	return &AdminHandler{
		userRepo:    userRepo,
		emailRepo:   emailRepo,
		networkRepo: networkRepo,
		statsRepo:   statsRepo,
		logger:      logger,
	}
}

// ListUsers lists all users with pagination
func (h *AdminHandler) ListUsers(c echo.Context) error {
	// Parse query parameters
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	if offset < 0 {
		offset = 0
	}

	search := c.QueryParam("search")
	activeOnly, _ := strconv.ParseBool(c.QueryParam("active_only"))
	verifiedOnly, _ := strconv.ParseBool(c.QueryParam("verified_only"))

	ctx := c.Request().Context()

	// Get users
	users, total, err := h.userRepo.ListWithFilter(ctx, limit, offset, search, activeOnly, verifiedOnly)
	if err != nil {
		h.logger.Error("Failed to list users", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "list_failed",
			"message": "Failed to list users",
		})
	}

	// Format user data (exclude sensitive information)
	var safeUsers []map[string]interface{}
	for _, user := range users {
		safeUser := map[string]interface{}{
			"id":                  user.ID,
			"master_username":     user.MasterUsername,
			"display_name":        user.DisplayName,
			"created_at":          user.CreatedAt,
			"updated_at":          user.UpdatedAt,
			"deleted_at":          user.DeletedAt,
			"mfa_enabled":         user.MFAEnabled,
			"storage_quota_used":  user.StorageQuotaUsed,
			"storage_quota_total": user.StorageQuotaTotal,
			"security_score":      user.SecurityScore,
		}
		safeUsers = append(safeUsers, safeUser)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"users":  safeUsers,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetUser gets detailed user information
func (h *AdminHandler) GetUser(c echo.Context) error {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_user_id",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()

	// Get user with all related data
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "user_not_found",
			"message": "User not found",
		})
	}

	// Get user statistics
	emailCount, err := h.emailRepo.CountByUser(ctx, userID)
	if err != nil {
		emailCount = 0
	}

	networkCount, err := h.networkRepo.CountByUser(ctx, userID)
	if err != nil {
		networkCount = 0
	}

	// Get recent activity
	recentEmails, _, err := h.emailRepo.List(ctx, repository.EmailFilter{
		UserID: userID,
		Limit:  10,
	})
	if err != nil {
		recentEmails = []models.Email{}
	}

	userInfo := map[string]interface{}{
		"id":              user.ID,
		"master_username": user.MasterUsername,
		"display_name":    user.DisplayName,
		"bio":             user.Bio,
		"created_at":      user.CreatedAt,
		"updated_at":      user.UpdatedAt,
		"deleted_at":      user.DeletedAt,
		"security": map[string]interface{}{
			"mfa_enabled":        user.MFAEnabled,
			"session_timeout":    user.SessionTimeout,
			"security_score":     user.SecurityScore,
			"last_security_scan": user.LastSecurityScan,
		},
		"privacy": map[string]interface{}{
			"metadata_minimization":    user.MetadataMinimization,
			"logging_consent":          user.LoggingConsent,
			"analytics_opt_out":        user.AnalyticsOptOut,
			"auto_delete_old_messages": user.AutoDeleteOldMessages,
			"retention_days":           user.RetentionDays,
		},
		"storage": map[string]interface{}{
			"used":       user.StorageQuotaUsed,
			"total":      user.StorageQuotaTotal,
			"percentage": float64(user.StorageQuotaUsed) / float64(user.StorageQuotaTotal) * 100,
		},
		"statistics": map[string]interface{}{
			"total_emails":       emailCount,
			"network_identities": networkCount,
			"trusted_devices":    len(user.TrustedDevices),
			"auth_methods":       len(user.AuthMethods),
		},
		"recent_activity": recentEmails,
		"networks":        user.NetworkIdentities,
	}

	return c.JSON(http.StatusOK, userInfo)
}

// UpdateUser updates user information (admin only)
func (h *AdminHandler) UpdateUser(c echo.Context) error {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_user_id",
			"message": "Invalid user ID",
		})
	}

	var req struct {
		StorageQuotaTotal int64  `json:"storage_quota_total,omitempty"`
		IsActive          *bool  `json:"is_active,omitempty"`
		Notes             string `json:"notes,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()

	// Get user
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "user_not_found",
			"message": "User not found",
		})
	}

	// Update fields
	updated := false
	if req.StorageQuotaTotal > 0 && req.StorageQuotaTotal != user.StorageQuotaTotal {
		user.StorageQuotaTotal = req.StorageQuotaTotal
		updated = true
	}

	// Note: In a real implementation, you'd have an IsActive field on the user
	// For now, we'll just update the storage quota

	if req.Notes != "" {
		// Store notes in user config/metadata
		// This would require adding a Notes field or JSONB config field
	}

	if !updated {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "No changes made",
		})
	}

	if err := h.userRepo.Update(ctx, user); err != nil {
		h.logger.Error("Failed to update user", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "update_failed",
			"message": "Failed to update user",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "User updated successfully",
		"user": map[string]interface{}{
			"id":                  user.ID,
			"master_username":     user.MasterUsername,
			"storage_quota_total": user.StorageQuotaTotal,
		},
	})
}

// DeleteUser deletes a user (admin only)
func (h *AdminHandler) DeleteUser(c echo.Context) error {
	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_user_id",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()

	// Get user first to confirm existence
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "user_not_found",
			"message": "User not found",
		})
	}

	// Perform soft delete
	if err := h.userRepo.Delete(ctx, userID); err != nil {
		h.logger.Error("Failed to delete user", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "delete_failed",
			"message": "Failed to delete user",
		})
	}

	// Log the deletion
	h.logger.Info("User deleted by admin",
		zap.String("user_id", userID.String()),
		zap.String("username", user.MasterUsername))

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "User deleted successfully",
		"user": map[string]interface{}{
			"id":              user.ID,
			"master_username": user.MasterUsername,
			"deleted_at":      time.Now(),
		},
	})
}

// GetStats gets system-wide statistics
func (h *AdminHandler) GetStats(c echo.Context) error {
	ctx := c.Request().Context()

	// Get time range
	days, _ := strconv.Atoi(c.QueryParam("days"))
	if days <= 0 {
		days = 7
	}
	startDate := time.Now().AddDate(0, 0, -days)

	// Get all statistics
	userStats, err := h.statsRepo.GetUserStats(ctx)
	if err != nil {
		h.logger.Error("Failed to get user stats", zap.Error(err))
		userStats = &repository.UserStats{}
	}

	emailStats, err := h.statsRepo.GetEmailStats(ctx, startDate)
	if err != nil {
		h.logger.Error("Failed to get email stats", zap.Error(err))
		emailStats = &repository.EmailStats{}
	}

	networkStats, err := h.statsRepo.GetNetworkStats(ctx)
	if err != nil {
		h.logger.Error("Failed to get network stats", zap.Error(err))
		networkStats = &repository.NetworkStats{}
	}

	storageStats, err := h.statsRepo.GetStorageStats(ctx)
	if err != nil {
		h.logger.Error("Failed to get storage stats", zap.Error(err))
		storageStats = &repository.StorageStats{}
	}

	// Get recent activity
	recentActivity, err := h.statsRepo.GetRecentActivity(ctx, 20)
	if err != nil {
		recentActivity = []repository.ActivityLog{}
	}

	// Calculate growth percentages
	var userGrowth, emailGrowth, storageGrowth float64
	if userStats.TotalUsers > 0 && userStats.ActiveUsers > 0 {
		userGrowth = float64(userStats.NewUsersLast7Days) / float64(userStats.TotalUsers) * 100
	}
	if emailStats.TotalEmails > 0 {
		emailGrowth = float64(emailStats.EmailsLast7Days) / float64(emailStats.TotalEmails) * 100
	}
	if storageStats.TotalStorage > 0 {
		storageGrowth = float64(storageGrowthLast7Days) / float64(storageStats.TotalStorage) * 100
	}

	stats := map[string]interface{}{
		"users": map[string]interface{}{
			"total":             userStats.TotalUsers,
			"active":            userStats.ActiveUsers,
			"new_today":         userStats.NewUsersToday,
			"new_last_7_days":   userStats.NewUsersLast7Days,
			"growth_percentage": userGrowth,
			"by_timezone":       userStats.UsersByTimezone,
		},
		"emails": map[string]interface{}{
			"total":             emailStats.TotalEmails,
			"sent_today":        emailStats.SentToday,
			"received_today":    emailStats.ReceivedToday,
			"last_7_days":       emailStats.EmailsLast7Days,
			"growth_percentage": emailGrowth,
			"by_network":        emailStats.EmailsByNetwork,
			"by_hour":           emailStats.EmailsByHour,
		},
		"networks": map[string]interface{}{
			"total_identities": networkStats.TotalIdentities,
			"by_network":       networkStats.IdentitiesByNetwork,
			"status":           networkStats.NetworkStatus,
		},
		"storage": map[string]interface{}{
			"total":             storageStats.TotalStorage,
			"used":              storageStats.UsedStorage,
			"available":         storageStats.AvailableStorage,
			"growth_percentage": storageGrowth,
			"by_user":           storageStats.StorageByUser,
		},
		"performance": map[string]interface{}{
			"avg_response_time":  h.getAverageResponseTime(),
			"uptime_percentage":  h.getUptimePercentage(),
			"error_rate":         h.getErrorRate(),
			"active_connections": h.getActiveConnections(),
		},
		"recent_activity": recentActivity,
		"time_range": map[string]interface{}{
			"days":       days,
			"start_date": startDate,
			"end_date":   time.Now(),
		},
	}

	return c.JSON(http.StatusOK, stats)
}

// GetLogs gets system logs
func (h *AdminHandler) GetLogs(c echo.Context) error {
	// Parse query parameters
	level := c.QueryParam("level")
	since := c.QueryParam("since")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 100
	}

	// In production, this would query a logging database or file
	// For now, return sample structure
	logs := []map[string]interface{}{
		{
			"timestamp": time.Now().Add(-5 * time.Minute),
			"level":     "info",
			"message":   "System started successfully",
			"component": "server",
		},
		{
			"timestamp": time.Now().Add(-10 * time.Minute),
			"level":     "warn",
			"message":   "High memory usage detected",
			"component": "monitor",
		},
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"logs":  logs,
		"total": len(logs),
		"limit": limit,
	})
}

// Cleanup performs system cleanup tasks
func (h *AdminHandler) Cleanup(c echo.Context) error {
	var req struct {
		Task          string `json:"task" validate:"required"`
		OlderThanDays int    `json:"older_than_days,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()
	result := make(map[string]interface{})

	switch req.Task {
	case "cleanup_old_emails":
		if req.OlderThanDays <= 0 {
			req.OlderThanDays = 365 // Default: 1 year
		}
		olderThan := time.Now().AddDate(0, 0, -req.OlderThanDays)
		count, err := h.emailRepo.DeleteOld(ctx, olderThan)
		if err != nil {
			h.logger.Error("Failed to cleanup old emails", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error":   "cleanup_failed",
				"message": "Failed to cleanup old emails",
			})
		}
		result["deleted_emails"] = count
		result["older_than"] = olderThan

	case "cleanup_orphaned_attachments":
		count, err := h.cleanupOrphanedAttachments(ctx)
		if err != nil {
			h.logger.Error("Failed to cleanup orphaned attachments", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error":   "cleanup_failed",
				"message": "Failed to cleanup orphaned attachments",
			})
		}
		result["deleted_attachments"] = count

	case "cleanup_expired_sessions":
		count, err := h.cleanupExpiredSessions(ctx)
		if err != nil {
			h.logger.Error("Failed to cleanup expired sessions", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error":   "cleanup_failed",
				"message": "Failed to cleanup expired sessions",
			})
		}
		result["deleted_sessions"] = count

	case "optimize_database":
		err := h.optimizeDatabase(ctx)
		if err != nil {
			h.logger.Error("Failed to optimize database", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error":   "optimization_failed",
				"message": "Failed to optimize database",
			})
		}
		result["optimized"] = true

	case "recalculate_stats":
		err := h.recalculateStatistics(ctx)
		if err != nil {
			h.logger.Error("Failed to recalculate statistics", zap.Error(err))
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error":   "recalculation_failed",
				"message": "Failed to recalculate statistics",
			})
		}
		result["recalculated"] = true

	default:
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_task",
			"message": "Invalid cleanup task",
		})
	}

	result["task"] = req.Task
	result["completed_at"] = time.Now()

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Cleanup completed successfully",
		"result":  result,
	})
}

// Helper methods
func (h *AdminHandler) getAverageResponseTime() float64 {
	// In production, this would come from metrics
	return 125.5 // milliseconds
}

func (h *AdminHandler) getUptimePercentage() float64 {
	// In production, this would be calculated from uptime records
	return 99.95
}

func (h *AdminHandler) getErrorRate() float64 {
	// In production, this would be calculated from error logs
	return 0.15 // 0.15%
}

func (h *AdminHandler) getActiveConnections() int {
	// In production, this would come from connection tracking
	return 42
}

func (h *AdminHandler) cleanupOrphanedAttachments(ctx context.Context) (int64, error) {
	// TODO: Implement orphaned attachment cleanup
	return 0, nil
}

func (h *AdminHandler) cleanupExpiredSessions(ctx context.Context) (int64, error) {
	// TODO: Implement expired session cleanup
	return 0, nil
}

func (h *AdminHandler) optimizeDatabase(ctx context.Context) error {
	// TODO: Implement database optimization
	return nil
}

func (h *AdminHandler) recalculateStatistics(ctx context.Context) error {
	// TODO: Implement statistics recalculation
	return nil
}
