package middleware

import (
	"os"
	ratelimiter "rate-limiter"
	"rate-limiter/storage"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	originalEnvs := map[string]string{
		"IP_RATE_LIMIT":         os.Getenv("IP_RATE_LIMIT"),
		"IP_BLOCK_TIME":         os.Getenv("IP_BLOCK_TIME"),
		"SERVER_PORT":           os.Getenv("SERVER_PORT"),
		"TOKEN_TEST_LIMIT":      os.Getenv("TOKEN_TEST_LIMIT"),
		"TOKEN_TEST_BLOCK_TIME": os.Getenv("TOKEN_TEST_BLOCK_TIME"),
		"TOKEN_PROD_LIMIT":      os.Getenv("TOKEN_PROD_LIMIT"),
		"TOKEN_PROD_BLOCK_TIME": os.Getenv("TOKEN_PROD_BLOCK_TIME"),
	}

	cleanup := func() {
		for key, value := range originalEnvs {
			if value == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, value)
			}
		}
	}
	defer cleanup()

	t.Run("defaults", func(t *testing.T) {
		for key := range originalEnvs {
			os.Unsetenv(key)
		}

		config, err := storage.LoadConfig()
		require.NoError(t, err)

		assert.Equal(t, 10, config.RateLimit.IPRateLimit)
		assert.Equal(t, 300, config.RateLimit.IPBlockTime)
		assert.Equal(t, "8080", config.RateLimit.ServerPort)
		assert.NotNil(t, config.RateLimit.TokenLimits)
		assert.NotNil(t, config.RateLimit.TokenBlockTimes)
	})

	t.Run("env_vars", func(t *testing.T) {
		os.Setenv("IP_RATE_LIMIT", "20")
		os.Setenv("IP_BLOCK_TIME", "600")
		os.Setenv("SERVER_PORT", "9000")
		os.Setenv("TOKEN_TEST_LIMIT", "50")
		os.Setenv("TOKEN_TEST_BLOCK_TIME", "120")
		os.Setenv("TOKEN_PROD_LIMIT", "200")
		os.Setenv("TOKEN_PROD_BLOCK_TIME", "900")

		config, err := storage.LoadConfig()
		require.NoError(t, err)

		assert.Equal(t, 20, config.RateLimit.IPRateLimit)
		assert.Equal(t, 600, config.RateLimit.IPBlockTime)
		assert.Equal(t, "9000", config.RateLimit.ServerPort)

		assert.Equal(t, 50, config.RateLimit.TokenLimits["TEST"])
		assert.Equal(t, 120, config.RateLimit.TokenBlockTimes["TEST"])
		assert.Equal(t, 200, config.RateLimit.TokenLimits["PROD"])
		assert.Equal(t, 900, config.RateLimit.TokenBlockTimes["PROD"])
	})

	t.Run("invalid_values", func(t *testing.T) {
		os.Setenv("IP_RATE_LIMIT", "invalid")
		os.Setenv("IP_BLOCK_TIME", "not-a-number")
		os.Setenv("TOKEN_INVALID_LIMIT", "abc")

		config, err := storage.LoadConfig()
		require.NoError(t, err)

		assert.Equal(t, 10, config.RateLimit.IPRateLimit)
		assert.Equal(t, 300, config.RateLimit.IPBlockTime)

		_, exists := config.RateLimit.TokenLimits["INVALID"]
		assert.False(t, exists)
	})
}

func TestGetDefaultConfig(t *testing.T) {
	config := storage.GetDefaultConfig()

	assert.Equal(t, 10, config.RateLimit.IPRateLimit)
	assert.Equal(t, 300, config.RateLimit.IPBlockTime)
	assert.Equal(t, "8080", config.RateLimit.ServerPort)
	assert.NotNil(t, config.RateLimit.TokenLimits)
	assert.NotNil(t, config.RateLimit.TokenBlockTimes)
	assert.Equal(t, 0, len(config.RateLimit.TokenLimits))
	assert.Equal(t, 0, len(config.RateLimit.TokenBlockTimes))
}

func TestNewService(t *testing.T) {
	testStorage := createTestStorage(t)
	defer testStorage.Close()

	appConfig := storage.GetDefaultConfig()
	service := NewService(appConfig.RateLimit, testStorage)

	assert.NotNil(t, service)
	assert.NotNil(t, service.storage)
}

// createTestStorage creates a Redis storage for testing
func createTestStorage(t *testing.T) ratelimiter.Storage {
	config := ratelimiter.StorageConfig{
		Host:     "localhost",
		Port:     "6379",
		Password: "",
		DB:       1, // Use DB 1 for tests
	}

	redisStorage, err := storage.NewRedisStorage(config)
	if err != nil {
		t.Skipf("Redis not available for testing: %v", err)
	}

	return redisStorage
}

func TestServiceGetLimit(t *testing.T) {
	service := &Service{
		config: storage.Config{
			IPRateLimit: 10,
			TokenLimits: map[string]int{
				"ABC123": 100,
				"XYZ789": 50,
			},
		},
	}

	tests := []struct {
		name          string
		key           string
		isToken       bool
		expectedLimit int
	}{
		{
			name:          "ip_limit",
			key:           "192.168.1.1",
			isToken:       false,
			expectedLimit: 10,
		},
		{
			name:          "valid_token",
			key:           "token:ABC123",
			isToken:       true,
			expectedLimit: 100,
		},
		{
			name:          "unknown_token",
			key:           "token:UNKNOWN",
			isToken:       true,
			expectedLimit: 10,
		},
		{
			name:          "another_token",
			key:           "token:XYZ789",
			isToken:       true,
			expectedLimit: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			limit := service.getLimit(tt.key, tt.isToken)
			assert.Equal(t, tt.expectedLimit, limit)
		})
	}
}

func TestServiceGetBlockTime(t *testing.T) {
	service := &Service{
		config: storage.Config{
			IPBlockTime: 300,
			TokenBlockTimes: map[string]int{
				"ABC123": 600,
				"XYZ789": 120,
			},
		},
	}

	tests := []struct {
		name              string
		key               string
		isToken           bool
		expectedBlockTime int
	}{
		{
			name:              "ip_block",
			key:               "192.168.1.1",
			isToken:           false,
			expectedBlockTime: 300,
		},
		{
			name:              "token_block",
			key:               "token:ABC123",
			isToken:           true,
			expectedBlockTime: 600,
		},
		{
			name:              "unknown_token_block",
			key:               "token:UNKNOWN",
			isToken:           true,
			expectedBlockTime: 300,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			blockTime := service.getBlockTime(tt.key, tt.isToken)
			assert.Equal(t, tt.expectedBlockTime, blockTime)
		})
	}
}

func TestServiceShouldResetWindow(t *testing.T) {
	service := &Service{}

	t.Run("after_second", func(t *testing.T) {
		rateLimit := &ratelimiter.RateLimit{
			LastReset: time.Now().Add(-2 * time.Second),
		}

		shouldReset := service.shouldResetWindow(rateLimit)
		assert.True(t, shouldReset)
	})

	t.Run("before_second", func(t *testing.T) {
		rateLimit := &ratelimiter.RateLimit{
			LastReset: time.Now().Add(-500 * time.Millisecond),
		}

		shouldReset := service.shouldResetWindow(rateLimit)
		assert.False(t, shouldReset)
	})

	t.Run("exactly_second", func(t *testing.T) {
		rateLimit := &ratelimiter.RateLimit{
			LastReset: time.Now().Add(-1 * time.Second),
		}

		shouldReset := service.shouldResetWindow(rateLimit)
		assert.True(t, shouldReset)
	})
}

func TestServiceIsBlocked(t *testing.T) {
	service := &Service{}

	t.Run("not_blocked_zero", func(t *testing.T) {
		rateLimit := &ratelimiter.RateLimit{
			BlockedAt: time.Time{},
		}

		isBlocked := service.isBlocked(rateLimit, 300)
		assert.False(t, isBlocked)
	})

	t.Run("blocked_within_time", func(t *testing.T) {
		rateLimit := &ratelimiter.RateLimit{
			BlockedAt: time.Now().Add(-100 * time.Second),
		}

		isBlocked := service.isBlocked(rateLimit, 300)
		assert.True(t, isBlocked)
	})

	t.Run("not_blocked_time_passed", func(t *testing.T) {
		rateLimit := &ratelimiter.RateLimit{
			BlockedAt: time.Now().Add(-400 * time.Second),
		}

		isBlocked := service.isBlocked(rateLimit, 300)
		assert.False(t, isBlocked)
	})
}

func TestServiceCheckRateLimit(t *testing.T) {
	t.Run("new_client", func(t *testing.T) {
		testStorage := createTestStorage(t)
		defer testStorage.Close()

		service := &Service{
			config: storage.Config{
				IPRateLimit: 5,
				IPBlockTime: 300,
			},
			storage: testStorage,
		}

		allowed, err := service.CheckRateLimit("192.168.1.1", false)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("limit_exceeded", func(t *testing.T) {
		testStorage := createTestStorage(t)
		defer testStorage.Close()

		service := &Service{
			config: storage.Config{
				IPRateLimit: 2,
				IPBlockTime: 300,
			},
			storage: testStorage,
		}

		for i := 0; i < 2; i++ {
			allowed, err := service.CheckRateLimit("192.168.1.2", false)
			require.NoError(t, err)
			assert.True(t, allowed)
		}

		allowed, err := service.CheckRateLimit("192.168.1.2", false)
		require.NoError(t, err)
		assert.False(t, allowed)
	})

	t.Run("token_higher_limit", func(t *testing.T) {
		testStorage := createTestStorage(t)
		defer testStorage.Close()

		service := &Service{
			config: storage.Config{
				IPRateLimit:     2,
				TokenLimits:     map[string]int{"ABC123": 5},
				TokenBlockTimes: map[string]int{"ABC123": 300},
			},
			storage: testStorage,
		}

		for i := 0; i < 4; i++ {
			allowed, err := service.CheckRateLimit("token:ABC123", true)
			require.NoError(t, err)
			assert.True(t, allowed)
		}
	})

	t.Run("window_reset", func(t *testing.T) {
		testStorage := createTestStorage(t)
		defer testStorage.Close()

		service := &Service{
			config: storage.Config{
				IPRateLimit: 1,
				IPBlockTime: 1,
			},
			storage: testStorage,
		}

		allowed, err := service.CheckRateLimit("192.168.1.3", false)
		require.NoError(t, err)
		assert.True(t, allowed)

		allowed, err = service.CheckRateLimit("192.168.1.3", false)
		require.NoError(t, err)
		assert.False(t, allowed)

		// Wait for window reset
		time.Sleep(1100 * time.Millisecond)

		allowed, err = service.CheckRateLimit("192.168.1.3", false)
		require.NoError(t, err)
		assert.True(t, allowed)
	})

	t.Run("concurrent_safety", func(t *testing.T) {
		testStorage := createTestStorage(t)
		defer testStorage.Close()

		service := &Service{
			config: storage.Config{
				IPRateLimit: 100,
				IPBlockTime: 300,
			},
			storage: testStorage,
		}

		done := make(chan bool, 10)
		for i := 0; i < 10; i++ {
			go func(id int) {
				for j := 0; j < 10; j++ {
					service.CheckRateLimit("concurrent-test", false)
				}
				done <- true
			}(i)
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func BenchmarkServiceCheckRateLimit(b *testing.B) {
	config := ratelimiter.StorageConfig{
		Host:     "localhost",
		Port:     "6379",
		Password: "",
		DB:       1,
	}

	redisStorage, err := storage.NewRedisStorage(config)
	if err != nil {
		b.Skip("Redis not available for benchmarking")
	}
	defer redisStorage.Close()

	service := &Service{
		config: storage.Config{
			IPRateLimit: 1000000,
			IPBlockTime: 300,
		},
		storage: redisStorage,
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			service.CheckRateLimit("bench-test", false)
		}
	})
}

func BenchmarkServiceGetLimit(b *testing.B) {
	service := &Service{
		config: storage.Config{
			IPRateLimit: 10,
			TokenLimits: map[string]int{
				"ABC123": 100,
				"XYZ789": 50,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.getLimit("token:ABC123", true)
	}
}
