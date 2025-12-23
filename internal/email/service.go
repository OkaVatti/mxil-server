// internal/email/service.go
package email

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service"
)

type Service struct {
	emailRepo *repository.EmailRepository
	userRepo  *repository.UserRepository
	parser    *Parser
	pipeline  *Pipeline
	storage   *StorageService
	crypto    *CryptoService
}

type Parser struct{}
type Pipeline struct{}
type StorageService struct{}
type CryptoService struct{}

func NewService(
	emailRepo *repository.EmailRepository,
	userRepo *repository.UserRepository,
	parser *Parser,
	pipeline *Pipeline,
	storeage *StorageService,
	crypto *CryptoService,
) *Service {
	return &Service{
		emailRepo: emailRepo,
		userRepo:  userRepo,
		parser:    parser,
		pipeline:  pipeline,
		storage:   storage,
		crypto:    crypto,
	}
}

func NewParser() *Parser {
	return &Parser{}
}

func NewPipeline() *Pipeline {
	return &Pipeline{}
}

func (s *Service) SendEmail(ctx context.Context, userID uuid.UUID, req service.SendEmailRequest) (*service.Email, error) {
	// Get user to check quota
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Check storage quota
	// TODO: Implement actual quota checking

	// Create email record
	email := &repository.Email{
		ID:             uuid.New(),
		UserID:         userID,
		ThreadID:       uuid.New(),
		FromAddress:    user.Email,
		ToAddresses:    req.To,
		Subject:        req.Subject,
		BodyPlain:      req.Body,
		Network:        req.Network,
		IsRead:         true, // Sent emails are marked as read
		HasAttachments: len(req.Attachments) > 0,
		SizeBytes:      int64(len(req.Body)),
		ReceivedAt:     time.Now(),
		CreatedAt:      time.Now(),
	}

	// Store attachments if any
	for _, attachment := range req.Attachments {
		// TODO: Store attachments
		_ = attachment
	}

	// Save email
	if err := s.emailRepo.Create(ctx, email); err != nil {
		return nil, fmt.Errorf("failed to save email: %w", err)
	}

	// Process through pipeline
	// TODO: Implement email pipeline

	// Return email response
	return &service.Email{
		ID:             email.ID,
		ThreadID:       email.ThreadID,
		From:           email.FromAddress,
		To:             email.ToAddresses,
		Subject:        email.Subject,
		Body:           email.BodyPlain,
		Network:        email.Network,
		IsRead:         email.IsRead,
		HasAttachments: email.HasAttachments,
		SizeBytes:      email.SizeBytes,
		ReceivedAt:     email.ReceivedAt,
	}, nil
}
