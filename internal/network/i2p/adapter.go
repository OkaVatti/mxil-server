// internal/network/i2p/adapter.go
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
	sam3 "github.com/go-i2p/sam3" // Changed import path
)

// I2PAdapter handles I2P network operations using the SAMv3 protocol.
type I2PAdapter struct {
	samAddr       string              // Address of the SAM bridge (e.g., "127.0.0.1:7656")
	streamSession *sam3.StreamSession // SAM stream session for reliable communication
	lastStatus    service.NetworkStatus
	base32Addr    string           // Our I2P destination address (base32)
	sam           *sam3.SAM        // SAM connection
	keys          *i2pkeys.I2PKeys // I2P keys for this session
}

// NewI2PAdapter creates a new I2P adapter.
// samAddr is typically "127.0.0.1:7656" for the default SAM bridge.
func NewI2PAdapter(samAddr string) *I2PAdapter {
	return &I2PAdapter{
		samAddr: samAddr,
		lastStatus: service.NetworkStatus{
			IsHealthy:   false,
			LastChecked: time.Now(),
		},
	}
}

// Connect establishes a connection to the I2P network via the SAM bridge.
func (a *I2PAdapter) Connect(ctx context.Context) error {
	sam, err := sam3.NewSAM(a.samAddr)
	if err != nil {
		a.updateStatus(false, fmt.Sprintf("Failed to connect to SAM bridge: %v", err))
		return err
	}
	a.sam = sam

	// Generate a new I2P session with a random destination for anonymity.
	keys, err := sam.NewKeys()
	if err != nil {
		sam.Close()
		a.updateStatus(false, fmt.Sprintf("Failed to generate I2P keys: %v", err))
		return err
	}
	a.keys = &keys

	// Create a streaming session (reliable, ordered delivery, suitable for email).
	session, err := sam.NewStreamSession("mxil-session", keys, sam3.Options_Medium)
	if err != nil {
		sam.Close()
		a.updateStatus(false, fmt.Sprintf("Failed to create I2P stream session: %v", err))
		return err
	}

	a.streamSession = session
	a.base32Addr = keys.Addr().Base32() // Save our public I2P address

	a.updateStatus(true, "")
	return nil
}

// Disconnect closes the SAM session and cleans up resources.
func (a *I2PAdapter) Disconnect(ctx context.Context) error {
	if a.streamSession != nil {
		a.streamSession.Close()
		a.streamSession = nil
	}
	if a.sam != nil {
		a.sam.Close()
		a.sam = nil
	}
	// Reset keys to zero value
	var zeroKeys i2pkeys.I2PKeys
	a.keys = &zeroKeys
	a.updateStatus(false, "Disconnected")
	return nil
}

// Send sends an email to an I2P destination address.
// The recipient address in email.ToAddresses should be a valid I2P base32 address (.i2p).
func (a *I2PAdapter) Send(ctx context.Context, email *models.Email) error {
	if a.streamSession == nil {
		return fmt.Errorf("not connected to I2P network")
	}

	// For simplicity, we send to the first recipient.
	// In production, you would iterate and handle each.
	if len(email.ToAddresses) == 0 {
		return fmt.Errorf("no recipient address provided")
	}
	recipient := email.ToAddresses[0]

	// Clean and validate the recipient address
	recipient = strings.TrimSpace(recipient)
	if !strings.HasSuffix(recipient, ".i2p") && !strings.Contains(recipient, ".b32.i2p") {
		return fmt.Errorf("invalid I2P address format: %s", recipient)
	}

	// Dial the recipient's I2P address.
	conn, err := a.streamSession.Dial("i2p", recipient)
	if err != nil {
		a.updateStatus(false, fmt.Sprintf("Failed to dial I2P recipient %s: %v", recipient, err))
		return err
	}
	defer conn.Close()

	// Set deadlines for the connection
	if deadline, ok := ctx.Deadline(); ok {
		conn.SetDeadline(deadline)
	} else {
		conn.SetDeadline(time.Now().Add(30 * time.Second))
	}

	// Convert email to a simple string format for transmission.
	message := a.convertToI2PMessage(email)

	// Write the message
	_, err = conn.Write([]byte(message))
	if err != nil {
		a.updateStatus(false, fmt.Sprintf("Failed to send data to %s: %v", recipient, err))
		return err
	}

	// Send end-of-message marker
	_, err = conn.Write([]byte("\n.\n"))
	if err != nil {
		a.updateStatus(false, fmt.Sprintf("Failed to send EOM to %s: %v", recipient, err))
		return err
	}

	a.updateStatus(true, "")
	return nil
}

// Receive starts listening for incoming I2P connections and emails.
// It returns a channel where received emails will be sent.
func (a *I2PAdapter) Receive(ctx context.Context) (chan *models.Email, error) {
	emailChan := make(chan *models.Email, 100)

	if a.streamSession == nil {
		return nil, fmt.Errorf("not connected to I2P network")
	}

	// Start listening for incoming connections.
	listener, err := a.streamSession.Listen()
	if err != nil {
		return nil, fmt.Errorf("failed to start I2P listener: %v", err)
	}

	go a.listenForConnections(ctx, listener, emailChan)

	return emailChan, nil
}

// HealthCheck verifies the I2P network and SAM bridge are accessible.
func (a *I2PAdapter) HealthCheck(ctx context.Context) (service.NetworkStatus, error) {
	start := time.Now()

	// Try to create a temporary SAM connection to test the bridge.
	testSam, err := sam3.NewSAM(a.samAddr)
	if err != nil {
		latency := time.Since(start)
		a.lastStatus.Latency = latency
		a.updateStatus(false, fmt.Sprintf("SAM bridge unreachable: %v", err))
		return a.lastStatus, err
	}
	testSam.Close()

	// If we have a session, also verify we can still perform a basic operation.
	if a.streamSession != nil {
		// A simple check: ensure our local destination is still valid.
		if a.base32Addr == "" {
			latency := time.Since(start)
			a.lastStatus.Latency = latency
			a.updateStatus(false, "I2P session has no valid address")
			return a.lastStatus, fmt.Errorf("I2P session invalid")
		}
	}

	latency := time.Since(start)
	a.lastStatus.Latency = latency
	a.updateStatus(true, "")
	return a.lastStatus, nil
}

// listenForConnections accepts incoming I2P connections and processes them.
func (a *I2PAdapter) listenForConnections(ctx context.Context, listener net.Listener, emailChan chan<- *models.Email) {
	defer close(emailChan)
	defer listener.Close()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Use a goroutine with timeout for non-blocking Accept
			connChan := make(chan net.Conn, 1)
			errChan := make(chan error, 1)

			go func() {
				conn, err := listener.Accept()
				if err != nil {
					errChan <- err
					return
				}
				connChan <- conn
			}()

			select {
			case <-time.After(1 * time.Second):
				continue // Timeout, check context
			case err := <-errChan:
				if ctx.Err() != nil {
					return // Context cancelled
				}
				if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
					continue
				}
				fmt.Printf("Error accepting I2P connection: %v\n", err)
				continue
			case conn := <-connChan:
				go a.handleIncomingConnection(ctx, conn, emailChan)
			}
		}
	}
}

// handleIncomingConnection reads data from a connection and converts it to an Email model.
func (a *I2PAdapter) handleIncomingConnection(ctx context.Context, conn net.Conn, emailChan chan<- *models.Email) {
	defer conn.Close()

	// Set read deadline
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))

	// Read the entire message
	buffer := make([]byte, 0, 65536)
	readBuffer := make([]byte, 4096)

	for {
		n, err := conn.Read(readBuffer)
		if err != nil {
			if err != io.EOF {
				fmt.Printf("Error reading from I2P connection: %v\n", err)
			}
			break
		}
		buffer = append(buffer, readBuffer[:n]...)

		// Check for end-of-message marker
		if n > 0 && strings.HasSuffix(string(buffer), "\n.\n") {
			buffer = buffer[:len(buffer)-3]
			break
		}

		// Prevent buffer overflow
		if len(buffer) > 65536 {
			fmt.Printf("Message too large, truncating\n")
			break
		}
	}

	// Parse the raw data into an Email model.
	rawMessage := string(buffer)
	email, err := a.parseI2PMessage(rawMessage)
	if err != nil {
		fmt.Printf("Failed to parse I2P message: %v\n", err)
		return
	}

	// Extract remote address if available
	if remoteAddr := conn.RemoteAddr(); remoteAddr != nil {
		email.FromAddress = remoteAddr.String()
	}

	select {
	case emailChan <- email:
		// Successfully queued the email for further processing.
	case <-ctx.Done():
		// Context cancelled, drop the email.
	case <-time.After(5 * time.Second):
		fmt.Printf("Timeout trying to send email to channel\n")
	}
}

// updateStatus is a helper to update the lastStatus field.
func (a *I2PAdapter) updateStatus(healthy bool, lastError string) {
	a.lastStatus = service.NetworkStatus{
		IsHealthy:    healthy,
		LastError:    lastError,
		LastChecked:  time.Now(),
		MessageCount: a.lastStatus.MessageCount,
		Latency:      a.lastStatus.Latency,
	}
}

// convertToI2PMessage transforms an Email model into a string for I2P transmission.
func (a *I2PAdapter) convertToI2PMessage(email *models.Email) string {
	headers := fmt.Sprintf(
		"I2P-EMAIL-V1\nFrom: %s\nTo: %s\nSubject: %s\nDate: %s\nMessage-ID: %s\n",
		email.FromAddress,
		joinAddresses(email.ToAddresses),
		email.Subject,
		time.Now().Format(time.RFC1123Z),
		email.MessageID,
	)

	if email.InReplyTo != "" {
		headers += fmt.Sprintf("In-Reply-To: %s\n", email.InReplyTo)
	}

	if len(email.References) > 0 {
		headers += fmt.Sprintf("References: %s\n", strings.Join(email.References, " "))
	}

	headers += fmt.Sprintf("Content-Type: text/plain; charset=utf-8\n\n")

	return headers + email.BodyPlain
}

// parseI2PMessage attempts to parse a raw string into an Email model.
func (a *I2PAdapter) parseI2PMessage(raw string) (*models.Email, error) {
	lines := strings.Split(raw, "\n")

	email := &models.Email{
		ID:          uuid.New(),
		MessageID:   fmt.Sprintf("<%s@mxil.i2p>", uuid.New().String()),
		FromAddress: "unknown@i2p",
		ToAddresses: []string{a.base32Addr},
		Subject:     "I2P Email",
		BodyPlain:   raw,
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
					email.ToAddresses = []string{value}
				case "Subject":
					email.Subject = value
				case "Message-ID":
					email.MessageID = value
				case "In-Reply-To":
					email.InReplyTo = value
				case "References":
					email.References = strings.Split(value, " ")
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

	return email, nil
}

// joinAddresses is a helper to join a slice of addresses into a comma-separated string.
func joinAddresses(addresses []string) string {
	return strings.Join(addresses, ", ")
}

// GetBase32Address returns our I2P base32 address.
func (a *I2PAdapter) GetBase32Address() string {
	return a.base32Addr
}
