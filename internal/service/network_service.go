package service

import (
	"context"
	"fmt"
	"time"

	"github.com/okavatti/mxil-server/m/internal/models"
)

// NetworkService handles network operations
type NetworkService struct {
	adapters map[models.NetworkType]NetworkAdapter
}

// NewNetworkService creates a new network service
func NewNetworkService(adapters map[models.NetworkType]NetworkAdapter) *NetworkService {
	return &NetworkService{
		adapters: adapters,
	}
}

// NetworkAdapter defines the interface for network adapters
type NetworkAdapter interface {
	Connect(ctx context.Context) error
	Disconnect() error
	SendEmail(ctx context.Context, email *models.Email) error
	ReceiveEmails(ctx context.Context) ([]*models.Email, error)
	TestConnection(ctx context.Context) (bool, error)
	GetStatus() NetworkStatus
}

// NetworkStatus represents network status information
type NetworkStatus struct {
	IsHealthy    bool
	LastError    string
	LastChecked  time.Time
	MessageCount int
	Latency      time.Duration
}

// GetNetworkStatus returns status of all networks
func (s *NetworkService) GetNetworkStatus(ctx context.Context) (map[models.NetworkType]NetworkStatus, error) {
	status := make(map[models.NetworkType]NetworkStatus)

	for networkType, adapter := range s.adapters {
		status[networkType] = adapter.GetStatus()
	}

	return status, nil
}

// TestNetwork tests connectivity for a network
func (s *NetworkService) TestNetwork(ctx context.Context, network models.NetworkType) (bool, error) {
	adapter, exists := s.adapters[network]
	if !exists {
		return false, fmt.Errorf("network adapter not found: %s", network)
	}

	return adapter.TestConnection(ctx)
}

// SendEmailViaNetwork sends an email via the specified network
func (s *NetworkService) SendEmailViaNetwork(ctx context.Context, email *models.Email) error {
	adapter, exists := s.adapters[email.Network]
	if !exists {
		return fmt.Errorf("network adapter not found: %s", email.Network)
	}

	return adapter.SendEmail(ctx, email)
}

// ReceiveEmailsFromNetwork receives emails from the specified network
func (s *NetworkService) ReceiveEmailsFromNetwork(ctx context.Context, network models.NetworkType) ([]*models.Email, error) {
	adapter, exists := s.adapters[network]
	if !exists {
		return nil, fmt.Errorf("network adapter not found: %s", network)
	}

	return adapter.ReceiveEmails(ctx)
}

// ConnectToNetwork connects to a network
func (s *NetworkService) ConnectToNetwork(ctx context.Context, network models.NetworkType) error {
	adapter, exists := s.adapters[network]
	if !exists {
		return fmt.Errorf("network adapter not found: %s", network)
	}

	return adapter.Connect(ctx)
}

// DisconnectFromNetwork disconnects from a network
func (s *NetworkService) DisconnectFromNetwork(network models.NetworkType) error {
	adapter, exists := s.adapters[network]
	if !exists {
		return fmt.Errorf("network adapter not found: %s", network)
	}

	return adapter.Disconnect()
}

// IsNetworkAvailable checks if a network is available
func (s *NetworkService) IsNetworkAvailable(ctx context.Context, network models.NetworkType) bool {
	adapter, exists := s.adapters[network]
	if !exists {
		return false
	}

	ok, _ := adapter.TestConnection(ctx)
	return ok
}

// GetAvailableNetworks returns list of available networks
func (s *NetworkService) GetAvailableNetworks(ctx context.Context) []models.NetworkType {
	var available []models.NetworkType

	for networkType, adapter := range s.adapters {
		if ok, _ := adapter.TestConnection(ctx); ok {
			available = append(available, networkType)
		}
	}

	return available
}

// MonitorNetworks starts monitoring all networks
func (s *NetworkService) MonitorNetworks(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.checkAllNetworks(ctx)
		}
	}
}

func (s *NetworkService) checkAllNetworks(ctx context.Context) {
	for networkType, adapter := range s.adapters {
		go func(network models.NetworkType, adapter NetworkAdapter) {
			if _, err := adapter.TestConnection(ctx); err != nil {
				// Log network error
				fmt.Printf("Network %s error: %v\n", network, err)
			}
		}(networkType, adapter)
	}
}
