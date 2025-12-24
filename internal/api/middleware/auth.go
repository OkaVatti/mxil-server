// internal/api/middleware/auth.go
package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/auth"
	"github.com/okavatti/mxil-server/m/internal/service"
)

// JWTAuth middleware validates JWT tokens
func JWTAuth(jwtService auth.JWTService, logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get token from header or cookie
			tokenString := extractToken(c)
			if tokenString == "" {
				logger.Debug("No authentication token found in request")
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
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
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error":   "unauthorized",
					"message": "Invalid or expired token",
					"code":    "invalid_token",
				})
			}

			// Set user info in context
			c.Set("user_id", claims.UserID.String())
			c.Set("username", claims.Username)
			c.Set("token", tokenString)

			return next(c)
		}
	}
}

// AdminAuth middleware checks for admin privileges
func AdminAuth(authService service.AuthService, logger *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userIDStr, ok := c.Get("user_id").(string)
			if !ok {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error":   "unauthorized",
					"message": "User not authenticated",
				})
			}

			userID, err := uuid.Parse(userIDStr)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error":   "unauthorized",
					"message": "Invalid user ID",
				})
			}

			ctx := context.Background()
			isAdmin, err := authService.IsAdmin(ctx, userID)
			if err != nil || !isAdmin {
				logger.Warn("Non-admin user attempted admin access",
					zap.String("user_id", userID.String()),
					zap.String("path", c.Path()))
				return c.JSON(http.StatusForbidden, map[string]interface{}{
					"error":   "forbidden",
					"message": "Insufficient privileges",
				})
			}

			c.Set("is_admin", true)
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

func maskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "..." + token[len(token)-4:]
}
