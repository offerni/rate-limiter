package storage

import (
	"context"
	"encoding/json"
	"fmt"
	ratelimiter "rate-limiter"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStorage struct {
	client *redis.Client
}

func NewRedisStorage(config ratelimiter.StorageConfig) (ratelimiter.Storage, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.Host, config.Port),
		Password: config.Password,
		DB:       config.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	return &RedisStorage{
		client: rdb,
	}, nil
}

func (r *RedisStorage) Get(ctx context.Context, key string) (*ratelimiter.RateLimit, error) {
	data, err := r.client.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get from Redis: %w", err)
	}

	var rateLimit ratelimiter.RateLimit
	if err := json.Unmarshal([]byte(data), &rateLimit); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rate limit: %w", err)
	}

	return &rateLimit, nil
}

func (r *RedisStorage) Set(ctx context.Context, key string, rateLimit *ratelimiter.RateLimit, expiration time.Duration) error {
	data, err := json.Marshal(rateLimit)
	if err != nil {
		return fmt.Errorf("failed to marshal rate limit: %w", err)
	}

	err = r.client.Set(ctx, key, data, expiration).Err()
	if err != nil {
		return fmt.Errorf("failed to set in Redis: %w", err)
	}

	return nil
}

func (r *RedisStorage) Close() error {
	return r.client.Close()
}
