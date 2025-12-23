// internal/api/handlers/user.go
package handlers

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service"
)

// UserHandler handles user-related requests
type UserHandler struct {
	userRepo    *repository.UserRepository
	sessionRepo *repository.SessionRepository
	authService service.AuthService
	logger      *zap.Logger
}

// NewUserHandler creates a new user handler
func NewUserHandler(
	userRepo *repository.UserRepository,
	sessionRepo *repository.SessionRepository,
	authService service.AuthService,
	logger *zap.Logger,
) *UserHandler {
	return &UserHandler{
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		authService: authService,
		logger:      logger,
	}
}

// GetProfile returns user profile
func (h *UserHandler) GetProfile(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get user profile", zap.Error(err))
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "user_not_found",
			"message": "User not found",
		})
	}

	// Return safe user data (exclude sensitive fields)
	profile := map[string]interface{}{
		"id":              user.ID,
		"master_username": user.MasterUsername,
		"display_name":    user.DisplayName,
		"bio":             user.Bio,
		"mfa_enabled":     user.MFAEnabled,
		"created_at":      user.CreatedAt,
		"security_score":  user.SecurityScore,
		"storage_quota": map[string]interface{}{
			"total":      user.StorageQuotaTotal,
			"used":       user.StorageQuotaUsed,
			"percentage": float64(user.StorageQuotaUsed) / float64(user.StorageQuotaTotal) * 100,
		},
	}

	return c.JSON(http.StatusOK, profile)
}

// UpdateProfile updates user profile
func (h *UserHandler) UpdateProfile(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	var req struct {
		DisplayName string `json:"display_name,omitempty"`
		Bio         string `json:"bio,omitempty"`
		UITheme     string `json:"ui_theme,omitempty"`
		AccentColor string `json:"accent_color,omitempty"`
		Density     string `json:"density,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "user_not_found",
			"message": "User not found",
		})
	}

	// Update fields if provided
	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Bio != "" {
		user.Bio = req.Bio
	}
	if req.UITheme != "" {
		user.UITheme = req.UITheme
	}
	if req.AccentColor != "" {
		user.AccentColor = req.AccentColor
	}
	if req.Density != "" {
		user.Density = req.Density
	}

	if err := h.userRepo.Update(ctx, user); err != nil {
		h.logger.Error("Failed to update user profile", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "update_failed",
			"message": "Failed to update profile",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Profile updated successfully",
		"user": map[string]interface{}{
			"id":           user.ID,
			"display_name": user.DisplayName,
			"bio":          user.Bio,
		},
	})
}

// GetSettings returns user settings
func (h *UserHandler) GetSettings(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "user_not_found",
			"message": "User not found",
		})
	}

	settings := map[string]interface{}{
		"privacy": map[string]interface{}{
			"metadata_minimization":    user.MetadataMinimization,
			"logging_consent":          user.LoggingConsent,
			"analytics_opt_out":        user.AnalyticsOptOut,
			"auto_delete_old_messages": user.AutoDeleteOldMessages,
			"retention_days":           user.RetentionDays,
		},
		"security": map[string]interface{}{
			"mfa_enabled":        user.MFAEnabled,
			"session_timeout":    user.SessionTimeout,
			"security_score":     user.SecurityScore,
			"last_security_scan": user.LastSecurityScan,
		},
		"appearance": map[string]interface{}{
			"ui_theme":     user.UITheme,
			"accent_color": user.AccentColor,
			"density":      user.Density,
		},
	}

	return c.JSON(http.StatusOK, settings)
}

// UpdateSettings updates user settings
func (h *UserHandler) UpdateSettings(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	var req struct {
		Privacy    map[string]interface{} `json:"privacy,omitempty"`
		Security   map[string]interface{} `json:"security,omitempty"`
		Appearance map[string]interface{} `json:"appearance,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "user_not_found",
			"message": "User not found",
		})
	}

	// Update privacy settings
	if privacy, ok := req.Privacy["metadata_minimization"]; ok {
		if val, ok := privacy.(bool); ok {
			user.MetadataMinimization = val
		}
	}
	if privacy, ok := req.Privacy["logging_consent"]; ok {
		if val, ok := privacy.(bool); ok {
			user.LoggingConsent = val
		}
	}
	if privacy, ok := req.Privacy["analytics_opt_out"]; ok {
		if val, ok := privacy.(bool); ok {
			user.AnalyticsOptOut = val
		}
	}
	if privacy, ok := req.Privacy["auto_delete_old_messages"]; ok {
		if val, ok := privacy.(bool); ok {
			user.AutoDeleteOldMessages = val
		}
	}
	if privacy, ok := req.Privacy["retention_days"]; ok {
		if val, ok := privacy.(float64); ok {
			user.RetentionDays = int(val)
		}
	}

	// Update security settings
	if security, ok := req.Security["session_timeout"]; ok {
		if val, ok := security.(float64); ok {
			user.SessionTimeout = int(val)
		}
	}

	// Update appearance settings
	if appearance, ok := req.Appearance["ui_theme"]; ok {
		if val, ok := appearance.(string); ok {
			user.UITheme = val
		}
	}
	if appearance, ok := req.Appearance["accent_color"]; ok {
		if val, ok := appearance.(string); ok {
			user.AccentColor = val
		}
	}
	if appearance, ok := req.Appearance["density"]; ok {
		if val, ok := appearance.(string); ok {
			user.Density = val
		}
	}

	if err := h.userRepo.Update(ctx, user); err != nil {
		h.logger.Error("Failed to update user settings", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "update_failed",
			"message": "Failed to update settings",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Settings updated successfully",
	})
}

// GetStorageUsage returns user storage usage
func (h *UserHandler) GetStorageUsage(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	used, total, err := h.userRepo.GetStorageUsage(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get storage usage", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "storage_error",
			"message": "Failed to get storage usage",
		})
	}

	percentage := float64(0)
	if total > 0 {
		percentage = float64(used) / float64(total) * 100
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"used":       used,
		"total":      total,
		"available":  total - used,
		"percentage": percentage,
		"human_readable": map[string]string{
			"used":      formatBytes(used),
			"total":     formatBytes(total),
			"available": formatBytes(total - used),
		},
	})
}

// GetSessions returns user's active sessions
func (h *UserHandler) GetSessions(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	sessions, err := h.authService.GetUserSessions(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get user sessions", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "sessions_error",
			"message": "Failed to get sessions",
		})
	}

	// Filter sensitive data
	var safeSessions []map[string]interface{}
	for _, session := range sessions {
		safeSession := map[string]interface{}{
			"id":            session.ID,
			"created_at":    session.CreatedAt,
			"last_activity": session.LastActivity,
			"expires_at":    session.ExpiresAt,
			"user_agent":    session.UserAgent,
			"location":      session.Location,
		}
		safeSessions = append(safeSessions, safeSession)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"sessions": safeSessions,
		"count":    len(safeSessions),
	})
}

// RevokeSession revokes a specific session
func (h *UserHandler) RevokeSession(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_session_id",
			"message": "Invalid session ID",
		})
	}

	ctx := c.Request().Context()
	if err := h.authService.RevokeSession(ctx, userID, sessionID); err != nil {
		h.logger.Error("Failed to revoke session", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "revoke_failed",
			"message": "Failed to revoke session",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Session revoked successfully",
	})
}

// RevokeAllSessions revokes all sessions except current one
func (h *UserHandler) RevokeAllSessions(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	if err := h.authService.RevokeAllSessions(ctx, userID); err != nil {
		h.logger.Error("Failed to revoke all sessions", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "revoke_failed",
			"message": "Failed to revoke sessions",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "All other sessions revoked successfully",
	})
}

// Helper function to format bytes
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
