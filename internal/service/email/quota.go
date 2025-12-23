// internal/service/email/quota.go
package email

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/okavatti/mxil-server/m/internal/models"
)

var (
	ErrQuotaExceeded      = fmt.Errorf("storage quota exceeded")
	ErrRateLimitExceeded  = fmt.Errorf("sending rate limit exceeded")
	ErrDailyLimitExceeded = fmt.Errorf("daily sending limit exceeded")
	ErrRecipientLimit     = fmt.Errorf("recipient limit exceeded")
)

// RateLimiter tracks sending rates
type RateLimiter struct {
	mu           sync.RWMutex
	dailyCounts  map[uuid.UUID]int64
	hourlyCounts map[uuid.UUID]int64
	minuteCounts map[uuid.UUID]int64
	dailyLimits  map[uuid.UUID]int64
	lastReset    time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		dailyCounts:  make(map[uuid.UUID]int64),
		hourlyCounts: make(map[uuid.UUID]int64),
		minuteCounts: make(map[uuid.UUID]int64),
		dailyLimits:  make(map[uuid.UUID]int64),
		lastReset:    time.Now(),
	}
}

func (s *emailService) checkQuota(ctx context.Context, userID uuid.UUID, additionalSize int64) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Check if user is over quota
	if user.StorageQuotaUsed+additionalSize > user.StorageQuotaTotal {
		return ErrQuotaExceeded
	}

	// Check if additional size is reasonable
	if additionalSize > s.maxEmailSize {
		return fmt.Errorf("email size exceeds maximum limit of %d bytes", s.maxEmailSize)
	}

	return nil
}

func (s *emailService) checkSendingRate(ctx context.Context, userID uuid.UUID) error {
	// Get user's sending limits
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Default limits if not set
	var (
		dailyLimit  int64 = 1000
		hourlyLimit int64 = 100
		minuteLimit int64 = 10
	)

	// TODO: Get limits from user settings or subscription

	// Check daily limit
	dailyCount := s.getDailyCount(userID)
	if dailyCount >= dailyLimit {
		return ErrDailyLimitExceeded
	}

	// Check hourly limit
	hourlyCount := s.getHourlyCount(userID)
	if hourlyCount >= hourlyLimit {
		return ErrRateLimitExceeded
	}

	// Check minute limit
	minuteCount := s.getMinuteCount(userID)
	if minuteCount >= minuteLimit {
		return ErrRateLimitExceeded
	}

	return nil
}

func (s *emailService) recordSending(ctx context.Context, userID uuid.UUID) {
	// Increment counters
	s.incrementDailyCount(userID)
	s.incrementHourlyCount(userID)
	s.incrementMinuteCount(userID)

	// Clean up old counters periodically
	go s.cleanupOldCounters()
}

func (s *emailService) sendViaNetwork(ctx context.Context, email *models.Email, req SendEmailRequest) error {
	// Use the NetworkService interface
	return s.networkService.SendEmail(ctx, email, req.Attachments)
}

// Helper methods for rate limiting
func (s *emailService) getDailyCount(userID uuid.UUID) int64 {
	// TODO: Implement with Redis or database for persistence
	// For now, use in-memory map
	key := fmt.Sprintf("daily:%s:%s", userID.String(), time.Now().Format("2006-01-02"))

	// This would query from Redis
	// count, _ := s.redis.Get(ctx, key).Int64()
	// return count

	return 0 // Placeholder
}

func (s *emailService) getHourlyCount(userID uuid.UUID) int64 {
	key := fmt.Sprintf("hourly:%s:%s", userID.String(), time.Now().Format("2006-01-02-15"))
	return 0 // Placeholder
}

func (s *emailService) getMinuteCount(userID uuid.UUID) int64 {
	key := fmt.Sprintf("minute:%s:%s", userID.String(), time.Now().Format("2006-01-02-15-04"))
	return 0 // Placeholder
}

func (s *emailService) incrementDailyCount(userID uuid.UUID) {
	key := fmt.Sprintf("daily:%s:%s", userID.String(), time.Now().Format("2006-01-02"))
	// s.redis.Incr(ctx, key)
	// s.redis.Expire(ctx, key, 48*time.Hour) // Keep for 2 days
}

func (s *emailService) incrementHourlyCount(userID uuid.UUID) {
	key := fmt.Sprintf("hourly:%s:%s", userID.String(), time.Now().Format("2006-01-02-15"))
	// s.redis.Incr(ctx, key)
	// s.redis.Expire(ctx, key, 2*time.Hour) // Keep for 2 hours
}

func (s *emailService) incrementMinuteCount(userID uuid.UUID) {
	key := fmt.Sprintf("minute:%s:%s", userID.String(), time.Now().Format("2006-01-02-15-04"))
	// s.redis.Incr(ctx, key)
	// s.redis.Expire(ctx, key, 2*time.Minute) // Keep for 2 minutes
}

func (s *emailService) cleanupOldCounters() {
	// This would be called periodically to clean up old rate limit counters
	// For now, it's a placeholder
}

// UpdateUserQuota updates user's storage quota
func (s *emailService) UpdateUserQuota(ctx context.Context, userID uuid.UUID, sizeChange int64) error {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	user.StorageQuotaUsed += sizeChange

	// Ensure quota doesn't go negative
	if user.StorageQuotaUsed < 0 {
		user.StorageQuotaUsed = 0
	}

	if err := s.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user quota: %w", err)
	}

	return nil
}

// GetQuotaInfo returns quota information for a user
func (s *emailService) GetQuotaInfo(ctx context.Context, userID uuid.UUID) (*QuotaInfo, error) {
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	return &QuotaInfo{
		Total:        user.StorageQuotaTotal,
		Used:         user.StorageQuotaUsed,
		Available:    user.StorageQuotaTotal - user.StorageQuotaUsed,
		UsagePercent: float64(user.StorageQuotaUsed) / float64(user.StorageQuotaTotal) * 100,
	}, nil
}

// QuotaInfo represents quota information
type QuotaInfo struct {
	Total        int64   `json:"total"`
	Used         int64   `json:"used"`
	Available    int64   `json:"available"`
	UsagePercent float64 `json:"usage_percent"`
}
