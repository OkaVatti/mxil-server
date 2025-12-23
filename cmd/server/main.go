package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/okavatti/mxil-server/m/internal/api"
	"github.com/okavatti/mxil-server/m/internal/api/handlers"
	"github.com/okavatti/mxil-server/m/internal/auth"
	"github.com/okavatti/mxil-server/m/internal/config"
	"github.com/okavatti/mxil-server/m/internal/crypto"
	"github.com/okavatti/mxil-server/m/internal/database"
	"github.com/okavatti/mxil-server/m/internal/email"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/network/clearnet"
	"github.com/okavatti/mxil-server/m/internal/network/i2p"
	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service"
	"github.com/okavatti/mxil-server/m/internal/storage"

	"go.uber.org/zap"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	var logger *zap.Logger
	if cfg.Logging.Format == "json" {
		logger, err = zap.NewProduction()
	} else {
		logger, err = zap.NewDevelopment()
	}
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync()

	logger.Info("Starting MXIL Server",
		zap.String("version", "1.0.0"),
		zap.String("environment", os.Getenv("ENVIRONMENT")),
	)

	// Connect to database
	db, err := database.NewDatabase(&cfg.Database)
	if err != nil {
		logger.Fatal("Failed to connect to database", zap.Error(err))
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("Failed to close database", zap.Error(err))
		}
	}()

	// Run migrations
	logger.Info("Running database migrations...")
	if err := db.Migrate(); err != nil {
		logger.Fatal("Failed to run migrations", zap.Error(err))
	}
	logger.Info("Database migrations completed")

	// Initialize repositories
	userRepo := repository.NewUserRepository(db.DB)
	emailRepo := repository.NewEmailRepository(db.DB)
	sessionRepo := repository.NewSessionRepository(db.DB)

	// Initialize crypto service
	cryptoService, err := crypto.NewCryptoService(string(cfg.Security.EncryptionKey))
	if err != nil {
		logger.Fatal("Failed to initialize crypto service", zap.Error(err))
	}

	// Initialize JWT service
	jwtService := auth.NewJWTService(cfg.Security.JWTSecret, cfg.Security.JWTExpiration)

	// Initialize storage service
	storageService, err := storage.NewLocalStorage(cfg.Storage.LocalPath, cfg.Storage.MaxFileSize)
	if err != nil {
		logger.Fatal("Failed to initialize storage service", zap.Error(err))
	}

	// Initialize email components
	emailParser := email.NewEmailParser()
	emailPipeline := email.NewPipeline([]email.PipelineStage{
		&email.SpamFilterStage{},
		&email.VirusScanStage{},
		&email.DKIMVerificationStage{},
	})

	// Initialize network adapters
	adapters := make(map[models.NetworkType]service.NetworkAdapter)

	// Clearnet adapter
	clearnetAdapter := clearnet.NewClearnetAdapter(
		"localhost",
		cfg.Email.SMTPPort,
		"", "", // SMTP credentials (should be configured)
		"localhost",
		cfg.Email.IMAPPort,
	)
	adapters[models.NetworkClearnet] = clearnetAdapter

	// I2P adapter (if enabled)
	if cfg.Network.EnableI2P {
		logger.Info("Initializing I2P network adapter...")
		i2pAdapter := i2p.NewI2PAdapter("127.0.0.1:7656")
		adapters[models.NetworkI2P] = i2pAdapter

		// Test I2P connection
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := i2pAdapter.Connect(ctx); err != nil {
			logger.Warn("Failed to connect to I2P network", zap.Error(err))
		} else {
			logger.Info("I2P network adapter initialized successfully")
		}
	}

	// Initialize network service
	networkService := service.NewNetworkService(adapters)

	// Initialize email service
	emailService := service.NewEmailService(
		emailRepo,
		userRepo,
		emailParser,
		emailPipeline,
		storageService,
		networkService,
		cryptoService,
		logger,
	)

	// Initialize auth service
	authService := service.NewAuthService(
		userRepo,
		sessionRepo,
		emailRepo,
		jwtService,
		cryptoService,
		emailService,
		cfg.Security.LockoutDuration,
		cfg.Security.MaxLoginAttempts,
	)

	// Initialize handlers
	handler := handlers.NewHandlers(
		authService,
		emailService,
		userRepo,
		emailRepo,
		sessionRepo,
		logger,
	)

	// Set database in health handler
	handler.Health.db = db

	// Create API server
	server := api.NewServer(cfg, logger, handler)

	// Setup routes
	logger.Info("Setting up API routes...")
	if err := server.Setup(); err != nil {
		logger.Fatal("Failed to setup server", zap.Error(err))
	}

	// Start server in goroutine
	go func() {
		logger.Info("Starting HTTP server",
			zap.String("host", cfg.Server.Host),
			zap.Int("port", cfg.Server.Port),
			zap.Bool("tls", cfg.Server.TLSEnabled),
		)

		if err := server.Start(); err != nil {
			logger.Fatal("Failed to start server", zap.Error(err))
		}
	}()

	// Start background services
	go startBackgroundServices(logger, storageService)

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit

	logger.Info("Shutting down server...", zap.String("signal", sig.String()))

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Shutdown server
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatal("Failed to shutdown server gracefully", zap.Error(err))
	}

	logger.Info("Server stopped")
}

// startBackgroundServices starts background maintenance tasks
func startBackgroundServices(logger *zap.Logger, storageService service.StorageService) {
	// Cleanup temporary files every hour
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		ctx := context.Background()

		// Cleanup temp files older than 24 hours
		if err := storageService.CleanupTempFiles(ctx, 24*time.Hour); err != nil {
			logger.Error("Failed to cleanup temp files", zap.Error(err))
		}

		// Log storage statistics daily
		if time.Now().Hour() == 0 {
			if stats, err := storageService.GetStats(ctx); err == nil {
				logger.Info("Storage statistics", zap.Any("stats", stats))
			}
		}
	}
}
