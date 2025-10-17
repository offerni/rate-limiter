package middleware

import (
	"context"
	ratelimiter "rate-limiter"
	"rate-limiter/storage"
	"strings"
	"time"
)

type Service struct {
	config  storage.Config
	storage ratelimiter.Storage
}

func NewService(config storage.Config, rateLimitStorage ratelimiter.Storage) *Service {
	return &Service{
		config:  config,
		storage: rateLimitStorage,
	}
}

func (s *Service) CheckRateLimit(key string, isToken bool) (bool, error) {
	ctx := context.Background()

	rateLimit, err := s.storage.Get(ctx, key)
	if err != nil {
		return false, err
	}

	if rateLimit == nil {
		rateLimit = &ratelimiter.RateLimit{
			Count:     0,
			LastReset: time.Now(),
			BlockedAt: time.Time{},
		}
	}

	limit := s.getLimit(key, isToken)
	blockTime := s.getBlockTime(key, isToken)

	if s.shouldResetWindow(rateLimit) {
		rateLimit.Count = 0
		rateLimit.LastReset = time.Now()
		rateLimit.BlockedAt = time.Time{}
	}

	if s.isBlocked(rateLimit, blockTime) {
		return false, nil
	}

	if rateLimit.Count >= limit {
		rateLimit.BlockedAt = time.Now()
		expiration := time.Duration(blockTime) * time.Second
		if err := s.storage.Set(ctx, key, rateLimit, expiration); err != nil {
			return false, err
		}
		return false, nil
	}

	rateLimit.Count++
	expiration := time.Duration(blockTime) * time.Second
	if err := s.storage.Set(ctx, key, rateLimit, expiration); err != nil {
		return false, err
	}

	return true, nil
}

func (s *Service) isBlocked(rateLimit *ratelimiter.RateLimit, blockTime int) bool {
	if rateLimit.BlockedAt.IsZero() {
		return false
	}
	return time.Since(rateLimit.BlockedAt).Seconds() < float64(blockTime)
}

func (s *Service) shouldResetWindow(rateLimit *ratelimiter.RateLimit) bool {
	return time.Since(rateLimit.LastReset).Seconds() >= 1.0
}

func (s *Service) getLimit(key string, isToken bool) int {
	if isToken {
		tokenParts := strings.Split(key, ":")
		if len(tokenParts) == 2 {
			tokenName := tokenParts[1]
			if limit, exists := s.config.TokenLimits[tokenName]; exists {
				return limit
			}
		}
	}
	return s.config.IPRateLimit
}

func (s *Service) getBlockTime(key string, isToken bool) int {
	if isToken {
		tokenParts := strings.Split(key, ":")
		if len(tokenParts) == 2 {
			tokenName := tokenParts[1]
			if blockTime, exists := s.config.TokenBlockTimes[tokenName]; exists {
				return blockTime
			}
		}
	}
	return s.config.IPBlockTime
}
