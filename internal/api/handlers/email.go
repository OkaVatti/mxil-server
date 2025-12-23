package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service"
)

// EmailHandler handles email requests
type EmailHandler struct {
	emailService service.EmailService
	emailRepo    *repository.EmailRepository
	userRepo     *repository.UserRepository
	logger       *zap.Logger
}

// NewEmailHandler creates a new email handler
func NewEmailHandler(
	emailService service.EmailService,
	emailRepo *repository.EmailRepository,
	userRepo *repository.UserRepository,
	logger *zap.Logger,
) *EmailHandler {
	return &EmailHandler{
		emailService: emailService,
		emailRepo:    emailRepo,
		userRepo:     userRepo,
		logger:       logger,
	}
}

// ListEmails lists emails with filtering
func (h *EmailHandler) ListEmails(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	// Parse query parameters
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	if offset < 0 {
		offset = 0
	}

	folderID := c.QueryParam("folder")
	labelID := c.QueryParam("label")
	search := c.QueryParam("search")
	isRead := c.QueryParam("read")
	isStarred := c.QueryParam("starred")
	isArchived := c.QueryParam("archived")
	isSpam := c.QueryParam("spam")
	network := c.QueryParam("network")
	fromDate := c.QueryParam("from_date")
	toDate := c.QueryParam("to_date")

	// Build filter
	filter := repository.EmailFilter{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
		Search: search,
	}

	// Parse folder ID
	if folderID != "" {
		folderUUID, err := uuid.Parse(folderID)
		if err == nil {
			filter.FolderID = &folderUUID
		}
	}

	// Parse label ID
	if labelID != "" {
		labelUUID, err := uuid.Parse(labelID)
		if err == nil {
			filter.LabelIDs = []uuid.UUID{labelUUID}
		}
	}

	// Parse boolean flags
	if isRead != "" {
		read, err := strconv.ParseBool(isRead)
		if err == nil {
			filter.IsRead = &read
		}
	}

	if isStarred != "" {
		starred, err := strconv.ParseBool(isStarred)
		if err == nil {
			filter.IsStarred = &starred
		}
	}

	if isArchived != "" {
		archived, err := strconv.ParseBool(isArchived)
		if err == nil {
			filter.IsArchived = &archived
		}
	}

	if isSpam != "" {
		spam, err := strconv.ParseBool(isSpam)
		if err == nil {
			filter.IsSpam = &spam
		}
	}

	// Parse network type
	if network != "" {
		networkType := models.NetworkType(network)
		filter.Network = &networkType
	}

	// Parse date range
	if fromDate != "" {
		fromTime, err := time.Parse(time.RFC3339, fromDate)
		if err == nil {
			filter.FromDate = &fromTime
		}
	}

	if toDate != "" {
		toTime, err := time.Parse(time.RFC3339, toDate)
		if err == nil {
			filter.ToDate = &toTime
		}
	}

	ctx := c.Request().Context()
	emails, total, err := h.emailRepo.List(ctx, filter)
	if err != nil {
		h.logger.Error("Failed to list emails", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "list_failed",
			"message": "Failed to retrieve emails",
		})
	}

	// Get unread count
	unreadCount, _ := h.emailRepo.CountUnread(ctx, userID)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"emails":       emails,
		"total":        total,
		"limit":        limit,
		"offset":       offset,
		"unread_count": unreadCount,
	})
}

// GetEmail retrieves a single email
func (h *EmailHandler) GetEmail(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	emailID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_email_id",
			"message": "Invalid email ID",
		})
	}

	ctx := c.Request().Context()
	email, err := h.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		h.logger.Error("Failed to get email", zap.Error(err))
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "email_not_found",
			"message": "Email not found",
		})
	}

	// Check ownership
	if email.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to access this email",
		})
	}

	// Mark as read if not already
	if !email.IsRead {
		if err := h.emailRepo.MarkAsRead(ctx, email.ID, true); err != nil {
			h.logger.Warn("Failed to mark email as read", zap.Error(err))
		}
		email.IsRead = true
	}

	return c.JSON(http.StatusOK, email)
}

// SendEmail handles sending a new email
func (h *EmailHandler) SendEmail(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	var req struct {
		To           []string                 `json:"to" validate:"required"`
		Cc           []string                 `json:"cc,omitempty"`
		Bcc          []string                 `json:"bcc,omitempty"`
		Subject      string                   `json:"subject" validate:"required"`
		BodyPlain    string                   `json:"body_plain,omitempty"`
		BodyHTML     string                   `json:"body_html,omitempty"`
		BodyMarkdown string                   `json:"body_markdown,omitempty"`
		Network      models.NetworkType       `json:"network" validate:"required"`
		Attachments  []service.AttachmentInfo `json:"attachments,omitempty"`
		Encryption   service.EncryptionConfig `json:"encryption,omitempty"`
		InReplyTo    string                   `json:"in_reply_to,omitempty"`
		References   []string                 `json:"references,omitempty"`
		DraftID      *uuid.UUID               `json:"draft_id,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	// Validate recipients
	if len(req.To) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "no_recipients",
			"message": "At least one recipient is required",
		})
	}

	// Validate email addresses
	for _, addr := range req.To {
		if !isValidEmail(addr) {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_email",
				"message": fmt.Sprintf("Invalid email address: %s", addr),
			})
		}
	}

	// Check content
	if req.BodyPlain == "" && req.BodyHTML == "" && req.BodyMarkdown == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "no_content",
			"message": "Email must have content",
		})
	}

	// Process attachments
	var attachments []service.AttachmentInfo
	for _, att := range req.Attachments {
		// Decode base64 data
		data, err := base64.StdEncoding.DecodeString(att.Data)
		if err != nil {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_attachment",
				"message": fmt.Sprintf("Invalid attachment data: %s", att.Filename),
			})
		}

		attachments = append(attachments, service.AttachmentInfo{
			Filename:    att.Filename,
			ContentType: att.ContentType,
			Size:        int64(len(data)),
			Data:        att.Data,
			IsInline:    att.IsInline,
		})
	}

	sendReq := service.SendEmailRequest{
		To:           req.To,
		Cc:           req.Cc,
		Bcc:          req.Bcc,
		Subject:      req.Subject,
		BodyPlain:    req.BodyPlain,
		BodyHTML:     req.BodyHTML,
		BodyMarkdown: req.BodyMarkdown,
		Network:      req.Network,
		Attachments:  attachments,
		Encryption:   req.Encryption,
		InReplyTo:    req.InReplyTo,
		References:   req.References,
		DraftID:      req.DraftID,
	}

	ctx := c.Request().Context()
	email, err := h.emailService.SendEmail(ctx, userID, sendReq)
	if err != nil {
		h.logger.Error("Failed to send email", zap.Error(err))

		if err == service.ErrQuotaExceeded {
			return c.JSON(http.StatusForbidden, map[string]interface{}{
				"error":   "quota_exceeded",
				"message": "Storage quota exceeded",
			})
		}

		if err == service.ErrNetworkUnavailable {
			return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
				"error":   "network_unavailable",
				"message": "Network is currently unavailable",
			})
		}

		if err == service.ErrRecipientNotFound {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "recipient_not_found",
				"message": "One or more recipients not found",
			})
		}

		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "send_failed",
			"message": "Failed to send email",
		})
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"email":   email,
		"message": "Email sent successfully",
		"sent_at": time.Now(),
	})
}

// MarkAsRead marks an email as read/unread
func (h *EmailHandler) MarkAsRead(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	emailID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_email_id",
			"message": "Invalid email ID",
		})
	}

	var req struct {
		Read bool `json:"read"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()

	// First get email to check ownership
	email, err := h.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "email_not_found",
			"message": "Email not found",
		})
	}

	if email.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to modify this email",
		})
	}

	// Update read status
	if err := h.emailRepo.MarkAsRead(ctx, emailID, req.Read); err != nil {
		h.logger.Error("Failed to mark email as read", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "update_failed",
			"message": "Failed to update email status",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":  fmt.Sprintf("Email marked as %s", map[bool]string{true: "read", false: "unread"}[req.Read]),
		"email_id": emailID,
		"read":     req.Read,
	})
}

// MarkAsStarred marks an email as starred/unstarred
func (h *EmailHandler) MarkAsStarred(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	emailID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_email_id",
			"message": "Invalid email ID",
		})
	}

	var req struct {
		Starred bool `json:"starred"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()

	// First get email to check ownership
	email, err := h.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "email_not_found",
			"message": "Email not found",
		})
	}

	if email.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to modify this email",
		})
	}

	// Update starred status
	if err := h.emailRepo.MarkAsStarred(ctx, emailID, req.Starred); err != nil {
		h.logger.Error("Failed to mark email as starred", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "update_failed",
			"message": "Failed to update email status",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":  fmt.Sprintf("Email %s", map[bool]string{true: "starred", false: "unstarred"}[req.Starred]),
		"email_id": emailID,
		"starred":  req.Starred,
	})
}

// MoveToFolder moves an email to a folder
func (h *EmailHandler) MoveToFolder(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	emailID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_email_id",
			"message": "Invalid email ID",
		})
	}

	var req struct {
		FolderID uuid.UUID `json:"folder_id" validate:"required"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()

	// Check email ownership
	email, err := h.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "email_not_found",
			"message": "Email not found",
		})
	}

	if email.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to modify this email",
		})
	}

	// TODO: Check folder ownership
	// This would require a folder repository

	// Move email
	if err := h.emailRepo.MoveToFolder(ctx, emailID, req.FolderID); err != nil {
		h.logger.Error("Failed to move email", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "move_failed",
			"message": "Failed to move email",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":   "Email moved successfully",
		"email_id":  emailID,
		"folder_id": req.FolderID,
	})
}

// DeleteEmail soft deletes an email
func (h *EmailHandler) DeleteEmail(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	emailID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_email_id",
			"message": "Invalid email ID",
		})
	}

	ctx := c.Request().Context()

	// Check email ownership
	email, err := h.emailRepo.GetByID(ctx, emailID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "email_not_found",
			"message": "Email not found",
		})
	}

	if email.UserID != userID {
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":   "access_denied",
			"message": "You don't have permission to delete this email",
		})
	}

	// Soft delete
	if err := h.emailRepo.Delete(ctx, emailID); err != nil {
		h.logger.Error("Failed to delete email", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "delete_failed",
			"message": "Failed to delete email",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message":  "Email deleted successfully",
		"email_id": emailID,
	})
}

// SearchEmails searches emails with full-text search
func (h *EmailHandler) SearchEmails(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	query := c.QueryParam("q")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	offset, _ := strconv.Atoi(c.QueryParam("offset"))
	if offset < 0 {
		offset = 0
	}

	ctx := c.Request().Context()
	emails, total, err := h.emailService.SearchEmails(ctx, userID, query, limit, offset)
	if err != nil {
		h.logger.Error("Search failed", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "search_failed",
			"message": "Failed to search emails",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"emails": emails,
		"total":  total,
		"limit":  limit,
		"offset": offset,
		"query":  query,
	})
}

// GetThread retrieves an email thread
func (h *EmailHandler) GetThread(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	threadID, err := uuid.Parse(c.Param("threadId"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_thread_id",
			"message": "Invalid thread ID",
		})
	}

	ctx := c.Request().Context()
	emails, err := h.emailService.GetEmailThread(ctx, userID, threadID)
	if err != nil {
		h.logger.Error("Failed to get thread", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "thread_failed",
			"message": "Failed to retrieve thread",
		})
	}

	if len(emails) == 0 {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "thread_not_found",
			"message": "Thread not found",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"thread_id": threadID,
		"emails":    emails,
		"count":     len(emails),
	})
}

// CheckDomainAvailability checks if a domain is available
func (h *EmailHandler) CheckDomainAvailability(c echo.Context) error {
	domain := c.QueryParam("domain")
	if domain == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "missing_domain",
			"message": "Domain parameter is required",
		})
	}

	// TODO: Implement domain availability check
	// This would check if the domain is already registered or blacklisted

	return c.JSON(http.StatusOK, map[string]interface{}{
		"domain":      domain,
		"available":   true, // Placeholder
		"suggestions": []string{},
	})
}

// ValidateEmail validates an email address
func (h *EmailHandler) ValidateEmail(c echo.Context) error {
	var req struct {
		Email string `json:"email" validate:"required,email"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	// Simple validation
	valid := isValidEmail(req.Email)

	// TODO: Add DNS lookup for MX records
	// TODO: Add SMTP verification (optional, can be slow)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"email":      req.Email,
		"valid":      valid,
		"format":     valid,
		"mx_found":   true, // Placeholder
		"disposable": isDisposableEmail(req.Email),
	})
}

// GetDomainConfig retrieves domain configuration
func (h *EmailHandler) GetDomainConfig(c echo.Context) error {
	domain := c.Param("domain")
	if domain == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "missing_domain",
			"message": "Domain parameter is required",
		})
	}

	// TODO: Implement domain config retrieval
	// This would return MX records, SPF, DKIM, DMARC settings

	return c.JSON(http.StatusOK, map[string]interface{}{
		"domain": domain,
		"mx_records": []map[string]interface{}{
			{"priority": 10, "server": fmt.Sprintf("mx1.%s", domain)},
			{"priority": 20, "server": fmt.Sprintf("mx2.%s", domain)},
		},
		"spf":          "v=spf1 include:_spf.%s ~all",
		"dkim":         true,
		"dmarc":        "v=DMARC1; p=none; rua=mailto:dmarc@%s",
		"tls_enforced": true,
	})
}

// Helper functions
func isValidEmail(email string) bool {
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}

	if parts[0] == "" || parts[1] == "" {
		return false
	}

	// Check for valid characters
	for _, part := range parts {
		if strings.Contains(part, "..") {
			return false
		}
		if strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".") {
			return false
		}
	}

	return true
}

func isDisposableEmail(email string) bool {
	disposableDomains := []string{
		"tempmail.com", "mailinator.com", "guerrillamail.com",
		"10minutemail.com", "yopmail.com", "trashmail.com",
	}

	domain := strings.ToLower(strings.Split(email, "@")[1])
	for _, d := range disposableDomains {
		if strings.Contains(domain, d) {
			return true
		}
	}

	return false
}
