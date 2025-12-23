package email

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
)

// Parser handles email parsing
type Parser struct{}

// Parse parses raw email data
func (p *Parser) Parse(rawEmail []byte) (*ParsedEmail, error) {
	// In a real implementation, this would parse MIME email
	// For now, return a simplified structure
	return &ParsedEmail{
		MessageID: generateMessageID(),
		From:      "sender@example.com",
		To:        []string{"recipient@example.com"},
		Subject:   "Test Email",
		BodyPlain: string(rawEmail),
		BodyHTML:  "<p>" + string(rawEmail) + "</p>",
		Date:      time.Now(),
	}, nil
}

// Attachment represents an email attachment
type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
	IsInline    bool
}

// Process processes an email through all stages
func (p *Pipeline) Process(ctx context.Context, email *models.Email) error {
	for _, stage := range p.stages {
		if err := stage.Process(ctx, email); err != nil {
			return err
		}
	}
	return nil
}

// Process implements PipelineStage
func (s *SpamFilterStage) Process(ctx context.Context, email *models.Email) error {
	// Simple spam filter based on keywords
	spamKeywords := []string{"viagra", "casino", "lottery", "free money"}
	content := strings.ToLower(email.Subject + " " + email.BodyPlain)

	for _, keyword := range spamKeywords {
		if strings.Contains(content, keyword) {
			email.IsSpam = true
			break
		}
	}

	return nil
}

// Process implements PipelineStage
func (s *VirusScanStage) Process(ctx context.Context, email *models.Email) error {
	// In a real implementation, this would scan attachments
	// For now, just pass through
	return nil
}

// Helper functions
func generateMessageID() string {
	return fmt.Sprintf("<%s@mxil>", uuid.New().String())
}

// ValidateEmailAddress validates an email address
func ValidateEmailAddress(email string) bool {
	if !strings.Contains(email, "@") {
		return false
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	local, domain := parts[0], parts[1]
	if local == "" || domain == "" {
		return false
	}

	// Check for valid characters
	if strings.Contains(local, "..") || strings.Contains(domain, "..") {
		return false
	}

	return true
}

// ExtractDomain extracts the domain from an email address
func ExtractDomain(email string) string {
	parts := strings.Split(email, "@")
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}
