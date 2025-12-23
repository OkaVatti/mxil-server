// internal/api/middleware/auth.go
package middleware

import (
	"strings"

	"github.com/okavatti/mxil-server/m/internal/auth"
	"github.com/okavatti/mxil-server/m/internal/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// JWTAuth middleware validates JWT tokens
func JWTAuth(jwtService *auth.JWTService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get token from header or cookie
			tokenString := extractToken(c)
			if tokenString == "" {
				return c.JSON(401, map[string]string{
					"error":   "unauthorized",
					"message": "Missing authentication token",
				})
			}

			// Validate token
			claims, err := jwtService.ValidateToken(tokenString)
			if err != nil {
				return c.JSON(401, map[string]string{
					"error":   "unauthorized",
					"message": "Invalid or expired token",
				})
			}

			// Set user info in context
			c.Set("user_id", claims.UserID.String())
			c.Set("username", claims.Username)

			return next(c)
		}
	}
}

// AdminAuth middleware checks for admin privileges
func AdminAuth(authService service.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// First check JWT
			tokenString := extractToken(c)
			if tokenString == "" {
				return c.JSON(401, map[string]string{
					"error":   "unauthorized",
					"message": "Missing authentication token",
				})
			}

			// Get user from token
			userIDStr, ok := c.Get("user_id").(string)
			if !ok {
				return c.JSON(401, map[string]string{
					"error":   "unauthorized",
					"message": "Invalid user context",
				})
			}

			_, err := uuid.Parse(userIDStr)
			if err != nil {
				return c.JSON(401, map[string]string{
					"error":   "unauthorized",
					"message": "Invalid user ID",
				})
			}

			// TODO: Check if user has admin privileges
			// This would require a user role/permission system

			// For now, allow all authenticated users (temporary)
			// In production, implement proper role checking

			return next(c)
		}
	}
}

// extractToken extracts JWT token from Authorization header or cookie
func extractToken(c echo.Context) string {
	// Check Authorization header
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	// Check cookie
	cookie, err := c.Cookie("session_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Check query parameter
	token := c.QueryParam("token")
	if token != "" {
		return token
	}

	return ""
}
