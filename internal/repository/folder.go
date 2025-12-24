// internal/repository/folder_repository.go
package repository

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// FolderRepository handles folder database operations
type FolderRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewFolderRepository creates a new folder repository
func NewFolderRepository(db *sqlx.DB, logger *zap.Logger) *FolderRepository {
	return &FolderRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new folder
func (r *FolderRepository) Create(ctx context.Context, folder *models.Folder) error {
	query := `
		INSERT INTO folders (
			id, user_id, parent_id, name, path, is_system, email_count,
			unread_count, created_at, updated_at
		) VALUES (
			:id, :user_id, :parent_id, :name, :path, :is_system, :email_count,
			:unread_count, :created_at, :updated_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, folder)
	return err
}

// GetByID gets a folder by ID
func (r *FolderRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Folder, error) {
	var folder models.Folder
	query := `SELECT * FROM folders WHERE id = $1`
	err := r.db.GetContext(ctx, &folder, query, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}
	return &folder, nil
}

// GetByPath gets a folder by user ID and path
func (r *FolderRepository) GetByPath(ctx context.Context, userID uuid.UUID, path string) (*models.Folder, error) {
	var folder models.Folder
	query := `SELECT * FROM folders WHERE user_id = $1 AND path = $2`
	err := r.db.GetContext(ctx, &folder, query, userID, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get folder: %w", err)
	}
	return &folder, nil
}

// Update updates a folder
func (r *FolderRepository) Update(ctx context.Context, folder *models.Folder) error {
	query := `
		UPDATE folders SET
			name = :name,
			path = :path,
			parent_id = :parent_id,
			email_count = :email_count,
			unread_count = :unread_count,
			updated_at = :updated_at
		WHERE id = :id
	`

	_, err := r.db.NamedExecContext(ctx, query, folder)
	return err
}

// Delete deletes a folder (only if not system folder)
func (r *FolderRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM folders WHERE id = $1 AND is_system = false`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// ListByUser lists folders for a user
func (r *FolderRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]models.Folder, error) {
	var folders []models.Folder
	query := `SELECT * FROM folders WHERE user_id = $1 ORDER BY is_system DESC, name ASC`
	err := r.db.SelectContext(ctx, &folders, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list folders: %w", err)
	}
	return folders, nil
}

// ListSystemFolders lists system folders for a user
func (r *FolderRepository) ListSystemFolders(ctx context.Context, userID uuid.UUID) ([]models.Folder, error) {
	var folders []models.Folder
	query := `SELECT * FROM folders WHERE user_id = $1 AND is_system = true ORDER BY name ASC`
	err := r.db.SelectContext(ctx, &folders, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list system folders: %w", err)
	}
	return folders, nil
}

// UpdateCounts updates email and unread counts for a folder
func (r *FolderRepository) UpdateCounts(ctx context.Context, folderID uuid.UUID) error {
	// Update email count
	emailCountQuery := `
		UPDATE folders SET 
			email_count = (SELECT COUNT(*) FROM emails WHERE folder_id = $1 AND deleted_at IS NULL),
			unread_count = (SELECT COUNT(*) FROM emails WHERE folder_id = $1 AND is_read = false AND deleted_at IS NULL),
			updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, emailCountQuery, folderID)
	return err
}

// GetByParentID gets folders by parent ID
func (r *FolderRepository) GetByParentID(ctx context.Context, parentID uuid.UUID) ([]models.Folder, error) {
	var folders []models.Folder
	query := `SELECT * FROM folders WHERE parent_id = $1 ORDER BY name ASC`
	err := r.db.SelectContext(ctx, &folders, query, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get folders by parent: %w", err)
	}
	return folders, nil
}

// CheckPathExists checks if a folder path already exists for user
func (r *FolderRepository) CheckPathExists(ctx context.Context, userID uuid.UUID, path string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM folders WHERE user_id = $1 AND path = $2)`
	err := r.db.GetContext(ctx, &exists, query, userID, path)
	if err != nil {
		return false, fmt.Errorf("failed to check path exists: %w", err)
	}
	return exists, nil
}

// Rename renames a folder
func (r *FolderRepository) Rename(ctx context.Context, folderID uuid.UUID, newName, newPath string) error {
	query := `UPDATE folders SET name = $1, path = $2, updated_at = NOW() WHERE id = $3`
	_, err := r.db.ExecContext(ctx, query, newName, newPath, folderID)
	return err
}

// GetInbox gets the inbox folder for a user
func (r *FolderRepository) GetInbox(ctx context.Context, userID uuid.UUID) (*models.Folder, error) {
	var folder models.Folder
	query := `SELECT * FROM folders WHERE user_id = $1 AND name = 'Inbox' AND is_system = true`
	err := r.db.GetContext(ctx, &folder, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get inbox: %w", err)
	}
	return &folder, nil
}

// GetSent gets the sent folder for a user
func (r *FolderRepository) GetSent(ctx context.Context, userID uuid.UUID) (*models.Folder, error) {
	var folder models.Folder
	query := `SELECT * FROM folders WHERE user_id = $1 AND name = 'Sent' AND is_system = true`
	err := r.db.GetContext(ctx, &folder, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get sent: %w", err)
	}
	return &folder, nil
}

// GetDrafts gets the drafts folder for a user
func (r *FolderRepository) GetDrafts(ctx context.Context, userID uuid.UUID) (*models.Folder, error) {
	var folder models.Folder
	query := `SELECT * FROM folders WHERE user_id = $1 AND name = 'Drafts' AND is_system = true`
	err := r.db.GetContext(ctx, &folder, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get drafts: %w", err)
	}
	return &folder, nil
}

// GetTrash gets the trash folder for a user
func (r *FolderRepository) GetTrash(ctx context.Context, userID uuid.UUID) (*models.Folder, error) {
	var folder models.Folder
	query := `SELECT * FROM folders WHERE user_id = $1 AND name = 'Trash' AND is_system = true`
	err := r.db.GetContext(ctx, &folder, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get trash: %w", err)
	}
	return &folder, nil
}

// GetSpam gets the spam folder for a user
func (r *FolderRepository) GetSpam(ctx context.Context, userID uuid.UUID) (*models.Folder, error) {
	var folder models.Folder
	query := `SELECT * FROM folders WHERE user_id = $1 AND name = 'Spam' AND is_system = true`
	err := r.db.GetContext(ctx, &folder, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get spam: %w", err)
	}
	return &folder, nil
}

// GetArchive gets the archive folder for a user
func (r *FolderRepository) GetArchive(ctx context.Context, userID uuid.UUID) (*models.Folder, error) {
	var folder models.Folder
	query := `SELECT * FROM folders WHERE user_id = $1 AND name = 'Archive' AND is_system = true`
	err := r.db.GetContext(ctx, &folder, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get archive: %w", err)
	}
	return &folder, nil
}
