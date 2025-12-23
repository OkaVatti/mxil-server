// internal/service/email/send.go - Complete SendEmail implementation
package email

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// SendEmail sends an email
func (s *emailService) SendEmail(ctx context.Context, userID uuid.UUID, req SendEmailRequest) (*models.Email, error) {
	// Validate request
	if err := s.validateSendRequest(ctx, userID, req); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	// Check quota
	estimatedSize := int64(len(req.BodyPlain) + len(req.BodyHTML) + len(req.BodyMarkdown))
	for _, att := range req.Attachments {
		estimatedSize += att.Size
	}

	if err := s.checkQuota(ctx, userID, estimatedSize); err != nil {
		return nil, err
	}

	// Check sending rate
	if err := s.checkSendingRate(ctx, userID); err != nil {
		return nil, err
	}

	// Get user
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	// Create email record
	email := &models.Email{
		ID:             uuid.New(),
		UserID:         userID,
		MessageID:      fmt.Sprintf("<%s@mxil>", uuid.New().String()),
		FromAddress:    user.Email,
		ToAddresses:    req.To,
		CCAddresses:    req.Cc,
		BCCAddresses:   req.Bcc,
		Subject:        req.Subject,
		BodyPlain:      req.BodyPlain,
		BodyHTML:       req.BodyHTML,
		BodyMarkdown:   req.BodyMarkdown,
		Network:        req.Network,
		Direction:      "outgoing",
		Status:         "pending",
		IsRead:         true, // Sent emails are marked as read
		HasAttachments: len(req.Attachments) > 0,
		Size:           estimatedSize,
		ReceivedAt:     time.Now(),
		ReceivedVia:    req.Network,
		InReplyTo:      req.InReplyTo,
		References:     req.References,
	}

	// Handle thread ID
	if req.InReplyTo != "" {
		parentEmail, err := s.emailRepo.GetByMessageID(ctx, req.InReplyTo)
		if err == nil && parentEmail.ThreadID != nil {
			email.ThreadID = parentEmail.ThreadID
		}
	}
	if email.ThreadID == nil {
		email.ThreadID = &email.ID
	}

	// Store attachments
	var attachmentIDs []uuid.UUID
	var totalAttachmentSize int64

	for _, att := range req.Attachments {
		attID, size, err := s.storeAttachment(ctx, userID, att)
		if err != nil {
			s.logger.Error("Failed to store attachment",
				zap.String("filename", att.Filename),
				zap.Error(err))
			return nil, fmt.Errorf("failed to store attachment %s: %w", att.Filename, err)
		}
		attachmentIDs = append(attachmentIDs, attID)
		totalAttachmentSize += size
	}

	email.AttachmentIDs = attachmentIDs
	email.Size += totalAttachmentSize

	// Encrypt if requested
	if req.Encryption.Algorithm != "" {
		encryptedContent, keyIDs, err := s.cryptoService.EncryptEmail(
			req.BodyPlain,
			req.Encryption.KeyIDs,
			req.Encryption.Algorithm,
		)
		if err != nil {
			return nil, fmt.Errorf("encryption failed: %w", err)
		}
		email.IsEncrypted = true
		email.EncryptionKeyIDs = keyIDs
		email.BodyPlain = string(encryptedContent)
		email.BodyHTML = ""
		email.BodyMarkdown = ""
	}

	// Save email
	if err := s.emailRepo.Create(ctx, email); err != nil {
		s.logger.Error("Failed to save email", zap.Error(err))
		return nil, fmt.Errorf("failed to save email: %w", err)
	}

	// Send via network
	if err := s.sendViaNetwork(ctx, email, req); err != nil {
		email.Status = "failed"
		s.emailRepo.Update(ctx, email)
		return nil, fmt.Errorf("failed to send via network: %w", err)
	}

	// Update status
	email.Status = "sent"
	now := time.Now()
	email.SentAt = &now
	s.emailRepo.Update(ctx, email)

	// Update user quota
	s.UpdateUserQuota(ctx, userID, email.Size)

	// Record sending for rate limiting
	s.recordSending(ctx, userID)

	s.logger.Info("Email sent successfully",
		zap.String("email_id", email.ID.String()),
		zap.String("user_id", userID.String()),
		zap.Int("recipients", len(req.To)+len(req.Cc)+len(req.Bcc)))

	return email, nil
}

// ProcessIncomingEmail processes an incoming email
func (s *emailService) ProcessIncomingEmail(ctx context.Context, rawEmail []byte, network models.NetworkType) error {
	// Parse email
	parsed, err := s.emailParser.Parse(rawEmail)
	if err != nil {
		s.logger.Error("Failed to parse email", zap.Error(err))
		return fmt.Errorf("failed to parse email: %w", err)
	}

	// Find recipient user
	// For now, use the first To address
	if len(parsed.To) == 0 {
		return fmt.Errorf("no recipients found")
	}

	recipientEmail := parsed.To[0]
	user, err := s.userRepo.GetByEmail(ctx, recipientEmail)
	if err != nil {
		s.logger.Warn("Recipient not found", zap.String("email", recipientEmail))
		return fmt.Errorf("recipient not found: %w", err)
	}

	// Convert to email model
	email := parsed.ToEmailModel(user.ID)
	email.ReceivedVia = network
	email.Direction = "incoming"
	email.Status = "received"

	// Save email
	if err := s.emailRepo.Create(ctx, email); err != nil {
		s.logger.Error("Failed to save incoming email", zap.Error(err))
		return fmt.Errorf("failed to save email: %w", err)
	}

	s.logger.Info("Incoming email processed",
		zap.String("email_id", email.ID.String()),
		zap.String("from", email.FromAddress))

	return nil
}

// GetEmailStats returns email statistics
func (s *emailService) GetEmailStats(ctx context.Context, userID uuid.UUID) (*EmailStats, error) {
	// This would query various statistics
	// For now, return placeholder
	return &EmailStats{
		TotalEmails:     0,
		UnreadCount:     0,
		SentCount:       0,
		ReceivedCount:   0,
		TotalSize:       0,
		EmailsByNetwork: make(map[models.NetworkType]int64),
		EmailsByDay:     make(map[string]int64),
	}, nil
}

// CleanupOldEmails implementation is already in service.go
