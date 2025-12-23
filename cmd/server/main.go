package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/api/handlers"
	"github.com/okavatti/mxil-server/m/internal/auth"
	"github.com/okavatti/mxil-server/m/internal/config"
	"github.com/okavatti/mxil-server/m/internal/email"
	"github.com/okavatti/mxil-server/m/internal/migrations"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service"
)

func main() {
	// Initialize logger
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}
	defer logger.Sync()

	logger.Info("Starting MXIL Server...")

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("Failed to load configuration", zap.Error(err))
	}

	// Initialize database
	db, err := initDatabase(cfg.Database, logger)
	if err != nil {
		logger.Fatal("Failed to initialize database", zap.Error(err))
	}
	defer db.Close()

	// Run migrations
	if err := runMigrations(db, logger); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}

	// Initialize services
	services, err := initServices(db, cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize services", zap.Error(err))
	}

	// Initialize HTTP server
	e := initHTTPServer(services, cfg, logger)

	// Start server
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		logger.Info("Starting HTTP server", zap.String("address", addr))
		if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Server failed", zap.Error(err))
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited properly")
}

func initDatabase(dbConfig config.DatabaseConfig, logger *zap.Logger) (*sqlx.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		dbConfig.Host,
		dbConfig.Port,
		dbConfig.User,
		dbConfig.Password,
		dbConfig.Database,
		dbConfig.SSLMode,
	)

	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(dbConfig.MaxOpenConns)
	db.SetMaxIdleConns(dbConfig.MaxIdleConns)
	db.SetConnMaxLifetime(dbConfig.ConnMaxLifetime)

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Database connection established")
	return db, nil
}

func runMigrations(db *sqlx.DB, logger *zap.Logger) error {
	migrationManager := migrations.NewMigrationManager(db)
	return migrationManager.Run()
}

func initServices(db *sqlx.DB, cfg *config.Config, logger *zap.Logger) (*service.Services, error) {
	// Initialize repositories
	repos := repository.NewRepositories(db, logger)

	// Initialize JWT service
	jwtService := auth.NewJWTService([]byte(cfg.Security.JWTSecret), cfg.Security.JWTExpiration)

	// Initialize crypto service
	cryptoService := service.NewCryptoService(cfg.Security.EncryptionKey)

	// Initialize email service
	emailService := email.NewService(
		repos.Email,
		repos.User,
		email.NewParser(),
		email.NewPipeline(),
		service.NewStorageService(cfg.Storage),
		cryptoService,
	)

	// Initialize auth service
	authService := service.NewAuthService(
		repos.User,
		repos.Session,
		jwtService,
		cryptoService,
		cfg.Security,
	)

	// Initialize network service
	networkService := service.NewNetworkService(cfg.Network)

	return &service.Services{
		Auth:    authService,
		Email:   emailService,
		Crypto:  cryptoService,
		Network: networkService,
	}, nil
}

func initHTTPServer(services *service.Services, cfg *config.Config, logger *zap.Logger) *echo.Echo {
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete},
	}))

	// Initialize handlers
	h := handlers.NewHandlers(services, logger)

	// Routes
	api := e.Group("/api/v1")

	// Public routes
	api.POST("/auth/register", h.Auth.Register)
	api.POST("/auth/login", h.Auth.Login)
	api.POST("/auth/refresh", h.Auth.RefreshToken)
	api.POST("/auth/reset-password", h.Auth.ResetPassword)
	api.POST("/auth/verify-email", h.Auth.VerifyEmail)

	// Protected routes (require auth)
	protected := api.Group("")
	protected.Use(h.AuthMiddleware())

	protected.GET("/profile", h.User.GetProfile)
	protected.PUT("/profile", h.User.UpdateProfile)

	protected.GET("/emails", h.Email.ListEmails)
	protected.GET("/emails/:id", h.Email.GetEmail)
	protected.POST("/emails", h.Email.SendEmail)
	protected.PUT("/emails/:id/read", h.Email.MarkAsRead)
	protected.PUT("/emails/:id/starred", h.Email.MarkAsStarred)
	protected.DELETE("/emails/:id", h.Email.DeleteEmail)

	protected.GET("/contacts", h.Contact.ListContacts)
	protected.POST("/contacts", h.Contact.CreateContact)
	protected.PUT("/contacts/:id", h.Contact.UpdateContact)
	protected.DELETE("/contacts/:id", h.Contact.DeleteContact)

	protected.GET("/folders", h.Folder.ListFolders)
	protected.POST("/folders", h.Folder.CreateFolder)
	protected.GET("/folders/:id/emails", h.Folder.GetFolderEmails)

	protected.GET("/network/status", h.Network.GetStatus)
	protected.GET("/network/identities", h.Network.GetIdentities)
	protected.POST("/network/identities", h.Network.CreateIdentity)

	// Admin routes
	admin := protected.Group("/admin")
	admin.Use(h.AdminMiddleware())
	admin.GET("/users", h.Admin.ListUsers)
	admin.GET("/stats", h.Admin.GetStats)

	// Health check
	e.GET("/health", h.Health.HealthCheck)
	e.GET("/ready", h.Health.ReadinessCheck)

	// WebSocket
	e.GET("/ws", h.WebSocket.HandleWebSocket)

	return e
}
