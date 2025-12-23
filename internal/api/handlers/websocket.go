// internal/api/handlers/websocket.go
package handlers

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"golang.org/x/net/websocket"

	"github.com/okavatti/mxil-server/m/internal/service"
)

// WebSocketHandler handles WebSocket connections
type WebSocketHandler struct {
	connections  sync.Map // userID -> []*WebSocketConnection
	authService  service.AuthService
	emailService service.EmailService
	logger       *zap.Logger
}

// NewWebSocketHandler creates a new WebSocket handler
func NewWebSocketHandler(
	authService service.AuthService,
	emailService service.EmailService,
	logger *zap.Logger,
) *WebSocketHandler {
	return &WebSocketHandler{
		authService:  authService,
		emailService: emailService,
		logger:       logger,
	}
}

// WebSocketConnection represents a WebSocket connection
type WebSocketConnection struct {
	ID         string
	UserID     uuid.UUID
	Connection *websocket.Conn
	Send       chan []byte
	Done       chan bool
	LastPing   time.Time
}

// WebSocketMessage represents a WebSocket message
type WebSocketMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// HandleWebSocket handles WebSocket connections
func (h *WebSocketHandler) HandleWebSocket(c echo.Context) error {
	// Get authentication token from query parameter
	token := c.QueryParam("token")
	if token == "" {
		return c.JSON(401, map[string]string{
			"error":   "unauthorized",
			"message": "Authentication token required",
		})
	}

	// Validate token and get user
	ctx := c.Request().Context()
	session, err := h.authService.ValidateSession(ctx, token)
	if err != nil {
		return c.JSON(401, map[string]string{
			"error":   "unauthorized",
			"message": "Invalid or expired token",
		})
	}

	// Upgrade to WebSocket
	websocket.Handler(func(ws *websocket.Conn) {
		defer ws.Close()

		// Create connection
		conn := &WebSocketConnection{
			ID:         uuid.New().String(),
			UserID:     session.UserID,
			Connection: ws,
			Send:       make(chan []byte, 256),
			Done:       make(chan bool),
			LastPing:   time.Now(),
		}

		// Register connection
		h.registerConnection(session.UserID, conn)
		defer h.unregisterConnection(session.UserID, conn)

		h.logger.Info("WebSocket connected",
			zap.String("connection_id", conn.ID),
			zap.String("user_id", session.UserID.String()))

		// Start writer goroutine
		go h.writePump(conn)

		// Handle incoming messages
		h.readPump(conn)

		h.logger.Info("WebSocket disconnected",
			zap.String("connection_id", conn.ID),
			zap.String("user_id", session.UserID.String()))
	}).ServeHTTP(c.Response(), c.Request())

	return nil
}

// readPump handles incoming WebSocket messages
func (h *WebSocketHandler) readPump(conn *WebSocketConnection) {
	defer func() {
		conn.Done <- true
	}()

	for {
		var msg WebSocketMessage
		err := websocket.JSON.Receive(conn.Connection, &msg)
		if err != nil {
			h.logger.Debug("WebSocket read error",
				zap.String("connection_id", conn.ID),
				zap.Error(err))
			break
		}

		h.handleMessage(conn, msg)
	}
}

// writePump sends messages to the WebSocket connection
func (h *WebSocketHandler) writePump(conn *WebSocketConnection) {
	ticker := time.NewTicker(30 * time.Second) // Ping interval
	defer ticker.Stop()

	for {
		select {
		case message := <-conn.Send:
			err := websocket.Message.Send(conn.Connection, message)
			if err != nil {
				h.logger.Debug("WebSocket write error",
					zap.String("connection_id", conn.ID),
					zap.Error(err))
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			pingMsg := WebSocketMessage{
				Type: "ping",
				Payload: map[string]interface{}{
					"timestamp": time.Now().Unix(),
				},
			}
			data, _ := json.Marshal(pingMsg)
			conn.Send <- data

			// Check if connection is stale (no pong in 60 seconds)
			if time.Since(conn.LastPing) > 60*time.Second {
				h.logger.Debug("WebSocket connection stale, closing",
					zap.String("connection_id", conn.ID))
				conn.Connection.Close()
				return
			}

		case <-conn.Done:
			return
		}
	}
}

// handleMessage processes incoming WebSocket messages
func (h *WebSocketHandler) handleMessage(conn *WebSocketConnection, msg WebSocketMessage) {
	switch msg.Type {
	case "ping":
		// Update last ping time
		conn.LastPing = time.Now()

		// Send pong response
		pongMsg := WebSocketMessage{
			Type: "pong",
			Payload: map[string]interface{}{
				"timestamp": time.Now().Unix(),
			},
		}
		data, _ := json.Marshal(pongMsg)
		conn.Send <- data

	case "subscribe":
		h.handleSubscribe(conn, msg)

	case "unsubscribe":
		h.handleUnsubscribe(conn, msg)

	case "email_sent":
		h.handleEmailSent(conn, msg)

	case "mark_read":
		h.handleMarkRead(conn, msg)

	case "mark_starred":
		h.handleMarkStarred(conn, msg)

	default:
		h.sendError(conn, "unknown_message_type", "Unknown message type")
	}
}

// handleSubscribe handles subscription requests
func (h *WebSocketHandler) handleSubscribe(conn *WebSocketConnection, msg WebSocketMessage) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		h.sendError(conn, "invalid_payload", "Invalid subscription payload")
		return
	}

	channel, _ := payload["channel"].(string)
	params, _ := payload["params"].(map[string]interface{})

	switch channel {
	case "new_emails":
		// Subscribe to new email notifications
		h.subscribeToNewEmails(conn, params)

	case "email_updates":
		// Subscribe to email updates (read/unread, starred, etc.)
		h.subscribeToEmailUpdates(conn, params)

	case "notifications":
		// Subscribe to system notifications
		h.subscribeToNotifications(conn)

	case "presence":
		// Subscribe to user presence
		h.subscribeToPresence(conn, params)

	default:
		h.sendError(conn, "unknown_channel", "Unknown subscription channel")
		return
	}

	// Send subscription confirmation
	response := WebSocketMessage{
		Type: "subscribed",
		Payload: map[string]interface{}{
			"channel": channel,
			"params":  params,
		},
	}
	data, _ := json.Marshal(response)
	conn.Send <- data
}

// handleUnsubscribe handles unsubscription requests
func (h *WebSocketHandler) handleUnsubscribe(conn *WebSocketConnection, msg WebSocketMessage) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		h.sendError(conn, "invalid_payload", "Invalid unsubscription payload")
		return
	}

	channel, _ := payload["channel"].(string)

	// TODO: Implement actual unsubscription logic

	response := WebSocketMessage{
		Type: "unsubscribed",
		Payload: map[string]interface{}{
			"channel": channel,
		},
	}
	data, _ := json.Marshal(response)
	conn.Send <- data
}

// handleEmailSent handles email sent notifications
func (h *WebSocketHandler) handleEmailSent(conn *WebSocketConnection, msg WebSocketMessage) {
	payload, ok := msg.Payload.(map[string]interface{})
	if !ok {
		h.sendError(conn, "invalid_payload", "Invalid email sent payload")
		return
	}

	emailID, _ := payload["email_id"].(string)
	status, _ := payload["status"].(string)

	// Notify other connections for the same user
	h.broadcastToUser(conn.UserID, WebSocketMessage{
		Type: "email_status",
		Payload: map[string]interface{}{
			"email_id":   emailID,
			"status":     status,
			"updated_at": time.Now(),
		},
	})
}

// handleMarkRead handles email read status updates
func (h *WebSocketHandler) handleMarkRead(conn *WebSocketConnection, msg WebSocketMessage) {
	// TODO: Implement email read status update
}

// handleMarkStarred handles email starred status updates
func (h *WebSocketHandler) handleMarkStarred(conn *WebSocketConnection, msg WebSocketMessage) {
	// TODO: Implement email starred status update
}

// Helper methods
func (h *WebSocketHandler) registerConnection(userID uuid.UUID, conn *WebSocketConnection) {
	connections, _ := h.connections.LoadOrStore(userID.String(), []*WebSocketConnection{})
	conns := connections.([]*WebSocketConnection)
	conns = append(conns, conn)
	h.connections.Store(userID.String(), conns)
}

func (h *WebSocketHandler) unregisterConnection(userID uuid.UUID, conn *WebSocketConnection) {
	connections, ok := h.connections.Load(userID.String())
	if !ok {
		return
	}

	conns := connections.([]*WebSocketConnection)
	for i, c := range conns {
		if c.ID == conn.ID {
			conns = append(conns[:i], conns[i+1:]...)
			break
		}
	}

	if len(conns) == 0 {
		h.connections.Delete(userID.String())
	} else {
		h.connections.Store(userID.String(), conns)
	}
}

func (h *WebSocketHandler) broadcastToUser(userID uuid.UUID, msg WebSocketMessage) {
	connections, ok := h.connections.Load(userID.String())
	if !ok {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("Failed to marshal WebSocket message", zap.Error(err))
		return
	}

	conns := connections.([]*WebSocketConnection)
	for _, conn := range conns {
		select {
		case conn.Send <- data:
			// Message sent
		default:
			// Channel full, connection might be stuck
			h.logger.Warn("WebSocket send channel full",
				zap.String("connection_id", conn.ID))
		}
	}
}

func (h *WebSocketHandler) sendError(conn *WebSocketConnection, code, message string) {
	errorMsg := WebSocketMessage{
		Type:  "error",
		Error: code,
		Payload: map[string]interface{}{
			"message": message,
		},
	}
	data, _ := json.Marshal(errorMsg)
	conn.Send <- data
}

func (h *WebSocketHandler) subscribeToNewEmails(conn *WebSocketConnection, params map[string]interface{}) {
	// TODO: Implement new email subscription
	// This would monitor the database for new emails for this user
}

func (h *WebSocketHandler) subscribeToEmailUpdates(conn *WebSocketConnection, params map[string]interface{}) {
	// TODO: Implement email updates subscription
	// This would monitor email status changes
}

func (h *WebSocketHandler) subscribeToNotifications(conn *WebSocketConnection) {
	// TODO: Implement notifications subscription
	// This would send system notifications to the user
}

func (h *WebSocketHandler) subscribeToPresence(conn *WebSocketConnection, params map[string]interface{}) {
	// TODO: Implement presence subscription
	// This would track user online status
}

// NotifyNewEmail sends a new email notification to a user
func (h *WebSocketHandler) NotifyNewEmail(userID uuid.UUID, emailID uuid.UUID, subject string) {
	msg := WebSocketMessage{
		Type: "new_email",
		Payload: map[string]interface{}{
			"email_id":    emailID.String(),
			"subject":     subject,
			"received_at": time.Now(),
		},
	}
	h.broadcastToUser(userID, msg)
}

// NotifyEmailRead sends an email read notification
func (h *WebSocketHandler) NotifyEmailRead(userID uuid.UUID, emailID uuid.UUID, read bool) {
	msg := WebSocketMessage{
		Type: "email_read",
		Payload: map[string]interface{}{
			"email_id":   emailID.String(),
			"read":       read,
			"updated_at": time.Now(),
		},
	}
	h.broadcastToUser(userID, msg)
}

// NotifyEmailStarred sends an email starred notification
func (h *WebSocketHandler) NotifyEmailStarred(userID uuid.UUID, emailID uuid.UUID, starred bool) {
	msg := WebSocketMessage{
		Type: "email_starred",
		Payload: map[string]interface{}{
			"email_id":   emailID.String(),
			"starred":    starred,
			"updated_at": time.Now(),
		},
	}
	h.broadcastToUser(userID, msg)
}

// NotifySystemMessage sends a system message to a user
func (h *WebSocketHandler) NotifySystemMessage(userID uuid.UUID, title, message, level string) {
	msg := WebSocketMessage{
		Type: "system_message",
		Payload: map[string]interface{}{
			"title":     title,
			"message":   message,
			"level":     level,
			"timestamp": time.Now(),
		},
	}
	h.broadcastToUser(userID, msg)
}
