// internal/service/types/enums.go
package types

// EmailStatus represents the status of an email
type EmailStatus string

const (
	EmailStatusDraft     EmailStatus = "draft"
	EmailStatusPending   EmailStatus = "pending"
	EmailStatusSent      EmailStatus = "sent"
	EmailStatusDelivered EmailStatus = "delivered"
	EmailStatusFailed    EmailStatus = "failed"
	EmailStatusBounced   EmailStatus = "bounced"
	EmailStatusReceived  EmailStatus = "received"
	EmailStatusRead      EmailStatus = "read"
)

// EmailDirection represents the direction of an email
type EmailDirection string

const (
	EmailDirectionIncoming EmailDirection = "incoming"
	EmailDirectionOutgoing EmailDirection = "outgoing"
)

// NetworkType represents the network type for communication
type NetworkType string

const (
	NetworkTypeClearnet NetworkType = "clearnet"
	NetworkTypeI2P      NetworkType = "i2p"
	NetworkTypeTor      NetworkType = "tor"
	NetworkTypeLAN      NetworkType = "lan"
	NetworkTypeIPFS     NetworkType = "ipfs"
)

// EncryptionAlgorithm represents encryption algorithms
type EncryptionAlgorithm string

const (
	EncryptionAESGCM128 EncryptionAlgorithm = "aes-gcm-128"
	EncryptionAESGCM256 EncryptionAlgorithm = "aes-gcm-256"
	EncryptionPGP       EncryptionAlgorithm = "pgp"
	EncryptionSMIME     EncryptionAlgorithm = "smime"
	EncryptionAutocrypt EncryptionAlgorithm = "autocrypt"
)

// AuthMethod represents authentication methods
type AuthMethod string

const (
	AuthMethodPassword AuthMethod = "password"
	AuthMethodPasskey  AuthMethod = "passkey"
	AuthMethodTOTP     AuthMethod = "totp"
	AuthMethodWebAuthn AuthMethod = "webauthn"
	AuthMethodRecovery AuthMethod = "recovery"
	AuthMethodOAuth    AuthMethod = "oauth"
)

// SessionStatus represents session status
type SessionStatus string

const (
	SessionStatusActive  SessionStatus = "active"
	SessionStatusExpired SessionStatus = "expired"
	SessionStatusRevoked SessionStatus = "revoked"
	SessionStatusInvalid SessionStatus = "invalid"
)

// UserRole represents user roles
type UserRole string

const (
	UserRoleUser      UserRole = "user"
	UserRoleAdmin     UserRole = "admin"
	UserRoleModerator UserRole = "moderator"
	UserRoleSystem    UserRole = "system"
)

// LogLevel represents log levels
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
	LogLevelFatal LogLevel = "fatal"
)

// NotificationType represents notification types
type NotificationType string

const (
	NotificationTypeEmail   NotificationType = "email"
	NotificationTypePush    NotificationType = "push"
	NotificationTypeSMS     NotificationType = "sms"
	NotificationTypeWebhook NotificationType = "webhook"
)

// SubscriptionTier represents subscription tiers
type SubscriptionTier string

const (
	SubscriptionTierFree       SubscriptionTier = "free"
	SubscriptionTierBasic      SubscriptionTier = "basic"
	SubscriptionTierPro        SubscriptionTier = "pro"
	SubscriptionTierBusiness   SubscriptionTier = "business"
	SubscriptionTierEnterprise SubscriptionTier = "enterprise"
)

// Validation functions
func IsValidEmailStatus(status EmailStatus) bool {
	switch status {
	case EmailStatusDraft, EmailStatusPending, EmailStatusSent,
		EmailStatusDelivered, EmailStatusFailed, EmailStatusBounced,
		EmailStatusReceived, EmailStatusRead:
		return true
	default:
		return false
	}
}

func IsValidNetworkType(network NetworkType) bool {
	switch network {
	case NetworkTypeClearnet, NetworkTypeI2P, NetworkTypeTor,
		NetworkTypeLAN, NetworkTypeIPFS:
		return true
	default:
		return false
	}
}

func IsValidEncryptionAlgorithm(algo EncryptionAlgorithm) bool {
	switch algo {
	case EncryptionAESGCM128, EncryptionAESGCM256,
		EncryptionPGP, EncryptionSMIME, EncryptionAutocrypt:
		return true
	default:
		return false
	}
}
