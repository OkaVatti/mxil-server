package email

import (
	"context"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/storage"
)

// EmailService handles email operations
type EmailService interface {
	SendEmail(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error)
	SearchEmails(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.Email, int64, error)
	GetEmailThread(ctx context.Context, userID, threadID uuid.UUID) ([]models.Email, error)
	ProcessIncomingEmail(ctx context.Context, rawEmail []byte, network models.NetworkType) error
	ValidateEmailAddress(email string) (bool, error)
	CleanupOldEmails(ctx context.Context, olderThan time.Duration) (int64, error)
	GetEmailStats(ctx context.Context, userID uuid.UUID) (*EmailStats, error)
}

// Request/Response structures
type SendEmailRequest struct {
	To           []string
	Cc           []string
	Bcc          []string
	Subject      string
	BodyPlain    string
	BodyHTML     string
	BodyMarkdown string
	Network      models.NetworkType
	Attachments  []AttachmentInfo
	Encryption   EncryptionConfig
	InReplyTo    string
	References   []string
	DraftID      *uuid.UUID
}

type AttachmentInfo struct {
	Filename    string
	ContentType string
	Size        int64
	Data        string // base64 encoded
	IsInline    bool
}

type EncryptionConfig struct {
	Algorithm string
	KeyIDs    []string
	Sign      bool
}

type EmailStats struct {
	TotalEmails     int64
	UnreadCount     int64
	SentCount       int64
	ReceivedCount   int64
	TotalSize       int64
	EmailsByNetwork map[models.NetworkType]int64
	EmailsByDay     map[string]int64
}

// Service dependencies
type ServiceDependencies struct {
	EmailRepo      *repository.EmailRepository
	UserRepo       *repository.UserRepository
	StorageService storage.StorageService
	Logger         *zap.Logger
	MaxEmailSize   int64
	MaxAttachments int
}
