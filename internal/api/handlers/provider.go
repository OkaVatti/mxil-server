// internal/api/handlers/provider.go
package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
)

// ProviderHandler handles email provider operations
type ProviderHandler struct {
	providerRepo *repository.ProviderBridgeRepository
	logger       *zap.Logger
}

// NewProviderHandler creates a new provider handler
func NewProviderHandler(
	providerRepo *repository.ProviderBridgeRepository,
	logger *zap.Logger,
) *ProviderHandler {
	return &ProviderHandler{
		providerRepo: providerRepo,
		logger:       logger,
	}
}

// ListProviders lists all provider bridges for a user
func (h *ProviderHandler) ListProviders(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	providers, err := h.providerRepo.ListByUser(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to list providers", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "list_failed",
			"message": "Failed to list providers",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"providers": providers,
		"count":     len(providers),
	})
}

// AddProvider adds a new provider bridge
func (h *ProviderHandler) AddProvider(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	var req struct {
		ProviderType      models.ProviderType `json:"provider_type" validate:"required"`
		AccountIdentifier string              `json:"account_identifier" validate:"required"`
		Credentials       string              `json:"credentials" validate:"required"`
		SyncInterval      int                 `json:"sync_interval,omitempty"`
		Config            string              `json:"config,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	// Validate provider type
	switch req.ProviderType {
	case models.ProviderGmail, models.ProviderOutlook, models.ProviderYahoo,
		models.ProviderProton, models.ProviderI2PBote, models.ProviderSusimail,
		models.ProviderOnionMail, models.ProviderCustomIMAP:
		// Valid provider type
	default:
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_provider",
			"message": "Invalid provider type",
		})
	}

	// Parse config JSON
	var config models.JSONB
	if req.Config != "" {
		if err := json.Unmarshal([]byte(req.Config), &config); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_config",
				"message": "Invalid configuration JSON",
			})
		}
	}

	// Set default sync interval
	if req.SyncInterval <= 0 {
		req.SyncInterval = 300 // 5 minutes
	}

	ctx := c.Request().Context()

	// Check if provider already exists
	existing, err := h.providerRepo.GetByAccount(ctx, userID, req.ProviderType, req.AccountIdentifier)
	if err == nil && existing != nil {
		return c.JSON(http.StatusConflict, map[string]interface{}{
			"error":   "provider_exists",
			"message": "Provider bridge already exists for this account",
		})
	}

	// TODO: Validate credentials by testing connection to provider
	// For now, just store them

	provider := &models.ProviderBridge{
		ID:                uuid.New(),
		UserID:            userID,
		ProviderType:      req.ProviderType,
		AccountIdentifier: req.AccountIdentifier,
		// Note: In production, credentials should be encrypted
		EncryptedCredentials: req.Credentials,
		IsActive:             true,
		SyncInterval:         req.SyncInterval,
		Config:               config,
	}

	if err := h.providerRepo.Create(ctx, provider); err != nil {
		h.logger.Error("Failed to add provider", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "add_failed",
			"message": "Failed to add provider bridge",
		})
	}

	// Start initial sync in background
	go h.syncProvider(provider)

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"provider": provider,
		"message":  "Provider bridge added successfully",
	})
}

// UpdateProvider updates a provider bridge
func (h *ProviderHandler) UpdateProvider(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_provider_id",
			"message": "Invalid provider ID",
		})
	}

	var req struct {
		AccountIdentifier string `json:"account_identifier,omitempty"`
		Credentials       string `json:"credentials,omitempty"`
		SyncInterval      int    `json:"sync_interval,omitempty"`
		IsActive          bool   `json:"is_active,omitempty"`
		Config            string `json:"config,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()

	// Get provider and check ownership
	provider, err := h.providerRepo.GetByID(ctx, providerID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "provider_not_found",
			"message": "Provider bridge not found",
		})
	}

	if provider.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to update this provider",
		})
	}

	// Update fields
	updated := false
	if req.AccountIdentifier != "" && req.AccountIdentifier != provider.AccountIdentifier {
		// Check for duplicate
		existing, err := h.providerRepo.GetByAccount(ctx, userID, provider.ProviderType, req.AccountIdentifier)
		if err == nil && existing != nil && existing.ID != providerID {
			return c.JSON(http.StatusConflict, map[string]interface{}{
				"error":   "account_exists",
				"message": "Provider bridge already exists for this account",
			})
		}
		provider.AccountIdentifier = req.AccountIdentifier
		updated = true
	}

	if req.Credentials != "" && req.Credentials != provider.EncryptedCredentials {
		provider.EncryptedCredentials = req.Credentials
		updated = true
	}

	if req.SyncInterval > 0 && req.SyncInterval != provider.SyncInterval {
		provider.SyncInterval = req.SyncInterval
		updated = true
	}

	if req.IsActive != provider.IsActive {
		provider.IsActive = req.IsActive
		updated = true
	}

	if req.Config != "" {
		var config models.JSONB
		if err := json.Unmarshal([]byte(req.Config), &config); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_config",
				"message": "Invalid configuration JSON",
			})
		}
		provider.Config = config
		updated = true
	}

	if !updated {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "No changes made",
		})
	}

	if err := h.providerRepo.Update(ctx, provider); err != nil {
		h.logger.Error("Failed to update provider", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "update_failed",
			"message": "Failed to update provider bridge",
		})
	}

	// If activated, start sync
	if provider.IsActive {
		go h.syncProvider(provider)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"provider": provider,
		"message":  "Provider bridge updated successfully",
	})
}

// RemoveProvider removes a provider bridge
func (h *ProviderHandler) RemoveProvider(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_provider_id",
			"message": "Invalid provider ID",
		})
	}

	ctx := c.Request().Context()

	// Get provider and check ownership
	provider, err := h.providerRepo.GetByID(ctx, providerID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "provider_not_found",
			"message": "Provider bridge not found",
		})
	}

	if provider.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to remove this provider",
		})
	}

	if err := h.providerRepo.Delete(ctx, providerID); err != nil {
		h.logger.Error("Failed to remove provider", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "remove_failed",
			"message": "Failed to remove provider bridge",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":     "Provider bridge removed successfully",
		"provider_id": providerID,
	})
}

// SyncProvider triggers a manual sync for a provider
func (h *ProviderHandler) SyncProvider(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_provider_id",
			"message": "Invalid provider ID",
		})
	}

	ctx := c.Request().Context()

	// Get provider and check ownership
	provider, err := h.providerRepo.GetByID(ctx, providerID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "provider_not_found",
			"message": "Provider bridge not found",
		})
	}

	if provider.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to sync this provider",
		})
	}

	if !provider.IsActive {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "provider_inactive",
			"message": "Provider bridge is not active",
		})
	}

	// Start sync in background
	go h.syncProvider(provider)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":     "Sync started",
		"provider_id": providerID,
		"started_at":  time.Now(),
	})
}

// GetProviderStatus gets sync status for a provider
func (h *ProviderHandler) GetProviderStatus(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	providerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_provider_id",
			"message": "Invalid provider ID",
		})
	}

	ctx := c.Request().Context()

	// Get provider and check ownership
	provider, err := h.providerRepo.GetByID(ctx, providerID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "provider_not_found",
			"message": "Provider bridge not found",
		})
	}

	if provider.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to access this provider",
		})
	}

	// Get sync statistics
	stats, err := h.providerRepo.GetSyncStats(ctx, providerID)
	if err != nil {
		stats = &repository.SyncStats{
			TotalEmails:  0,
			LastSync:     provider.LastSync,
			SyncDuration: 0,
			SuccessRate:  0,
			ErrorCount:   0,
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"provider": provider,
		"stats":    stats,
		"status": map[string]interface{}{
			"is_active":  provider.IsActive,
			"last_sync":  provider.LastSync,
			"next_sync":  h.calculateNextSync(provider),
			"sync_count": stats.TotalEmails,
		},
	})
}

// Helper methods
func (h *ProviderHandler) syncProvider(provider *models.ProviderBridge) {
	h.logger.Info("Starting provider sync",
		zap.String("provider", string(provider.ProviderType)),
		zap.String("account", provider.AccountIdentifier))

	startTime := time.Now()

	// TODO: Implement actual provider synchronization
	// This would connect to the provider's API/IMAP/SMTP and sync emails

	// Simulate sync process
	time.Sleep(5 * time.Second)

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	// Update last sync time
	ctx := context.Background()
	provider.LastSync = &endTime
	if err := h.providerRepo.Update(ctx, provider); err != nil {
		h.logger.Error("Failed to update provider sync time", zap.Error(err))
	}

	h.logger.Info("Provider sync completed",
		zap.String("provider", string(provider.ProviderType)),
		zap.String("account", provider.AccountIdentifier),
		zap.Duration("duration", duration))
}

func (h *ProviderHandler) calculateNextSync(provider *models.ProviderBridge) *time.Time {
	if provider.LastSync == nil || !provider.IsActive {
		return nil
	}

	nextSync := provider.LastSync.Add(time.Duration(provider.SyncInterval) * time.Second)
	return &nextSync
}
