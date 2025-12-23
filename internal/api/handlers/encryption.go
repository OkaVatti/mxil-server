// internal/api/handlers/encryption.go
package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service"
)

// EncryptionHandler handles encryption key operations
type EncryptionHandler struct {
	cryptoService service.CryptoService
	keyRepo       *repository.EncryptionKeyRepository
	logger        *zap.Logger
}

// NewEncryptionHandler creates a new encryption handler
func NewEncryptionHandler(
	cryptoService service.CryptoService,
	keyRepo *repository.EncryptionKeyRepository,
	logger *zap.Logger,
) *EncryptionHandler {
	return &EncryptionHandler{
		cryptoService: cryptoService,
		keyRepo:       keyRepo,
		logger:        logger,
	}
}

// GetKeys lists all encryption keys for a user
func (h *EncryptionHandler) GetKeys(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	keys, err := h.keyRepo.ListByUser(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to list encryption keys", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "list_failed",
			"message": "Failed to list encryption keys",
		})
	}

	// Filter sensitive data
	var safeKeys []map[string]interface{}
	for _, key := range keys {
		safeKey := map[string]interface{}{
			"id":          key.ID,
			"key_type":    key.KeyType,
			"key_id":      key.KeyID,
			"fingerprint": key.Fingerprint,
			"is_primary":  key.IsPrimary,
			"is_active":   key.IsActive,
			"created_at":  key.CreatedAt,
			"last_used":   key.LastUsed,
			"expires_at":  key.ExpiresAt,
		}
		safeKeys = append(safeKeys, safeKey)
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"keys":  safeKeys,
		"count": len(safeKeys),
	})
}

// GenerateKey generates a new encryption key pair
func (h *EncryptionHandler) GenerateKey(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	var req struct {
		Algorithm string `json:"algorithm" validate:"required"`
		KeySize   int    `json:"key_size,omitempty"`
		IsPrimary bool   `json:"is_primary,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	// Validate algorithm
	supportedAlgorithms := map[string]bool{
		"rsa-2048": true,
		"rsa-4096": true,
		"ec-p256":  true,
		"ec-p384":  true,
	}
	if !supportedAlgorithms[req.Algorithm] {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "unsupported_algorithm",
			"message": "Unsupported encryption algorithm",
		})
	}

	ctx := c.Request().Context()

	// Generate key pair
	publicKey, privateKey, fingerprint, err := h.cryptoService.GenerateKeyPair(userID.String(), req.Algorithm)
	if err != nil {
		h.logger.Error("Failed to generate key pair", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "generation_failed",
			"message": "Failed to generate encryption key",
		})
	}

	// Encrypt private key for storage
	encryptedPrivateKey, err := h.cryptoService.EncryptString(privateKey)
	if err != nil {
		h.logger.Error("Failed to encrypt private key", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "encryption_failed",
			"message": "Failed to encrypt private key",
		})
	}

	// If this is marked as primary, unset other primaries
	if req.IsPrimary {
		if err := h.keyRepo.ClearPrimary(ctx, userID); err != nil {
			h.logger.Error("Failed to clear primary key", zap.Error(err))
		}
	}

	// Generate unique key ID
	keyID := fmt.Sprintf("%s-%s", req.Algorithm, fingerprint[:8])

	key := &models.EncryptionKey{
		ID:                  uuid.New(),
		UserID:              userID,
		KeyType:             req.Algorithm,
		KeyID:               keyID,
		PublicKey:           publicKey,
		PrivateKeyEncrypted: encryptedPrivateKey,
		Fingerprint:         fingerprint,
		IsPrimary:           req.IsPrimary,
		IsActive:            true,
		CreatedAt:           time.Now(),
	}

	if err := h.keyRepo.Create(ctx, key); err != nil {
		h.logger.Error("Failed to save encryption key", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "save_failed",
			"message": "Failed to save encryption key",
		})
	}

	// Return key info (without private key)
	keyInfo := map[string]interface{}{
		"id":          key.ID,
		"key_type":    key.KeyType,
		"key_id":      key.KeyID,
		"fingerprint": key.Fingerprint,
		"is_primary":  key.IsPrimary,
		"is_active":   key.IsActive,
		"created_at":  key.CreatedAt,
		"public_key":  key.PublicKey,
		"algorithm":   req.Algorithm,
		"key_size":    req.KeySize,
		"warning":     "Private key is stored securely and cannot be retrieved",
	}

	return c.JSON(http.StatusCreated, keyInfo)
}

// DeleteKey deletes an encryption key
func (h *EncryptionHandler) DeleteKey(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	keyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_key_id",
			"message": "Invalid key ID",
		})
	}

	ctx := c.Request().Context()

	// Get key and check ownership
	key, err := h.keyRepo.GetByID(ctx, keyID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "key_not_found",
			"message": "Encryption key not found",
		})
	}

	if key.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to delete this key",
		})
	}

	// Don't allow deletion of primary key if it's the only one
	if key.IsPrimary {
		count, err := h.keyRepo.CountActive(ctx, userID)
		if err == nil && count <= 1 {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "last_primary",
				"message": "Cannot delete the last primary encryption key",
			})
		}
	}

	// Mark as inactive instead of deleting (for audit trail)
	key.IsActive = false
	key.DeletedAt = gorm.DeletedAt{Time: time.Now(), Valid: true}

	if err := h.keyRepo.Update(ctx, key); err != nil {
		h.logger.Error("Failed to delete encryption key", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "delete_failed",
			"message": "Failed to delete encryption key",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Encryption key deactivated successfully",
		"key_id":  keyID,
	})
}

// RotateKey rotates an encryption key (creates new, marks old as inactive)
func (h *EncryptionHandler) RotateKey(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	oldKeyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_key_id",
			"message": "Invalid key ID",
		})
	}

	var req struct {
		Algorithm string `json:"algorithm,omitempty"`
		KeySize   int    `json:"key_size,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()

	// Get old key and check ownership
	oldKey, err := h.keyRepo.GetByID(ctx, oldKeyID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "key_not_found",
			"message": "Encryption key not found",
		})
	}

	if oldKey.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to rotate this key",
		})
	}

	// Use same algorithm if not specified
	if req.Algorithm == "" {
		req.Algorithm = oldKey.KeyType
	}

	// Generate new key pair
	newKeyID, err := h.cryptoService.RotateKey(userID.String(), req.Algorithm)
	if err != nil {
		h.logger.Error("Failed to rotate key", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "rotation_failed",
			"message": "Failed to rotate encryption key",
		})
	}

	// Mark old key as inactive
	oldKey.IsActive = false
	oldKey.ExpiresAt = &time.Time{}
	if err := h.keyRepo.Update(ctx, oldKey); err != nil {
		h.logger.Error("Failed to update old key", zap.Error(err))
	}

	// Get new key details
	newKey, err := h.keyRepo.GetByKeyID(ctx, newKeyID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "key_not_found",
			"message": "New key not found after rotation",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Encryption key rotated successfully",
		"old_key": map[string]interface{}{
			"id":          oldKey.ID,
			"key_id":      oldKey.KeyID,
			"fingerprint": oldKey.Fingerprint,
			"status":      "inactive",
		},
		"new_key": map[string]interface{}{
			"id":          newKey.ID,
			"key_id":      newKey.KeyID,
			"fingerprint": newKey.Fingerprint,
			"is_primary":  newKey.IsPrimary,
			"status":      "active",
		},
		"rotation_time": time.Now(),
	})
}

// TestEncryption tests encryption/decryption with user's keys
func (h *EncryptionHandler) TestEncryption(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	var req struct {
		Algorithm string `json:"algorithm" validate:"required"`
		Plaintext string `json:"plaintext" validate:"required"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()

	// Get user's primary key
	primaryKey, err := h.keyRepo.GetPrimaryKey(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "no_primary_key",
			"message": "No primary encryption key found",
		})
	}

	// Test encryption
	startTime := time.Now()
	encrypted, keyIDs, err := h.cryptoService.EncryptEmail(req.Plaintext, []string{primaryKey.KeyID}, req.Algorithm)
	encryptDuration := time.Since(startTime)

	if err != nil {
		h.logger.Error("Encryption test failed", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "encryption_failed",
			"message": "Encryption test failed",
		})
	}

	// Test decryption (if we have the private key)
	// Note: In production, this would require the private key to be available
	// For now, just verify the encryption worked

	result := map[string]interface{}{
		"test_passed":         true,
		"algorithm":           req.Algorithm,
		"original_length":     len(req.Plaintext),
		"encrypted_length":    len(encrypted),
		"encrypt_duration":    encryptDuration.Milliseconds(),
		"keys_used":           keyIDs,
		"primary_key_id":      primaryKey.KeyID,
		"primary_fingerprint": primaryKey.Fingerprint,
		"encrypted_sample":    string(encrypted)[:min(100, len(encrypted))] + "...",
	}

	return c.JSON(http.StatusOK, result)
}

// GetKeyInfo gets detailed information about a key
func (h *EncryptionHandler) GetKeyInfo(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	keyID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_key_id",
			"message": "Invalid key ID",
		})
	}

	ctx := c.Request().Context()

	// Get key and check ownership
	key, err := h.keyRepo.GetByID(ctx, keyID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "key_not_found",
			"message": "Encryption key not found",
		})
	}

	if key.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to view this key",
		})
	}

	// Get technical information about the key
	keyInfo, err := h.cryptoService.GetKeyInfo(key.PublicKey)
	if err != nil {
		keyInfo = map[string]interface{}{
			"type":    "unknown",
			"error":   "failed_to_parse",
			"message": err.Error(),
		}
	}

	// Get usage statistics
	usage, err := h.keyRepo.GetKeyUsage(ctx, keyID)
	if err != nil {
		usage = &repository.KeyUsage{
			TotalEncrypted: 0,
			LastUsed:       key.LastUsed,
			FirstUsed:      &key.CreatedAt,
		}
	}

	result := map[string]interface{}{
		"key": map[string]interface{}{
			"id":          key.ID,
			"key_type":    key.KeyType,
			"key_id":      key.KeyID,
			"fingerprint": key.Fingerprint,
			"is_primary":  key.IsPrimary,
			"is_active":   key.IsActive,
			"created_at":  key.CreatedAt,
			"last_used":   key.LastUsed,
			"expires_at":  key.ExpiresAt,
		},
		"technical": keyInfo,
		"usage":     usage,
		"security": map[string]interface{}{
			"is_expired":     key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()),
			"is_compromised": false, // TODO: Add compromise detection
			"recommendation": h.getKeyRecommendation(key, usage),
		},
	}

	return c.JSON(http.StatusOK, result)
}

// Helper methods
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (h *EncryptionHandler) getKeyRecommendation(key *models.EncryptionKey, usage *repository.KeyUsage) string {
	now := time.Now()
	created := key.CreatedAt

	// Key age check
	if now.Sub(created) > 365*24*time.Hour { // 1 year
		return "Consider rotating this key as it's over 1 year old"
	}

	// Expiration check
	if key.ExpiresAt != nil && now.After(*key.ExpiresAt) {
		return "Key has expired. Rotate immediately."
	}

	// Usage check
	if usage.TotalEncrypted == 0 && now.Sub(created) > 30*24*time.Hour { // 1 month
		return "Key has never been used. Consider removing if not needed."
	}

	return "Key is in good standing"
}
