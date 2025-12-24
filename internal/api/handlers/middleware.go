// internal/api/handlers/middleware.go
package handlers

import (
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// AuthMiddleware creates authentication middleware
func (h *Handlers) AuthMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Extract token from header or cookie
			token := extractTokenFromRequest(c)
			if token == "" {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error":   "unauthorized",
					"message": "Authentication token required",
				})
			}

			// Validate token using auth service
			ctx := c.Request().Context()
			session, err := h.Auth.ValidateToken(ctx, token)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error":   "invalid_token",
					"message": "Invalid or expired authentication token",
				})
			}

			// Set user context
			c.Set("user_id", session.UserID.String())
			c.Set("session_id", session.ID.String())

			return next(c)
		}
	}
}

// AdminMiddleware creates admin-only middleware
func (h *Handlers) AdminMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			userIDStr := c.Get("user_id").(string)
			userID, err := uuid.Parse(userIDStr)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]interface{}{
					"error":   "invalid_user",
					"message": "Invalid user ID",
				})
			}

			// Check if user is admin
			ctx := c.Request().Context()
			isAdmin, err := h.Auth.IsAdmin(ctx, userID)
			if err != nil || !isAdmin {
				return c.JSON(http.StatusForbidden, map[string]interface{}{
					"error":   "forbidden",
					"message": "Admin privileges required",
				})
			}

			return next(c)
		}
	}
}

// extractTokenFromRequest extracts JWT token from request
func extractTokenFromRequest(c echo.Context) string {
	// Try Authorization header
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}

	// Try cookie
	cookie, err := c.Cookie("session_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Try query parameter
	return c.QueryParam("token")
}
