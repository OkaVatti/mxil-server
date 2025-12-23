package email

import (
	"context"
	"fmt"
	"net/mail"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/crypto"
	"github.com/okavatti/mxil-server/m/internal/email"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/storage"
)

// emailService implements EmailService
type emailService struct {
	emailRepo         repository.EmailRepository
	userRepo          repository.UserRepository
	emailParser       *email.EmailParser
	emailPipeline     *email.Pipeline
	storageService    storage.StorageService
	networkService    NetworkService
	cryptoService     crypto.CryptoService
	logger            *zap.Logger
	maxEmailSize      int64
	maxAttachments    int
	maxAttachmentSize int64
}

// NewEmailService creates a new email service
func NewEmailService(
	emailRepo repository.EmailRepository,
	userRepo repository.UserRepository,
	emailParser *email.EmailParser,
	emailPipeline *email.Pipeline,
	storageService storage.StorageService,
	networkService NetworkService,
	cryptoService crypto.CryptoService,
	logger *zap.Logger,
) EmailService {
	return &emailService{
		emailRepo:         emailRepo,
		userRepo:          userRepo,
		emailParser:       emailParser,
		emailPipeline:     emailPipeline,
		storageService:    storageService,
		networkService:    networkService,
		cryptoService:     cryptoService,
		logger:            logger,
		maxEmailSize:      50 * 1024 * 1024, // 50MB
		maxAttachments:    10,
		maxAttachmentSize: 25 * 1024 * 1024, // 25MB per attachment
	}
}

// SearchEmails searches emails with full-text search
func (s *emailService) SearchEmails(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.Email, int64, error) {
	if query == "" {
		// Return regular list if no query
		filter := repository.EmailFilter{
			UserID:   userID,
			Limit:    limit,
			Offset:   offset,
			SortBy:   "created_at",
			SortDesc: true,
		}
		return s.emailRepo.List(ctx, filter)
	}

	// Perform full-text search
	emails, total, err := s.emailRepo.Search(ctx, userID, query, limit, offset)
	if err != nil {
		s.logger.Error("Email search failed",
			zap.String("user_id", userID.String()),
			zap.String("query", query),
			zap.Error(err))
		return nil, 0, fmt.Errorf("search failed: %w", err)
	}

	return emails, total, nil
}

// GetEmailThread retrieves an email thread
func (s *emailService) GetEmailThread(ctx context.Context, userID, threadID uuid.UUID) ([]models.Email, error) {
	emails, err := s.emailRepo.GetByThreadID(ctx, userID, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve thread: %w", err)
	}

	if len(emails) == 0 {
		return []models.Email{}, nil
	}

	// Sort by date
	sort.Slice(emails, func(i, j int) bool {
		return emails[i].CreatedAt.Before(emails[j].CreatedAt)
	})

	return emails, nil
}

// ValidateEmailAddress validates an email address
func (s *emailService) ValidateEmailAddress(emailAddr string) (bool, error) {
	// Basic format validation
	if _, err := mail.ParseAddress(emailAddr); err != nil {
		return false, nil
	}

	// Check for disposable email domains
	disposableDomains := []string{
		"tempmail.com", "mailinator.com", "guerrillamail.com",
		"10minutemail.com", "yopmail.com", "trashmail.com",
		"fakeinbox.com", "throwawaymail.com", "tempmail.net",
		"disposablemail.com", "getairmail.com", "mailnesia.com",
	}

	parts := strings.Split(emailAddr, "@")
	if len(parts) != 2 {
		return false, nil
	}

	domain := strings.ToLower(parts[1])

	// Check against disposable domains
	for _, d := range disposableDomains {
		if strings.HasSuffix(domain, d) {
			return false, nil
		}
	}

	// Check for valid TLD
	validTLDs := []string{
		".com", ".org", ".net", ".edu", ".gov", ".io", ".co",
		".me", ".info", ".biz", ".name", ".pro", ".tv", ".us",
		".uk", ".ca", ".au", ".de", ".fr", ".es", ".it", ".nl",
		".se", ".no", ".dk", ".fi", ".ru", ".jp", ".cn", ".in",
		".br", ".mx", ".ar",
	}

	hasValidTLD := false
	for _, tld := range validTLDs {
		if strings.HasSuffix(domain, tld) {
			hasValidTLD = true
			break
		}
	}

	if !hasValidTLD {
		// Could be a new TLD or local domain
		// We'll accept it but log
		s.logger.Debug("Uncommon TLD detected", zap.String("domain", domain))
	}

	// TODO: DNS MX record check (async)
	// TODO: SMTP verification (async)

	return true, nil
}

// CleanupOldEmails deletes emails older than specified duration
func (s *emailService) CleanupOldEmails(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)

	// Get emails to delete
	emails, err := s.emailRepo.GetOlderThan(ctx, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to get old emails: %w", err)
	}

	deletedCount := int64(0)

	for _, email := range emails {
		// Delete attachments from storage
		for _, attachmentID := range email.AttachmentIDs {
			if err := s.storageService.Delete(ctx, attachmentID.String()); err != nil {
				s.logger.Warn("Failed to delete attachment",
					zap.String("attachment_id", attachmentID.String()),
					zap.Error(err))
			}
		}

		// Delete email record
		if err := s.emailRepo.Delete(ctx, email.ID); err != nil {
			s.logger.Warn("Failed to delete email",
				zap.String("email_id", email.ID.String()),
				zap.Error(err))
			continue
		}

		deletedCount++

		// Update user storage usage
		user, err := s.userRepo.GetByID(ctx, email.UserID)
		if err == nil {
			user.StorageQuotaUsed -= email.Size
			if user.StorageQuotaUsed < 0 {
				user.StorageQuotaUsed = 0
			}
			s.userRepo.Update(ctx, user)
		}
	}

	s.logger.Info("Cleaned up old emails",
		zap.Int64("count", deletedCount),
		zap.Time("cutoff", cutoff))

	return deletedCount, nil
}
