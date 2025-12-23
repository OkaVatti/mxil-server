// internal/service/errors/errors.go
package errors

import "errors"

// Authentication errors
var (
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrAccountLocked      = errors.New("account is locked")
	ErrMFARequired        = errors.New("MFA token required")
	ErrInvalidMFAToken    = errors.New("invalid MFA token")
	ErrInvalidToken       = errors.New("invalid or expired token")
	ErrQuotaExceeded      = errors.New("storage quota exceeded")
	ErrNetworkUnavailable = errors.New("network unavailable")
	ErrRecipientNotFound  = errors.New("recipient not found")
	ErrWeakPassword       = errors.New("password is too weak")
	ErrEmailExists        = errors.New("email already registered")
	ErrSessionNotFound    = errors.New("session not found")
	ErrTooManySessions    = errors.New("too many active sessions")
	ErrInvalidDevice      = errors.New("unrecognized device")
)

// Email errors
var (
	ErrEmailNotFound      = errors.New("email not found")
	ErrInvalidRecipient   = errors.New("invalid recipient")
	ErrEmailQuotaExceeded = errors.New("email quota exceeded")
	ErrAttachmentTooLarge = errors.New("attachment too large")
	ErrTooManyAttachments = errors.New("too many attachments")
	ErrInvalidAttachment  = errors.New("invalid attachment")
	ErrVirusDetected      = errors.New("virus detected")
)

// Network errors
var (
	ErrNetworkTimeout      = errors.New("network timeout")
	ErrConnectionFailed    = errors.New("connection failed")
	ErrDNSResolutionFailed = errors.New("DNS resolution failed")
	ErrSMTPError           = errors.New("SMTP error")
	ErrIMAPError           = errors.New("IMAP error")
)
