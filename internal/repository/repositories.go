// internal/repository/repositories.go
package repository

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

type Repositories struct {
	User    *UserRepository
	Session *SessionRepository
	Email   *EmailRepository
	// Add other repositories as needed
}

func NewRepositories(db *sqlx.DB, logger *zap.Logger) *Repositories {
	return &Repositories{
		User:    NewUserRepository(db, logger),
		Session: NewSessionRepository(db, logger),
		Email:   NewEmailRepository(db, logger),
	}
}

// User model
type User struct {
	ID                uuid.UUID `db:"id"`
	MasterUsername    string    `db:"master_username"`
	Email             string    `db:"email"`
	DisplayName       string    `db:"display_name"`
	PasswordHash      string    `db:"password_hash"`
	MFAEnabled        bool      `db:"mfa_enabled"`
	MFASecret         string    `db:"mfa_secret"`
	SecurityScore     int       `db:"security_score"`
	StorageQuotaUsed  int64     `db:"storage_quota_used"`
	StorageQuotaTotal int64     `db:"storage_quota_total"`
	IsActive          bool      `db:"is_active"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}

// Session model
type Session struct {
	ID                uuid.UUID `db:"id"`
	UserID            uuid.UUID `db:"user_id"`
	TokenHash         string    `db:"token_hash"`
	UserAgent         string    `db:"user_agent"`
	IPAddress         string    `db:"ip_address"`
	DeviceFingerprint string    `db:"device_fingerprint"`
	DeviceName        string    `db:"device_name"`
	LastActivity      time.Time `db:"last_activity"`
	ExpiresAt         time.Time `db:"expires_at"`
	CreatedAt         time.Time `db:"created_at"`
}

// Email model
type Email struct {
	ID             uuid.UUID `db:"id"`
	UserID         uuid.UUID `db:"user_id"`
	ThreadID       uuid.UUID `db:"thread_id"`
	FromAddress    string    `db:"from_address"`
	ToAddresses    []string  `db:"to_addresses"`
	Subject        string    `db:"subject"`
	BodyPlain      string    `db:"body_plain"`
	Network        string    `db:"network"`
	FolderID       uuid.UUID `db:"folder_id"`
	IsRead         bool      `db:"is_read"`
	IsStarred      bool      `db:"is_starred"`
	HasAttachments bool      `db:"has_attachments"`
	SizeBytes      int64     `db:"size_bytes"`
	ReceivedAt     time.Time `db:"received_at"`
	CreatedAt      time.Time `db:"created_at"`
}

// UserRepository interface
type UserRepository interface {
	Create(ctx context.Context, user *models.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByUsernameOrEmail(ctx context.Context, username, email string) (*models.User, error)
	GetByVerificationToken(ctx context.Context, token string) (*models.User, error)
	GetByResetToken(ctx context.Context, token string) (*models.User, error)
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	Count(ctx context.Context) (int, error)
	ListWithFilter(ctx context.Context, limit, offset int, search string, activeOnly, verifiedOnly bool) ([]*models.User, int64, error)
	GetStorageUsage(ctx context.Context, userID uuid.UUID) (used, total int64, err error)
}

// SessionRepository interface
type SessionRepository interface {
	Create(ctx context.Context, session *models.Session) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Session, error)
	GetByTokenHash(ctx context.Context, tokenHash string) (*models.Session, error)
	GetActiveSessionsByUser(ctx context.Context, userID uuid.UUID) ([]models.Session, error)
	Update(ctx context.Context, session *models.Session) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteExpired(ctx context.Context, before time.Time) (int64, error)
}

// EmailRepository interface
type EmailRepository interface {
	Create(ctx context.Context, email *models.Email) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Email, error)
	GetByMessageID(ctx context.Context, messageID string) (*models.Email, error)
	GetByThreadID(ctx context.Context, userID, threadID uuid.UUID) ([]models.Email, error)
	List(ctx context.Context, filter EmailFilter) ([]models.Email, int64, error)
	Search(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.Email, int64, error)
	Update(ctx context.Context, email *models.Email) error
	Delete(ctx context.Context, id uuid.UUID) error
	MarkAsRead(ctx context.Context, id uuid.UUID, read bool) error
	MarkAsStarred(ctx context.Context, id uuid.UUID, starred bool) error
	MoveToFolder(ctx context.Context, id, folderID uuid.UUID) error
	MoveAllToFolder(ctx context.Context, fromFolderID, toFolderID uuid.UUID) error
	CountByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountByLabel(ctx context.Context, labelID uuid.UUID) (int64, error)
	CountUnread(ctx context.Context, userID uuid.UUID) (int64, error)
	DeleteOld(ctx context.Context, olderThan time.Time) (int64, error)
	GetOlderThan(ctx context.Context, cutoff time.Time) ([]models.Email, error)
	AddLabel(ctx context.Context, emailID, labelID uuid.UUID) error
	RemoveLabel(ctx context.Context, emailID, labelID uuid.UUID) error
	RemoveLabelFromAll(ctx context.Context, labelID uuid.UUID) error
}

// EmailFilter for querying emails
type EmailFilter struct {
	UserID     uuid.UUID
	FolderID   *uuid.UUID
	LabelIDs   []uuid.UUID
	IsRead     *bool
	IsStarred  *bool
	IsArchived *bool
	IsSpam     *bool
	Network    *models.NetworkType
	FromDate   *time.Time
	ToDate     *time.Time
	Search     string
	Limit      int
	Offset     int
	SortBy     string
	SortDesc   bool
}

// FolderRepository interface
type FolderRepository interface {
	Create(ctx context.Context, folder *models.Folder) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Folder, error)
	GetByPath(ctx context.Context, userID uuid.UUID, path string) (*models.Folder, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.Folder, error)
	GetSubfolders(ctx context.Context, parentID uuid.UUID) ([]*models.Folder, error)
	GetAncestors(ctx context.Context, folderID uuid.UUID) ([]*models.Folder, error)
	Update(ctx context.Context, folder *models.Folder) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// LabelRepository interface
type LabelRepository interface {
	Create(ctx context.Context, label *models.Label) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Label, error)
	GetByName(ctx context.Context, userID uuid.UUID, name string) (*models.Label, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.Label, error)
	Update(ctx context.Context, label *models.Label) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// ContactRepository interface
type ContactRepository interface {
	Create(ctx context.Context, contact *models.Contact) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Contact, error)
	GetByEmail(ctx context.Context, userID uuid.UUID, email string) (*models.Contact, error)
	List(ctx context.Context, userID uuid.UUID, limit, offset int, trustedOnly, recentOnly bool) ([]*models.Contact, int64, error)
	Search(ctx context.Context, userID uuid.UUID, query string, limit int) ([]*models.Contact, error)
	Update(ctx context.Context, contact *models.Contact) error
	Delete(ctx context.Context, id uuid.UUID) error
	Import(ctx context.Context, userID uuid.UUID, format string, data io.Reader) (*ImportStats, error)
	Export(ctx context.Context, userID uuid.UUID, format string) ([]byte, error)
}

// ImportStats for contact import
type ImportStats struct {
	Total      int
	Imported   int
	Skipped    int
	Duplicates int
	Failed     int
}

// NetworkIdentityRepository interface
type NetworkIdentityRepository interface {
	Create(ctx context.Context, identity *models.NetworkIdentity) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.NetworkIdentity, error)
	GetByAddress(ctx context.Context, address string) (*models.NetworkIdentity, error)
	GetByKeyID(ctx context.Context, keyID string) (*models.NetworkIdentity, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.NetworkIdentity, error)
	GetPrimaryKey(ctx context.Context, userID uuid.UUID) (*models.NetworkIdentity, error)
	ClearPrimary(ctx context.Context, userID uuid.UUID, network models.NetworkType) error
	CountByUser(ctx context.Context, userID uuid.UUID) (int64, error)
	CountByNetwork(ctx context.Context) (map[models.NetworkType]int64, error)
	Update(ctx context.Context, identity *models.NetworkIdentity) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// EncryptionKeyRepository interface
type EncryptionKeyRepository interface {
	Create(ctx context.Context, key *models.EncryptionKey) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.EncryptionKey, error)
	GetByKeyID(ctx context.Context, keyID string) (*models.EncryptionKey, error)
	GetPrimaryKey(ctx context.Context, userID uuid.UUID) (*models.EncryptionKey, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.EncryptionKey, error)
	CountActive(ctx context.Context, userID uuid.UUID) (int64, error)
	ClearPrimary(ctx context.Context, userID uuid.UUID) error
	GetKeyUsage(ctx context.Context, keyID uuid.UUID) (*KeyUsage, error)
	Update(ctx context.Context, key *models.EncryptionKey) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// KeyUsage statistics
type KeyUsage struct {
	TotalEncrypted int64
	LastUsed       *time.Time
	FirstUsed      *time.Time
}

// ProviderBridgeRepository interface
type ProviderBridgeRepository interface {
	Create(ctx context.Context, bridge *models.ProviderBridge) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.ProviderBridge, error)
	GetByAccount(ctx context.Context, userID uuid.UUID, providerType models.ProviderType, accountID string) (*models.ProviderBridge, error)
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.ProviderBridge, error)
	GetSyncStats(ctx context.Context, bridgeID uuid.UUID) (*SyncStats, error)
	Update(ctx context.Context, bridge *models.ProviderBridge) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// SyncStats for provider sync
type SyncStats struct {
	TotalEmails  int64
	LastSync     *time.Time
	SyncDuration time.Duration
	SuccessRate  float64
	ErrorCount   int64
}

// StatsRepository interface
type StatsRepository interface {
	GetUserStats(ctx context.Context) (*UserStats, error)
	GetEmailStats(ctx context.Context, startDate time.Time) (*EmailStats, error)
	GetNetworkStats(ctx context.Context) (*NetworkStats, error)
	GetStorageStats(ctx context.Context) (*StorageStats, error)
	GetRecentActivity(ctx context.Context, limit int) ([]ActivityLog, error)
}

// Stats types
type UserStats struct {
	TotalUsers        int64
	ActiveUsers       int64
	NewUsersToday     int64
	NewUsersLast7Days int64
	UsersByTimezone   map[string]int64
}

type EmailStats struct {
	TotalEmails     int64
	SentToday       int64
	ReceivedToday   int64
	EmailsLast7Days int64
	EmailsByNetwork map[models.NetworkType]int64
	EmailsByHour    map[int]int64
}

type NetworkStats struct {
	TotalIdentities     int64
	IdentitiesByNetwork map[models.NetworkType]int64
	NetworkStatus       map[models.NetworkType]string
}

type StorageStats struct {
	TotalStorage     int64
	UsedStorage      int64
	AvailableStorage int64
	GrowthLast7Days  int64
	StorageByUser    map[uuid.UUID]int64
}

type ActivityLog struct {
	Timestamp time.Time
	UserID    uuid.UUID
	Action    string
	Resource  string
	Details   string
}

// PasswordResetRepository interface
type PasswordResetRepository interface {
	Create(ctx context.Context, reset *PasswordReset) error
	GetByToken(ctx context.Context, token string) (*PasswordReset, error)
	DeleteByUserID(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type PasswordReset struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Token     string
	ExpiresAt time.Time
	CreatedAt time.Time
	UsedAt    *time.Time
}

func NewUserRepository(db *sqlx.DB, logger *zap.Logger) *UserRepository {
	return &UserRepository{db: db, logger: logger}
}

func (r *UserRepository) Create(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (
			id, master_username, email, display_name, password_hash,
			storage_quota_total, is_active, created_at, updated_at
		) VALUES (
			:id, :master_username, :email, :display_name, :password_hash,
			:storage_quota_total, :is_active, :created_at, :updated_at
		)
	`

	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	_, err := r.db.NamedExecContext(ctx, query, user)
	return err
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `SELECT * FROM users WHERE id = $1 AND deleted_at IS NULL`
	user := &User{}
	err := r.db.GetContext(ctx, user, query, id)
	return user, err
}

func (r *UserRepository) GetByUsername(ctx context.Context, username string) (*User, error) {
	query := `SELECT * FROM users WHERE master_username = $1 AND deleted_at IS NULL`
	user := &User{}
	err := r.db.GetContext(ctx, user, query, username)
	return user, err
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT * FROM users WHERE email = $1 AND deleted_at IS NULL`
	user := &User{}
	err := r.db.GetContext(ctx, user, query, email)
	return user, err
}

func (r *UserRepository) Update(ctx context.Context, user *User) error {
	query := `
		UPDATE users SET
			display_name = :display_name,
			password_hash = :password_hash,
			mfa_enabled = :mfa_enabled,
			storage_quota_used = :storage_quota_used,
			updated_at = :updated_at
		WHERE id = :id
	`

	user.UpdatedAt = time.Now()
	_, err := r.db.NamedExecContext(ctx, query, user)
	return err
}

func (r *UserRepository) Count(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM users WHERE deleted_at IS NULL`
	err := r.db.GetContext(ctx, &count, query)
	return count, err
}

func NewSessionRepository(db *sqlx.DB, logger *zap.Logger) *SessionRepository {
	return &SessionRepository{db: db, logger: logger}
}

func (r *SessionRepository) Create(ctx context.Context, session *Session) error {
	query := `
		INSERT INTO sessions (
			id, user_id, token_hash, user_agent, ip_address,
			device_fingerprint, device_name, expires_at, created_at
		) VALUES (
			:id, :user_id, :token_hash, :user_agent, :ip_address,
			:device_fingerprint, :device_name, :expires_at, :created_at
		)
	`

	_, err := r.db.NamedExecContext(ctx, query, session)
	return err
}

func (r *SessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*Session, error) {
	query := `SELECT * FROM sessions WHERE token_hash = $1`
	session := &Session{}
	err := r.db.GetContext(ctx, session, query, tokenHash)
	return session, err
}

func (r *SessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM sessions WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func NewEmailRepository(db *sqlx.DB, logger *zap.Logger) *EmailRepository {
	return &EmailRepository{db: db, logger: logger}
}

func (r *EmailRepository) Create(ctx context.Context, email *Email) error {
	query := `
		INSERT INTO emails (
			id, user_id, thread_id, from_address, to_addresses,
			subject, body_plain, network, folder_id, is_read,
			is_starred, has_attachments, size_bytes, received_at, created_at
		) VALUES (
			:id, :user_id, :thread_id, :from_address, :to_addresses,
			:subject, :body_plain, :network, :folder_id, :is_read,
			:is_starred, :has_attachments, :size_bytes, :received_at, :created_at
		)
	`

	email.CreatedAt = time.Now()
	if email.ReceivedAt.IsZero() {
		email.ReceivedAt = time.Now()
	}

	_, err := r.db.NamedExecContext(ctx, query, email)
	return err
}

func (r *EmailRepository) GetByID(ctx context.Context, id uuid.UUID) (*Email, error) {
	query := `SELECT * FROM emails WHERE id = $1`
	email := &Email{}
	err := r.db.GetContext(ctx, email, query, id)
	return email, err
}

func (r *EmailRepository) List(ctx context.Context, userID uuid.UUID, limit, offset int) ([]Email, error) {
	query := `
		SELECT * FROM emails 
		WHERE user_id = $1 
		ORDER BY received_at DESC 
		LIMIT $2 OFFSET $3
	`

	var emails []Email
	err := r.db.SelectContext(ctx, &emails, query, userID, limit, offset)
	return emails, err
}

func (r *EmailRepository) MarkAsRead(ctx context.Context, id uuid.UUID, read bool) error {
	query := `UPDATE emails SET is_read = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, read, id)
	return err
}

func (r *EmailRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM emails WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
