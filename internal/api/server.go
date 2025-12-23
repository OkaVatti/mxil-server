// internal/api/server.go
package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	echolog "github.com/labstack/gommon/log"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/api/handlers"
	"github.com/okavatti/mxil-server/m/internal/config"
)

// Server represents the HTTP API server
type Server struct {
	e         *echo.Echo
	cfg       *config.Config
	logger    *zap.Logger
	handlers  *handlers.Handlers
	isRunning bool
	server    *http.Server
}

// NewServer creates a new API server
func NewServer(
	cfg *config.Config,
	logger *zap.Logger,
	handlers *handlers.Handlers,
) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Configure Echo logger
	e.Logger.SetLevel(echolog.INFO)
	e.Logger = newEchoLogger(logger)

	// Create HTTP server
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	return &Server{
		e:        e,
		cfg:      cfg,
		logger:   logger,
		handlers: handlers,
		server:   server,
	}
}

// Setup configures the server routes and middleware
func (s *Server) Setup() error {
	// Middleware
	s.e.Use(middleware.Recover())
	s.e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions, http.MethodPatch},
		AllowHeaders:     []string{echo.HeaderAuthorization, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderXRequestedWith},
		ExposeHeaders:    []string{echo.HeaderContentLength, echo.HeaderContentType, "X-Total-Count"},
		AllowCredentials: true,
		MaxAge:           86400,
	}))
	s.e.Use(middleware.SecureWithConfig(middleware.SecureConfig{
		XSSProtection:         "1; mode=block",
		ContentTypeNosniff:    "nosniff",
		XFrameOptions:         "DENY",
		HSTSMaxAge:            31536000,
		ContentSecurityPolicy: "default-src 'self'",
	}))
	s.e.Use(middleware.GzipWithConfig(middleware.GzipConfig{
		Level: 5,
	}))
	s.e.Use(middleware.RequestLogger())
	s.e.Use(middleware.RequestID())

	// Rate limiting
	rateLimiterConfig := middleware.RateLimiterConfig{
		Skipper: middleware.DefaultSkipper,
		Store: middleware.NewRateLimiterMemoryStoreWithConfig(
			middleware.RateLimiterMemoryStoreConfig{
				Rate:      10, // requests per second
				Burst:     30,
				ExpiresIn: 3 * time.Minute,
			},
		),
		IdentifierExtractor: func(ctx echo.Context) (string, error) {
			id := ctx.RealIP()
			return id, nil
		},
		ErrorHandler: func(context echo.Context, err error) error {
			return context.JSON(http.StatusTooManyRequests, map[string]interface{}{
				"error":   "rate_limit_exceeded",
				"message": "Too many requests. Please try again later.",
			})
		},
		DenyHandler: func(context echo.Context, identifier string, err error) error {
			return context.JSON(http.StatusTooManyRequests, map[string]interface{}{
				"error":   "rate_limit_exceeded",
				"message": "Too many requests. Please try again later.",
			})
		},
	}
	s.e.Use(middleware.RateLimiterWithConfig(rateLimiterConfig))

	// Static files
	s.e.Static("/static", "./static")
	s.e.File("/favicon.ico", "./static/favicon.ico")

	// API Routes
	s.setupRoutes()

	return nil
}

// setupRoutes configures all API routes
func (s *Server) setupRoutes() {
	api := s.e.Group("/api/v1")

	// Public routes
	public := api.Group("")
	{
		// Health check
		public.GET("/health", s.handlers.Health.HealthCheck)
		public.GET("/ready", s.handlers.Health.ReadinessCheck)

		// Authentication
		public.POST("/auth/register", s.handlers.Auth.Register)
		public.POST("/auth/login", s.handlers.Auth.Login)
		public.POST("/auth/refresh", s.handlers.Auth.RefreshToken)
		public.POST("/auth/verify-email", s.handlers.Auth.VerifyEmail)
		public.POST("/auth/resend-verification", s.handlers.Auth.ResendVerification)
		public.POST("/auth/reset-password", s.handlers.Auth.ResetPassword)
		public.POST("/auth/verify-reset", s.handlers.Auth.VerifyReset)
		public.GET("/auth/check-reset-token", s.handlers.Auth.CheckResetToken)

		// Public info
		public.GET("/networks/status", s.handlers.Network.GetStatus)
		public.GET("/email/validate", s.handlers.Email.ValidateEmail)
		public.GET("/email/check-domain", s.handlers.Email.CheckDomainAvailability)
		public.GET("/email/domain/:domain/config", s.handlers.Email.GetDomainConfig)
	}

	// Protected routes (require authentication)
	protected := api.Group("")
	protected.Use(middleware.JWTWithConfig(s.handlers.Auth.getJWTConfig()))
	{
		// User management
		protected.GET("/users/me", s.handlers.User.GetProfile)
		protected.PUT("/users/me", s.handlers.User.UpdateProfile)
		protected.GET("/users/me/settings", s.handlers.User.GetSettings)
		protected.PUT("/users/me/settings", s.handlers.User.UpdateSettings)
		protected.GET("/users/me/storage", s.handlers.User.GetStorageUsage)
		protected.GET("/users/me/sessions", s.handlers.User.GetSessions)
		protected.DELETE("/users/me/sessions/:sessionId", s.handlers.User.RevokeSession)
		protected.DELETE("/users/me/sessions", s.handlers.User.RevokeAllSessions)

		// MFA
		protected.GET("/users/me/mfa/setup", s.handlers.Auth.GetMFASetup)
		protected.POST("/users/me/mfa/verify", s.handlers.Auth.VerifyMFASetup)
		protected.POST("/users/me/mfa/disable", s.handlers.Auth.DisableMFA)
		protected.GET("/users/me/mfa/recovery-codes", s.handlers.Auth.GetRecoveryCodes)

		// Email management
		protected.GET("/emails", s.handlers.Email.ListEmails)
		protected.GET("/emails/search", s.handlers.Email.SearchEmails)
		protected.POST("/emails", s.handlers.Email.SendEmail)
		protected.GET("/emails/:id", s.handlers.Email.GetEmail)
		protected.PUT("/emails/:id/read", s.handlers.Email.MarkAsRead)
		protected.PUT("/emails/:id/starred", s.handlers.Email.MarkAsStarred)
		protected.PUT("/emails/:id/folder", s.handlers.Email.MoveToFolder)
		protected.DELETE("/emails/:id", s.handlers.Email.DeleteEmail)
		protected.GET("/emails/thread/:threadId", s.handlers.Email.GetThread)

		// Network identities
		protected.GET("/networks/identities", s.handlers.Network.GetIdentities)
		protected.POST("/networks/identities", s.handlers.Network.CreateIdentity)
		protected.PUT("/networks/identities/:id", s.handlers.Network.UpdateIdentity)
		protected.DELETE("/networks/identities/:id", s.handlers.Network.DeleteIdentity)
		protected.POST("/networks/identities/:id/test", s.handlers.Network.TestIdentityConnection)

		// Contacts
		protected.GET("/contacts", s.handlers.Contact.ListContacts)
		protected.POST("/contacts", s.handlers.Contact.CreateContact)
		protected.PUT("/contacts/:id", s.handlers.Contact.UpdateContact)
		protected.DELETE("/contacts/:id", s.handlers.Contact.DeleteContact)
		protected.GET("/contacts/search", s.handlers.Contact.SearchContacts)
		protected.POST("/contacts/import", s.handlers.Contact.ImportContacts)
		protected.GET("/contacts/export", s.handlers.Contact.ExportContacts)

		// Folders
		protected.GET("/folders", s.handlers.Folder.ListFolders)
		protected.POST("/folders", s.handlers.Folder.CreateFolder)
		protected.PUT("/folders/:id", s.handlers.Folder.UpdateFolder)
		protected.DELETE("/folders/:id", s.handlers.Folder.DeleteFolder)
		protected.GET("/folders/:id/emails", s.handlers.Folder.GetFolderEmails)

		// Labels
		protected.GET("/labels", s.handlers.Label.ListLabels)
		protected.POST("/labels", s.handlers.Label.CreateLabel)
		protected.PUT("/labels/:id", s.handlers.Label.UpdateLabel)
		protected.DELETE("/labels/:id", s.handlers.Label.DeleteLabel)
		protected.POST("/emails/:emailId/labels/:labelId", s.handlers.Label.ApplyLabelToEmail)
		protected.DELETE("/emails/:emailId/labels/:labelId", s.handlers.Label.RemoveLabelFromEmail)

		// Encryption keys
		protected.GET("/keys", s.handlers.Encryption.GetKeys)
		protected.POST("/keys", s.handlers.Encryption.GenerateKey)
		protected.DELETE("/keys/:id", s.handlers.Encryption.DeleteKey)
		protected.POST("/keys/:id/rotate", s.handlers.Encryption.RotateKey)
		protected.POST("/keys/test", s.handlers.Encryption.TestEncryption)
		protected.GET("/keys/:id", s.handlers.Encryption.GetKeyInfo)

		// Provider bridges
		protected.GET("/providers", s.handlers.Provider.ListProviders)
		protected.POST("/providers", s.handlers.Provider.AddProvider)
		protected.PUT("/providers/:id", s.handlers.Provider.UpdateProvider)
		protected.DELETE("/providers/:id", s.handlers.Provider.RemoveProvider)
		protected.POST("/providers/:id/sync", s.handlers.Provider.SyncProvider)
		protected.GET("/providers/:id/status", s.handlers.Provider.GetProviderStatus)

		// WebSocket
		protected.GET("/ws", s.handlers.WebSocket.HandleWebSocket)
	}

	// Admin routes (require admin privileges)
	admin := api.Group("/admin")
	admin.Use(middleware.AdminAuth(s.handlers.Auth.authService))
	{
		admin.GET("/users", s.handlers.Admin.ListUsers)
		admin.GET("/users/:id", s.handlers.Admin.GetUser)
		admin.PUT("/users/:id", s.handlers.Admin.UpdateUser)
		admin.DELETE("/users/:id", s.handlers.Admin.DeleteUser)
		admin.GET("/stats", s.handlers.Admin.GetStats)
		admin.GET("/logs", s.handlers.Admin.GetLogs)
		admin.POST("/cleanup", s.handlers.Admin.Cleanup)
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.isRunning = true

	// Set the Echo instance to use our HTTP server
	s.e.Server = s.server

	s.logger.Info("Starting server",
		zap.String("address", s.server.Addr),
		zap.Bool("tls", s.cfg.Server.TLSEnabled))

	if s.cfg.Server.TLSEnabled {
		return s.e.StartTLS(
			s.server.Addr,
			s.cfg.Server.TLSCertPath,
			s.cfg.Server.TLSKeyPath,
		)
	}

	return s.e.Start(s.server.Addr)
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.isRunning = false
	return s.e.Shutdown(ctx)
}

// IsRunning returns true if the server is running
func (s *Server) IsRunning() bool {
	return s.isRunning
}

// GetEcho returns the Echo instance
func (s *Server) GetEcho() *echo.Echo {
	return s.e
}

// newEchoLogger creates a zap logger adapter for Echo
func newEchoLogger(logger *zap.Logger) *echoLogger {
	return &echoLogger{logger: logger}
}

type echoLogger struct {
	logger *zap.Logger
}

func (l *echoLogger) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	l.logger.Info(msg)
	return len(p), nil
}

func (l *echoLogger) Print(i ...interface{}) {
	l.logger.Info(fmt.Sprint(i...))
}

func (l *echoLogger) Printf(format string, args ...interface{}) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

func (l *echoLogger) Printj(j middleware.JSON) {
	l.logger.Info("", zap.Any("data", j))
}

func (l *echoLogger) Debug(i ...interface{}) {
	l.logger.Debug(fmt.Sprint(i...))
}

func (l *echoLogger) Debugf(format string, args ...interface{}) {
	l.logger.Debug(fmt.Sprintf(format, args...))
}

func (l *echoLogger) Debugj(j middleware.JSON) {
	l.logger.Debug("", zap.Any("data", j))
}

func (l *echoLogger) Info(i ...interface{}) {
	l.logger.Info(fmt.Sprint(i...))
}

func (l *echoLogger) Infof(format string, args ...interface{}) {
	l.logger.Info(fmt.Sprintf(format, args...))
}

func (l *echoLogger) Infoj(j middleware.JSON) {
	l.logger.Info("", zap.Any("data", j))
}

func (l *echoLogger) Warn(i ...interface{}) {
	l.logger.Warn(fmt.Sprint(i...))
}

func (l *echoLogger) Warnf(format string, args ...interface{}) {
	l.logger.Warn(fmt.Sprintf(format, args...))
}

func (l *echoLogger) Warnj(j middleware.JSON) {
	l.logger.Warn("", zap.Any("data", j))
}

func (l *echoLogger) Error(i ...interface{}) {
	l.logger.Error(fmt.Sprint(i...))
}

func (l *echoLogger) Errorf(format string, args ...interface{}) {
	l.logger.Error(fmt.Sprintf(format, args...))
}

func (l *echoLogger) Errorj(j middleware.JSON) {
	l.logger.Error("", zap.Any("data", j))
}

func (l *echoLogger) Fatal(i ...interface{}) {
	l.logger.Fatal(fmt.Sprint(i...))
	os.Exit(1)
}

func (l *echoLogger) Fatalf(format string, args ...interface{}) {
	l.logger.Fatal(fmt.Sprintf(format, args...))
	os.Exit(1)
}

func (l *echoLogger) Fatalj(j middleware.JSON) {
	l.logger.Fatal("", zap.Any("data", j))
	os.Exit(1)
}

func (l *echoLogger) Panic(i ...interface{}) {
	l.logger.Panic(fmt.Sprint(i...))
}

func (l *echoLogger) Panicf(format string, args ...interface{}) {
	l.logger.Panic(fmt.Sprintf(format, args...))
}

func (l *echoLogger) Panicj(j middleware.JSON) {
	l.logger.Panic("", zap.Any("data", j))
}

func (l *echoLogger) SetLevel(level echolog.Lvl) {
	// Not implemented - zap handles levels differently
}

func (l *echoLogger) Level() echolog.Lvl {
	return echolog.INFO
}

func (l *echoLogger) SetHeader(h string) {
	// Not implemented
}

func (l *echoLogger) Prefix() string {
	return ""
}

func (l *echoLogger) SetPrefix(p string) {
	// Not implemented
}

func (l *echoLogger) Output() io.Writer {
	return l
}

func (l *echoLogger) SetOutput(w io.Writer) {
	// Not implemented
}
