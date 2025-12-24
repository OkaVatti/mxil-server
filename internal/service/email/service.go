package email

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/email"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service"
	"github.com/okavatti/mxil-server/m/internal/service/crypto"
	"github.com/okavatti/mxil-server/m/internal/service/network"
)

// NewEmailService creates a new email service
func NewEmailService(
	emailRepo *repository.EmailRepository,
	userRepo *repository.UserRepository,
	attachmentRepo *repository.AttachmentRepository,
	emailParser *email.EmailParser,
	emailPipeline *email.EmailPipeline,
	storageService StorageService,
	networkService service.NetworkService,
	cryptoService service.CryptoService,
	logger *zap.Logger,
) (EmailService, error) {
	return &emailServiceImpl{
		attachmentRepo: attachmentRepo,
		emailRepo:      emailRepo,
		userRepo:       userRepo,
		attachmentRepo: attachmentRepo,
		emailParser:    emailParser,
		emailPipeline:  emailPipeline,
		storageService: storageService,
		networkService: networkService,
		cryptoService:  cryptoService,
		logger:         logger,
	}, nil
}

type emailServiceImpl struct {
	emailRepo      *repository.EmailRepository
	userRepo       *repository.UserRepository
	attachmentRepo *repository.AttachmentRepository
	emailParser    *email.EmailParser
	emailPipeline  *email.EmailPipeline
	storageService StorageService
	networkService network.NetworkService
	cryptoService  crypto.CryptoService
	logger         *zap.Logger
}

func (s *emailServiceImpl) SendEmail(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error) {
	// Create email model
	email := &models.Email{
		ID:           uuid.New(),
		UserID:       userID,
		ThreadID:     uuid.New(),
		From:         "", // Will be set from user's email
		To:           models.StringArray(req.To),
		Cc:           models.StringArray(req.Cc),
		Bcc:          models.StringArray(req.Bcc),
		Subject:      req.Subject,
		BodyPlain:    req.BodyPlain,
		BodyHTML:     req.BodyHTML,
		BodyMarkdown: req.BodyMarkdown,
		Network:      req.Network,
		IsRead:       true, // Sent emails are marked as read
		IsStarred:    false,
		IsArchived:   false,
		IsSpam:       false,
		IsEncrypted:  req.Encryption.Algorithm != "",
		ReceivedAt:   time.Now(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Get user to set From address
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	email.From = user.Email

	// Process attachments
	if len(req.Attachments) > 0 {
		attachments := make(map[string]interface{})
		for _, attachment := range req.Attachments {
			// Upload attachment to storage
			path, err := s.storageService.UploadFile(ctx, []byte(attachment.Data), attachment.Filename, attachment.ContentType)
			if err != nil {
				return nil, fmt.Errorf("failed to upload attachment: %w", err)
			}

			attachments[attachment.Filename] = map[string]interface{}{
				"content_type": attachment.ContentType,
				"size":         attachment.Size,
				"path":         path,
				"is_inline":    attachment.IsInline,
			}
		}
		email.Attachments = attachments
	}

	// Process through pipeline
	if err := s.emailPipeline.ProcessOutgoing(email); err != nil {
		return nil, fmt.Errorf("failed to process email: %w", err)
	}

	// Send via network
	if err := s.networkService.SendMessage(ctx, string(req.Network), email); err != nil {
		return nil, fmt.Errorf("failed to send email: %w", err)
	}

	// Save to database
	if err := s.emailRepo.Create(ctx, email); err != nil {
		return nil, fmt.Errorf("failed to save email: %w", err)
	}

	return email, nil
}

func (s *emailServiceImpl) SearchEmails(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.Email, int, error) {
	emails, total, err := s.emailRepo.Search(ctx, userID, query, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to search emails: %w", err)
	}
	return emails, int(total), nil
}

func (s *emailServiceImpl) GetEmail(ctx context.Context, emailID uuid.UUID) (*models.Email, error) {
	email, err := s.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		return nil, fmt.Errorf("failed to get email: %w", err)
	}
	return email, nil
}

func (s *emailServiceImpl) MarkAsRead(ctx context.Context, emailID uuid.UUID, read bool) error {
	if err := s.emailRepo.MarkAsRead(ctx, emailID, read); err != nil {
		return fmt.Errorf("failed to mark email as read: %w", err)
	}
	return nil
}

func (s *emailServiceImpl) MarkAsStarred(ctx context.Context, emailID uuid.UUID, starred bool) error {
	if err := s.emailRepo.MarkAsStarred(ctx, emailID, starred); err != nil {
		return fmt.Errorf("failed to mark email as starred: %w", err)
	}
	return nil
}

func (s *emailServiceImpl) MoveToFolder(ctx context.Context, emailID, folderID uuid.UUID) error {
	if err := s.emailRepo.MoveToFolder(ctx, emailID, folderID); err != nil {
		return fmt.Errorf("failed to move email to folder: %w", err)
	}
	return nil
}

func (s *emailServiceImpl) DeleteEmail(ctx context.Context, emailID uuid.UUID) error {
	if err := s.emailRepo.Delete(ctx, emailID); err != nil {
		return fmt.Errorf("failed to delete email: %w", err)
	}
	return nil
}

func (s *emailServiceImpl) GetThread(ctx context.Context, threadID uuid.UUID) ([]models.Email, error) {
	// This would need userID to be passed, but for now return empty
	return []models.Email{}, nil
}
