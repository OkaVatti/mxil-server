package email

import (
	"fmt"
	"strings"
)

// PipelineStage defines an email processing stage
type PipelineStage interface {
	Process(email *ParsedEmail) error
	Name() string
	Priority() int
}

// EmailPipeline processes emails through multiple stages
type EmailPipeline struct {
	stages []PipelineStage
}

// NewEmailPipeline creates a new email pipeline
func NewEmailPipeline() *EmailPipeline {
	return &EmailPipeline{
		stages: []PipelineStage{
			&SizeValidationStage{maxSize: 50 * 1024 * 1024}, // 50MB
			&VirusScanStage{},
			&SpamFilterStage{},
			&DKIMVerificationStage{},
			&SPFVerificationStage{},
			&ContentAnalysisStage{},
			&AttachmentScanStage{},
			&AutoTaggingStage{},
		},
	}
}

// Process processes an email through all stages
func (p *EmailPipeline) Process(email *ParsedEmail) error {
	for _, stage := range p.stages {
		if err := stage.Process(email); err != nil {
			return fmt.Errorf("stage %s failed: %w", stage.Name(), err)
		}
	}
	return nil
}

// SizeValidationStage validates email size
type SizeValidationStage struct {
	maxSize int64
}

func (s *SizeValidationStage) Process(email *ParsedEmail) error {
	if email.RawSize > s.maxSize {
		return fmt.Errorf("email size %d exceeds maximum %d", email.RawSize, s.maxSize)
	}
	return nil
}

func (s *SizeValidationStage) Name() string  { return "size_validation" }
func (s *SizeValidationStage) Priority() int { return 10 }

// VirusScanStage scans for viruses
type VirusScanStage struct{}

func (s *VirusScanStage) Process(email *ParsedEmail) error {
	// TODO: Integrate with ClamAV or other virus scanner
	// For now, do basic checks
	for _, attachment := range email.Attachments {
		if isPotentiallyMalicious(attachment.Filename, attachment.Data) {
			email.IsSpam = true
			email.SpamScore += 100
			return fmt.Errorf("potential virus detected in attachment %s", attachment.Filename)
		}
	}
	return nil
}

func (s *VirusScanStage) Name() string  { return "virus_scan" }
func (s *VirusScanStage) Priority() int { return 20 }

// SpamFilterStage filters spam emails
type SpamFilterStage struct{}

func (s *SpamFilterStage) Process(email *ParsedEmail) error {
	score := 0.0

	// Check for spam keywords in subject
	spamKeywords := []string{
		"viagra", "cialis", "penis", "sex", "porn",
		"casino", "gambling", "lottery", "winner",
		"nigerian", "prince", "inheritance",
		"credit card", "debt", "loan",
		"work from home", "make money fast",
	}

	lowerSubject := strings.ToLower(email.Subject)
	for _, keyword := range spamKeywords {
		if strings.Contains(lowerSubject, keyword) {
			score += 10
		}
	}

	// Check body for spam patterns
	if email.BodyPlain != "" {
		lowerBody := strings.ToLower(email.BodyPlain)
		if strings.Contains(lowerBody, "click here") ||
			strings.Contains(lowerBody, "unsubscribe") ||
			strings.Contains(lowerBody, "opt out") {
			score += 5
		}
	}

	// Check sender domain
	if isSuspiciousDomain(email.From) {
		score += 20
	}

	// Check for too many recipients
	if len(email.To)+len(email.Cc)+len(email.Bcc) > 50 {
		score += 15
	}

	// Update spam score
	email.SpamScore += score
	if email.SpamScore >= 30 {
		email.IsSpam = true
	}

	return nil
}

func (s *SpamFilterStage) Name() string  { return "spam_filter" }
func (s *SpamFilterStage) Priority() int { return 30 }

// DKIMVerificationStage verifies DKIM signatures
type DKIMVerificationStage struct{}

func (s *DKIMVerificationStage) Process(email *ParsedEmail) error {
	// TODO: Implement DKIM verification
	// This would check the DKIM-Signature header
	return nil
}

func (s *DKIMVerificationStage) Name() string  { return "dkim_verification" }
func (s *DKIMVerificationStage) Priority() int { return 40 }

// SPFVerificationStage verifies SPF records
type SPFVerificationStage struct{}

func (s *SPFVerificationStage) Process(email *ParsedEmail) error {
	// TODO: Implement SPF verification
	// This would check if the sending IP is authorized for the domain
	return nil
}

func (s *SPFVerificationStage) Name() string  { return "spf_verification" }
func (s *SPFVerificationStage) Priority() int { return 50 }

// ContentAnalysisStage analyzes email content
type ContentAnalysisStage struct{}

func (s *ContentAnalysisStage) Process(email *ParsedEmail) error {
	// TODO: Implement content analysis
	// This could include sentiment analysis, language detection, etc.
	return nil
}

func (s *ContentAnalysisStage) Name() string  { return "content_analysis" }
func (s *ContentAnalysisStage) Priority() int { return 60 }

// AttachmentScanStage scans attachments
type AttachmentScanStage struct{}

func (s *AttachmentScanStage) Process(email *ParsedEmail) error {
	for _, attachment := range email.Attachments {
		// Check for dangerous file types
		if isDangerousFileType(attachment.Filename) {
			email.IsSpam = true
			email.SpamScore += 50
			return fmt.Errorf("dangerous file type: %s", attachment.Filename)
		}
	}
	return nil
}

func (s *AttachmentScanStage) Name() string  { return "attachment_scan" }
func (s *AttachmentScanStage) Priority() int { return 70 }

// AutoTaggingStage automatically tags emails
type AutoTaggingStage struct{}

func (s *AutoTaggingStage) Process(email *ParsedEmail) error {
	// TODO: Implement auto-tagging based on content
	// This could use machine learning or rule-based systems
	return nil
}

func (s *AutoTaggingStage) Name() string  { return "auto_tagging" }
func (s *AutoTaggingStage) Priority() int { return 80 }

// Helper functions
func isPotentiallyMalicious(filename string, data []byte) bool {
	// Check for executable signatures
	dangerousSignatures := [][]byte{
		{0x4D, 0x5A},             // DOS executable
		{0x7F, 0x45, 0x4C, 0x46}, // ELF
		{0xCA, 0xFE, 0xBA, 0xBE}, // Java class
	}

	for _, sig := range dangerousSignatures {
		if len(data) >= len(sig) {
			match := true
			for i, b := range sig {
				if data[i] != b {
					match = false
					break
				}
			}
			if match {
				return true
			}
		}
	}

	// Check file extension
	dangerousExtensions := []string{
		".exe", ".dll", ".bat", ".cmd", ".vbs", ".js",
		".ps1", ".sh", ".jar", ".class",
	}

	for _, ext := range dangerousExtensions {
		if strings.HasSuffix(strings.ToLower(filename), ext) {
			return true
		}
	}

	return false
}

func isSuspiciousDomain(email string) bool {
	suspiciousDomains := []string{
		"tempmail.com", "mailinator.com", "guerrillamail.com",
		"10minutemail.com", "yopmail.com", "trashmail.com",
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return true
	}

	domain := strings.ToLower(parts[1])
	for _, suspicious := range suspiciousDomains {
		if strings.Contains(domain, suspicious) {
			return true
		}
	}

	return false
}

func isDangerousFileType(filename string) bool {
	dangerousExtensions := []string{
		".exe", ".dll", ".bat", ".cmd", ".vbs", ".js",
		".ps1", ".psm1", ".sh", ".bash", ".zsh",
		".py", ".php", ".pl", ".rb",
		".jar", ".class", ".war", ".ear",
		".msi", ".com", ".scr", ".pif",
		".application", ".gadget", ".msh",
		".reg", ".inf", ".docm", ".dotm",
		".xlsm", ".xltm", ".xlam", ".pptm",
		".potm", ".ppam", ".ppsm", ".sldm",
	}

	lowerName := strings.ToLower(filename)
	for _, ext := range dangerousExtensions {
		if strings.HasSuffix(lowerName, ext) {
			return true
		}
	}

	return false
}
