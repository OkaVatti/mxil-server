// internal/api/middleware/auth.go
package middleware

import (
	"context"
	"strings"

	"github.com/okavatti/mxil-server/m/internal/auth"
	"github.com/okavatti/mxil-server/m/internal/service"

	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// JWTAuth middleware validates JWT tokens
func JWTAuth(jwtService *auth.JWTService, logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get token from header or cookie
			tokenString := extractToken(c)
			if tokenString == "" {
				logger.Debug("No authentication token found in request")
				return c.JSON(401, map[string]interface{}{
					"error":   "unauthorized",
					"message": "Missing authentication token",
					"code":    "missing_token",
				})
			}

			// Validate token
			claims, err := jwtService.ValidateToken(tokenString)
			if err != nil {
				logger.Debug("Invalid authentication token",
					zap.String("token", maskToken(tokenString)),
					zap.Error(err))
				return c.JSON(401, map[string]interface{}{
					"error":   "unauthorized",
					"message": "Invalid or expired token",
					"code":    "invalid_token",
				})
			}

			// Set user info in context
			c.Set("user_id", claims.UserID.String())
			c.Set("username", claims.Username)
			c.Set("token", tokenString)

			// Add user info to request context for logging
			req := c.Request()
			ctx := context.WithValue(req.Context(), "user_id", claims.UserID)
			ctx = context.WithValue(ctx, "username", claims.Username)
			c.SetRequest(req.WithContext(ctx))

			return next(c)
		}
	}
}

// AdminAuth middleware checks for admin privileges
func AdminAuth(authService service.AuthService, logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// First, validate JWT if not already done
			tokenString := extractToken(c)
			if tokenString == "" {
				return c.JSON(401, map[string]interface{}{
					"error":   "unauthorized",
					"message": "Missing authentication token",
					"code":    "missing_token",
				})
			}

			// Validate the session
			ctx := c.Request().Context()
			session, err := authService.ValidateSession(ctx, tokenString)
			if err != nil {
				logger.Debug("Invalid session for admin access", zap.Error(err))
				return c.JSON(401, map[string]interface{}{
					"error":   "unauthorized",
					"message": "Invalid or expired session",
					"code":    "invalid_session",
				})
			}

			// Check if user has admin privileges
			isAdmin, err := checkAdminPrivileges(ctx, authService, session.UserID)
			if err != nil {
				logger.Error("Failed to check admin privileges",
					zap.String("user_id", session.UserID.String()),
					zap.Error(err))
				return c.JSON(500, map[string]interface{}{
					"error":   "internal_error",
					"message": "Failed to verify admin privileges",
					"code":    "admin_check_failed",
				})
			}

			if !isAdmin {
				logger.Warn("Non-admin user attempted admin access",
					zap.String("user_id", session.UserID.String()),
					zap.String("path", c.Path()))
				return c.JSON(403, map[string]interface{}{
					"error":   "forbidden",
					"message": "Insufficient privileges",
					"code":    "insufficient_privileges",
				})
			}

			// Set admin flag in context
			c.Set("is_admin", true)
			c.Set("session_id", session.ID.String())

			// Add admin context to request
			req := c.Request()
			reqCtx := context.WithValue(req.Context(), "is_admin", true)
			c.SetRequest(req.WithContext(reqCtx))

			return next(c)
		}
	}
}

// RateLimiter middleware to prevent brute force attacks
func RateLimiter(store RateLimiterStore) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			clientIP := c.RealIP()
			path := c.Path()

			// Create a key for rate limiting
			key := clientIP + ":" + path

			// Check rate limit
			allowed, remaining, reset, err := store.Check(key)
			if err != nil {
				// If rate limiting fails, allow the request but log it
				logger := zap.L()
				logger.Error("Rate limiter error", zap.Error(err))
				return next(c)
			}

			if !allowed {
				c.Response().Header().Set("X-RateLimit-Limit", strconv.Itoa(store.GetLimit()))
				c.Response().Header().Set("X-RateLimit-Remaining", "0")
				c.Response().Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

				return c.JSON(429, map[string]interface{}{
					"error":       "rate_limit_exceeded",
					"message":     "Too many requests",
					"retry_after": reset - time.Now().Unix(),
				})
			}

			// Set rate limit headers
			c.Response().Header().Set("X-RateLimit-Limit", strconv.Itoa(store.GetLimit()))
			c.Response().Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
			c.Response().Header().Set("X-RateLimit-Reset", strconv.FormatInt(reset, 10))

			return next(c)
		}
	}
}

// CSRFProtection middleware for CSRF protection
func CSRFProtection(secret string, cookieName string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip CSRF for GET, HEAD, OPTIONS
			method := c.Request().Method
			if method == "GET" || method == "HEAD" || method == "OPTIONS" {
				return next(c)
			}

			// Check for CSRF token
			token := c.Request().Header.Get("X-CSRF-Token")
			if token == "" {
				// Try form value
				token = c.FormValue("csrf_token")
			}

			if token == "" {
				return c.JSON(403, map[string]interface{}{
					"error":   "csrf_token_missing",
					"message": "CSRF token is required",
				})
			}

			// Get CSRF token from cookie
			cookie, err := c.Cookie(cookieName)
			if err != nil {
				return c.JSON(403, map[string]interface{}{
					"error":   "csrf_token_invalid",
					"message": "Invalid CSRF token",
				})
			}

			// Verify token
			if !verifyCSRFToken(token, cookie.Value, secret) {
				return c.JSON(403, map[string]interface{}{
					"error":   "csrf_token_invalid",
					"message": "Invalid CSRF token",
				})
			}

			return next(c)
		}
	}
}

// RequireMFA middleware for endpoints that require MFA
func RequireMFA(authService service.AuthService, logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get user ID from context
			userIDStr, ok := c.Get("user_id").(string)
			if !ok {
				return c.JSON(401, map[string]interface{}{
					"error":   "unauthorized",
					"message": "User not authenticated",
				})
			}

			userID, err := uuid.Parse(userIDStr)
			if err != nil {
				return c.JSON(401, map[string]interface{}{
					"error":   "unauthorized",
					"message": "Invalid user ID",
				})
			}

			// Check if MFA is required for this endpoint
			// This would typically involve checking a list of endpoints that require MFA
			// For now, we'll assume all protected routes require MFA
			ctx := c.Request().Context()
			if !authService.IsMFARequired(ctx, userID) {
				return next(c)
			} else {
				logger.Debug("MFA required for user", zap.String("user_id", userID.String()))
			}

			// Get user to check MFA status
			// Note: This would require a method in authService to get user MFA status
			// For now, we'll assume a placeholder implementation

			// Check MFA token from header
			mfaToken := c.Request().Header.Get("X-MFA-Token")
			if mfaToken == "" {
				return c.JSON(403, map[string]interface{}{
					"error":   "mfa_required",
					"message": "MFA token is required for this operation",
				})
			}

			// Verify MFA token (placeholder - implement based on your MFA system)
			if !verifyMFAToken(userID, mfaToken) {
				return c.JSON(403, map[string]interface{}{
					"error":   "mfa_invalid",
					"message": "Invalid MFA token",
				})
			}

			return next(c)
		}
	}
}

// extractToken extracts JWT token from Authorization header or cookie
func extractToken(c echo.Context) string {
	// Check Authorization header first (Bearer token)
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	// Check X-Auth-Token header
	tokenHeader := c.Request().Header.Get("X-Auth-Token")
	if tokenHeader != "" {
		return tokenHeader
	}

	// Check session_token cookie
	cookie, err := c.Cookie("session_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Check access_token cookie
	cookie, err = c.Cookie("access_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Check query parameter (for WebSocket connections)
	token := c.QueryParam("token")
	if token != "" {
		return token
	}

	return ""
}

// Helper functions
func checkAdminPrivileges(ctx context.Context, authService service.AuthService, userID uuid.UUID) (bool, error) {
	// In a real implementation, you would:
	// 1. Get user from database
	// 2. Check user.role or user.is_admin field
	// 3. Check specific permissions

	// For now, implement a simple check
	// You could check against a list of admin user IDs from config
	// Or check a database field

	// Placeholder: Always return false for now
	// In production, implement proper admin check
	return false, nil
}

func verifyCSRFToken(token, cookieValue, secret string) bool {
	// Implement CSRF token verification
	// This should compare the token with the cookie value using HMAC
	// For now, return true for testing
	return token == cookieValue
}

func verifyMFAToken(userID uuid.UUID, token string) bool {
	// Implement MFA token verification
	// This would verify TOTP codes, push notifications, etc.
	// For now, return true for testing
	return len(token) == 6 // Simple check for 6-digit TOTP
}

func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}

// RateLimiterStore interface for rate limiting
type RateLimiterStore interface {
	Check(key string) (allowed bool, remaining int, reset int64, err error)
	GetLimit() int
}

// MemoryRateLimiterStore implements RateLimiterStore using in-memory storage
type MemoryRateLimiterStore struct {
	limit    int
	window   time.Duration
	requests map[string][]int64
	mu       sync.RWMutex
}

// NewMemoryRateLimiterStore creates a new memory rate limiter store
func NewMemoryRateLimiterStore(limit int, window time.Duration) *MemoryRateLimiterStore {
	return &MemoryRateLimiterStore{
		limit:    limit,
		window:   window,
		requests: make(map[string][]int64),
	}
}

// Check implements RateLimiterStore interface
func (s *MemoryRateLimiterStore) Check(key string) (bool, int, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	windowStart := now - int64(s.window.Seconds())

	// Clean old requests
	requests := s.requests[key]
	var validRequests []int64
	for _, ts := range requests {
		if ts > windowStart {
			validRequests = append(validRequests, ts)
		}
	}

	// Check if limit exceeded
	if len(validRequests) >= s.limit {
		// Find the oldest request to calculate reset time
		oldest := now
		for _, ts := range validRequests {
			if ts < oldest {
				oldest = ts
			}
		}
		reset := oldest + int64(s.window.Seconds())
		return false, 0, reset, nil
	}

	// Add current request
	validRequests = append(validRequests, now)
	s.requests[key] = validRequests

	remaining := s.limit - len(validRequests)
	reset := now + int64(s.window.Seconds())

	return true, remaining, reset, nil
}

// GetLimit returns the rate limit
func (s *MemoryRateLimiterStore) GetLimit() int {
	return s.limit
}

// Cleanup removes old entries (call this periodically)
func (s *MemoryRateLimiterStore) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().Unix()
	windowStart := now - int64(s.window.Seconds())

	for key, requests := range s.requests {
		var validRequests []int64
		for _, ts := range requests {
			if ts > windowStart {
				validRequests = append(validRequests, ts)
			}
		}

		if len(validRequests) == 0 {
			delete(s.requests, key)
		} else {
			s.requests[key] = validRequests
		}
	}
}
