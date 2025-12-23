// internal/api/handlers/network.go
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service"
)

// NetworkHandler handles network operations
type NetworkHandler struct {
	networkService service.NetworkService
	networkRepo    *repository.NetworkIdentityRepository
	logger         *zap.Logger
}

// NewNetworkHandler creates a new network handler
func NewNetworkHandler(
	networkService service.NetworkService,
	networkRepo *repository.NetworkIdentityRepository,
	logger *zap.Logger,
) *NetworkHandler {
	return &NetworkHandler{
		networkService: networkService,
		networkRepo:    networkRepo,
		logger:         logger,
	}
}

// GetStatus returns status of all networks
func (h *NetworkHandler) GetStatus(c echo.Context) error {
	ctx := c.Request().Context()
	status, err := h.networkService.GetNetworkStatus(ctx)
	if err != nil {
		h.logger.Error("Failed to get network status", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "status_failed",
			"message": "Failed to get network status",
		})
	}

	// Convert to map for JSON serialization
	result := make(map[string]interface{})
	for network, netStatus := range status {
		result[string(network)] = map[string]interface{}{
			"is_healthy":    netStatus.IsHealthy,
			"last_error":    netStatus.LastError,
			"last_checked":  netStatus.LastChecked,
			"message_count": netStatus.MessageCount,
			"latency_ms":    netStatus.Latency.Milliseconds(),
		}
	}

	return c.JSON(http.StatusOK, result)
}

// GetIdentities returns all network identities for a user
func (h *NetworkHandler) GetIdentities(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	identities, err := h.networkRepo.ListByUser(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get network identities", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "list_failed",
			"message": "Failed to get network identities",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"identities": identities,
		"count":      len(identities),
	})
}

// CreateIdentity creates a new network identity
func (h *NetworkHandler) CreateIdentity(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	var req struct {
		Network   models.NetworkType `json:"network" validate:"required"`
		Address   string             `json:"address" validate:"required"`
		IsPrimary bool               `json:"is_primary,omitempty"`
		ForwardTo string             `json:"forward_to,omitempty"`
		Config    string             `json:"config,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	// Validate network type
	switch req.Network {
	case models.NetworkClearnet, models.NetworkI2P, models.NetworkTor, models.NetworkLAN, models.NetworkIPFS:
		// Valid network type
	default:
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_network",
			"message": "Invalid network type",
		})
	}

	// Validate address format based on network
	if !h.validateAddressForNetwork(req.Network, req.Address) {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_address",
			"message": fmt.Sprintf("Invalid address format for network %s", req.Network),
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

	ctx := c.Request().Context()

	// Check if address already exists
	existing, err := h.networkRepo.GetByAddress(ctx, req.Address)
	if err == nil && existing != nil {
		return c.JSON(http.StatusConflict, map[string]interface{}{
			"error":   "address_exists",
			"message": "Address already exists",
		})
	}

	// If this is marked as primary, unset other primaries
	if req.IsPrimary {
		if err := h.networkRepo.ClearPrimary(ctx, userID, req.Network); err != nil {
			h.logger.Error("Failed to clear primary identity", zap.Error(err))
		}
	}

	identity := &models.NetworkIdentity{
		ID:        uuid.New(),
		UserID:    userID,
		Network:   req.Network,
		Address:   req.Address,
		IsPrimary: req.IsPrimary,
		ForwardTo: req.ForwardTo,
		Config:    config,
		IsActive:  true,
		CreatedAt: time.Now(),
	}

	if err := h.networkRepo.Create(ctx, identity); err != nil {
		h.logger.Error("Failed to create network identity", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "create_failed",
			"message": "Failed to create network identity",
		})
	}

	// Test network connection if possible
	go h.testNetworkConnection(identity)

	return c.JSON(http.StatusCreated, identity)
}

// UpdateIdentity updates a network identity
func (h *NetworkHandler) UpdateIdentity(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_identity_id",
			"message": "Invalid identity ID",
		})
	}

	var req struct {
		Address   string `json:"address,omitempty"`
		IsPrimary bool   `json:"is_primary,omitempty"`
		ForwardTo string `json:"forward_to,omitempty"`
		IsActive  bool   `json:"is_active,omitempty"`
		Config    string `json:"config,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()

	// Get identity and check ownership
	identity, err := h.networkRepo.GetByID(ctx, identityID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "identity_not_found",
			"message": "Network identity not found",
		})
	}

	if identity.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to update this identity",
		})
	}

	// Update fields
	updated := false
	if req.Address != "" && req.Address != identity.Address {
		// Validate new address
		if !h.validateAddressForNetwork(identity.Network, req.Address) {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_address",
				"message": fmt.Sprintf("Invalid address format for network %s", identity.Network),
			})
		}
		// Check for duplicate
		existing, err := h.networkRepo.GetByAddress(ctx, req.Address)
		if err == nil && existing != nil && existing.ID != identityID {
			return c.JSON(http.StatusConflict, map[string]interface{}{
				"error":   "address_exists",
				"message": "Address already exists",
			})
		}
		identity.Address = req.Address
		updated = true
	}

	if req.IsPrimary != identity.IsPrimary {
		if req.IsPrimary {
			// Unset other primaries for this network
			if err := h.networkRepo.ClearPrimary(ctx, userID, identity.Network); err != nil {
				h.logger.Error("Failed to clear primary identity", zap.Error(err))
			}
		}
		identity.IsPrimary = req.IsPrimary
		updated = true
	}

	if req.ForwardTo != identity.ForwardTo {
		identity.ForwardTo = req.ForwardTo
		updated = true
	}

	if req.IsActive != identity.IsActive {
		identity.IsActive = req.IsActive
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
		identity.Config = config
		updated = true
	}

	if !updated {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "No changes made",
		})
	}

	if err := h.networkRepo.Update(ctx, identity); err != nil {
		h.logger.Error("Failed to update network identity", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "update_failed",
			"message": "Failed to update network identity",
		})
	}

	// Test network connection if activated
	if identity.IsActive {
		go h.testNetworkConnection(identity)
	}

	return c.JSON(http.StatusOK, identity)
}

// DeleteIdentity deletes a network identity
func (h *NetworkHandler) DeleteIdentity(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_identity_id",
			"message": "Invalid identity ID",
		})
	}

	ctx := c.Request().Context()

	// Get identity and check ownership
	identity, err := h.networkRepo.GetByID(ctx, identityID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "identity_not_found",
			"message": "Network identity not found",
		})
	}

	if identity.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to delete this identity",
		})
	}

	// Don't allow deletion if it's the only identity for a network
	counts, err := h.networkRepo.CountByNetwork(ctx)
	if err == nil && counts[identity.Network] <= 1 {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "last_identity",
			"message": "Cannot delete the last identity for this network",
		})
	}

	if err := h.networkRepo.Delete(ctx, identityID); err != nil {
		h.logger.Error("Failed to delete network identity", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "delete_failed",
			"message": "Failed to delete network identity",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":     "Network identity deleted successfully",
		"identity_id": identityID,
	})
}

// TestIdentityConnection tests connectivity for a network identity
func (h *NetworkHandler) TestIdentityConnection(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	identityID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_identity_id",
			"message": "Invalid identity ID",
		})
	}

	ctx := c.Request().Context()

	// Get identity and check ownership
	identity, err := h.networkRepo.GetByID(ctx, identityID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "identity_not_found",
			"message": "Network identity not found",
		})
	}

	if identity.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to test this identity",
		})
	}

	// Test the network
	ok, err := h.networkService.TestNetwork(ctx, identity.Network)
	if err != nil {
		h.logger.Error("Network test failed", zap.Error(err))
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"connected": false,
			"error":     err.Error(),
			"message":   "Network test failed",
		})
	}

	// Update last used time
	now := time.Now()
	identity.LastUsed = &now
	h.networkRepo.Update(ctx, identity)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"connected": ok,
		"network":   identity.Network,
		"address":   identity.Address,
		"tested_at": now,
	})
}

// Helper methods
func (h *NetworkHandler) validateAddressForNetwork(network models.NetworkType, address string) bool {
	switch network {
	case models.NetworkClearnet:
		return strings.Contains(address, "@") && len(address) > 3
	case models.NetworkI2P:
		return strings.HasSuffix(address, ".i2p") || strings.Contains(address, ".b32.i2p")
	case models.NetworkTor:
		return strings.HasSuffix(address, ".onion")
	case models.NetworkLAN:
		return strings.Contains(address, "@") || strings.Contains(address, ".")
	case models.NetworkIPFS:
		return strings.HasPrefix(address, "Qm") && len(address) > 10
	default:
		return false
	}
}

func (h *NetworkHandler) testNetworkConnection(identity *models.NetworkIdentity) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ok, err := h.networkService.TestNetwork(ctx, identity.Network)
	if err != nil {
		h.logger.Warn("Network test failed",
			zap.String("network", string(identity.Network)),
			zap.String("address", identity.Address),
			zap.Error(err))
		return
	}

	h.logger.Info("Network test completed",
		zap.String("network", string(identity.Network)),
		zap.String("address", identity.Address),
		zap.Bool("connected", ok))
}
