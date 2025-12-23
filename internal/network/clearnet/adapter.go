package clearnet

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/smtp"
	"strings"
	"time"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/service"
)

// ClearnetAdapter handles clearnet (SMTP/IMAP) operations
type ClearnetAdapter struct {
	smtpHost     string
	smtpPort     int
	smtpUsername string
	smtpPassword string
	smtpUseTLS   bool

	imapHost string
	imapPort int

	lastStatus service.NetworkStatus
}

// NewClearnetAdapter creates a new clearnet adapter
func NewClearnetAdapter(
	smtpHost string,
	smtpPort int,
	smtpUsername, smtpPassword string,
	imapHost string,
	imapPort int,
) *ClearnetAdapter {
	return &ClearnetAdapter{
		smtpHost:     smtpHost,
		smtpPort:     smtpPort,
		smtpUsername: smtpUsername,
		smtpPassword: smtpPassword,
		smtpUseTLS:   smtpPort == 587 || smtpPort == 465,
		imapHost:     imapHost,
		imapPort:     imapPort,
		lastStatus: service.NetworkStatus{
			IsHealthy:   false,
			LastChecked: time.Now(),
		},
	}
}

// Connect establishes connection
func (a *ClearnetAdapter) Connect(ctx context.Context) error {
	// Test SMTP connection
	addr := fmt.Sprintf("%s:%d", a.smtpHost, a.smtpPort)

	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP: %w", err)
	}
	defer conn.Close()

	a.lastStatus = service.NetworkStatus{
		IsHealthy:   true,
		LastChecked: time.Now(),
	}

	return nil
}

// Disconnect closes connection
func (a *ClearnetAdapter) Disconnect(ctx context.Context) error {
	a.lastStatus.IsHealthy = false
	return nil
}

// Send sends an email via SMTP
func (a *ClearnetAdapter) Send(ctx context.Context, email *models.Email) error {
	// Build MIME message
	message := a.buildMIMEMessage(email)

	// Send via SMTP
	addr := fmt.Sprintf("%s:%d", a.smtpHost, a.smtpPort)

	// Prepare authentication
	var auth smtp.Auth
	if a.smtpUsername != "" && a.smtpPassword != "" {
		auth = smtp.PlainAuth("", a.smtpUsername, a.smtpPassword, a.smtpHost)
	}

	// Collect all recipients
	recipients := append([]string{}, email.ToAddresses...)
	recipients = append(recipients, email.CCAddresses...)
	recipients = append(recipients, email.BCCAddresses...)

	// Send with TLS if needed
	if a.smtpUseTLS {
		return a.sendWithTLS(addr, auth, email.FromAddress, recipients, message)
	}

	return smtp.SendMail(addr, auth, email.FromAddress, recipients, []byte(message))
}

// sendWithTLS sends email with TLS
func (a *ClearnetAdapter) sendWithTLS(addr string, auth smtp.Auth, from string, to []string, msg string) error {
	// Connect to server
	conn, err := tls.Dial("tcp", addr, &tls.Config{
		ServerName: a.smtpHost,
		MinVersion: tls.VersionTLS12,
	})
	if err != nil {
		return fmt.Errorf("TLS connection failed: %w", err)
	}
	defer conn.Close()

	// Create SMTP client
	client, err := smtp.NewClient(conn, a.smtpHost)
	if err != nil {
		return fmt.Errorf("SMTP client creation failed: %w", err)
	}
	defer client.Close()

	// Authenticate if needed
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authentication failed: %w", err)
		}
	}

	// Set sender
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("setting sender failed: %w", err)
	}

	// Set recipients
	for _, recipient := range to {
		if err := client.Rcpt(recipient); err != nil {
			return fmt.Errorf("setting recipient failed: %w", err)
		}
	}

	// Send message
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("data command failed: %w", err)
	}

	_, err = io.WriteString(wc, msg)
	if err != nil {
		wc.Close()
		return fmt.Errorf("writing message failed: %w", err)
	}

	err = wc.Close()
	if err != nil {
		return fmt.Errorf("closing data writer failed: %w", err)
	}

	return client.Quit()
}

// Receive receives emails (would implement IMAP)
func (a *ClearnetAdapter) Receive(ctx context.Context) (chan *models.Email, error) {
	emailChan := make(chan *models.Email, 100)

	// In a full implementation, this would connect to IMAP and fetch emails
	// For now, return empty channel
	go func() {
		<-ctx.Done()
		close(emailChan)
	}()

	return emailChan, nil
}

// HealthCheck checks clearnet connectivity
func (a *ClearnetAdapter) HealthCheck(ctx context.Context) (service.NetworkStatus, error) {
	start := time.Now()
	addr := fmt.Sprintf("%s:%d", a.smtpHost, a.smtpPort)

	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		a.lastStatus = service.NetworkStatus{
			IsHealthy:   false,
			LastError:   err.Error(),
			LastChecked: time.Now(),
		}
		return a.lastStatus, err
	}
	conn.Close()

	latency := time.Since(start)
	a.lastStatus = service.NetworkStatus{
		IsHealthy:   true,
		Latency:     latency,
		LastChecked: time.Now(),
	}

	return a.lastStatus, nil
}

// buildMIMEMessage builds RFC 2822 compliant message
func (a *ClearnetAdapter) buildMIMEMessage(email *models.Email) string {
	var builder strings.Builder

	// Headers
	builder.WriteString(fmt.Sprintf("From: %s\r\n", email.FromAddress))
	builder.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(email.ToAddresses, ", ")))

	if len(email.CCAddresses) > 0 {
		builder.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(email.CCAddresses, ", ")))
	}

	builder.WriteString(fmt.Sprintf("Subject: %s\r\n", email.Subject))
	builder.WriteString(fmt.Sprintf("Message-ID: %s\r\n", email.MessageID))
	builder.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))

	if email.InReplyTo != "" {
		builder.WriteString(fmt.Sprintf("In-Reply-To: %s\r\n", email.InReplyTo))
	}

	if len(email.References) > 0 {
		builder.WriteString(fmt.Sprintf("References: %s\r\n", strings.Join(email.References, " ")))
	}

	builder.WriteString("MIME-Version: 1.0\r\n")

	// Handle different body types
	if len(email.Attachments) > 0 {
		boundary := fmt.Sprintf("boundary_%d", time.Now().Unix())
		builder.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=\"%s\"\r\n\r\n", boundary))

		// Text part
		builder.WriteString(fmt.Sprintf("--%s\r\n", boundary))
		if email.BodyHTML != "" {
			builder.WriteString("Content-Type: text/html; charset=utf-8\r\n")
			builder.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
			builder.WriteString(email.BodyHTML)
		} else {
			builder.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
			builder.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
			builder.WriteString(email.BodyPlain)
		}
		builder.WriteString("\r\n")

		// Attachments would be added here
		builder.WriteString(fmt.Sprintf("--%s--\r\n", boundary))
	} else if email.BodyHTML != "" {
		builder.WriteString("Content-Type: text/html; charset=utf-8\r\n")
		builder.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		builder.WriteString(email.BodyHTML)
	} else {
		builder.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
		builder.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		builder.WriteString(email.BodyPlain)
	}

	return builder.String()
}
