// internal/api/handlers/label.go
package handlers

import (
	"math/rand"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
)

// LabelHandler handles label operations
type LabelHandler struct {
	labelRepo *repository.LabelRepository
	emailRepo *repository.EmailRepository
	logger    *zap.Logger
}

// NewLabelHandler creates a new label handler
func NewLabelHandler(
	labelRepo *repository.LabelRepository,
	emailRepo *repository.EmailRepository,
	logger *zap.Logger,
) *LabelHandler {
	return &LabelHandler{
		labelRepo: labelRepo,
		emailRepo: emailRepo,
		logger:    logger,
	}
}

// ListLabels lists all labels for a user
func (h *LabelHandler) ListLabels(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	labels, err := h.labelRepo.ListByUser(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to list labels", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "list_failed",
			"message": "Failed to list labels",
		})
	}

	// Count emails per label
	for i, label := range labels {
		count, err := h.emailRepo.CountByLabel(ctx, label.ID)
		if err == nil {
			labels[i].EmailCount = count
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"labels": labels,
		"count":  len(labels),
	})
}

// CreateLabel creates a new label
func (h *LabelHandler) CreateLabel(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	var req struct {
		Name  string `json:"name" validate:"required"`
		Color string `json:"color,omitempty"`
		Icon  string `json:"icon,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	// Validate label name
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_name",
			"message": "Label name cannot be empty",
		})
	}

	// Validate color
	if req.Color != "" && !isValidHexColor(req.Color) {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_color",
			"message": "Invalid color format (expected #RRGGBB)",
		})
	}

	ctx := c.Request().Context()

	// Check if label already exists
	existing, err := h.labelRepo.GetByName(ctx, userID, name)
	if err == nil && existing != nil {
		return c.JSON(http.StatusConflict, map[string]interface{}{
			"error":   "label_exists",
			"message": "Label already exists",
		})
	}

	label := &models.Label{
		ID:     uuid.New(),
		UserID: userID,
		Name:   name,
		Color:  req.Color,
		Icon:   req.Icon,
	}

	if label.Color == "" {
		label.Color = generateRandomColor()
	}

	ctx := c.Request().Context()
	if err := h.labelRepo.Create(ctx, label); err != nil {
		h.logger.Error("Failed to create label", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "create_failed",
			"message": "Failed to create label",
		})
	}

	return c.JSON(http.StatusCreated, label)
}

// UpdateLabel updates a label
func (h *LabelHandler) UpdateLabel(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	labelID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_label_id",
			"message": "Invalid label ID",
		})
	}

	var req struct {
		Name  string `json:"name,omitempty"`
		Color string `json:"color,omitempty"`
		Icon  string `json:"icon,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()

	// Get label and check ownership
	label, err := h.labelRepo.GetByID(ctx, labelID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "label_not_found",
			"message": "Label not found",
		})
	}

	if label.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to update this label",
		})
	}

	// System labels cannot be modified
	if label.IsSystem {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "system_label",
			"message": "System labels cannot be modified",
		})
	}

	// Update fields
	updated := false
	if req.Name != "" && req.Name != label.Name {
		// Check for name conflict
		existing, err := h.labelRepo.GetByName(ctx, userID, req.Name)
		if err == nil && existing != nil && existing.ID != labelID {
			return c.JSON(http.StatusConflict, map[string]interface{}{
				"error":   "label_exists",
				"message": "Label with this name already exists",
			})
		}
		label.Name = req.Name
		updated = true
	}

	if req.Color != "" && req.Color != label.Color {
		if !isValidHexColor(req.Color) {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_color",
				"message": "Invalid color format (expected #RRGGBB)",
			})
		}
		label.Color = req.Color
		updated = true
	}

	if req.Icon != "" && req.Icon != label.Icon {
		label.Icon = req.Icon
		updated = true
	}

	if !updated {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "No changes made",
		})
	}

	if err := h.labelRepo.Update(ctx, label); err != nil {
		h.logger.Error("Failed to update label", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "update_failed",
			"message": "Failed to update label",
		})
	}

	return c.JSON(http.StatusOK, label)
}

// DeleteLabel deletes a label
func (h *LabelHandler) DeleteLabel(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	labelID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_label_id",
			"message": "Invalid label ID",
		})
	}

	ctx := c.Request().Context()

	// Get label and check ownership
	label, err := h.labelRepo.GetByID(ctx, labelID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "label_not_found",
			"message": "Label not found",
		})
	}

	if label.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to delete this label",
		})
	}

	// System labels cannot be deleted
	if label.IsSystem {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "system_label",
			"message": "System labels cannot be deleted",
		})
	}

	// Remove label from all emails
	if err := h.emailRepo.RemoveLabelFromAll(ctx, labelID); err != nil {
		h.logger.Error("Failed to remove label from emails", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "cleanup_failed",
			"message": "Failed to remove label from emails",
		})
	}

	// Delete label
	if err := h.labelRepo.Delete(ctx, labelID); err != nil {
		h.logger.Error("Failed to delete label", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "delete_failed",
			"message": "Failed to delete label",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":  "Label deleted successfully",
		"label_id": labelID,
	})
}

// Helper functions
func isValidHexColor(color string) bool {
	if len(color) != 7 || color[0] != '#' {
		return false
	}
	for i := 1; i < 7; i++ {
		c := color[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func generateRandomColor() string {
	colors := []string{
		"#6d4aff", "#4a90e2", "#50e3c2", "#f5a623",
		"#7ed321", "#bd10e0", "#ff0080", "#b8e986",
		"#4a4a4a", "#9b9b9b",
	}
	return colors[rand.Intn(len(colors))]
}

// Additional label operations
func (h *LabelHandler) ApplyLabelToEmail(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	emailID, err := uuid.Parse(c.Param("emailId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_email_id",
			"message": "Invalid email ID",
		})
	}

	labelID, err := uuid.Parse(c.Param("labelId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_label_id",
			"message": "Invalid label ID",
		})
	}

	ctx := c.Request().Context()

	// Check email ownership
	email, err := h.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "email_not_found",
			"message": "Email not found",
		})
	}

	if email.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to modify this email",
		})
	}

	// Check label ownership
	label, err := h.labelRepo.GetByID(ctx, labelID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "label_not_found",
			"message": "Label not found",
		})
	}

	if label.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to use this label",
		})
	}

	// Apply label
	if err := h.emailRepo.AddLabel(ctx, emailID, labelID); err != nil {
		h.logger.Error("Failed to apply label", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "apply_failed",
			"message": "Failed to apply label to email",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":  "Label applied successfully",
		"email_id": emailID,
		"label_id": labelID,
	})
}

func (h *LabelHandler) RemoveLabelFromEmail(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	emailID, err := uuid.Parse(c.Param("emailId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_email_id",
			"message": "Invalid email ID",
		})
	}

	labelID, err := uuid.Parse(c.Param("labelId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_label_id",
			"message": "Invalid label ID",
		})
	}

	ctx := c.Request().Context()

	// Check email ownership
	email, err := h.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "email_not_found",
			"message": "Email not found",
		})
	}

	if email.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to modify this email",
		})
	}

	// Remove label
	if err := h.emailRepo.RemoveLabel(ctx, emailID, labelID); err != nil {
		h.logger.Error("Failed to remove label", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "remove_failed",
			"message": "Failed to remove label from email",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":  "Label removed successfully",
		"email_id": emailID,
		"label_id": labelID,
	})
}
