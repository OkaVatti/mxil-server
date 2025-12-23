package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"gorm.io/gorm"
)

// FolderRepository handles folder database operations
type FolderRepository struct {
	db *gorm.DB
}

// NewFolderRepository creates a new folder repository
func NewFolderRepository(db *gorm.DB) *FolderRepository {
	return &FolderRepository{db: db}
}

// Create creates a new folder
func (r *FolderRepository) Create(ctx context.Context, folder *models.Folder) error {
	return r.db.WithContext(ctx).Create(folder).Error
}

// GetByID gets a folder by ID
func (r *FolderRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Folder, error) {
	var folder models.Folder
	if err := r.db.WithContext(ctx).
		First(&folder, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &folder, nil
}

// GetByPath gets a folder by path for a user
func (r *FolderRepository) GetByPath(ctx context.Context, userID uuid.UUID, path string) (*models.Folder, error) {
	var folder models.Folder
	if err := r.db.WithContext(ctx).
		Where("user_id = ? AND path = ?", userID, path).
		First(&folder).Error; err != nil {
		return nil, err
	}
	return &folder, nil
}

// Update updates a folder
func (r *FolderRepository) Update(ctx context.Context, folder *models.Folder) error {
	return r.db.WithContext(ctx).Save(folder).Error
}

// Delete deletes a folder
func (r *FolderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Delete(&models.Folder{}, "id = ?", id).Error
}

// ListByUser lists folders for a user
func (r *FolderRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Folder, error) {
	var folders []models.Folder
	if err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("path ASC").
		Find(&folders).Error; err != nil {
		return nil, err
	}
	return folders, nil
}

// GetSubfolders gets all subfolders of a folder
func (r *FolderRepository) GetSubfolders(ctx context.Context, folderID uuid.UUID) ([]models.Folder, error) {
	var folders []models.Folder
	if err := r.db.WithContext(ctx).
		Where("parent_id = ?", folderID).
		Order("name ASC").
		Find(&folders).Error; err != nil {
		return nil, err
	}
	return folders, nil
}

// GetAncestors gets all ancestors of a folder
func (r *FolderRepository) GetAncestors(ctx context.Context, folderID uuid.UUID) ([]models.Folder, error) {
	var ancestors []models.Folder
	currentID := folderID

	for {
		var folder models.Folder
		if err := r.db.WithContext(ctx).
			First(&folder, "id = ?", currentID).Error; err != nil {
			break
		}

		if folder.ParentID == nil || *folder.ParentID == uuid.Nil {
			break
		}

		var parent models.Folder
		if err := r.db.WithContext(ctx).
			First(&parent, "id = ?", *folder.ParentID).Error; err != nil {
			break
		}

		ancestors = append([]models.Folder{parent}, ancestors...)
		currentID = parent.ID
	}

	return ancestors, nil
}

// BuildPath builds the full path for a folder
func (r *FolderRepository) BuildPath(ctx context.Context, folderID uuid.UUID) (string, error) {
	ancestors, err := r.GetAncestors(ctx, folderID)
	if err != nil {
		return "", err
	}

	var folder models.Folder
	if err := r.db.WithContext(ctx).
		First(&folder, "id = ?", folderID).Error; err != nil {
		return "", err
	}

	pathParts := []string{}
	for _, ancestor := range ancestors {
		pathParts = append(pathParts, ancestor.Name)
	}
	pathParts = append(pathParts, folder.Name)

	return strings.Join(pathParts, "/"), nil
}

// ValidateParent validates that a folder can be moved to a new parent
func (r *FolderRepository) ValidateParent(ctx context.Context, folderID, newParentID uuid.UUID) error {
	if folderID == newParentID {
		return fmt.Errorf("folder cannot be its own parent")
	}

	// Check for circular reference
	ancestors, err := r.GetAncestors(ctx, newParentID)
	if err != nil {
		return err
	}

	for _, ancestor := range ancestors {
		if ancestor.ID == folderID {
			return fmt.Errorf("circular reference detected")
		}
	}

	return nil
}
