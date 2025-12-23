package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/okavatti/mxil-server/m/internal/models"
	"github.com/okavatti/mxil-server/m/internal/service/email"
)

// networkService implements NetworkService
type networkService struct {
	adapters        map[models.NetworkType]NetworkAdapter
	logger          *zap.Logger
	statusCache     sync.Map
	connectionPool  map[models.NetworkType][]interface{}
	poolMutex       sync.RWMutex
	maxConnections  int
	healthCheckFreq time.Duration
}

// NewNetworkService creates a new network service
func NewNetworkService(adapters map[models.NetworkType]NetworkAdapter, logger *zap.Logger) NetworkService {
	service := &networkService{
		adapters:        adapters,
		logger:          logger,
		connectionPool:  make(map[models.NetworkType][]interface{}),
		maxConnections:  10,
		healthCheckFreq: 5 * time.Minute,
	}

	// Start background health checks
	go service.startHealthChecks()

	return service
}

// GetNetworkStatus returns status of all networks
func (s *networkService) GetNetworkStatus(ctx context.Context) (map[models.NetworkType]NetworkStatus, error) {
	status := make(map[models.NetworkType]NetworkStatus)

	// Check cached status first
	for network := range s.adapters {
		if cached, ok := s.statusCache.Load(network); ok {
			if cachedStatus, ok := cached.(NetworkStatus); ok {
				// If cache is fresh (less than 30 seconds old), use it
				if time.Since(cachedStatus.LastChecked) < 30*time.Second {
					status[network] = cachedStatus
					continue
				}
			}
		}

		// Perform fresh check
		adapter := s.adapters[network]
		if adapter != nil {
			status[network] = adapter.GetStatus()
			s.statusCache.Store(network, status[network])
		}
	}

	return status, nil
}

// TestNetwork tests connectivity to a network
func (s *networkService) TestNetwork(ctx context.Context, network models.NetworkType) (bool, error) {
	adapter, ok := s.adapters[network]
	if !ok {
		return false, fmt.Errorf("network adapter not found: %s", network)
	}

	healthy, err := adapter.TestConnection()
	if err != nil {
		s.logger.Error("Network test failed",
			zap.String("network", string(network)),
			zap.Error(err))
		return false, fmt.Errorf("network test failed: %w", err)
	}

	return healthy, nil
}

// SendEmail sends an email via specified network
func (s *networkService) SendEmail(ctx context.Context, email *models.Email, attachments []email.AttachmentInfo) error {
	adapter, ok := s.adapters[email.Network]
	if !ok {
		return fmt.Errorf("network adapter not found: %s", email.Network)
	}

	// Add retry logic
	var lastErr error
	maxRetries := 3
	baseDelay := 1 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := adapter.Send(email)
		if err == nil {
			s.logger.Info("Email sent successfully",
				zap.String("email_id", email.ID.String()),
				zap.String("network", string(email.Network)))
			return nil
		}

		lastErr = err
		s.logger.Warn("Failed to send email, retrying",
			zap.String("email_id", email.ID.String()),
			zap.Int("attempt", attempt),
			zap.Error(err))

		if attempt < maxRetries {
			// Exponential backoff
			delay := baseDelay * time.Duration(1<<(attempt-1))
			time.Sleep(delay)
		}
	}

	s.logger.Error("All attempts to send email failed",
		zap.String("email_id", email.ID.String()),
		zap.String("network", string(email.Network)),
		zap.Error(lastErr))

	return fmt.Errorf("failed to send email after %d attempts: %w", maxRetries, lastErr)
}

// Helper method to start background health checks
func (s *networkService) startHealthChecks() {
	ticker := time.NewTicker(s.healthCheckFreq)
	defer ticker.Stop()

	for range ticker.C {
		for network, adapter := range s.adapters {
			status := adapter.GetStatus()
			s.statusCache.Store(network, status)

			if !status.IsHealthy {
				s.logger.Warn("Network health check failed",
					zap.String("network", string(network)),
					zap.String("error", status.LastError))
			}
		}
	}
}

// Helper method to get connection from pool
func (s *networkService) getConnection(network models.NetworkType) (interface{}, error) {
	s.poolMutex.RLock()
	pool, exists := s.connectionPool[network]
	s.poolMutex.RUnlock()

	if exists && len(pool) > 0 {
		s.poolMutex.Lock()
		conn := pool[0]
		pool = pool[1:]
		s.connectionPool[network] = pool
		s.poolMutex.Unlock()
		return conn, nil
	}

	// Create new connection
	adapter := s.adapters[network]
	if adapter == nil {
		return nil, fmt.Errorf("no adapter for network: %s", network)
	}

	// TODO: Implement connection creation based on adapter type
	return nil, nil
}

// Helper method to return connection to pool
func (s *networkService) returnConnection(network models.NetworkType, conn interface{}) {
	s.poolMutex.Lock()
	defer s.poolMutex.Unlock()

	pool := s.connectionPool[network]
	if len(pool) < s.maxConnections {
		s.connectionPool[network] = append(pool, conn)
	}
	// If pool is full, just discard the connection
}
