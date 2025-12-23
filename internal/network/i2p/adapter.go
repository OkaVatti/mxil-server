package i2p

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/service"

	i2pkeys "github.com/go-i2p/i2pkeys"
	"github.com/go-i2p/sam3"
)

// I2PAdapter handles I2P network operations
type I2PAdapter struct {
	samAddr       string
	streamSession *sam3.StreamSession
	sam           *sam3.SAM
	keys          i2pkeys.I2PKeys
	base32Addr    string
	lastStatus    service.NetworkStatus
	listener      net.Listener
}

// NewI2PAdapter creates a new I2P adapter
func NewI2PAdapter(samAddr string) *I2PAdapter {
	return &I2PAdapter{
		samAddr: samAddr,
		lastStatus: service.NetworkStatus{
			IsHealthy:   false,
			LastChecked: time.Now(),
		},
	}
}

// Connect establishes I2P connection
func (a *I2PAdapter) Connect(ctx context.Context) error {
	// Connect to SAM bridge
	sam, err := sam3.NewSAM(a.samAddr)
	if err != nil {
		return fmt.Errorf("SAM connection failed: %w", err)
	}
	a.sam = sam

	// Generate or load keys
	keys, err := sam.NewKeys()
	if err != nil {
		sam.Close()
		return fmt.Errorf("key generation failed: %w", err)
	}
	a.keys = keys

	// Create stream session
	session, err := sam.NewStreamSession("mxil-email", keys, sam3.Options_Medium)
	if err != nil {
		sam.Close()
		return fmt.Errorf("session creation failed: %w", err)
	}
	a.streamSession = session

	// Get our address
	a.base32Addr = keys.Addr().Base32()

	// Start listener
	listener, err := session.Listen()
	if err != nil {
		return fmt.Errorf("listener creation failed: %w", err)
	}
	a.listener = listener

	a.lastStatus = service.NetworkStatus{
		IsHealthy:   true,
		LastChecked: time.Now(),
	}

	return nil
}

// Disconnect closes I2P connection
func (a *I2PAdapter) Disconnect(ctx context.Context) error {
	if a.listener != nil {
		a.listener.Close()
	}
	if a.streamSession != nil {
		a.streamSession.Close()
	}
	if a.sam != nil {
		a.sam.Close()
	}

	a.lastStatus.IsHealthy = false
	return nil
}

// Send sends email via I2P
func (a *I2PAdapter) Send(ctx context.Context, email *models.Email) error {
	if a.streamSession == nil {
		return fmt.Errorf("not connected to I2P")
	}

	// Get first recipient (for now)
	if len(email.ToAddresses) == 0 {
		return fmt.Errorf("no recipients")
	}

	recipient := email.ToAddresses[0]
	if !strings.HasSuffix(recipient, ".i2p") && !strings.Contains(recipient, ".b32.i2p") {
		return fmt.Errorf("invalid I2P address: %s", recipient)
	}

	// Dial recipient
	conn, err := a.streamSession.DialContext(ctx, "i2p", recipient)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	defer conn.Close()

	// Set deadline
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(30 * time.Second))
	}

	// Build I2P email message
	message := a.buildI2PMessage(email)

	// Send message
	_, err = conn.Write([]byte(message))
	if err != nil {
		return fmt.Errorf("write failed: %w", err)
	}

	// Send end marker
	_, err = conn.Write([]byte("\n.\n"))
	if err != nil {
		return fmt.Errorf("end marker failed: %w", err)
	}

	return nil
}

// Receive receives emails from I2P
func (a *I2PAdapter) Receive(ctx context.Context) (chan *models.Email, error) {
	if a.listener == nil {
		return nil, fmt.Errorf("listener not initialized")
	}

	emailChan := make(chan *models.Email, 100)

	go a.acceptConnections(ctx, emailChan)

	return emailChan, nil
}

// acceptConnections accepts incoming I2P connections
func (a *I2PAdapter) acceptConnections(ctx context.Context, emailChan chan<- *models.Email) {
	defer close(emailChan)

	for {
		select {
		case <-ctx.Done():
			return
		default:
			a.listener.(*net.TCPListener).SetDeadline(time.Now().Add(1 * time.Second))

			conn, err := a.listener.Accept()
			if err != nil {
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				return
			}

			go a.handleConnection(ctx, conn, emailChan)
		}
	}
}

// handleConnection handles an incoming I2P connection
func (a *I2PAdapter) handleConnection(ctx context.Context, conn net.Conn, emailChan chan<- *models.Email) {
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// Read message
	var buffer []byte
	readBuffer := make([]byte, 4096)

	for {
		n, err := conn.Read(readBuffer)
		if err != nil {
			if err != io.EOF {
				return
			}
			break
		}

		buffer = append(buffer, readBuffer[:n]...)

		if strings.HasSuffix(string(buffer), "\n.\n") {
			buffer = buffer[:len(buffer)-3]
			break
		}

		if len(buffer) > 10*1024*1024 { // 10MB limit
			return
		}
	}

	// Parse message
	email := a.parseI2PMessage(string(buffer))
	if email == nil {
		return
	}

	// Set remote address
	if remoteAddr := conn.RemoteAddr(); remoteAddr != nil {
		email.FromAddress = remoteAddr.String()
	}

	select {
	case emailChan <- email:
	case <-ctx.Done():
	case <-time.After(5 * time.Second):
	}
}

// HealthCheck checks I2P connectivity
func (a *I2PAdapter) HealthCheck(ctx context.Context) (service.NetworkStatus, error) {
	start := time.Now()

	// Test SAM connection
	testSAM, err := sam3.NewSAM(a.samAddr)
	if err != nil {
		a.lastStatus = service.NetworkStatus{
			IsHealthy:   false,
			LastError:   err.Error(),
			LastChecked: time.Now(),
			Latency:     time.Since(start),
		}
		return a.lastStatus, err
	}
	testSAM.Close()

	a.lastStatus = service.NetworkStatus{
		IsHealthy:   true,
		LastChecked: time.Now(),
		Latency:     time.Since(start),
	}

	return a.lastStatus, nil
}

// buildI2PMessage builds I2P email format
func (a *I2PAdapter) buildI2PMessage(email *models.Email) string {
	var builder strings.Builder

	builder.WriteString("I2P-EMAIL-V1\n")
	builder.WriteString(fmt.Sprintf("From: %s\n", email.FromAddress))
	builder.WriteString(fmt.Sprintf("To: %s\n", strings.Join(email.ToAddresses, ", ")))
	builder.WriteString(fmt.Sprintf("Subject: %s\n", email.Subject))
	builder.WriteString(fmt.Sprintf("Message-ID: %s\n", email.MessageID))
	builder.WriteString(fmt.Sprintf("Date: %s\n", time.Now().Format(time.RFC1123Z)))

	if email.InReplyTo != "" {
		builder.WriteString(fmt.Sprintf("In-Reply-To: %s\n", email.InReplyTo))
	}

	if len(email.References) > 0 {
		builder.WriteString(fmt.Sprintf("References: %s\n", strings.Join(email.References, " ")))
	}

	builder.WriteString("Content-Type: text/plain; charset=utf-8\n\n")
	builder.WriteString(email.BodyPlain)

	return builder.String()
}

// parseI2PMessage parses I2P email format
func (a *I2PAdapter) parseI2PMessage(raw string) *models.Email {
	lines := strings.Split(raw, "\n")

	email := &models.Email{
		ID:          uuid.New(),
		MessageID:   fmt.Sprintf("<%s@i2p>", uuid.New().String()),
		ReceivedVia: models.NetworkI2P,
		ReceivedAt:  time.Now(),
	}

	inHeaders := true
	var bodyLines []string

	for i, line := range lines {
		if i == 0 && strings.HasPrefix(line, "I2P-EMAIL-V") {
			continue
		}

		if inHeaders {
			if line == "" {
				inHeaders = false
				continue
			}

			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				key := parts[0]
				value := parts[1]

				switch key {
				case "From":
					email.FromAddress = value
				case "To":
					email.ToAddresses = models.StringArray(strings.Split(value, ", "))
				case "Subject":
					email.Subject = value
				case "Message-ID":
					email.MessageID = value
				case "In-Reply-To":
					email.InReplyTo = value
				case "References":
					email.References = models.StringArray(strings.Fields(value))
				case "Date":
					if t, err := time.Parse(time.RFC1123Z, value); err == nil {
						email.SentAt = &t
					}
				}
			}
		} else {
			bodyLines = append(bodyLines, line)
		}
	}

	if len(bodyLines) > 0 {
		email.BodyPlain = strings.Join(bodyLines, "\n")
	}

	return email
}

// GetBase32Address returns our I2P address
func (a *I2PAdapter) GetBase32Address() string {
	return a.base32Addr
}
