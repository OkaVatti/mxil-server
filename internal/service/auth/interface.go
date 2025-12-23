package auth

import (
	"context"

	"github.com/google/uuid"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// UserRepository defines the user repository interface
type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	GetByUsername(ctx context.Context, username string) (*models.User, error)
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	GetByUsernameOrEmail(ctx context.Context, username, email string) (*models.User, error)
	GetByVerificationToken(ctx context.Context, token string) (*models.User, error)
	GetByResetToken(ctx context.Context, token string) (*models.User, error)
	Create(ctx context.Context, user *models.User) error
	Update(ctx context.Context, user *models.User) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// AuthService handles authentication and authorization
type AuthService interface {
	Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)
	Login(ctx context.Context, req LoginRequest) (*LoginResponse, error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	RefreshToken(ctx context.Context, refreshToken string) (string, error)
	VerifyEmail(ctx context.Context, token string) error
	ResendVerification(ctx context.Context, email string) error
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, token, newPassword string) error
	ValidateResetToken(ctx context.Context, token string) (bool, error)
	GetMFASetup(ctx context.Context, userID uuid.UUID) (*MFASetupResponse, error)
	VerifyMFASetup(ctx context.Context, userID uuid.UUID, token string) error
	DisableMFA(ctx context.Context, userID uuid.UUID, token string) error
	GenerateRecoveryCodes(ctx context.Context, userID uuid.UUID) ([]string, error)
	GetUserSessions(ctx context.Context, userID uuid.UUID) ([]models.Session, error)
	RevokeSession(ctx context.Context, userID, sessionID uuid.UUID) error
	RevokeAllSessions(ctx context.Context, userID uuid.UUID) error
	ValidateSession(ctx context.Context, token string) (*models.Session, error)
	CheckPasswordStrength(password string) error
}

// Request/Response structures
type RegisterRequest struct {
	Username    string `validate:"required,min=3,max=50"`
	Password    string `validate:"required,min=12"`
	Email       string `validate:"required,email"`
	DisplayName string `validate:"omitempty,min=1,max=100"`
	InviteCode  string `validate:"omitempty"`
}

type RegisterResponse struct {
	User      *models.User
	Token     string
	SessionID uuid.UUID
}

type LoginRequest struct {
	Username   string     `validate:"required"`
	Password   string     `validate:"required"`
	DeviceInfo DeviceInfo `validate:"required"`
	MFAToken   string     `validate:"omitempty"`
}

type LoginResponse struct {
	User          *models.User
	Token         string
	SessionID     uuid.UUID
	TrustedDevice bool
	RequiresMFA   bool
}

type DeviceInfo struct {
	Fingerprint string `validate:"required"`
	UserAgent   string `validate:"omitempty"`
	IPAddress   string `validate:"omitempty,ip"`
	DeviceName  string `validate:"omitempty"`
}

type MFASetupResponse struct {
	Secret        string
	QRCode        string
	RecoveryCodes []string
}
