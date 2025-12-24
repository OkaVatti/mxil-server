package email

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// ParseRawEmail parses a raw email message
func (p *EmailParser) ParseRawEmail(rawEmail []byte, userID uuid.UUID) (*models.Email, error) {
	// Parse the email using Go's mail package
	msg, err := mail.ReadMessage(bytes.NewReader(rawEmail))
	if err != nil {
		return nil, fmt.Errorf("failed to parse email: %w", err)
	}

	// Extract headers
	subject := msg.Header.Get("Subject")
	from := msg.Header.Get("From")
	to := parseAddressList(msg.Header.Get("To"))
	cc := parseAddressList(msg.Header.Get("Cc"))
	bcc := parseAddressList(msg.Header.Get("Bcc"))
	messageID := msg.Header.Get("Message-ID")
	inReplyTo := msg.Header.Get("In-Reply-To")
	references := parseReferences(msg.Header.Get("References"))

	// Parse date
	var sentAt *time.Time
	if dateStr := msg.Header.Get("Date"); dateStr != "" {
		if t, err := mail.ParseDate(dateStr); err == nil {
			sentAt = &t
		}
	}

	// Parse body
	bodyPlain, bodyHTML, attachments, err := p.parseBody(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to parse email body: %w", err)
	}

	// Build email model
	email := &models.Email{
		ID:           uuid.New(),
		UserID:       userID,
		ThreadID:     uuid.New(), // This should be calculated based on threading
		MessageID:    messageID,
		From:         from,
		To:           to,
		Cc:           cc,
		Bcc:          bcc,
		Subject:      subject,
		BodyPlain:    bodyPlain,
		BodyHTML:     bodyHTML,
		BodyMarkdown: "", // Could convert from HTML if needed
		Network:      models.NetworkClearnet,
		Priority:     0,
		IsRead:       false,
		IsStarred:    false,
		IsArchived:   false,
		IsSpam:       false,
		IsEncrypted:  false,
		Attachments:  attachments,
		Headers:      parseHeaders(msg.Header),
		InReplyTo:    inReplyTo,
		References:   references,
		SentAt:       sentAt,
		ReceivedAt:   time.Now(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	return email, nil
}

// parseBody parses the email body and extracts content
func (p *EmailParser) parseBody(msg *mail.Message) (string, string, map[string]interface{}, error) {
	contentType := msg.Header.Get("Content-Type")
	attachments := make(map[string]interface{})

	if contentType == "" {
		// Read plain text
		body, err := io.ReadAll(msg.Body)
		if err != nil {
			return "", "", nil, fmt.Errorf("failed to read body: %w", err)
		}
		return string(body), "", attachments, nil
	}

	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to parse content type: %w", err)
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		return p.parseMultipart(msg.Body, params["boundary"])
	}

	// Single part
	body, err := io.ReadAll(msg.Body)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to read body: %w", err)
	}

	if mediaType == "text/plain" {
		return string(body), "", attachments, nil
	} else if mediaType == "text/html" {
		return "", string(body), attachments, nil
	}

	// Handle other content types as attachments
	filename := params["name"]
	if filename == "" {
		filename = "attachment"
	}
	attachments[filename] = map[string]interface{}{
		"content_type": mediaType,
		"size":         len(body),
		"data":         body,
	}

	return "", "", attachments, nil
}

// parseMultipart parses multipart email
func (p *EmailParser) parseMultipart(body io.Reader, boundary string) (string, string, map[string]interface{}, error) {
	reader := multipart.NewReader(body, boundary)
	var plainText, htmlText string
	attachments := make(map[string]interface{})

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", "", nil, fmt.Errorf("failed to read part: %w", err)
		}

		contentType := part.Header.Get("Content-Type")
		mediaType, params, _ := mime.ParseMediaType(contentType)
		disposition := part.Header.Get("Content-Disposition")

		content, err := io.ReadAll(part)
		if err != nil {
			return "", "", nil, fmt.Errorf("failed to read part content: %w", err)
		}

		// Check if this is an attachment
		if strings.Contains(disposition, "attachment") {
			filename := params["filename"]
			if filename == "" {
				filename = "attachment"
			}
			attachments[filename] = map[string]interface{}{
				"content_type": mediaType,
				"size":         len(content),
				"data":         content,
			}
			continue
		}

		// Check content type
		if mediaType == "text/plain" {
			plainText = string(content)
		} else if mediaType == "text/html" {
			htmlText = string(content)
		} else if strings.HasPrefix(mediaType, "multipart/") {
			// Nested multipart
			nestedPlain, nestedHTML, nestedAttachments, err := p.parseMultipart(
				bytes.NewReader(content),
				params["boundary"],
			)
			if err == nil {
				if plainText == "" {
					plainText = nestedPlain
				}
				if htmlText == "" {
					htmlText = nestedHTML
				}
				for k, v := range nestedAttachments {
					attachments[k] = v
				}
			}
		}
	}

	return plainText, htmlText, attachments, nil
}

// parseReferences parses references header
func parseReferences(header string) models.StringArray {
	if header == "" {
		return models.StringArray{}
	}
	return models.StringArray(strings.Fields(header))
}

// parseHeaders converts mail headers to map
func parseHeaders(headers mail.Header) map[string]interface{} {
	result := make(map[string]interface{})
	for key, values := range headers {
		if len(values) == 1 {
			result[key] = values[0]
		} else {
			result[key] = values
		}
	}
	return result
}

// EmailPipeline handles email processing pipeline
type EmailPipeline struct {
	logger *zap.Logger
}

// NewEmailPipeline creates a new email pipeline
func NewEmailPipeline() *EmailPipeline {
	return &EmailPipeline{}
}

// ProcessIncoming processes incoming email
func (p *EmailPipeline) ProcessIncoming(email *models.Email) error {
	// Apply spam filtering
	email.IsSpam = p.isSpam(email)

	// Apply auto-filing rules
	p.autoFile(email)

	// Extract and process attachments
	p.processAttachments(email)

	return nil
}

// ProcessOutgoing processes outgoing email
func (p *EmailPipeline) ProcessOutgoing(email *models.Email) error {
	// Validate email
	if err := p.validateEmail(email); err != nil {
		return fmt.Errorf("email validation failed: %w", err)
	}

	// Apply DKIM signing if enabled
	p.applyDKIM(email)

	// Apply encryption if recipients support it
	p.applyEncryption(email)

	// Format email for sending
	email.BodyPlain = p.formatPlainText(email)
	if email.BodyHTML != "" {
		email.BodyHTML = p.formatHTML(email)
	}

	return nil
}

// isSpam checks if email is spam
func (p *EmailPipeline) isSpam(email *models.Email) bool {
	// Basic spam detection
	spamKeywords := []string{
		"viagra", "casino", "lottery", "prize", "winner",
		"click here", "buy now", "limited offer",
	}

	content := strings.ToLower(email.Subject + " " + email.BodyPlain)
	for _, keyword := range spamKeywords {
		if strings.Contains(content, keyword) {
			return true
		}
	}

	return false
}

// autoFile automatically files email
func (p *EmailPipeline) autoFile(email *models.Email) {
	// Auto-file to spam folder if spam
	if email.IsSpam {
		// This would set folder ID to spam folder
		return
	}

	// Check for mailing lists
	if strings.Contains(email.From, "list") || strings.Contains(email.From, "newsletter") {
		// File to mailing list folder
		return
	}
}

// processAttachments processes email attachments
func (p *EmailPipeline) processAttachments(email *models.Email) {
	// Extract attachment metadata
	if attachments, ok := email.Attachments.(map[string]interface{}); ok {
		for filename, data := range attachments {
			p.logger.Debug("Processing attachment",
				zap.String("filename", filename),
				zap.Any("data", data))
		}
	}
}

// validateEmail validates outgoing email
func (p *EmailPipeline) validateEmail(email *models.Email) error {
	if email.From == "" {
		return fmt.Errorf("sender address is required")
	}

	if len(email.To) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	if email.Subject == "" {
		email.Subject = "(No subject)"
	}

	return nil
}

// applyDKIM applies DKIM signing
func (p *EmailPipeline) applyDKIM(email *models.Email) {
	// TODO: Implement DKIM signing
}

// applyEncryption applies encryption
func (p *EmailPipeline) applyEncryption(email *models.Email) {
	// TODO: Implement encryption based on recipient keys
}

// formatPlainText formats plain text email
func (p *EmailPipeline) formatPlainText(email *models.Email) string {
	var buf bytes.Buffer
	buf.WriteString(email.BodyPlain)
	buf.WriteString("\n\n--\nSent via MXIL")
	return buf.String()
}

// formatHTML formats HTML email
func (p *EmailPipeline) formatHTML(email *models.Email) string {
	// Wrap HTML in proper structure
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>%s</title>
</head>
<body>
%s
<hr>
<footer>Sent via MXIL</footer>
</body>
</html>`, email.Subject, email.BodyHTML)

	return html
}

// BuildRawEmail builds raw email from model
func (p *EmailParser) BuildRawEmail(email *models.Email) ([]byte, error) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)

	// Write headers
	fmt.Fprintf(writer, "From: %s\r\n", email.From)
	fmt.Fprintf(writer, "To: %s\r\n", strings.Join(email.To, ", "))

	if len(email.Cc) > 0 {
		fmt.Fprintf(writer, "Cc: %s\r\n", strings.Join(email.Cc, ", "))
	}

	fmt.Fprintf(writer, "Subject: %s\r\n", email.Subject)
	fmt.Fprintf(writer, "Message-ID: %s\r\n", email.MessageID)
	fmt.Fprintf(writer, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))

	if email.InReplyTo != "" {
		fmt.Fprintf(writer, "In-Reply-To: %s\r\n", email.InReplyTo)
	}

	if len(email.References) > 0 {
		fmt.Fprintf(writer, "References: %s\r\n", strings.Join(email.References, " "))
	}

	// Write content type
	if email.BodyHTML != "" {
		// Multipart email
		boundary := fmt.Sprintf("mxil-%d", time.Now().UnixNano())
		fmt.Fprintf(writer, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary)
		fmt.Fprintf(writer, "\r\n")

		// Plain text part
		fmt.Fprintf(writer, "--%s\r\n", boundary)
		fmt.Fprintf(writer, "Content-Type: text/plain; charset=utf-8\r\n")
		fmt.Fprintf(writer, "Content-Transfer-Encoding: quoted-printable\r\n")
		fmt.Fprintf(writer, "\r\n")
		fmt.Fprintf(writer, "%s\r\n", email.BodyPlain)

		// HTML part
		fmt.Fprintf(writer, "--%s\r\n", boundary)
		fmt.Fprintf(writer, "Content-Type: text/html; charset=utf-8\r\n")
		fmt.Fprintf(writer, "Content-Transfer-Encoding: quoted-printable\r\n")
		fmt.Fprintf(writer, "\r\n")
		fmt.Fprintf(writer, "%s\r\n", email.BodyHTML)

		fmt.Fprintf(writer, "--%s--\r\n", boundary)
	} else {
		// Plain text only
		fmt.Fprintf(writer, "Content-Type: text/plain; charset=utf-8\r\n")
		fmt.Fprintf(writer, "\r\n")
		fmt.Fprintf(writer, "%s\r\n", email.BodyPlain)
	}

	writer.Flush()
	return buf.Bytes(), nil
}
