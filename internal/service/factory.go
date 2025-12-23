package service

import (
	"time"

	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/auth"
	"github.com/okavatti/mxil-server/m/internal/email"
	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/repository"
	authservice "github.com/okavatti/mxil-server/m/internal/service/auth"
	cryptoservice "github.com/okavatti/mxil-server/m/internal/service/crypto"
	emailservice "github.com/okavatti/mxil-server/m/internal/service/email"
	errorservice "github.com/okavatti/mxil-server/m/internal/service/errors"
	networkservice "github.com/okavatti/mxil-server/m/internal/service/network"
	storageservice "github.com/okavatti/mxil-server/m/internal/service/storage"
	"github.com/okavatti/mxil-server/m/internal/service/types"
)

// ServiceFactory creates all services with proper dependencies
type ServiceFactory struct {
	Logger *zap.Logger
	Config *types.ServiceConfig
}

// NewServiceFactory creates a new service factory
func NewServiceFactory(logger *zap.Logger, config *types.ServiceConfig) *ServiceFactory {
	return &ServiceFactory{
		Logger: logger,
		Config: config,
	}
}

// CreateAuthService creates an AuthService instance
func (f *ServiceFactory) CreateAuthService(
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	passwordResetRepo repository.PasswordResetRepository,
	jwtService *auth.JWTService,
) authservice.AuthService {
	return authservice.NewAuthService(
		userRepo,
		sessionRepo,
		passwordResetRepo,
		jwtService,
		f.CreateCryptoService(),
		time.Duration(f.Config.LockoutDuration)*time.Minute,
		f.Config.MaxLoginAttempts,
		f.Logger,
	)
}

// CreateCryptoService creates a CryptoService instance
func (f *ServiceFactory) CreateCryptoService() cryptoservice.CryptoService {
	return cryptoservice.NewCryptoService()
}

// CreateEmailService creates an EmailService instance
func (f *ServiceFactory) CreateEmailService(
	emailRepo repository.EmailRepository,
	userRepo repository.UserRepository,
	emailParser *email.EmailParser,
	emailPipeline *email.Pipeline,
	storageService storageservice.StorageService,
	networkService networkservice.NetworkService,
) emailservice.EmailService {
	return emailservice.NewEmailService(
		emailRepo,
		userRepo,
		emailParser,
		emailPipeline,
		storageService,
		networkService,
		f.CreateCryptoService(),
		f.Logger,
	)
}

// CreateNetworkService creates a NetworkService instance
func (f *ServiceFactory) CreateNetworkService(
	adapters map[models.NetworkType]networkservice.NetworkAdapter,
) networkservice.NetworkService {
	return networkservice.NewNetworkService(adapters, f.Logger)
}

// CreateStorageService creates a StorageService instance
func (f *ServiceFactory) CreateStorageService(
	config *storageservice.StorageConfig,
) (storageservice.StorageService, error) {
	return storageservice.StorageFactory(config, f.Logger)
}

// CreateErrorHandler creates an ErrorHandler instance
func (f *ServiceFactory) CreateErrorHandler() errorservice.ErrorHandler {
	return errorservice.NewErrorHandler(f.Logger)
}
