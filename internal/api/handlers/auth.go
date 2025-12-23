package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/crypto"
	"github.com/okavatti/mxil-server/m/internal/service"
)

// AuthHandler handles authentication requests
type AuthHandler struct {
	authService service.AuthService
	logger      *zap.Logger
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService service.AuthService, logger *zap.Logger) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		logger:      logger,
	}
}

// Register handles user registration
func (h *AuthHandler) Register(c echo.Context) error {
	var req struct {
		Username    string `json:"username" validate:"required,min=3,max=50"`
		Password    string `json:"password" validate:"required,min=12"`
		Email       string `json:"email" validate:"required,email"`
		DisplayName string `json:"display_name,omitempty"`
		InviteCode  string `json:"invite_code,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	// Validate password strength
	if len(req.Password) < 12 {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "weak_password",
			"message": "Password must be at least 12 characters",
		})
	}

	// Check for uppercase letters
	if !strings.ContainsAny(req.Password, "ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "weak_password",
			"message": "Password must contain at least one uppercase letter",
		})
	}

	// Check for numbers
	if !strings.ContainsAny(req.Password, "0123456789") {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "weak_password",
			"message": "Password must contain at least one number",
		})
	}

	// Check for symbols
	if !strings.ContainsAny(req.Password, "!@#$%^&*()_+-=[]{}|;:,.<>?") {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "weak_password",
			"message": "Password must contain at least one symbol",
		})
	}

	ctx := c.Request().Context()
	registerReq := service.RegisterRequest{
		Username:    req.Username,
		Password:    req.Password,
		Email:       req.Email,
		DisplayName: req.DisplayName,
	}

	resp, err := h.authService.Register(ctx, registerReq)
	if err != nil {
		h.logger.Error("Registration failed", zap.Error(err))

		if err == service.ErrUserExists {
			return c.JSON(http.StatusConflict, map[string]interface{}{
				"error":   "user_exists",
				"message": "Username or email already exists",
			})
		}

		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "internal_error",
			"message": "Failed to create account",
		})
	}

	// Set secure HTTP-only cookie
	c.SetCookie(&http.Cookie{
		Name:     "session_token",
		Value:    resp.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400, // 24 hours
	})

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"user":         resp.User,
		"token":        resp.Token,
		"session_id":   resp.SessionID,
		"expires_in":   86400,
		"requires_mfa": false,
	})
}

// Login handles user login
func (h *AuthHandler) Login(c echo.Context) error {
	var req struct {
		Username   string `json:"username" validate:"required"`
		Password   string `json:"password" validate:"required"`
		DeviceName string `json:"device_name,omitempty"`
		RememberMe bool   `json:"remember_me,omitempty"`
		MFAToken   string `json:"mfa_token,omitempty"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	deviceInfo := service.DeviceInfo{
		Fingerprint: generateDeviceFingerprint(c),
		UserAgent:   c.Request().UserAgent(),
		IPAddress:   c.RealIP(),
		DeviceName:  req.DeviceName,
	}

	loginReq := service.LoginRequest{
		Username:   req.Username,
		Password:   req.Password,
		DeviceInfo: deviceInfo,
		MFAToken:   req.MFAToken,
	}

	ctx := c.Request().Context()
	resp, err := h.authService.Login(ctx, loginReq)
	if err != nil {
		h.logger.Error("Login failed", zap.Error(err))

		if err == service.ErrInvalidCredentials {
			return c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"error":   "invalid_credentials",
				"message": "Invalid username or password",
			})
		}

		if err == service.ErrAccountLocked {
			return c.JSON(http.StatusForbidden, map[string]interface{}{
				"error":   "account_locked",
				"message": "Account is locked due to too many failed attempts",
			})
		}

		if err == service.ErrMFARequired {
			return c.JSON(http.StatusOK, map[string]interface{}{
				"requires_mfa": true,
				"message":      "MFA token required",
			})
		}

		if err == service.ErrInvalidMFAToken {
			return c.JSON(http.StatusUnauthorized, map[string]interface{}{
				"error":   "invalid_mfa_token",
				"message": "Invalid MFA token",
			})
		}

		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "internal_error",
			"message": "Login failed",
		})
	}

	// Set cookie based on remember me
	maxAge := 86400 // 24 hours
	if req.RememberMe {
		maxAge = 2592000 // 30 days
	}

	c.SetCookie(&http.Cookie{
		Name:     "session_token",
		Value:    resp.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   maxAge,
	})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"user":           resp.User,
		"token":          resp.Token,
		"session_id":     resp.SessionID,
		"expires_in":     maxAge,
		"requires_mfa":   false,
		"trusted_device": resp.TrustedDevice,
	})
}

// Logout handles user logout
func (h *AuthHandler) Logout(c echo.Context) error {
	// Get token from cookie or header
	token := extractToken(c)
	if token == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "no_token",
			"message": "No authentication token provided",
		})
	}

	// Get session ID from context
	sessionID := c.Get("session_id")
	if sessionID == nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "no_session",
			"message": "No active session",
		})
	}

	sessionUUID, err := uuid.Parse(sessionID.(string))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_session",
			"message": "Invalid session ID",
		})
	}

	ctx := c.Request().Context()
	if err := h.authService.Logout(ctx, sessionUUID); err != nil {
		h.logger.Error("Logout failed", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "internal_error",
			"message": "Failed to logout",
		})
	}

	// Clear all auth cookies
	cookies := []string{"session_token", "refresh_token", "mfa_token"}
	for _, cookieName := range cookies {
		c.SetCookie(&http.Cookie{
			Name:     cookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteStrictMode,
			MaxAge:   -1,
			Expires:  time.Now().Add(-24 * time.Hour),
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Logged out successfully",
	})
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c echo.Context) error {
	// Get refresh token from cookie or body
	var req struct {
		RefreshToken string `json:"refresh_token,omitempty"`
	}

	refreshToken := ""
	if err := c.Bind(&req); err == nil && req.RefreshToken != "" {
		refreshToken = req.RefreshToken
	} else {
		// Try to get from cookie
		if cookie, err := c.Cookie("refresh_token"); err == nil {
			refreshToken = cookie.Value
		}
	}

	if refreshToken == "" {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "no_refresh_token",
			"message": "No refresh token provided",
		})
	}

	ctx := c.Request().Context()
	newToken, err := h.authService.RefreshToken(ctx, refreshToken)
	if err != nil {
		h.logger.Error("Token refresh failed", zap.Error(err))
		return c.JSON(http.StatusUnauthorized, map[string]interface{}{
			"error":   "invalid_refresh_token",
			"message": "Invalid or expired refresh token",
		})
	}

	// Set new session cookie
	c.SetCookie(&http.Cookie{
		Name:     "session_token",
		Value:    newToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		MaxAge:   86400,
	})

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token":      newToken,
		"expires_in": 86400,
		"token_type": "bearer",
	})
}

// VerifyEmail handles email verification
func (h *AuthHandler) VerifyEmail(c echo.Context) error {
	var req struct {
		Token string `json:"token" validate:"required"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()
	if err := h.authService.VerifyEmail(ctx, req.Token); err != nil {
		h.logger.Error("Email verification failed", zap.Error(err))

		if err == service.ErrInvalidToken {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_token",
				"message": "Invalid or expired verification token",
			})
		}

		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "verification_failed",
			"message": "Failed to verify email",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Email verified successfully",
	})
}

// ResendVerification handles resending verification email
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
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "send_failed",
			"message": "Failed to send verification email",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Verification email sent",
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
		h.logger.Error("Password reset request failed", zap.Error(err))
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message": "If an account exists with this email, a reset link will be sent",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "Password reset email sent",
	})
}

// VerifyReset handles password reset verification
func (h *AuthHandler) VerifyReset(c echo.Context) error {
	var req struct {
		Token    string `json:"token" validate:"required"`
		Password string `json:"password" validate:"required,min=12"`
	}

	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_request",
			"message": "Invalid request format",
		})
	}

	ctx := c.Request().Context()
	if err := h.authService.ResetPassword(ctx, req.Token, req.Password); err != nil {
		h.logger.Error("Password reset failed", zap.Error(err))

		if err == service.ErrInvalidToken {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_token",
				"message": "Invalid or expired reset token",
			})
		}

		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "reset_failed",
			"message": "Failed to reset password",
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
			"message": "No token provided",
		})
	}

	ctx := c.Request().Context()
	valid, err := h.authService.ValidateResetToken(ctx, token)
	if err != nil {
		h.logger.Error("Token validation failed", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "validation_error",
			"message": "Failed to validate token",
		})
	}

	if !valid {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{
			"error":   "invalid_token",
			"message": "Invalid or expired token",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"valid": true,
	})
}

// GetMFASetup returns MFA setup information
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
	setup, err := h.authService.GetMFASetup(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to get MFA setup", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "mfa_setup_failed",
			"message": "Failed to get MFA setup",
		})
	}

	return c.JSON(http.StatusOK, setup)
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

	ctx := c.Request().Context()
	if err := h.authService.VerifyMFASetup(ctx, userID, req.Token); err != nil {
		h.logger.Error("MFA setup verification failed", zap.Error(err))

		if err == service.ErrInvalidMFAToken {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_token",
				"message": "Invalid token",
			})
		}

		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "verification_failed",
			"message": "Failed to verify MFA setup",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "MFA enabled successfully",
	})
}

// DisableMFA disables MFA for a user
func (h *AuthHandler) DisableMFA(c echo.Context) error {
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

	ctx := c.Request().Context()
	if err := h.authService.DisableMFA(ctx, userID, req.Token); err != nil {
		h.logger.Error("Failed to disable MFA", zap.Error(err))

		if err == service.ErrInvalidMFAToken {
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":   "invalid_token",
				"message": "Invalid token",
			})
		}

		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "disable_failed",
			"message": "Failed to disable MFA",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"message": "MFA disabled successfully",
	})
}

// GetRecoveryCodes generates recovery codes for MFA
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
	codes, err := h.authService.GenerateRecoveryCodes(ctx, userID)
	if err != nil {
		h.logger.Error("Failed to generate recovery codes", zap.Error(err))
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "generation_failed",
			"message": "Failed to generate recovery codes",
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"codes": codes,
	})
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
	hash := crypto.HashToken(fingerprint)
	return hash
}
