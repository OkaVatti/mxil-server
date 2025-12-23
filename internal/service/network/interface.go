package network

import (
	"context"
	"time"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/service/email"
)

// NetworkService handles network operations
type NetworkService interface {
	GetNetworkStatus(ctx context.Context) (map[models.NetworkType]NetworkStatus, error)
	TestNetwork(ctx context.Context, network models.NetworkType) (bool, error)
	SendEmail(ctx context.Context, email *models.Email, attachments []email.AttachmentInfo) error
}

// NetworkStatus represents the status of a network
type NetworkStatus struct {
	IsHealthy    bool
	LastError    string
	LastChecked  time.Time
	MessageCount int64
	Latency      time.Duration
}

// NetworkAdapter interface
type NetworkAdapter interface {
	Send(email *models.Email) error
	TestConnection() (bool, error)
	GetStatus() NetworkStatus
}
