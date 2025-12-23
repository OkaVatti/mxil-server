package email

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

func (s *emailService) validateSendRequest(ctx context.Context, userID uuid.UUID, req SendEmailRequest) error {
	// Validate recipients
	if len(req.To) == 0 && len(req.Cc) == 0 && len(req.Bcc) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	// Validate subject
	if req.Subject == "" {
		return fmt.Errorf("subject is required")
	}
	if len(req.Subject) > 998 { // RFC 2822 limit
		return fmt.Errorf("subject is too long")
	}

	// Validate content
	if req.BodyPlain == "" && req.BodyHTML == "" && req.BodyMarkdown == "" {
		return fmt.Errorf("email content is required")
	}

	// Validate recipients
	allRecipients := make([]string, 0, len(req.To)+len(req.Cc)+len(req.Bcc))
	allRecipients = append(allRecipients, req.To...)
	allRecipients = append(allRecipients, req.Cc...)
	allRecipients = append(allRecipients, req.Bcc...)

	// Check for duplicate recipients
	recipientSet := make(map[string]bool)
	for _, recipient := range allRecipients {
		if recipientSet[recipient] {
			return fmt.Errorf("duplicate recipient: %s", recipient)
		}
		recipientSet[recipient] = true

		// Validate email format
		if valid, err := s.ValidateEmailAddress(recipient); err != nil || !valid {
			return fmt.Errorf("invalid email address: %s", recipient)
		}

		// Check for self-sending (if applicable)
		user, err := s.userRepo.GetByID(ctx, userID)
		if err == nil && strings.EqualFold(recipient, user.Email) {
			// Allow self-sending but log
			s.logger.Debug("User sending email to themselves",
				zap.String("user_id", userID.String()),
				zap.String("email", recipient))
		}
	}

	// Check total recipients limit
	if len(allRecipients) > 100 {
		return fmt.Errorf("too many recipients: %d exceeds maximum of 100", len(allRecipients))
	}

	// Check email size
	totalSize := int64(len(req.BodyPlain) + len(req.BodyHTML) + len(req.BodyMarkdown))
	for _, att := range req.Attachments {
		totalSize += att.Size
	}
	if totalSize > s.maxEmailSize {
		return fmt.Errorf("email size exceeds limit of %d bytes", s.maxEmailSize)
	}

	// Check attachment count
	if len(req.Attachments) > s.maxAttachments {
		return fmt.Errorf("too many attachments: %d exceeds maximum of %d",
			len(req.Attachments), s.maxAttachments)
	}

	// Validate network
	if req.Network == "" {
		return fmt.Errorf("network is required")
	}

	// Validate encryption config
	if req.Encryption.Algorithm != "" {
		validAlgorithms := []string{"aes-gcm-128", "aes-gcm-256", "pgp", "smime"}
		valid := false
		for _, algo := range validAlgorithms {
			if algo == req.Encryption.Algorithm {
				valid = true
				break
			}
		}
		if !valid {
			return fmt.Errorf("unsupported encryption algorithm: %s", req.Encryption.Algorithm)
		}

		if len(req.Encryption.KeyIDs) == 0 {
			return fmt.Errorf("key IDs required for encryption")
		}
	}

	// Validate reply-to reference
	if req.InReplyTo != "" {
		// Check if referenced email exists
		parentEmail, err := s.emailRepo.GetByMessageID(ctx, req.InReplyTo)
		if err != nil {
			return fmt.Errorf("referenced email not found: %s", req.InReplyTo)
		}

		// Check if user has access to referenced email
		if parentEmail.UserID != userID {
			return fmt.Errorf("cannot reply to email you don't have access to")
		}
	}

	return nil
}
