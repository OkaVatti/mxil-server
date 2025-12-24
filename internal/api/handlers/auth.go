// internal/api/handlers/auth.go
package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	authService   service.AuthService
	userRepo      *repository.UserRepository
	sessionRepo   *repository.SessionRepository
	cryptoService service.CryptoService
	logger        *zap.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(
	authService service.AuthService,
	userRepo *repository.UserRepository,
	sessionRepo *repository.SessionRepository,
	cryptoService service.CryptoService,
	logger *zap.Logger,
) *AuthHandler {
	return &AuthHandler{
		authService:   authService,
		userRepo:      userRepo,
		sessionRepo:   sessionRepo,
		cryptoService: cryptoService,
		logger:        logger,
	}
}

// Register handles user registration
func (h *AuthHandler) Register(c echo.Context) error {
	var req struct {
		Username    string `json:"username" validate:"required,min=3,max=50"`
		Email       string `json:"email" validate:"required,email"`
		Password    string `json:"password" validate:"required,min=12"`
		DisplayName string `json:"display_name,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	// Validate password strength
	if !h.isPasswordStrong(req.Password) {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "weak_password",
			"message": "Password must contain uppercase, lowercase, number, and special character",
		})
	}

	ctx := c.Request().Context()

	// Check if user already exists
	existingUser, _ := h.userRepo.GetByUsername(ctx, req.Username)
	if existingUser != nil {
		return c.JSON(http.StatusConflict, map[string]interface{}{
			"error":   "username_exists",
			"message": "Username already taken",
		})
	}

	existingEmail, _ := h.userRepo.GetByEmail(ctx, req.Email)
	if existingEmail != nil {
		return c.JSON(http.StatusConflict, map[string]interface{}{
			"error":   "email_exists",
			"message": "Email already registered",
		})
	}

	// Create user via auth service
	registerReq := service.RegisterRequest{
		Username:    req.Username,
		Email:       req.Email,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	}

	resp, err := h.authService.Register(ctx, registerReq)
	if err != nil {
		h.logger.Error("Failed to register user", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "registration_failed",
			"message": "Failed to create user account",
		})
	}

	// Generate device fingerprint
	deviceFingerprint := generateDeviceFingerprint(c)

	// Create session
	session := &models.Session{
		ID:                uuid.New(),
		UserID:            resp.User.ID,
		Token:             resp.Token,
		UserAgent:         c.Request().UserAgent(),
		IPAddress:         c.RealIP(),
		DeviceFingerprint: deviceFingerprint,
		ExpiresAt:         time.Now().Add(24 * time.Hour),
		CreatedAt:         time.Now(),
		LastActivity:      time.Now(),
	}

	if err := h.sessionRepo.Create(ctx, session); err != nil {
		h.logger.Error("Failed to create session", zap.Error(err))
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"user": map[string]interface{}{
			"id":           resp.User.ID,
			"username":     resp.User.MasterUsername,
			"email":        resp.User.Email,
			"display_name": resp.User.DisplayName,
			"created_at":   resp.User.CreatedAt,
		},
		"token":        resp.Token,
		"session_id":   session.ID,
		"expires_in":   86400,
		"requires_mfa": resp.User.MFAEnabled,
	})
}

// Login handles user login
func (h *AuthHandler) Login(c echo.Context) error {
	var req struct {
		Username   string `json:"username" validate:"required"`
		Password   string `json:"password" validate:"required"`
		MFAToken   string `json:"mfa_token,omitempty"`
		RememberMe bool   `json:"remember_me,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()

	// Check if user is locked out
	user, _ := h.userRepo.GetByUsername(ctx, req.Username)
	if user == nil {
		// Try email
		user, _ = h.userRepo.GetByEmail(ctx, req.Username)
	}

	if user != nil && user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		remaining := user.LockedUntil.Sub(time.Now())
		return c.JSON(http.StatusForbidden, map[string]interface{}{
			"error":       "account_locked",
			"message":     "Account is temporarily locked",
			"retry_after": int(remaining.Seconds()),
		})
	}

	// Create login request
	loginReq := service.LoginRequest{
		Username: req.Username,
		Password: req.Password,
		MFAToken: req.MFAToken,
		DeviceInfo: service.DeviceInfo{
			UserAgent:   c.Request().UserAgent(),
			IPAddress:   c.RealIP(),
			Fingerprint: generateDeviceFingerprint(c),
		},
	}

	resp, err := h.authService.Login(ctx, loginReq)
	if err != nil {
		h.logger.Error("Login failed", zap.Error(err))

		switch err {
		case service.ErrInvalidCredentials:
			// Increment failed attempts
			if user != nil {
				user.FailedLoginAttempts++
				if user.FailedLoginAttempts >= 5 {
					lockout := time.Now().Add(15 * time.Minute)
					user.LockedUntil = &lockout
				}
				h.userRepo.Update(ctx, user)
			}

			return c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"error":   "invalid_credentials",
				"message": "Invalid username or password",
			})
		case service.ErrMFARequired:
			return c.JSON(http.StatusOK, map[string]interface{}{
				"requires_mfa": true,
				"message":      "MFA verification required",
			})
		case service.ErrInvalidMFAToken:
			return c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"error":   "invalid_mfa_token",
				"message": "Invalid MFA token",
			})
		case service.ErrAccountLocked:
			return c.JSON(http.StatusForbidden, map[string]interface{}{
				"error":   "account_locked",
				"message": "Account is locked due to too many failed attempts",
			})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error":   "login_failed",
				"message": "Failed to authenticate user",
			})
		}
	}

	// Set session cookie
	if req.RememberMe {
		c.SetCookie(&http.Cookie{
			Name:     "session_token",
			Value:    resp.Token,
			Path:     "/",
			Expires:  time.Now().Add(30 * 24 * time.Hour), // 30 days
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"user": map[string]interface{}{
			"id":           resp.User.ID,
			"username":     resp.User.MasterUsername,
			"display_name": resp.User.DisplayName,
			"email":        resp.User.Email,
		},
		"token":          resp.Token,
		"session_id":     resp.SessionID,
		"expires_in":     86400,
		"trusted_device": resp.TrustedDevice,
	})
}

// RefreshToken refreshes an access token
func (h *AuthHandler) RefreshToken(c echo.Context) error {
	var req struct {
		RefreshToken string `json:"refresh_token" validate:"required"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()
	newToken, err := h.authService.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_refresh_token",
			"message": "Invalid or expired refresh token",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token":      newToken,
		"expires_in": 86400,
	})
}

// Logout handles user logout
func (h *AuthHandler) Logout(c echo.Context) error {
	token := extractToken(c)
	if token == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "no_token",
			"message": "No authentication token found",
		})
	}

	ctx := c.Request().Context()
	session, err := h.authService.ValidateSession(ctx, token)
	if err != nil {
		// Already logged out or invalid token
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "Logged out successfully",
		})
	}

	if err := h.authService.Logout(ctx, session.ID); err != nil {
		h.logger.Error("Failed to logout", zap.Error(err))
	}

	// Clear cookie
	c.SetCookie(&http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-1 * time.Hour),
		HttpOnly: true,
		Secure:   true,
	})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Logged out successfully",
	})
}

// ResetPassword handles password reset request
func (h *AuthHandler) ResetPassword(c echo.Context) error {
	var req struct {
		Email string `json:"email" validate:"required,email"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()
	if err := h.authService.RequestPasswordReset(ctx, req.Email); err != nil {
		// Don't reveal if email exists or not for security
		h.logger.Debug("Password reset request failed", zap.Error(err))
	}

	// Always return success to prevent email enumeration
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "If the email exists, a reset link has been sent",
	})
}

// VerifyEmail handles email verification
func (h *AuthHandler) VerifyEmail(c echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "missing_token",
			"message": "Verification token is required",
		})
	}

	ctx := c.Request().Context()
	if err := h.authService.VerifyEmail(ctx, token); err != nil {
		h.logger.Error("Email verification failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "verification_failed",
			"message": "Invalid or expired verification token",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Email verified successfully",
	})
}

// ResendVerification resends verification email
func (h *AuthHandler) ResendVerification(c echo.Context) error {
	var req struct {
		Email string `json:"email" validate:"required,email"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()
	if err := h.authService.ResendVerification(ctx, req.Email); err != nil {
		h.logger.Error("Failed to resend verification", zap.Error(err))
	}

	// Always return success to prevent email enumeration
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "If the email exists and is not verified, a verification email has been sent",
	})
}

// VerifyReset handles password reset verification
func (h *AuthHandler) VerifyReset(c echo.Context) error {
	var req struct {
		Token       string `json:"token" validate:"required"`
		NewPassword string `json:"new_password" validate:"required,min=12"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	// Validate password strength
	if !h.isPasswordStrong(req.NewPassword) {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "weak_password",
			"message": "Password must contain uppercase, lowercase, number, and special character",
		})
	}

	ctx := c.Request().Context()
	if err := h.authService.ResetPassword(ctx, req.Token, req.NewPassword); err != nil {
		h.logger.Error("Password reset failed", zap.Error(err))
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "reset_failed",
			"message": "Invalid or expired reset token",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Password reset successfully",
	})
}

// CheckResetToken checks if a reset token is valid
func (h *AuthHandler) CheckResetToken(c echo.Context) error {
	token := c.QueryParam("token")
	if token == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "missing_token",
			"message": "Reset token is required",
		})
	}

	ctx := c.Request().Context()
	valid, err := h.authService.ValidateResetToken(ctx, token)
	if err != nil || !valid {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_token",
			"message": "Invalid or expired reset token",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid":   true,
		"message": "Reset token is valid",
	})
}

// GetMFASetup gets MFA setup information
func (h *AuthHandler) GetMFASetup(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "user_not_found",
			"message": "User not found",
		})
	}

	if user.MFAEnabled {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"enabled": true,
			"message": "MFA is already enabled",
		})
	}

	// Generate new MFA secret
	secret, qrCode, err := h.cryptoService.GenerateMFA(user.Email)
	if err != nil {
		h.logger.Error("Failed to generate MFA", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "mfa_generation_failed",
			"message": "Failed to generate MFA setup",
		})
	}

	// Store temporary secret in session (in production, use secure storage)
	c.Set("mfa_temp_secret", secret)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"enabled": false,
		"secret":  secret,
		"qr_code": qrCode,
		"message": "Scan QR code with authenticator app",
	})
}

// VerifyMFASetup verifies MFA setup
func (h *AuthHandler) VerifyMFASetup(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	var req struct {
		Token string `json:"token" validate:"required"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	secret := c.Get("mfa_temp_secret").(string)
	if secret == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "no_mfa_session",
			"message": "MFA setup session expired",
		})
	}

	valid, err := h.cryptoService.VerifyMFA(secret, req.Token)
	if err != nil || !valid {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_mfa_token",
			"message": "Invalid MFA token",
		})
	}

	ctx := c.Request().Context()
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "user_not_found",
			"message": "User not found",
		})
	}

	// Enable MFA
	user.MFAEnabled = true
	user.MFASecret = secret // Store encrypted in production
	if err := h.userRepo.Update(ctx, user); err != nil {
		h.logger.Error("Failed to enable MFA", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "mfa_enable_failed",
			"message": "Failed to enable MFA",
		})
	}

	// Clear temporary secret
	c.Set("mfa_temp_secret", "")

	// Generate recovery codes
	recoveryCodes := h.cryptoService.GenerateRecoveryCodes()
	// Store recovery codes securely (hashed)

	return c.JSON(http.StatusOK, map[string]interface{}{
		"enabled":        true,
		"message":        "MFA enabled successfully",
		"recovery_codes": recoveryCodes,
		"warning":        "Save these recovery codes in a secure place",
	})
}

// DisableMFA disables MFA
func (h *AuthHandler) DisableMFA(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "user_not_found",
			"message": "User not found",
		})
	}

	if !user.MFAEnabled {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "MFA is already disabled",
		})
	}

	user.MFAEnabled = false
	user.MFASecret = ""
	if err := h.userRepo.Update(ctx, user); err != nil {
		h.logger.Error("Failed to disable MFA", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "mfa_disable_failed",
			"message": "Failed to disable MFA",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "MFA disabled successfully",
	})
}

// GetRecoveryCodes gets MFA recovery codes
func (h *AuthHandler) GetRecoveryCodes(c echo.Context) error {
	userIDStr := c.Get("user_id").(string)
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_user",
			"message": "Invalid user ID",
		})
	}

	ctx := c.Request().Context()
	user, err := h.userRepo.GetByID(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error":   "user_not_found",
			"message": "User not found",
		})
	}

	if !user.MFAEnabled {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "mfa_not_enabled",
			"message": "MFA is not enabled",
		})
	}

	// In production, fetch from secure storage
	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Recovery codes should be stored securely during setup",
	})
}

// ValidateToken validates a session token
func (h *AuthHandler) ValidateToken(ctx context.Context, token string) (*models.Session, error) {
	return h.authService.ValidateSession(ctx, token)
}

// IsAdmin checks if user has admin privileges
func (h *AuthHandler) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	return h.authService.IsAdmin(ctx, userID)
}

// Helper functions
func extractToken(c echo.Context) string {
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	cookie, err := c.Cookie("session_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	return c.QueryParam("token")
}

func generateDeviceFingerprint(c echo.Context) string {
	userAgent := c.Request().UserAgent()
	acceptLang := c.Request().Header.Get("Accept-Language")
	acceptEnc := c.Request().Header.Get("Accept-Encoding")

	fingerprint := userAgent + acceptLang + acceptEnc
	// In production, use proper hashing
	return fingerprint
}

func (h *AuthHandler) isPasswordStrong(password string) bool {
	if len(password) < 12 {
		return false
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false
	specialChars := "!@#$%^&*()_+-=[]{}|;:,.<>?"

	for _, char := range password {
		switch {
		case 'A' <= char && char <= 'Z':
			hasUpper = true
		case 'a' <= char && char <= 'z':
			hasLower = true
		case '0' <= char && char <= '9':
			hasDigit = true
		case strings.ContainsRune(specialChars, char):
			hasSpecial = true
		}
	}

	return hasUpper && hasLower && hasDigit && hasSpecial
}
