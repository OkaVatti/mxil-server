// internal/service/email_service.go
package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/okavatti/mxil-server/m/internal/email"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// EmailServiceImpl implements EmailService
type EmailServiceImpl struct {
	emailRepo     *repository.EmailRepository
	userRepo      *repository.UserRepository
	emailParser   *email.EmailParser
	emailPipeline *email.Pipeline
	storage       StorageService
	network       NetworkService
	crypto        CryptoService
	logger        *zap.Logger
}

// NewEmailService creates a new email service
func NewEmailService(
	emailRepo *repository.EmailRepository,
	userRepo *repository.UserRepository,
	emailParser *email.EmailParser,
	emailPipeline *email.Pipeline,
	storage StorageService,
	network NetworkService,
	crypto CryptoService,
	logger *zap.Logger,
) *EmailServiceImpl {
	return &EmailServiceImpl{
		emailRepo:     emailRepo,
		userRepo:      userRepo,
		emailParser:   emailParser,
		emailPipeline: emailPipeline,
		storage:       storage,
		network:       network,
		crypto:        crypto,
		logger:        logger,
	}
}

// SendEmail sends a new email
func (s *EmailServiceImpl) SendEmail(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error) {
	// Get user to check quota and get email address
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Calculate email size
	emailSize := int64(len(req.BodyPlain) + len(req.BodyHTML) + len(req.BodyMarkdown))
	for _, att := range req.Attachments {
		emailSize += att.Size
	}

	// Check storage quota
	used, total, err := s.userRepo.GetStorageUsage(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check storage quota: %w", err)
	}

	if used+emailSize > total {
		return nil, ErrQuotaExceeded
	}

	// Get user's primary email
	fromAddress := ""
	for _, identity := range user.NetworkIdentities {
		if identity.IsPrimary {
			fromAddress = identity.Address
			break
		}
	}

	if fromAddress == "" {
		return nil, NewServiceError("no_primary_email", "No primary email address found")
	}

	// Validate recipients
	for _, recipient := range req.To {
		if !isValidEmailAddress(recipient) {
			return nil, NewServiceError("invalid_email", fmt.Sprintf("Invalid email address: %s", recipient))
		}
	}

	// Generate message ID
	messageID := generateMessageID()

	// Create email model
	email := &models.Email{
		ID:           uuid.New(),
		UserID:       userID,
		MessageID:    messageID,
		FromAddress:  fromAddress,
		ToAddresses:  models.StringArray(req.To),
		CCAddresses:  models.StringArray(req.Cc),
		BCCAddresses: models.StringArray(req.Bcc),
		Subject:      req.Subject,
		BodyPlain:    req.BodyPlain,
		BodyHTML:     req.BodyHTML,
		IsSent:       true,
		SentVia:      &req.Network,
		SentAt:       &time.Time{},
		ReceivedAt:   time.Now(),
		IsEncrypted:  req.Encryption.Enabled,
	}

	if req.InReplyTo != "" {
		email.InReplyTo = req.InReplyTo
	}

	if len(req.References) > 0 {
		email.References = models.StringArray(req.References)
	}

	// Handle draft
	if req.DraftID != nil {
		// Delete the draft
		s.emailRepo.Delete(ctx, *req.DraftID)
	}

	// Process attachments
	if len(req.Attachments) > 0 {
		if err := s.processAttachments(ctx, email, req.Attachments); err != nil {
			return nil, fmt.Errorf("failed to process attachments: %w", err)
		}
	}

	// Apply encryption if requested
	if req.Encryption.Enabled {
		if err := s.encryptEmail(ctx, email, req.Encryption); err != nil {
			return nil, fmt.Errorf("failed to encrypt email: %w", err)
		}
	}

	// Save to database first
	if err := s.emailRepo.Create(ctx, email); err != nil {
		return nil, fmt.Errorf("failed to save email: %w", err)
	}

	// Update storage usage
	if err := s.userRepo.UpdateStorageUsed(ctx, userID, emailSize); err != nil {
		s.logger.Warn("Failed to update storage usage", zap.Error(err))
	}

	// Send via network
	if err := s.network.SendEmail(ctx, email, req.Network); err != nil {
		// Mark as failed but keep in database
		email.IsSent = false
		if updateErr := s.emailRepo.Update(ctx, email); updateErr != nil {
			return nil, fmt.Errorf("failed to send email and update status: %w", err)
		}
		return nil, fmt.Errorf("failed to send email: %w", err)
	}

	// Update sent time
	now := time.Now()
	email.SentAt = &now
	if err := s.emailRepo.Update(ctx, email); err != nil {
		s.logger.Warn("Failed to update sent time", zap.Error(err))
	}

	return email, nil
}

// GetEmail retrieves a single email
func (s *EmailServiceImpl) GetEmail(ctx context.Context, userID, emailID uuid.UUID) (*models.Email, error) {
	email, err := s.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		return nil, ErrEmailNotFound
	}

	// Check ownership
	if email.UserID != userID {
		return nil, ErrAccessDenied
	}

	// Decrypt if encrypted
	if email.IsEncrypted && len(email.BodyEncrypted) > 0 {
		// TODO: Implement decryption
		// This would require user's private key
	}

	return email, nil
}

// ListEmails lists emails with filtering
func (s *EmailServiceImpl) ListEmails(ctx context.Context, userID uuid.UUID, filter repository.EmailFilter) ([]models.Email, int64, error) {
	// Ensure filter is for this user
	filter.UserID = userID
	return s.emailRepo.List(ctx, filter)
}

// DeleteEmail deletes an email
func (s *EmailServiceImpl) DeleteEmail(ctx context.Context, userID, emailID uuid.UUID) error {
	// Get email to check ownership
	email, err := s.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		return ErrEmailNotFound
	}

	if email.UserID != userID {
		return ErrAccessDenied
	}

	// Delete attachments from storage
	for _, att := range email.Attachments {
		if att.StoragePath != "" {
			if err := s.storage.DeleteAttachment(ctx, att.StoragePath); err != nil {
				s.logger.Warn("Failed to delete attachment from storage",
					zap.String("path", att.StoragePath), zap.Error(err))
			}
		}
	}

	// Update storage usage (reduce)
	totalSize := s.calculateEmailSize(email)
	if err := s.userRepo.UpdateStorageUsed(ctx, userID, -totalSize); err != nil {
		s.logger.Warn("Failed to update storage usage after deletion", zap.Error(err))
	}

	// Soft delete from database
	return s.emailRepo.Delete(ctx, emailID)
}

// MoveEmail moves an email to a folder
func (s *EmailServiceImpl) MoveEmail(ctx context.Context, userID, emailID, folderID uuid.UUID) error {
	// Check ownership
	email, err := s.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		return ErrEmailNotFound
	}

	if email.UserID != userID {
		return ErrAccessDenied
	}

	// TODO: Verify folder exists and belongs to user

	return s.emailRepo.MoveToFolder(ctx, emailID, folderID)
}

// MarkEmailRead marks an email as read/unread
func (s *EmailServiceImpl) MarkEmailRead(ctx context.Context, userID, emailID uuid.UUID, read bool) error {
	// Check ownership
	email, err := s.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		return ErrEmailNotFound
	}

	if email.UserID != userID {
		return ErrAccessDenied
	}

	return s.emailRepo.MarkAsRead(ctx, emailID, read)
}

// MarkEmailStarred marks an email as starred/unstarred
func (s *EmailServiceImpl) MarkEmailStarred(ctx context.Context, userID, emailID uuid.UUID, starred bool) error {
	// Check ownership
	email, err := s.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		return ErrEmailNotFound
	}

	if email.UserID != userID {
		return ErrAccessDenied
	}

	return s.emailRepo.MarkAsStarred(ctx, emailID, starred)
}

// GetEmailThread retrieves an email thread
func (s *EmailServiceImpl) GetEmailThread(ctx context.Context, userID, threadID uuid.UUID) ([]models.Email, error) {
	emails, err := s.emailRepo.GetThreadEmails(ctx, threadID)
	if err != nil {
		return nil, fmt.Errorf("failed to get thread emails: %w", err)
	}

	// Filter to only show emails belonging to the user
	var userEmails []models.Email
	for _, email := range emails {
		if email.UserID == userID {
			userEmails = append(userEmails, email)
		}
	}

	return userEmails, nil
}

// SearchEmails searches emails with full-text search
func (s *EmailServiceImpl) SearchEmails(ctx context.Context, userID uuid.UUID, query string, limit, offset int) ([]models.Email, int64, error) {
	filter := repository.EmailFilter{
		UserID: userID,
		Search: query,
		Limit:  limit,
		Offset: offset,
	}

	return s.emailRepo.List(ctx, filter)
}

// ProcessIncomingEmail processes an incoming email
func (s *EmailServiceImpl) ProcessIncomingEmail(ctx context.Context, rawEmail []byte, network models.NetworkType) (*models.Email, error) {
	// Parse email
	parsed, err := s.emailParser.Parse(rawEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to parse email: %w", err)
	}

	// Find recipient user
	userID, err := s.findRecipientUserID(ctx, parsed.To)
	if err != nil {
		return nil, fmt.Errorf("recipient not found: %w", err)
	}

	// Convert to model
	email := parsed.ToEmailModel(userID)
	email.ReceivedVia = network

	// Run through processing pipeline
	if err := s.emailPipeline.Process(ctx, email); err != nil {
		s.logger.Warn("Email pipeline processing failed", zap.Error(err))
		// Continue anyway
	}

	// Save to database
	if err := s.emailRepo.Create(ctx, email); err != nil {
		return nil, fmt.Errorf("failed to save email: %w", err)
	}

	// Update storage usage
	emailSize := s.calculateEmailSize(email)
	if err := s.userRepo.UpdateStorageUsed(ctx, userID, emailSize); err != nil {
		s.logger.Warn("Failed to update storage usage", zap.Error(err))
	}

	return email, nil
}

// SaveDraft saves an email draft
func (s *EmailServiceImpl) SaveDraft(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, ErrUserNotFound
	}

	// Get user's primary email
	fromAddress := ""
	for _, identity := range user.NetworkIdentities {
		if identity.IsPrimary {
			fromAddress = identity.Address
			break
		}
	}

	if fromAddress == "" {
		return nil, NewServiceError("no_primary_email", "No primary email address found")
	}

	// Create draft email
	draft := &models.Email{
		ID:           uuid.New(),
		UserID:       userID,
		MessageID:    generateMessageID(),
		FromAddress:  fromAddress,
		ToAddresses:  models.StringArray(req.To),
		CCAddresses:  models.StringArray(req.Cc),
		BCCAddresses: models.StringArray(req.Bcc),
		Subject:      req.Subject,
		BodyPlain:    req.BodyPlain,
		BodyHTML:     req.BodyHTML,
		IsDraft:      true,
		ReceivedAt:   time.Now(),
	}

	// Save draft
	if err := s.emailRepo.Create(ctx, draft); err != nil {
		return nil, fmt.Errorf("failed to save draft: %w", err)
	}

	return draft, nil
}

// UpdateDraft updates an existing draft
func (s *EmailServiceImpl) UpdateDraft(ctx context.Context, userID, draftID uuid.UUID, req SendEmailRequest) (*models.Email, error) {
	// Get draft
	draft, err := s.emailRepo.GetByID(ctx, draftID)
	if err != nil {
		return nil, ErrEmailNotFound
	}

	// Check ownership
	if draft.UserID != userID {
		return nil, ErrAccessDenied
	}

	// Check if it's actually a draft
	if !draft.IsDraft {
		return nil, NewServiceError("not_a_draft", "Email is not a draft")
	}

	// Update draft
	draft.ToAddresses = models.StringArray(req.To)
	draft.CCAddresses = models.StringArray(req.Cc)
	draft.BCCAddresses = models.StringArray(req.Bcc)
	draft.Subject = req.Subject
	draft.BodyPlain = req.BodyPlain
	draft.BodyHTML = req.BodyHTML
	draft.UpdatedAt = time.Now()

	if err := s.emailRepo.Update(ctx, draft); err != nil {
		return nil, fmt.Errorf("failed to update draft: %w", err)
	}

	return draft, nil
}

// DeleteDraft deletes a draft
func (s *EmailServiceImpl) DeleteDraft(ctx context.Context, userID, draftID uuid.UUID) error {
	// Get draft
	draft, err := s.emailRepo.GetByID(ctx, draftID)
	if err != nil {
		return ErrEmailNotFound
	}

	// Check ownership and draft status
	if draft.UserID != userID || !draft.IsDraft {
		return ErrAccessDenied
	}

	return s.emailRepo.Delete(ctx, draftID)
}

// ListDrafts lists all drafts for a user
func (s *EmailServiceImpl) ListDrafts(ctx context.Context, userID uuid.UUID) ([]models.Email, error) {
	filter := repository.EmailFilter{
		UserID:  userID,
		IsDraft: &[]bool{true}[0],
		Limit:   100,
	}

	emails, _, err := s.emailRepo.List(ctx, filter)
	return emails, err
}

// Helper methods
func (s *EmailServiceImpl) processAttachments(ctx context.Context, email *models.Email, attachments []AttachmentInfo) error {
	for _, attInfo := range attachments {
		// Decode base64 data
		data, err := base64.StdEncoding.DecodeString(attInfo.Data)
		if err != nil {
			return fmt.Errorf("failed to decode attachment: %w", err)
		}

		// Save to storage
		storagePath, err := s.storage.SaveAttachment(ctx, data, attInfo.Filename)
		if err != nil {
			return fmt.Errorf("failed to save attachment: %w", err)
		}

		// Create attachment model
		attachment := models.Attachment{
			ID:             uuid.New(),
			EmailID:        email.ID,
			Filename:       attInfo.Filename,
			ContentType:    attInfo.ContentType,
			Size:           attInfo.Size,
			ContentID:      attInfo.ContentID,
			IsInline:       attInfo.IsInline,
			StoragePath:    storagePath,
			StorageBackend: s.storage.GetBackend(),
		}

		email.Attachments = append(email.Attachments, attachment)
	}
	return nil
}

func (s *EmailServiceImpl) encryptEmail(ctx context.Context, email *models.Email, config EncryptionConfig) error {
	// Get recipient public keys
	var recipientKeys []string
	// TODO: Fetch recipient public keys from database or key server

	// Encrypt the email
	plaintext := email.BodyPlain
	if email.BodyHTML != "" {
		plaintext = email.BodyHTML
	}

	encryptedBody, keyIDs, err := s.crypto.EncryptEmail(plaintext, recipientKeys, config.Algorithm)
	if err != nil {
		return fmt.Errorf("failed to encrypt email: %w", err)
	}

	// Store encrypted data
	email.BodyEncrypted = encryptedBody
	email.BodyPlain = "" // Clear plaintext
	email.BodyHTML = ""  // Clear HTML
	email.IsEncrypted = true
	email.EncryptionAlgorithm = config.Algorithm
	email.EncryptionKeyIDs = models.StringArray(keyIDs)

	// Create encryption metadata
	if len(keyIDs) > 0 {
		encryption := &models.EmailEncryption{
			ID:        uuid.New(),
			EmailID:   email.ID,
			KeyIDs:    models.StringArray(keyIDs),
			Algorithm: config.Algorithm,
			KeySize:   &config.KeySize,
			Layers: models.JSONB{
				"algorithm": config.Algorithm,
				"key_size":  config.KeySize,
			},
			CreatedAt: time.Now(),
		}
		email.Encryption = encryption
	}

	return nil
}

func (s *EmailServiceImpl) findRecipientUserID(ctx context.Context, recipients []string) (uuid.UUID, error) {
	// For simplicity, check first recipient
	if len(recipients) == 0 {
		return uuid.Nil, fmt.Errorf("no recipients")
	}

	// TODO: Implement actual user lookup by email
	// For now, return a dummy UUID
	return uuid.New(), nil
}

func (s *EmailServiceImpl) calculateEmailSize(email *models.Email) int64 {
	size := int64(0)
	size += int64(len(email.BodyPlain))
	size += int64(len(email.BodyHTML))
	size += int64(len(email.BodyEncrypted))

	for _, att := range email.Attachments {
		size += att.Size
	}

	return size
}

func generateMessageID() string {
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	return fmt.Sprintf("<%s@mxil>", base64.URLEncoding.EncodeToString(randomBytes))
}

func isValidEmailAddress(email string) bool {
	return strings.Contains(email, "@") && len(email) > 3
}

// Additional service interface implementations
func (s *EmailServiceImpl) GetEmailStatistics(ctx context.Context, userID uuid.UUID) (*EmailStatistics, error) {
	// Get total email count
	totalFilter := repository.EmailFilter{
		UserID:  userID,
		IsTrash: &[]bool{false}[0],
		Limit:   1,
	}
	totalEmails, totalCount, err := s.emailRepo.List(ctx, totalFilter)
	if err != nil {
		return nil, err
	}

	// Get unread count
	unreadFilter := repository.EmailFilter{
		UserID:  userID,
		IsRead:  &[]bool{false}[0],
		IsTrash: &[]bool{false}[0],
		Limit:   1,
	}
	_, unreadCount, err := s.emailRepo.List(ctx, unreadFilter)
	if err != nil {
		return nil, err
	}

	// Get today's emails
	today := time.Now().Truncate(24 * time.Hour)
	todayFilter := repository.EmailFilter{
		UserID:   userID,
		FromDate: &today,
		IsTrash:  &[]bool{false}[0],
		Limit:    1,
	}
	_, todayCount, err := s.emailRepo.List(ctx, todayFilter)
	if err != nil {
		return nil, err
	}

	return &EmailStatistics{
		Total:      totalCount,
		Unread:     unreadCount,
		Today:      todayCount,
		ByNetwork:  make(map[string]int64),
		ByProvider: make(map[string]int64),
	}, nil
}

func (s *EmailServiceImpl) ExportEmails(ctx context.Context, userID uuid.UUID, format string, filter repository.EmailFilter) (io.ReadCloser, error) {
	// Get emails based on filter
	filter.UserID = userID
	filter.Limit = 1000 // Export limit

	emails, _, err := s.emailRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Create export based on format
	switch format {
	case "json":
		return s.exportJSON(emails)
	case "eml":
		return s.exportEML(emails)
	case "csv":
		return s.exportCSV(emails)
	default:
		return nil, NewServiceError("unsupported_format", "Unsupported export format")
	}
}

func (s *EmailServiceImpl) exportJSON(emails []models.Email) (io.ReadCloser, error) {
	// TODO: Implement JSON export
	return nil, nil
}

func (s *EmailServiceImpl) exportEML(emails []models.Email) (io.ReadCloser, error) {
	// TODO: Implement EML export
	return nil, nil
}

func (s *EmailServiceImpl) exportCSV(emails []models.Email) (io.ReadCloser, error) {
	// TODO: Implement CSV export
	return nil, nil
}

func (s *EmailServiceImpl) ImportEmails(ctx context.Context, userID uuid.UUID, format string, data io.Reader) (int, error) {
	// TODO: Implement email import
	return 0, nil
}

func (s *EmailServiceImpl) CleanupOldEmails(ctx context.Context, olderThan time.Duration) (int64, error) {
	// TODO: Implement cleanup of old emails
	return 0, nil
}

func (s *EmailServiceImpl) GetEmailRules(ctx context.Context, userID uuid.UUID) ([]models.EmailRule, error) {
	// TODO: Implement email rules retrieval
	return nil, nil
}

func (s *EmailServiceImpl) CreateEmailRule(ctx context.Context, userID uuid.UUID, rule *models.EmailRule) error {
	// TODO: Implement email rule creation
	return nil
}

func (s *EmailServiceImpl) UpdateEmailRule(ctx context.Context, userID, ruleID uuid.UUID, rule *models.EmailRule) error {
	// TODO: Implement email rule update
	return nil
}

func (s *EmailServiceImpl) DeleteEmailRule(ctx context.Context, userID, ruleID uuid.UUID) error {
	// TODO: Implement email rule deletion
	return nil
}

func (s *EmailServiceImpl) TestEmailRule(ctx context.Context, userID uuid.UUID, conditions models.JSONB) ([]models.Email, error) {
	// TODO: Implement email rule testing
	return nil, nil
}
