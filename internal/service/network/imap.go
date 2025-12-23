// internal/service/network/adapters/imap.go
package adapters

import (
	"crypto/tls"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/okavatti/mxil-server/m/internal/models"
)

// IMAPAdapter implements NetworkAdapter for IMAP
type IMAPAdapter struct {
	*BaseAdapter
	host      string
	port      int
	username  string
	password  string
	useSSL    bool
	tlsConfig *tls.Config
	logger    *zap.Logger
}

// NewIMAPAdapter creates a new IMAP adapter
func NewIMAPAdapter(host string, port int, username, password string, useSSL bool, logger *zap.Logger) *IMAPAdapter {
	return &IMAPAdapter{
		BaseAdapter: NewBaseAdapter("imap"),
		host:        host,
		port:        port,
		username:    username,
		password:    password,
		useSSL:      useSSL,
		tlsConfig: &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: false, // Always verify certificates in production
			MinVersion:         tls.VersionTLS12,
		},
		logger: logger,
	}
}

// Connect establishes an IMAP connection
func (a *IMAPAdapter) Connect() (*client.Client, error) {
	addr := fmt.Sprintf("%s:%d", a.host, a.port)
	var c *client.Client
	var err error

	if a.useSSL {
		c, err = client.DialTLS(addr, a.tlsConfig)
	} else {
		c, err = client.Dial(addr)
		// Upgrade to TLS
		if err == nil {
			err = c.StartTLS(a.tlsConfig)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to IMAP server: %w", err)
	}

	// Login
	if err := c.Login(a.username, a.password); err != nil {
		c.Logout()
		return nil, fmt.Errorf("failed to login: %w", err)
	}

	return c, nil
}

// Send sends an email via IMAP (stores in sent folder)
func (a *IMAPAdapter) Send(email *models.Email) error {
	// IMAP doesn't send emails, it only stores them
	// For sending, use SMTP
	a.logger.Debug("IMAP adapter storing sent email",
		zap.String("email_id", email.ID.String()))

	// TODO: Store email in IMAP sent folder
	// This would require connecting and appending to sent folder

	a.RecordSuccess()
	return nil
}

// TestConnection tests IMAP connectivity
func (a *IMAPAdapter) TestConnection() (bool, error) {
	c, err := a.Connect()
	if err != nil {
		a.RecordError(err.Error())
		return false, fmt.Errorf("IMAP connection failed: %w", err)
	}
	defer c.Logout()

	// Check capability
	_, err = c.Capability()
	if err != nil {
		a.RecordError(err.Error())
		return false, fmt.Errorf("IMAP capability check failed: %w", err)
	}

	// List mailboxes
	mailboxes := make(chan *imap.MailboxInfo, 10)
	done := make(chan error, 1)
	go func() {
		done <- c.List("", "*", mailboxes)
	}()

	select {
	case err := <-done:
		if err != nil {
			a.RecordError(err.Error())
			return false, fmt.Errorf("IMAP list mailboxes failed: %w", err)
		}
	case <-time.After(10 * time.Second):
		a.RecordError("timeout listing mailboxes")
		return false, fmt.Errorf("IMAP connection timeout")
	}

	a.RecordSuccess()
	return true, nil
}

// GetStatus returns IMAP connection status
func (a *IMAPAdapter) GetStatus() NetworkStatus {
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
func (a *IMAPAdapter) measureLatency() time.Duration {
	start := time.Now()
	_, err := a.TestConnection()
	if err != nil {
		return 0
	}
	return time.Since(start)
}

// BaseAdapter (to be created in a base file)
type BaseAdapter struct {
	name      string
	lastError string
	lastCheck time.Time
	healthy   bool
}

func NewBaseAdapter(name string) *BaseAdapter {
	return &BaseAdapter{
		name: name,
	}
}

func (b *BaseAdapter) RecordError(err string) {
	b.lastError = err
	b.healthy = false
	b.lastCheck = time.Now()
}

func (b *BaseAdapter) RecordSuccess() {
	b.lastError = ""
	b.healthy = true
	b.lastCheck = time.Now()
}

func (b *BaseAdapter) LastError() string {
	return b.lastError
}
