// cmd/server/main.go
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
	_ "github.com/lib/pq"
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/api"
	"github.com/okavatti/mxil-server/m/internal/api/handlers"
	"github.com/okavatti/mxil-server/m/internal/config"
	"github.com/okavatti/mxil-server/m/internal/email"
	"github.com/okavatti/mxil-server/m/internal/migrations"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service"
)

// Version variables set by build process
var (
	version   = "1.0.0"
	buildTime = "2025-12-23_21:01:53"
	gitCommit = "unknown"
	goVersion = "go1.25.4"
)

func main() {
	// Initialize logger
	logger := initLogger()
	defer logger.Sync()

	logger.Info("Starting MXIL Server...",
		zap.String("version", version),
		zap.String("build_time", buildTime),
		zap.String("git_commit", gitCommit),
		zap.String("go_version", goVersion))

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

	// Initialize services and handlers
	h, jwtService, authService, err := initServicesAndHandlers(db, cfg, logger)
	if err != nil {
		logger.Fatal("Failed to initialize services", zap.Error(err))
	}

	// Initialize HTTP server
	server := initHTTPServer(h, cfg, logger, jwtService, authService)

	// Start server
	go func() {
		addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
		logger.Info("Starting HTTP server", zap.String("address", addr))
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
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

	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Server forced to shutdown", zap.Error(err))
	}

	logger.Info("Server exited properly")
}

func initLogger() *zap.Logger {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stdout"}
	cfg.ErrorOutputPaths = []string{"stderr"}

	logger, err := cfg.Build()
	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}

	return logger
}

func initDatabase(dbConfig config.DatabaseConfig, logger *zap.Logger) (*sqlx.DB, error) {
	connStr := dbConfig.GetDSN()

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

func initServicesAndHandlers(db *sqlx.DB, cfg *config.Config, logger *zap.Logger) (
	*handlers.Handlers, service.JWTService, service.AuthService, error) {

	// Initialize repositories
	userRepo := repository.NewUserRepository(db, logger)
	sessionRepo := repository.NewSessionRepository(db, logger)
	emailRepo := repository.NewEmailRepository(db, logger)
	folderRepo := repository.NewFolderRepository(db, logger)
	labelRepo := repository.NewLabelRepository(db, logger)
	contactRepo := repository.NewContactRepository(db, logger)
	networkRepo := repository.NewNetworkIdentityRepository(db, logger)
	providerRepo := repository.NewProviderBridgeRepository(db, logger)
	keyRepo := repository.NewEncryptionKeyRepository(db, logger)
	statsRepo := repository.NewStatsRepository(db, logger)
	passwordResetRepo := repository.NewPasswordResetRepository(db, logger)
	attachmentRepo := repository.NewAttachmentRepository(db, logger)
	auditRepo := repository.NewAuditRepository(db, logger)

	// Initialize JWT service
	jwtService := service.NewJWTService(cfg.Security.JWTSecret, cfg.Security.JWTExpiration)

	// Initialize crypto service
	cryptoService := service.NewCryptoService(cfg.Security.EncryptionKey)

	// Initialize storage service
	storageService, err := service.NewStorageService(cfg.Storage, logger)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize network service
	networkAdapters := make(map[models.NetworkType]service.NetworkAdapter)

	// Initialize adapters based on config
	// For now, create placeholders
	networkAdapters[models.NetworkClearnet] = &ClearnetAdapter{logger: logger}

	if cfg.Network.EnableI2P {
		networkAdapters[models.NetworkI2P] = &I2PAdapter{logger: logger}
	}

	if cfg.Network.EnableTor {
		networkAdapters[models.NetworkTor] = &TorAdapter{logger: logger}
	}

	if cfg.Network.EnableIPFS {
		networkAdapters[models.NetworkIPFS] = &IPFSAdapter{logger: logger}
	}

	networkSvc := service.NewNetworkService(networkAdapters, logger)

	// Initialize email parser and pipeline
	emailParser := email.NewEmailParser()
	emailPipeline := email.NewEmailPipeline()

	// Initialize email service
	emailSvc, err := service.NewEmailService(
		emailRepo,
		userRepo,
		attachmentRepo,
		emailParser,
		emailPipeline,
		storageService,
		networkSvc,
		cryptoService,
		logger,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to initialize email service: %w", err)
	}

	// Initialize auth service
	authSvc := service.NewAuthService(
		userRepo,
		sessionRepo,
		passwordResetRepo,
		jwtService,
		cryptoService,
		cfg.Security.LockoutDuration,
		cfg.Security.MaxLoginAttempts,
		logger,
	)

	// Initialize handlers
	h := handlers.NewHandlers(
		authSvc,
		emailSvc,
		networkSvc,
		cryptoService,
		userRepo,
		emailRepo,
		sessionRepo,
		folderRepo,
		labelRepo,
		contactRepo,
		networkRepo,
		providerRepo,
		keyRepo,
		statsRepo,
		auditRepo,
		logger,
	)

	return h, jwtService, authSvc, nil
}

func initHTTPServer(
	h *handlers.Handlers,
	cfg *config.Config,
	logger *zap.Logger,
	jwtService service.JWTService,
	authService service.AuthService,
) *api.Server {

	server := api.NewServer(cfg, logger, h, jwtService, authService)

	if err := server.Setup(); err != nil {
		logger.Fatal("Failed to setup server", zap.Error(err))
	}

	return server
}

// Placeholder adapters
type ClearnetAdapter struct {
	logger *zap.Logger
}

func (c *ClearnetAdapter) TestConnection(ctx context.Context) (bool, error) {
	return true, nil
}

func (c *ClearnetAdapter) SendMessage(ctx context.Context, message interface{}) error {
	return nil
}

func (c *ClearnetAdapter) ReceiveMessages(ctx context.Context) ([]interface{}, error) {
	return nil, nil
}

func (c *ClearnetAdapter) GetStatus(ctx context.Context) service.NetworkStatus {
	return service.NetworkStatus{
		IsHealthy:    true,
		LastChecked:  time.Now(),
		MessageCount: 0,
		Latency:      0,
	}
}

type I2PAdapter struct {
	logger *zap.Logger
}

func (i *I2PAdapter) TestConnection(ctx context.Context) (bool, error) {
	return false, nil
}

func (i *I2PAdapter) SendMessage(ctx context.Context, message interface{}) error {
	return fmt.Errorf("not implemented")
}

func (i *I2PAdapter) ReceiveMessages(ctx context.Context) ([]interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (i *I2PAdapter) GetStatus(ctx context.Context) service.NetworkStatus {
	return service.NetworkStatus{
		IsHealthy:    false,
		LastError:    "I2P not configured",
		LastChecked:  time.Now(),
		MessageCount: 0,
		Latency:      0,
	}
}

type TorAdapter struct {
	logger *zap.Logger
}

func (t *TorAdapter) TestConnection(ctx context.Context) (bool, error) {
	return false, nil
}

func (t *TorAdapter) SendMessage(ctx context.Context, message interface{}) error {
	return fmt.Errorf("not implemented")
}

func (t *TorAdapter) ReceiveMessages(ctx context.Context) ([]interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (t *TorAdapter) GetStatus(ctx context.Context) service.NetworkStatus {
	return service.NetworkStatus{
		IsHealthy:    false,
		LastError:    "Tor not configured",
		LastChecked:  time.Now(),
		MessageCount: 0,
		Latency:      0,
	}
}

type IPFSAdapter struct {
	logger *zap.Logger
}

func (i *IPFSAdapter) TestConnection(ctx context.Context) (bool, error) {
	return false, nil
}

func (i *IPFSAdapter) SendMessage(ctx context.Context, message interface{}) error {
	return fmt.Errorf("not implemented")
}

func (i *IPFSAdapter) ReceiveMessages(ctx context.Context) ([]interface{}, error) {
	return nil, fmt.Errorf("not implemented")
}

func (i *IPFSAdapter) GetStatus(ctx context.Context) service.NetworkStatus {
	return service.NetworkStatus{
		IsHealthy:    false,
		LastError:    "IPFS not configured",
		LastChecked:  time.Now(),
		MessageCount: 0,
		Latency:      0,
	}
}
