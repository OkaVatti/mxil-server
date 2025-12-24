// internal/api/server.go
package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/api/handlers"
	apimiddleware "github.com/okavatti/mxil-server/m/internal/api/middleware"
	"github.com/okavatti/mxil-server/m/internal/config"
	"github.com/okavatti/mxil-server/m/internal/service"
)

// Server represents the HTTP server
type Server struct {
	echo        *echo.Echo
	cfg         *config.Config
	logger      *zap.Logger
	handlers    *handlers.Handlers
	jwtService  service.JWTService
	authService service.AuthService
	httpServer  *http.Server
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config, logger *zap.Logger, handlers *handlers.Handlers,
	jwtService service.JWTService, authService service.AuthService) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Configure server
	e.Server.ReadTimeout = 30 * time.Second
	e.Server.WriteTimeout = 30 * time.Second
	e.Server.IdleTimeout = 120 * time.Second

	return &Server{
		echo:        e,
		cfg:         cfg,
		logger:      logger,
		handlers:    handlers,
		jwtService:  jwtService,
		authService: authService,
	}
}

// Setup configures middleware and routes
func (s *Server) Setup() error {
	// Global middleware
	s.echo.Use(middleware.Recover())
	s.echo.Use(middleware.Secure())
	s.echo.Use(middleware.RequestID())
	s.echo.Use(s.loggingMiddleware())

	// CORS configuration
	s.echo.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"}, // In production, specify domains
		AllowMethods: []string{http.MethodGet, http.MethodHead, http.MethodPut,
			http.MethodPatch, http.MethodPost, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType,
			echo.HeaderAccept, echo.HeaderAuthorization, "X-Requested-With"},
		AllowCredentials: true,
		MaxAge:           86400,
	}))

	// Rate limiting
	s.echo.Use(middleware.RateLimiter(middleware.NewRateLimiterMemoryStore(100)))

	// Health checks (public)
	s.echo.GET("/health", s.handlers.Health.HealthCheck)
	s.echo.GET("/ready", s.handlers.Health.ReadinessCheck)

	// API v1 routes
	api := s.echo.Group("/api/v1")

	// Auth routes (public)
	authGroup := api.Group("/auth")
	{
		authGroup.POST("/register", s.handlers.Auth.Register)
		authGroup.POST("/login", s.handlers.Auth.Login)
		authGroup.POST("/logout", s.handlers.Auth.Logout)
		authGroup.POST("/refresh", s.handlers.Auth.RefreshToken)
		authGroup.POST("/verify-email", s.handlers.Auth.VerifyEmail)
		authGroup.POST("/resend-verification", s.handlers.Auth.ResendVerification)
		authGroup.POST("/reset-password", s.handlers.Auth.ResetPassword)
		authGroup.POST("/verify-reset", s.handlers.Auth.VerifyReset)
		authGroup.GET("/check-reset-token", s.handlers.Auth.CheckResetToken)
	}

	// Protected routes (require authentication)
	protected := api.Group("")
	protected.Use(apimiddleware.JWTAuth(s.jwtService, s.logger))
	{
		// User profile
		protected.GET("/profile", s.handlers.User.GetProfile)
		protected.PUT("/profile", s.handlers.User.UpdateProfile)
		protected.GET("/settings", s.handlers.User.GetSettings)
		protected.PUT("/settings", s.handlers.User.UpdateSettings)
		protected.GET("/storage", s.handlers.User.GetStorageUsage)
		protected.GET("/sessions", s.handlers.User.GetSessions)
		protected.DELETE("/sessions/:sessionId", s.handlers.User.RevokeSession)
		protected.DELETE("/sessions", s.handlers.User.RevokeAllSessions)

		// MFA
		protected.GET("/mfa/setup", s.handlers.Auth.GetMFASetup)
		protected.POST("/mfa/verify", s.handlers.Auth.VerifyMFASetup)
		protected.POST("/mfa/disable", s.handlers.Auth.DisableMFA)
		protected.GET("/mfa/recovery-codes", s.handlers.Auth.GetRecoveryCodes)

		// Emails
		protected.GET("/emails", s.handlers.Email.ListEmails)
		protected.GET("/emails/search", s.handlers.Email.SearchEmails)
		protected.POST("/emails", s.handlers.Email.SendEmail)
		protected.GET("/emails/:id", s.handlers.Email.GetEmail)
		protected.PUT("/emails/:id/read", s.handlers.Email.MarkAsRead)
		protected.PUT("/emails/:id/starred", s.handlers.Email.MarkAsStarred)
		protected.PUT("/emails/:id/folder", s.handlers.Email.MoveToFolder)
		protected.DELETE("/emails/:id", s.handlers.Email.DeleteEmail)
		protected.GET("/threads/:threadId", s.handlers.Email.GetThread)

		// Folders
		protected.GET("/folders", s.handlers.Folder.ListFolders)
		protected.POST("/folders", s.handlers.Folder.CreateFolder)
		protected.GET("/folders/:id", s.handlers.Folder.GetFolderEmails)
		protected.PUT("/folders/:id", s.handlers.Folder.UpdateFolder)
		protected.DELETE("/folders/:id", s.handlers.Folder.DeleteFolder)

		// Labels
		protected.GET("/labels", s.handlers.Label.ListLabels)
		protected.POST("/labels", s.handlers.Label.CreateLabel)
		protected.PUT("/labels/:id", s.handlers.Label.UpdateLabel)
		protected.DELETE("/labels/:id", s.handlers.Label.DeleteLabel)
		protected.POST("/emails/:emailId/labels/:labelId", s.handlers.Label.ApplyLabelToEmail)
		protected.DELETE("/emails/:emailId/labels/:labelId", s.handlers.Label.RemoveLabelFromEmail)

		// Contacts
		protected.GET("/contacts", s.handlers.Contact.ListContacts)
		protected.POST("/contacts", s.handlers.Contact.CreateContact)
		protected.GET("/contacts/search", s.handlers.Contact.SearchContacts)
		protected.GET("/contacts/:id", s.handlers.Contact.GetEmail)
		protected.PUT("/contacts/:id", s.handlers.Contact.UpdateContact)
		protected.DELETE("/contacts/:id", s.handlers.Contact.DeleteContact)
		protected.POST("/contacts/import", s.handlers.Contact.ImportContacts)
		protected.GET("/contacts/export", s.handlers.Contact.ExportContacts)

		// Network identities
		protected.GET("/networks/status", s.handlers.Network.GetStatus)
		protected.GET("/networks/identities", s.handlers.Network.GetIdentities)
		protected.POST("/networks/identities", s.handlers.Network.CreateIdentity)
		protected.PUT("/networks/identities/:id", s.handlers.Network.UpdateIdentity)
		protected.DELETE("/networks/identities/:id", s.handlers.Network.DeleteIdentity)
		protected.POST("/networks/identities/:id/test", s.handlers.Network.TestIdentityConnection)

		// Provider bridges
		protected.GET("/providers", s.handlers.Provider.ListProviders)
		protected.POST("/providers", s.handlers.Provider.AddProvider)
		protected.PUT("/providers/:id", s.handlers.Provider.UpdateProvider)
		protected.DELETE("/providers/:id", s.handlers.Provider.RemoveProvider)
		protected.POST("/providers/:id/sync", s.handlers.Provider.SyncProvider)
		protected.GET("/providers/:id/status", s.handlers.Provider.GetProviderStatus)

		// Encryption keys
		protected.GET("/keys", s.handlers.Encryption.GetKeys)
		protected.POST("/keys", s.handlers.Encryption.GenerateKey)
		protected.GET("/keys/:id", s.handlers.Encryption.GetKeyInfo)
		protected.DELETE("/keys/:id", s.handlers.Encryption.DeleteKey)
		protected.POST("/keys/:id/rotate", s.handlers.Encryption.RotateKey)
		protected.POST("/keys/test", s.handlers.Encryption.TestEncryption)

		// WebSocket
		protected.GET("/ws", s.handlers.WebSocket.HandleWebSocket)

		// Email validation
		protected.GET("/email/validate", s.handlers.Email.ValidateEmail)
		protected.GET("/domain/check", s.handlers.Email.CheckDomainAvailability)
		protected.GET("/domain/:domain/config", s.handlers.Email.GetDomainConfig)
	}

	// Admin routes (require admin privileges)
	admin := api.Group("/admin")
	admin.Use(apimiddleware.JWTAuth(s.jwtService, s.logger))
	admin.Use(apimiddleware.AdminAuth(s.authService, s.logger))
	{
		admin.GET("/users", s.handlers.Admin.ListUsers)
		admin.GET("/users/:id", s.handlers.Admin.GetUser)
		admin.PUT("/users/:id", s.handlers.Admin.UpdateUser)
		admin.DELETE("/users/:id", s.handlers.Admin.DeleteUser)
		admin.GET("/stats", s.handlers.Admin.GetStats)
		admin.GET("/logs", s.handlers.Admin.GetLogs)
		admin.POST("/cleanup", s.handlers.Admin.Cleanup)
	}

	// Static files (for web interface)
	s.echo.Static("/static", "./static")
	s.echo.File("/", "./static/index.html")
	s.echo.File("/favicon.ico", "./static/favicon.ico")

	// 404 handler
	s.echo.HTTPErrorHandler = s.customHTTPErrorHandler

	return nil
}

// Start starts the HTTP server
func (s *Server) Start() error {
	address := fmt.Sprintf("%s:%d", s.cfg.Server.Host, s.cfg.Server.Port)
	s.logger.Info("Starting HTTP server", zap.String("address", address))

	s.httpServer = &http.Server{
		Addr:         address,
		Handler:      s.echo,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// loggingMiddleware returns a middleware that logs requests
func (s *Server) loggingMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()
			err := next(c)
			stop := time.Now()

			req := c.Request()
			res := c.Response()

			// Log only if there's an error or if it's a non-healthcheck request
			if err != nil || !strings.Contains(req.URL.Path, "/health") {
				s.logger.Info("request",
					zap.String("method", req.Method),
					zap.String("path", req.URL.Path),
					zap.Int("status", res.Status),
					zap.Duration("duration", stop.Sub(start)),
					zap.String("ip", c.RealIP()),
					zap.String("user_agent", req.UserAgent()),
				)
			}

			return err
		}
	}
}

// customHTTPErrorHandler handles errors in a consistent way
func (s *Server) customHTTPErrorHandler(err error, c echo.Context) {
	code := http.StatusInternalServerError
	message := "Internal Server Error"

	if he, ok := err.(*echo.HTTPError); ok {
		code = he.Code
		message = he.Message.(string)
	}

	// Log error
	s.logger.Error("HTTP error",
		zap.Int("code", code),
		zap.String("message", message),
		zap.String("path", c.Path()),
		zap.String("method", c.Request().Method),
	)

	// Send JSON response
	c.JSON(code, map[string]interface{}{
		"error":   http.StatusText(code),
		"message": message,
		"code":    code,
	})
}
