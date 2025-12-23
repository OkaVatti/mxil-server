package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/okavatti/mxil-server/m/internal/database"
)

// HealthHandler handles health checks
type HealthHandler struct {
	db *database.DB
}

// SetDatabase sets the database for the health handler
func (h *HealthHandler) SetDatabase(db *database.DB) {
	h.db = db
}

// HealthCheck performs a basic health check
func (h *HealthHandler) HealthCheck(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().UTC(),
		"service":   "mxil-backend",
		"version":   "1.0.0",
	})
}

// ReadinessCheck performs a readiness check
func (h *HealthHandler) ReadinessCheck(c echo.Context) error {
	// Check database connection
	if h.db != nil {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Second)
		defer cancel()

		if err := h.db.HealthCheck(ctx); err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
				"status":    "unhealthy",
				"timestamp": time.Now().UTC(),
				"error":     "database_unavailable",
			})
		}
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "ready",
		"timestamp": time.Now().UTC(),
		"components": map[string]string{
			"database": "connected",
			"api":      "running",
		},
	})
}
