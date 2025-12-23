// internal/service/network/adapters/smtp.go
package adapters

import (
	"crypto/tls"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// SMTPAdapter implements NetworkAdapter for SMTP
type SMTPAdapter struct {
	*BaseAdapter
	host      string
	port      int
	username  string
	password  string
	useTLS    bool
	useSSL    bool
	tlsConfig *tls.Config
	dkimKey   []byte // For DKIM signing
}

// NewSMTPAdapter creates a new SMTP adapter
func NewSMTPAdapter(host string, port int, username, password string, useTLS, useSSL bool, dkimKey []byte) *SMTPAdapter {
	return &SMTPAdapter{
		BaseAdapter: NewBaseAdapter("smtp"),
		host:        host,
		port:        port,
		username:    username,
		password:    password,
		useTLS:      useTLS,
		useSSL:      useSSL,
		dkimKey:     dkimKey,
		tlsConfig: &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
	}
}

// Send sends an email via SMTP
func (a *SMTPAdapter) Send(email *models.Email) error {
	// Validate email
	if err := a.validateEmail(email); err != nil {
		return fmt.Errorf("email validation failed: %w", err)
	}

	// Create SMTP client
	client, err := a.connect()
	if err != nil {
		return fmt.Errorf("failed to connect to SMTP server: %w", err)
	}
	defer client.Close()

	// Create message
	msg, err := a.createMessage(email)
	if err != nil {
		return fmt.Errorf("failed to create message: %w", err)
	}

	// Send email
	from := mail.Address{Address: email.FromAddress}
	to := make([]string, len(email.ToAddresses))
	for i, recipient := range email.ToAddresses {
		to[i] = recipient
	}

	err = client.SendMail(from.Address, to, msg)
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	a.RecordSuccess()
	return nil
}

// TestConnection tests SMTP connectivity
func (a *SMTPAdapter) TestConnection() (bool, error) {
	client, err := a.connect()
	if err != nil {
		a.RecordError(err.Error())
		return false, fmt.Errorf("SMTP connection failed: %w", err)
	}
	defer client.Close()

	// Test by sending NOOP command
	err = client.Noop()
	if err != nil {
		a.RecordError(err.Error())
		return false, fmt.Errorf("SMTP NOOP failed: %w", err)
	}

	a.RecordSuccess()
	return true, nil
}

// Helper method to connect to SMTP server
func (a *SMTPAdapter) connect() (*mail.SMTPClient, error) {
	server := mail.NewSMTPClient()

	// Server settings
	server.Host = a.host
	server.Port = a.port
	server.Username = a.username
	server.Password = a.password
	server.Encryption = mail.EncryptionNone

	if a.useSSL {
		server.Encryption = mail.EncryptionSSL
	} else if a.useTLS {
		server.Encryption = mail.EncryptionTLS
	}

	server.TLSConfig = a.tlsConfig
	server.KeepAlive = true
	server.ConnectTimeout = 10 * time.Second
	server.SendTimeout = 30 * time.Second

	// Connect
	smtpClient, err := server.Connect()
	if err != nil {
		return nil, fmt.Errorf("SMTP connection failed: %w", err)
	}

	return smtpClient, nil
}

// Helper method to create message
func (a *SMTPAdapter) createMessage(email *models.Email) ([]byte, error) {
	msg := mail.NewMSG()

	// Set headers
	msg.SetFrom(email.FromAddress)
	msg.AddTo(email.ToAddresses...)
	msg.AddCc(email.CcAddresses...)
	msg.AddBcc(email.BccAddresses...)
	msg.SetSubject(email.Subject)

	// Set body
	if email.BodyHTML != "" {
		msg.SetBody(mail.TextHTML, email.BodyHTML)
		if email.BodyPlain != "" {
			msg.AddAlternative(mail.TextPlain, email.BodyPlain)
		}
	} else if email.BodyPlain != "" {
		msg.SetBody(mail.TextPlain, email.BodyPlain)
	}

	// Add custom headers
	msg.SetMessageID(fmt.Sprintf("<%s@mxil>", email.ID.String()))
	if email.InReplyTo != nil {
		msg.AddHeader("In-Reply-To", *email.InReplyTo)
	}
	if len(email.References) > 0 {
		msg.AddHeader("References", strings.Join(email.References, " "))
	}

	// TODO: Add DKIM signing if dkimKey is provided
	// TODO: Add attachments

	if msg.Error != nil {
		return nil, fmt.Errorf("failed to create message: %w", msg.Error)
	}

	return msg.GetMessage(), nil
}

// Helper method to validate email
func (a *SMTPAdapter) validateEmail(email *models.Email) error {
	if email.FromAddress == "" {
		return fmt.Errorf("sender is required")
	}

	if len(email.ToAddresses) == 0 && len(email.CcAddresses) == 0 && len(email.BccAddresses) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}

	// Validate sender
	if _, err := mail.ParseAddress(email.FromAddress); err != nil {
		return fmt.Errorf("invalid sender address: %w", err)
	}

	// Validate recipients
	allRecipients := append(append(email.ToAddresses, email.CcAddresses...), email.BccAddresses...)
	for _, recipient := range allRecipients {
		if _, err := mail.ParseAddress(recipient); err != nil {
			return fmt.Errorf("invalid recipient address %s: %w", recipient, err)
		}
	}

	return nil
}

// GetStatus returns SMTP connection status
func (a *SMTPAdapter) GetStatus() NetworkStatus {
	connected, _ := a.TestConnection()
	latency := a.measureLatency()

	return NetworkStatus{
		IsHealthy:   connected,
		LastError:   a.LastError(),
		LastChecked: time.Now(),
		Latency:     latency,
	}
}

// Helper method to measure latency
func (a *SMTPAdapter) measureLatency() time.Duration {
	start := time.Now()
	_, err := a.TestConnection()
	if err != nil {
		return 0
	}
	return time.Since(start)
}
