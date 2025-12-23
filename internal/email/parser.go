// internal/email/parser.go
package email

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"time"

	"github.com/okavatti/mxil-server/m/internal/models"

	"github.com/google/uuid"
)

// ParsedEmail represents a parsed email
type ParsedEmail struct {
	MessageID   string
	InReplyTo   string
	References  []string
	From        string
	To          []string
	CC          []string
	BCC         []string
	ReplyTo     string
	Subject     string
	Date        time.Time
	BodyPlain   string
	BodyHTML    string
	Attachments []ParsedAttachment
	Headers     map[string][]string
}

// ParsedAttachment represents a parsed attachment
type ParsedAttachment struct {
	Filename    string
	ContentType string
	Content     []byte
	ContentID   string
	IsInline    bool
}

// EmailParser parses raw email messages
type EmailParser struct{}

// NewEmailParser creates a new email parser
func NewEmailParser() *EmailParser {
	return &EmailParser{}
}

// Parse parses a raw email message
func (p *EmailParser) Parse(rawEmail []byte) (*ParsedEmail, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(rawEmail))
	if err != nil {
		return nil, fmt.Errorf("failed to read email: %w", err)
	}

	parsed := &ParsedEmail{
		Headers: make(map[string][]string),
	}

	// Parse headers
	parsed.MessageID = msg.Header.Get("Message-ID")
	parsed.InReplyTo = msg.Header.Get("In-Reply-To")
	parsed.Subject = decodeHeader(msg.Header.Get("Subject"))
	parsed.ReplyTo = msg.Header.Get("Reply-To")

	// Parse references
	if refs := msg.Header.Get("References"); refs != "" {
		parsed.References = strings.Fields(refs)
	}

	// Parse addresses
	if from, err := mail.ParseAddress(msg.Header.Get("From")); err == nil {
		parsed.From = from.Address
	}

	if toAddrs, err := mail.ParseAddressList(msg.Header.Get("To")); err == nil {
		for _, addr := range toAddrs {
			parsed.To = append(parsed.To, addr.Address)
		}
	}

	if ccAddrs, err := mail.ParseAddressList(msg.Header.Get("Cc")); err == nil {
		for _, addr := range ccAddrs {
			parsed.CC = append(parsed.CC, addr.Address)
		}
	}

	// Parse date
	if dateStr := msg.Header.Get("Date"); dateStr != "" {
		if date, err := mail.ParseDate(dateStr); err == nil {
			parsed.Date = date
		}
	}

	// Store all headers
	for k, v := range msg.Header {
		parsed.Headers[k] = v
	}

	// Parse body
	contentType := msg.Header.Get("Content-Type")
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		mediaType = "text/plain"
	}

	if strings.HasPrefix(mediaType, "multipart/") {
		if err := p.parseMultipart(msg.Body, params["boundary"], parsed); err != nil {
			return nil, err
		}
	} else {
		body, err := io.ReadAll(msg.Body)
		if err != nil {
			return nil, err
		}

		if strings.HasPrefix(mediaType, "text/plain") {
			parsed.BodyPlain = string(body)
		} else if strings.HasPrefix(mediaType, "text/html") {
			parsed.BodyHTML = string(body)
		}
	}

	return parsed, nil
}

// parseMultipart parses multipart content
func (p *EmailParser) parseMultipart(body io.Reader, boundary string, parsed *ParsedEmail) error {
	mr := multipart.NewReader(body, boundary)

	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		contentType := part.Header.Get("Content-Type")
		mediaType, params, _ := mime.ParseMediaType(contentType)

		// Handle nested multipart
		if strings.HasPrefix(mediaType, "multipart/") {
			if err := p.parseMultipart(part, params["boundary"], parsed); err != nil {
				return err
			}
			continue
		}

		content, err := io.ReadAll(part)
		if err != nil {
			return err
		}

		// Decode if needed
		if encoding := part.Header.Get("Content-Transfer-Encoding"); encoding != "" {
			content = decodeContent(content, encoding)
		}

		// Handle body parts
		if strings.HasPrefix(mediaType, "text/plain") && parsed.BodyPlain == "" {
			parsed.BodyPlain = string(content)
		} else if strings.HasPrefix(mediaType, "text/html") && parsed.BodyHTML == "" {
			parsed.BodyHTML = string(content)
		} else {
			// Handle attachments
			filename := part.FileName()
			if filename == "" {
				if name := params["name"]; name != "" {
					filename = name
				}
			}

			if filename != "" {
				attachment := ParsedAttachment{
					Filename:    decodeHeader(filename),
					ContentType: mediaType,
					Content:     content,
					ContentID:   part.Header.Get("Content-ID"),
					IsInline:    strings.Contains(part.Header.Get("Content-Disposition"), "inline"),
				}
				parsed.Attachments = append(parsed.Attachments, attachment)
			}
		}
	}

	return nil
}

// ToEmailModel converts parsed email to email model
func (p *ParsedEmail) ToEmailModel(userID uuid.UUID) *models.Email {
	email := &models.Email{
		ID:           uuid.New(),
		UserID:       userID,
		MessageID:    p.MessageID,
		InReplyTo:    p.InReplyTo,
		References:   p.References,
		FromAddress:  p.From,
		ToAddresses:  p.To,
		CCAddresses:  p.CC,
		BCCAddresses: p.BCC,
		ReplyTo:      p.ReplyTo,
		Subject:      p.Subject,
		BodyPlain:    p.BodyPlain,
		BodyHTML:     p.BodyHTML,
		ReceivedAt:   time.Now(),
	}

	if !p.Date.IsZero() {
		email.SentAt = &p.Date
	}

	// Convert attachments
	for _, att := range p.Attachments {
		attachment := models.Attachment{
			ID:          uuid.New(),
			EmailID:     email.ID,
			Filename:    att.Filename,
			ContentType: att.ContentType,
			Size:        int64(len(att.Content)),
			ContentID:   att.ContentID,
			IsInline:    att.IsInline,
		}
		email.Attachments = append(email.Attachments, attachment)
	}

	return email
}

// Helper functions
func decodeHeader(header string) string {
	dec := new(mime.WordDecoder)
	decoded, err := dec.DecodeHeader(header)
	if err != nil {
		return header
	}
	return decoded
}

func decodeContent(content []byte, encoding string) []byte {
	encoding = strings.ToLower(encoding)
	switch encoding {
	case "base64":
		decoded := make([]byte, base64.StdEncoding.DecodedLen(len(content)))
		n, err := base64.StdEncoding.Decode(decoded, content)
		if err != nil {
			return content
		}
		return decoded[:n]
	case "quoted-printable":
		// Simple quoted-printable decoder
		return content // TODO: Implement proper QP decoder
	default:
		return content
	}
}
