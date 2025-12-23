// internal/repository/repositories.go
package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
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

type UserRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
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

type SessionRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
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

type EmailRepository struct {
	db     *sqlx.DB
	logger *zap.Logger
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
