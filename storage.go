package ratelimiter

import (
	"context"
	"time"
)

// RateLimit stores rate limiting information for a specific key
type RateLimit struct {
	Count     int
	LastReset time.Time
	BlockedAt time.Time
}

// Storage defines the interface for rate limit storage backends
type Storage interface {
	Get(ctx context.Context, key string) (*RateLimit, error)
	Set(ctx context.Context, key string, rateLimit *RateLimit, expiration time.Duration) error
	Close() error
}

// StorageConfig holds configuration for storage backends
type StorageConfig struct {
	Host     string
	Port     string
	Password string
	DB       int
}
