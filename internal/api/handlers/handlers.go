// internal/api/handlers/handlers.go
package handlers

import (
	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/repository"
	"github.com/okavatti/mxil-server/m/internal/service"
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
	authService service.AuthService,
	emailService service.EmailService,
	networkService service.NetworkService,
	cryptoService service.CryptoService,
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
	return &Handlers{
		Auth:       NewAuthHandler(authService, logger),
		Email:      NewEmailHandler(emailService, emailRepo, userRepo, logger),
		User:       NewUserHandler(userRepo, sessionRepo, authService, logger),
		Folder:     NewFolderHandler(folderRepo, emailRepo, logger),
		Label:      NewLabelHandler(labelRepo, emailRepo, logger),
		Contact:    NewContactHandler(contactRepo, logger),
		Network:    NewNetworkHandler(networkService, networkRepo, logger),
		Provider:   NewProviderHandler(providerRepo, logger),
		Encryption: NewEncryptionHandler(cryptoService, keyRepo, logger),
		Admin:      NewAdminHandler(userRepo, emailRepo, networkRepo, statsRepo, logger),
		WebSocket:  NewWebSocketHandler(authService, emailService, logger),
		Health:     &HealthHandler{},
	}
}
