// internal/api/handlers/contact.go
package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
)

// ContactHandler handles contact operations
type ContactHandler struct {
	contactRepo *repository.ContactRepository
	logger      *zap.Logger
}

// NewContactHandler creates a new contact handler
func NewContactHandler(
	contactRepo *repository.ContactRepository,
	logger *zap.Logger,
) *ContactHandler {
	return &ContactHandler{
		contactRepo: contactRepo,
		logger:      logger,
	}
}

// ListContacts lists all contacts for a user
func (h *ContactHandler) ListContacts(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	// Parse query parameters
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	if offset < 0 {
		offset = 0
	}

	trustedOnly, _ := strconv.ParseBool(c.QueryParam("trusted_only"))
	recentOnly, _ := strconv.ParseBool(c.QueryParam("recent_only"))

	ctx := c.Request().Context()
	contacts, total, err := h.contactRepo.List(ctx, userID, limit, offset, trustedOnly, recentOnly)
	if err != nil {
		h.logger.Error("Failed to list contacts", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "list_failed",
			"message": "Failed to list contacts",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"contacts": contacts,
		"total":    total,
		"limit":    limit,
		"offset":   offset,
	})
}

// CreateContact creates a new contact
func (h *ContactHandler) CreateContact(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	var req struct {
		Name              string   `json:"name,omitempty"`
		EmailAddress      string   `json:"email_address" validate:"required"`
		PublicKeys        []string `json:"public_keys,omitempty"`
		NetworkIdentities string   `json:"network_identities,omitempty"`
		Notes             string   `json:"notes,omitempty"`
		IsTrusted         bool     `json:"is_trusted,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	// Validate email address
	if !isValidEmailAddress(req.EmailAddress) {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_email",
			"message": "Invalid email address",
		})
	}

	ctx := c.Request().Context()

	// Check if contact already exists
	existing, err := h.contactRepo.GetByEmail(ctx, userID, req.EmailAddress)
	if err == nil && existing != nil {
		return c.JSON(http.StatusConflict, map[string]interface{}{
			"error":   "contact_exists",
			"message": "Contact already exists",
		})
	}

	// Parse network identities JSON
	var networkIdentities models.JSONB
	if req.NetworkIdentities != "" {
		if err := json.Unmarshal([]byte(req.NetworkIdentities), &networkIdentities); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_network_identities",
				"message": "Invalid network identities JSON",
			})
		}
	}

	contact := &models.Contact{
		ID:                uuid.New(),
		UserID:            userID,
		Name:              req.Name,
		EmailAddress:      req.EmailAddress,
		PublicKeys:        models.StringArray(req.PublicKeys),
		NetworkIdentities: networkIdentities,
		Notes:             req.Notes,
		IsTrusted:         req.IsTrusted,
	}

	if err := h.contactRepo.Create(ctx, contact); err != nil {
		h.logger.Error("Failed to create contact", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "create_failed",
			"message": "Failed to create contact",
		})
	}

	return c.JSON(http.StatusCreated, contact)
}

// UpdateContact updates a contact
func (h *ContactHandler) UpdateContact(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	contactID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_contact_id",
			"message": "Invalid contact ID",
		})
	}

	var req struct {
		Name              string   `json:"name,omitempty"`
		EmailAddress      string   `json:"email_address,omitempty"`
		PublicKeys        []string `json:"public_keys,omitempty"`
		NetworkIdentities string   `json:"network_identities,omitempty"`
		Notes             string   `json:"notes,omitempty"`
		IsTrusted         *bool    `json:"is_trusted,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()

	// Get contact and check ownership
	contact, err := h.contactRepo.GetByID(ctx, contactID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "contact_not_found",
			"message": "Contact not found",
		})
	}

	if contact.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to update this contact",
		})
	}

	// Update fields
	updated := false
	if req.Name != "" && req.Name != contact.Name {
		contact.Name = req.Name
		updated = true
	}

	if req.EmailAddress != "" && req.EmailAddress != contact.EmailAddress {
		if !isValidEmailAddress(req.EmailAddress) {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_email",
				"message": "Invalid email address",
			})
		}
		// Check for duplicate email
		existing, err := h.contactRepo.GetByEmail(ctx, userID, req.EmailAddress)
		if err == nil && existing != nil && existing.ID != contactID {
			return c.JSON(http.StatusConflict, map[string]interface{}{
				"error":   "email_exists",
				"message": "Another contact with this email already exists",
			})
		}
		contact.EmailAddress = req.EmailAddress
		updated = true
	}

	if req.PublicKeys != nil {
		contact.PublicKeys = models.StringArray(req.PublicKeys)
		updated = true
	}

	if req.NetworkIdentities != "" {
		var networkIdentities models.JSONB
		if err := json.Unmarshal([]byte(req.NetworkIdentities), &networkIdentities); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_network_identities",
				"message": "Invalid network identities JSON",
			})
		}
		contact.NetworkIdentities = networkIdentities
		updated = true
	}

	if req.Notes != "" && req.Notes != contact.Notes {
		contact.Notes = req.Notes
		updated = true
	}

	if req.IsTrusted != nil && *req.IsTrusted != contact.IsTrusted {
		contact.IsTrusted = *req.IsTrusted
		updated = true
	}

	if !updated {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "No changes made",
		})
	}

	if err := h.contactRepo.Update(ctx, contact); err != nil {
		h.logger.Error("Failed to update contact", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "update_failed",
			"message": "Failed to update contact",
		})
	}

	return c.JSON(http.StatusOK, contact)
}

// DeleteContact deletes a contact
func (h *ContactHandler) DeleteContact(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	contactID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_contact_id",
			"message": "Invalid contact ID",
		})
	}

	ctx := c.Request().Context()

	// Get contact and check ownership
	contact, err := h.contactRepo.GetByID(ctx, contactID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "contact_not_found",
			"message": "Contact not found",
		})
	}

	if contact.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to delete this contact",
		})
	}

	if err := h.contactRepo.Delete(ctx, contactID); err != nil {
		h.logger.Error("Failed to delete contact", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "delete_failed",
			"message": "Failed to delete contact",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":    "Contact deleted successfully",
		"contact_id": contactID,
	})
}

// SearchContacts searches contacts by name or email
func (h *ContactHandler) SearchContacts(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	query := c.QueryParam("q")
	if query == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "missing_query",
			"message": "Search query is required",
		})
	}

	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	ctx := c.Request().Context()
	contacts, err := h.contactRepo.Search(ctx, userID, query, limit)
	if err != nil {
		h.logger.Error("Failed to search contacts", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "search_failed",
			"message": "Failed to search contacts",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"contacts": contacts,
		"count":    len(contacts),
		"query":    query,
	})
}

// ImportContacts imports contacts from various formats
func (h *ContactHandler) ImportContacts(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	format := c.QueryParam("format")
	if format == "" {
		format = "vcard"
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "no_file",
			"message": "No file uploaded",
		})
	}

	// Open file
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "file_error",
			"message": "Failed to open uploaded file",
		})
	}
	defer src.Close()

	ctx := c.Request().Context()
	stats, err := h.contactRepo.Import(ctx, userID, format, src)
	if err != nil {
		h.logger.Error("Failed to import contacts", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "import_failed",
			"message": err.Error(),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":    "Contacts imported successfully",
		"total":      stats.Total,
		"imported":   stats.Imported,
		"skipped":    stats.Skipped,
		"duplicates": stats.Duplicates,
		"failed":     stats.Failed,
	})
}

// ExportContacts exports contacts in specified format
func (h *ContactHandler) ExportContacts(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	format := c.QueryParam("format")
	if format == "" {
		format = "vcard"
	}

	ctx := c.Request().Context()
	data, err := h.contactRepo.Export(ctx, userID, format)
	if err != nil {
		h.logger.Error("Failed to export contacts", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "export_failed",
			"message": "Failed to export contacts",
		})
	}

	// Set appropriate content type
	var contentType, filename string
	switch format {
	case "vcard":
		contentType = "text/vcard"
		filename = "contacts.vcf"
	case "csv":
		contentType = "text/csv"
		filename = "contacts.csv"
	case "json":
		contentType = "application/json"
		filename = "contacts.json"
	default:
		contentType = "application/octet-stream"
		filename = "contacts.export"
	}

	c.Response().Header().Set("Content-Type", contentType)
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	return c.Blob(http.StatusOK, contentType, data)
}

// Helper function for email validation
func isValidEmailAddress(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	return len(parts[0]) > 0 && len(parts[1]) > 0
}
