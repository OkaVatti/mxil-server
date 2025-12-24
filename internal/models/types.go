// internal/models/types.go
package models

import (
	"time"
)

// AuthMethodType represents authentication method types
type AuthMethodType string

const (
	AuthPassword  AuthMethodType = "password"
	AuthPasskey   AuthMethodType = "passkey"
	AuthPublicKey AuthMethodType = "public_key"
	AuthTOTP      AuthMethodType = "totp"
	AuthWebAuthn  AuthMethodType = "webauthn"
	AuthU2F       AuthMethodType = "u2f"
	AuthRecovery  AuthMethodType = "recovery"
	AuthOAuth     AuthMethodType = "oauth"
	AuthSSO       AuthMethodType = "sso"
)

// Timestamps is embedded in most models
type Timestamps struct {
	CreatedAt time.Time  `json:"created_at" gorm:"default:CURRENT_TIMESTAMP"`
	UpdatedAt time.Time  `json:"updated_at" gorm:"default:CURRENT_TIMESTAMP"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" gorm:"index"`
}

// Helper functions for PostgreSQL array handling
func parsePostgresArray(s string) []string {
	var result []string
	var current string
	inQuote := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				result = append(result, current)
				current = ""
			} else {
				current += string(c)
			}
		default:
			current += string(c)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	return result
}

func joinPostgresArray(arr []string) string {
	if len(arr) == 0 {
		return ""
	}

	result := ""
	for i, s := range arr {
		if i > 0 {
			result += ","
		}
		// Quote if contains special characters
		if needsQuoting(s) {
			result += `"` + s + `"`
		} else {
			result += s
		}
	}
	return result
}

func needsQuoting(s string) bool {
	for _, c := range s {
		if c == ',' || c == '"' || c == '{' || c == '}' || c == ' ' {
			return true
		}
	}
	return false
}
