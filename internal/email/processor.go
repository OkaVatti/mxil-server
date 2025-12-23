// internal/email/processor.go
package email

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
)

// EmailProcessor processes incoming emails
type EmailProcessor struct {
	emailRepo *repository.EmailRepository
	parser    *EmailParser
	pipeline  *Pipeline
}

// NewEmailProcessor creates a new email processor
func NewEmailProcessor(
	emailRepo *repository.EmailRepository,
	parser *EmailParser,
	pipeline *Pipeline,
) *EmailProcessor {
	return &EmailProcessor{
		emailRepo: emailRepo,
		parser:    parser,
		pipeline:  pipeline,
	}
}

// ProcessIncoming processes an incoming email
func (p *EmailProcessor) ProcessIncoming(ctx context.Context, userID string, rawEmail []byte) (*models.Email, error) {
	// Parse email
	parsed, err := p.parser.Parse(rawEmail)
	if err != nil {
		return nil, fmt.Errorf("failed to parse email: %w", err)
	}

	// Convert to model
	email := parsed.ToEmailModel(uuid.MustParse(userID))

	// Run through pipeline
	if err := p.pipeline.Process(ctx, email); err != nil {
		return nil, fmt.Errorf("pipeline processing failed: %w", err)
	}

	// Save to database
	if err := p.emailRepo.Create(ctx, email); err != nil {
		return nil, fmt.Errorf("failed to save email: %w", err)
	}

	return email, nil
}
