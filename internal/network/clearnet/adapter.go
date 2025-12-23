// internal/network/clearnet/adapter.go
package clearnet

import (
	"context"
	"fmt"
	"net/smtp"
	"time"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/service"
)

// ClearnetAdapter handles clearnet email operations
type ClearnetAdapter struct {
	smtpHost     string
	smtpPort     int
	smtpUsername string
	smtpPassword string
	imapHost     string
	imapPort     int
	lastStatus   service.NetworkStatus
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
		imapHost:     imapHost,
		imapPort:     imapPort,
		lastStatus: service.NetworkStatus{
			IsHealthy:   false,
			LastChecked: time.Now(),
		},
	}
}

// Connect connects to clearnet services
func (a *ClearnetAdapter) Connect(ctx context.Context) error {
	// Test SMTP connection
	addr := fmt.Sprintf("%s:%d", a.smtpHost, a.smtpPort)
	auth := smtp.PlainAuth("", a.smtpUsername, a.smtpPassword, a.smtpHost)

	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP: %w", err)
	}
	defer client.Close()

	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	a.lastStatus = service.NetworkStatus{
		IsHealthy:   true,
		LastChecked: time.Now(),
	}

	return nil
}

// Disconnect disconnects from clearnet services
func (a *ClearnetAdapter) Disconnect(ctx context.Context) error {
	// Nothing to disconnect for basic SMTP
	a.lastStatus.IsHealthy = false
	return nil
}

// Send sends an email via clearnet
func (a *ClearnetAdapter) Send(ctx context.Context, email *models.Email) error {
	// Build email message
	msg := a.buildEmailMessage(email)

	// Send via SMTP
	addr := fmt.Sprintf("%s:%d", a.smtpHost, a.smtpPort)
	auth := smtp.PlainAuth("", a.smtpUsername, a.smtpPassword, a.smtpHost)

	if err := smtp.SendMail(addr, auth, email.FromAddress, email.ToAddresses, []byte(msg)); err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// Receive receives emails from clearnet
func (a *ClearnetAdapter) Receive(ctx context.Context) (chan *models.Email, error) {
	emailChan := make(chan *models.Email, 100)

	// In a real implementation, this would connect to IMAP
	// and continuously fetch new emails
	// For now, we'll return an empty channel
	go func() {
		<-ctx.Done()
		close(emailChan)
	}()

	return emailChan, nil
}

// HealthCheck checks clearnet health
func (a *ClearnetAdapter) HealthCheck(ctx context.Context) (service.NetworkStatus, error) {
	// Test SMTP connection
	addr := fmt.Sprintf("%s:%d", a.smtpHost, a.smtpPort)
	start := time.Now()

	client, err := smtp.Dial(addr)
	if err != nil {
		a.lastStatus = service.NetworkStatus{
			IsHealthy:   false,
			LastError:   err.Error(),
			LastChecked: time.Now(),
		}
		return a.lastStatus, err
	}
	defer client.Close()

	latency := time.Since(start)
	a.lastStatus = service.NetworkStatus{
		IsHealthy:    true,
		Latency:      latency,
		LastChecked:  time.Now(),
		MessageCount: a.lastStatus.MessageCount,
	}

	return a.lastStatus, nil
}

// Helper methods
func (a *ClearnetAdapter) buildEmailMessage(email *models.Email) string {
	// Build MIME message
	// This is a simplified version
	msg := fmt.Sprintf("From: %s\r\n", email.FromAddress)
	msg += fmt.Sprintf("To: %s\r\n", joinAddresses(email.ToAddresses))
	if len(email.CCAddresses) > 0 {
		msg += fmt.Sprintf("Cc: %s\r\n", joinAddresses(email.CCAddresses))
	}
	msg += fmt.Sprintf("Subject: %s\r\n", email.Subject)
	msg += "MIME-Version: 1.0\r\n"
	msg += "Content-Type: text/plain; charset=utf-8\r\n"
	msg += "\r\n"
	msg += email.BodyPlain

	return msg
}

func joinAddresses(addresses []string) string {
	result := ""
	for i, addr := range addresses {
		if i > 0 {
			result += ", "
		}
		result += addr
	}
	return result
}
