// internal/api/handlers/folder.go
package handlers

import (
	"context"
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

// FolderHandler handles folder operations
type FolderHandler struct {
	folderRepo *repository.FolderRepository
	emailRepo  *repository.EmailRepository
	logger     *zap.Logger
}

// NewFolderHandler creates a new folder handler
func NewFolderHandler(
	folderRepo *repository.FolderRepository,
	emailRepo *repository.EmailRepository,
	logger *zap.Logger,
) *FolderHandler {
	return &FolderHandler{
		folderRepo: folderRepo,
		emailRepo:  emailRepo,
		logger:     logger,
	}
}

// ListFolders lists all folders for a user
func (h *FolderHandler) ListFolders(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	folders, err := h.folderRepo.ListByUser(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to list folders", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "list_failed",
			"message": "Failed to list folders",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"folders": folders,
		"count":   len(folders),
	})
}

// CreateFolder creates a new folder
func (h *FolderHandler) CreateFolder(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	var req struct {
		Name     string    `json:"name" validate:"required"`
		ParentID uuid.UUID `json:"parent_id,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	// Validate folder name
	if strings.TrimSpace(req.Name) == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_name",
			"message": "Folder name cannot be empty",
		})
	}

	ctx := c.Request().Context()

	// Build folder path
	path := req.Name
	if req.ParentID != uuid.Nil {
		// Get parent folder to build path
		parent, err := h.folderRepo.GetByID(ctx, req.ParentID)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "parent_not_found",
				"message": "Parent folder not found",
			})
		}
		path = parent.Path + "/" + req.Name
	}

	// Check if folder already exists
	existing, err := h.folderRepo.GetByPath(ctx, userID, path)
	if err == nil && existing != nil {
		return c.JSON(http.StatusConflict, map[string]interface{}{
			"error":   "folder_exists",
			"message": "Folder already exists",
		})
	}

	folder := &models.Folder{
		ID:       uuid.New(),
		UserID:   userID,
		ParentID: &req.ParentID,
		Name:     req.Name,
		Path:     path,
		IsSystem: false,
	}

	if err := h.folderRepo.Create(ctx, folder); err != nil {
		h.logger.Error("Failed to create folder", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "create_failed",
			"message": "Failed to create folder",
		})
	}

	return c.JSON(http.StatusCreated, folder)
}

// UpdateFolder updates a folder
func (h *FolderHandler) UpdateFolder(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_folder_id",
			"message": "Invalid folder ID",
		})
	}

	var req struct {
		Name     string    `json:"name,omitempty"`
		ParentID uuid.UUID `json:"parent_id,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()

	// Get folder and check ownership
	folder, err := h.folderRepo.GetByID(ctx, folderID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "folder_not_found",
			"message": "Folder not found",
		})
	}

	if folder.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to update this folder",
		})
	}

	// System folders cannot be renamed
	if folder.IsSystem && req.Name != "" && req.Name != folder.Name {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "system_folder",
			"message": "System folders cannot be renamed",
		})
	}

	// Update fields
	updated := false
	if req.Name != "" && req.Name != folder.Name {
		folder.Name = req.Name
		updated = true
	}

	if req.ParentID != uuid.Nil && req.ParentID != *folder.ParentID {
		// Check if parent exists and is not a descendant
		if err := h.validateParentFolder(ctx, folderID, req.ParentID); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_parent",
				"message": err.Error(),
			})
		}
		folder.ParentID = &req.ParentID
		updated = true
	}

	if !updated {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "No changes made",
		})
	}

	// Recalculate path if name or parent changed
	if updated {
		path := folder.Name
		if folder.ParentID != nil && *folder.ParentID != uuid.Nil {
			parent, err := h.folderRepo.GetByID(ctx, *folder.ParentID)
			if err != nil {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"error":   "parent_not_found",
					"message": "Parent folder not found",
				})
			}
			path = parent.Path + "/" + folder.Name
		}
		folder.Path = path
	}

	if err := h.folderRepo.Update(ctx, folder); err != nil {
		h.logger.Error("Failed to update folder", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "update_failed",
			"message": "Failed to update folder",
		})
	}

	return c.JSON(http.StatusOK, folder)
}

// DeleteFolder deletes a folder
func (h *FolderHandler) DeleteFolder(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_folder_id",
			"message": "Invalid folder ID",
		})
	}

	ctx := c.Request().Context()

	// Get folder and check ownership
	folder, err := h.folderRepo.GetByID(ctx, folderID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "folder_not_found",
			"message": "Folder not found",
		})
	}

	if folder.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to delete this folder",
		})
	}

	// System folders cannot be deleted
	if folder.IsSystem {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "system_folder",
			"message": "System folders cannot be deleted",
		})
	}

	// Check if folder has subfolders
	subfolders, err := h.folderRepo.GetSubfolders(ctx, folderID)
	if err != nil {
		h.logger.Error("Failed to check subfolders", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "check_failed",
			"message": "Failed to check subfolders",
		})
	}

	if len(subfolders) > 0 {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "has_subfolders",
			"message": "Cannot delete folder with subfolders",
		})
	}

	// Move emails to parent folder or default
	if folder.ParentID != nil && *folder.ParentID != uuid.Nil {
		if err := h.emailRepo.MoveAllToFolder(ctx, folderID, *folder.ParentID); err != nil {
			h.logger.Error("Failed to move emails", zap.Error(err))
		}
	}

	// Delete folder
	if err := h.folderRepo.Delete(ctx, folderID); err != nil {
		h.logger.Error("Failed to delete folder", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "delete_failed",
			"message": "Failed to delete folder",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":   "Folder deleted successfully",
		"folder_id": folderID,
	})
}

// GetFolderEmails gets emails in a folder
func (h *FolderHandler) GetFolderEmails(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	folderID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_folder_id",
			"message": "Invalid folder ID",
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

	ctx := c.Request().Context()

	// Check folder ownership
	folder, err := h.folderRepo.GetByID(ctx, folderID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "folder_not_found",
			"message": "Folder not found",
		})
	}

	if folder.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to access this folder",
		})
	}

	// Get emails in folder
	filter := repository.EmailFilter{
		UserID:   userID,
		FolderID: &folderID,
		Limit:    limit,
		Offset:   offset,
	}

	emails, total, err := h.emailRepo.List(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to get folder emails", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "list_failed",
			"message": "Failed to get folder emails",
		})
	}

	// Get unread count for this folder
	unreadFilter := repository.EmailFilter{
		UserID:   userID,
		FolderID: &folderID,
		IsRead:   &[]bool{false}[0],
	}
	_, unreadCount, err := h.emailRepo.List(ctx, unreadFilter)
	if err != nil {
		unreadCount = 0
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"folder":       folder,
		"emails":       emails,
		"total":        total,
		"unread_count": unreadCount,
		"limit":        limit,
		"offset":       offset,
	})
}

// Helper methods
func (h *FolderHandler) validateParentFolder(ctx context.Context, folderID, parentID uuid.UUID) error {
	// Check if parent exists
	_, err := h.folderRepo.GetByID(ctx, parentID)
	if err != nil {
		return fmt.Errorf("parent folder not found")
	}

	// Check for circular reference
	if folderID == parentID {
		return fmt.Errorf("folder cannot be its own parent")
	}

	// Check if parent is a descendant of this folder
	ancestors, err := h.folderRepo.GetAncestors(ctx, parentID)
	if err != nil {
		return fmt.Errorf("failed to validate folder hierarchy")
	}

	for _, ancestor := range ancestors {
		if ancestor.ID == folderID {
			return fmt.Errorf("cannot move folder into its own descendant")
		}
	}

	return nil
}
