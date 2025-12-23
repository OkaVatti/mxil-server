// internal/api/handlers/handlers.go
package handlers

import (
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/repository"
	authservice "github.com/okavatti/mxil-server/m/internal/service/auth"
	cryptoservice "github.com/okavatti/mxil-server/m/internal/service/crypto"
	emailservice "github.com/okavatti/mxil-server/m/internal/service/email"
	networkservice "github.com/okavatti/mxil-server/m/internal/service/network"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	Auth       *AuthHandler
	Email      *EmailHandler
	User       *UserHandler
	Folder     *FolderHandler
	Label      *LabelHandler
	Contact    *ContactHandler
	Network    *NetworkHandler
	Provider   *ProviderHandler
	Encryption *EncryptionHandler
	Admin      *AdminHandler
	WebSocket  *WebSocketHandler
	Health     *HealthHandler
}

// NewHandlers creates all HTTP handlers
func NewHandlers(
	authService authservice.AuthService,
	emailService emailservice.EmailService,
	networkService networkservice.NetworkService,
	cryptoService cryptoservice.CryptoService,
	userRepo *repository.UserRepository,
	emailRepo *repository.EmailRepository,
	sessionRepo *repository.SessionRepository,
	folderRepo *repository.FolderRepository,
	labelRepo *repository.LabelRepository,
	contactRepo *repository.ContactRepository,
	networkRepo *repository.NetworkIdentityRepository,
	providerRepo *repository.ProviderBridgeRepository,
	keyRepo *repository.EncryptionKeyRepository,
	statsRepo *repository.StatsRepository,
	logger *zap.Logger,
) *Handlers {
	// Initialize individual handlers
	authHandler := NewAuthHandler(authService, logger)
	emailHandler := NewEmailHandler(emailService, emailRepo, userRepo, logger)
	userHandler := NewUserHandler(userRepo, sessionRepo, authService, logger)
	folderHandler := NewFolderHandler(folderRepo, emailRepo, logger)
	labelHandler := NewLabelHandler(labelRepo, emailRepo, logger)
	contactHandler := NewContactHandler(contactRepo, logger)
	networkHandler := NewNetworkHandler(networkService, networkRepo, logger)
	providerHandler := NewProviderHandler(providerRepo, logger)
	encryptionHandler := NewEncryptionHandler(cryptoService, keyRepo, logger)
	adminHandler := NewAdminHandler(userRepo, emailRepo, sessionRepo, networkRepo, statsRepo, logger)
	webSocketHandler := NewWebSocketHandler(authService, emailService, logger)
	healthHandler := &HealthHandler{}

	return &Handlers{
		Auth:       authHandler,
		Email:      emailHandler,
		User:       userHandler,
		Folder:     folderHandler,
		Label:      labelHandler,
		Contact:    contactHandler,
		Network:    networkHandler,
		Provider:   providerHandler,
		Encryption: encryptionHandler,
		Admin:      adminHandler,
		WebSocket:  webSocketHandler,
		Health:     healthHandler,
	}
}
