// internal/email/pipeline.go
package email

import (
	"context"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// PipelineStage represents a stage in the email processing pipeline
type PipelineStage interface {
	Name() string
	Priority() int
	Process(ctx context.Context, email *models.Email) error
	CanBypass() bool
}

// Pipeline manages email processing stages
type Pipeline struct {
	stages []PipelineStage
}

// NewPipeline creates a new pipeline
func NewPipeline(stages []PipelineStage) *Pipeline {
	return &Pipeline{
		stages: stages,
	}
}

// SpamFilterStage filters spam
type SpamFilterStage struct{}

func (s *SpamFilterStage) Name() string    { return "SpamFilter" }
func (s *SpamFilterStage) Priority() int   { return 2 }
func (s *SpamFilterStage) CanBypass() bool { return false }

// VirusScanStage scans for viruses
type VirusScanStage struct{}

func (s *VirusScanStage) Name() string    { return "VirusScan" }
func (s *VirusScanStage) Priority() int   { return 1 }
func (s *VirusScanStage) CanBypass() bool { return false }

func (s *VirusScanStage) Process(ctx context.Context, email *models.Email) error {
	// Mark as scanned (actual virus scanning would integrate with ClamAV)
	email.VirusScanned = true
	clean := true
	email.VirusClean = &clean

	return nil
}

// DKIMVerificationStage verifies DKIM signatures
type DKIMVerificationStage struct{}

func (s *DKIMVerificationStage) Name() string    { return "DKIMVerification" }
func (s *DKIMVerificationStage) Priority() int   { return 3 }
func (s *DKIMVerificationStage) CanBypass() bool { return true }

func (s *DKIMVerificationStage) Process(ctx context.Context, email *models.Email) error {
	// DKIM verification would happen here
	// For now, mark as unverified
	verified := false
	email.DKIMVerified = &verified

	return nil
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(len(s) >= len(substr)) &&
		indexAny(s, substr) >= 0
}

func indexAny(s, chars string) int {
	for i := 0; i < len(s); i++ {
		for j := 0; j < len(chars); j++ {
			if s[i] == chars[j] {
				return i
			}
		}
	}
	return -1
}
