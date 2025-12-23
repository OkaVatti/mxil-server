package errors

import (
	"net/http"
)

// ErrorResponse represents a structured error response
type ErrorResponse struct {
	Code      string      `json:"code"`
	Message   string      `json:"message"`
	Details   interface{} `json:"details,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
	Status    int         `json:"-"`
}

// NewErrorResponse creates a new error response
func NewErrorResponse(err error, details interface{}) *ErrorResponse {
	code := "internal_error"
	message := err.Error()
	status := http.StatusInternalServerError

	// Map specific errors to codes
	switch err {
	case ErrUnauthorized:
		code = "unauthorized"
		status = http.StatusUnauthorized
	case ErrInvalidCredentials:
		code = "invalid_credentials"
		status = http.StatusUnauthorized
	case ErrAccountLocked:
		code = "account_locked"
		status = http.StatusForbidden
	case ErrUserNotFound:
		code = "user_not_found"
		status = http.StatusNotFound
	case ErrUserExists:
		code = "user_exists"
		status = http.StatusConflict
	case ErrEmailExists:
		code = "email_exists"
		status = http.StatusConflict
	case ErrWeakPassword:
		code = "weak_password"
		status = http.StatusBadRequest
	case ErrEmailNotFound:
		code = "email_not_found"
		status = http.StatusNotFound
	case ErrRecipientNotFound:
		code = "recipient_not_found"
		status = http.StatusBadRequest
	case ErrQuotaExceeded:
		code = "quota_exceeded"
		status = http.StatusForbidden
	case ErrNetworkUnavailable:
		code = "network_unavailable"
		status = http.StatusServiceUnavailable
	case ErrRateLimitExceeded:
		code = "rate_limit_exceeded"
		status = http.StatusTooManyRequests
	case ErrValidationFailed:
		code = "validation_failed"
		status = http.StatusBadRequest
	case ErrSessionNotFound:
		code = "session_not_found"
		status = http.StatusNotFound
	case ErrTooManySessions:
		code = "too_many_sessions"
		status = http.StatusForbidden
	case ErrInvalidDevice:
		code = "invalid_device"
		status = http.StatusBadRequest
	case ErrAttachmentTooLarge:
		code = "attachment_too_large"
		status = http.StatusBadRequest
	case ErrTooManyAttachments:
		code = "too_many_attachments"
		status = http.StatusBadRequest
	case ErrInvalidAttachment:
		code = "invalid_attachment"
		status = http.StatusBadRequest
	case ErrVirusDetected:
		code = "virus_detected"
		status = http.StatusBadRequest
	}

	return &ErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
		Status:  status,
	}
}
