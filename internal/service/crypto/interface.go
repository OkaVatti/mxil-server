// internal/service/crypto/interface.go
package crypto

// CryptoService handles cryptographic operations
type CryptoService interface {
	HashPassword(password string) (string, error)
	VerifyPassword(hashedPassword, password string) bool
	GenerateSecureToken(length int) string
	HashToken(token string) string
	EncryptEmail(content string, keyIDs []string, algorithm string) ([]byte, []string, error)
}
