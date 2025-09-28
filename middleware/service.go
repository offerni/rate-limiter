package middleware

import (
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	IPRateLimit     int
	IPBlockTime     int
	TokenLimits     map[string]int
	TokenBlockTimes map[string]int
	ServerPort      string
}

type RateLimit struct {
	Count     int
	LastReset time.Time
	BlockedAt time.Time
}

type Service struct {
	config     Config
	rateLimits map[string]*RateLimit
	mutex      sync.RWMutex
}

func NewService() *Service {
	config, err := LoadConfig()
	if err != nil {
		config = getDefaultConfig()
	}

	return &Service{
		config:     config,
		rateLimits: make(map[string]*RateLimit),
		mutex:      sync.RWMutex{},
	}
}

func LoadConfig() (Config, error) {
	err := godotenv.Load()
	if err != nil {
	}

	config := Config{
		TokenLimits:     make(map[string]int),
		TokenBlockTimes: make(map[string]int),
	}

	if val := os.Getenv("IP_RATE_LIMIT"); val != "" {
		if limit, err := strconv.Atoi(val); err == nil {
			config.IPRateLimit = limit
		} else {
			config.IPRateLimit = 10
		}
	} else {
		config.IPRateLimit = 10
	}

	if val := os.Getenv("IP_BLOCK_TIME"); val != "" {
		if blockTime, err := strconv.Atoi(val); err == nil {
			config.IPBlockTime = blockTime
		} else {
			config.IPBlockTime = 300
		}
	} else {
		config.IPBlockTime = 300
	}

	config.ServerPort = os.Getenv("SERVER_PORT")
	if config.ServerPort == "" {
		config.ServerPort = "8080"
	}

	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) != 2 {
			continue
		}

		key, value := pair[0], pair[1]

		if strings.HasPrefix(key, "TOKEN_") && strings.HasSuffix(key, "_LIMIT") {
			tokenName := strings.TrimPrefix(key, "TOKEN_")
			tokenName = strings.TrimSuffix(tokenName, "_LIMIT")

			if limit, err := strconv.Atoi(value); err == nil {
				config.TokenLimits[tokenName] = limit
			}
		}

		if strings.HasPrefix(key, "TOKEN_") && strings.HasSuffix(key, "_BLOCK_TIME") {
			tokenName := strings.TrimPrefix(key, "TOKEN_")
			tokenName = strings.TrimSuffix(tokenName, "_BLOCK_TIME")

			if blockTime, err := strconv.Atoi(value); err == nil {
				config.TokenBlockTimes[tokenName] = blockTime
			}
		}
	}

	return config, nil
}

func getDefaultConfig() Config {
	return Config{
		IPRateLimit:     10,
		IPBlockTime:     300,
		TokenLimits:     make(map[string]int),
		TokenBlockTimes: make(map[string]int),
		ServerPort:      "8080",
	}
}

func (s *Service) CheckRateLimit(key string, isToken bool) (bool, error) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	rateLimit, exists := s.rateLimits[key]
	if !exists {
		rateLimit = &RateLimit{
			Count:     0,
			LastReset: time.Now(),
			BlockedAt: time.Time{},
		}
		s.rateLimits[key] = rateLimit
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
		return false, nil
	}

	rateLimit.Count++
	return true, nil
}

func (s *Service) isBlocked(rateLimit *RateLimit, blockTime int) bool {
	if rateLimit.BlockedAt.IsZero() {
		return false
	}
	return time.Since(rateLimit.BlockedAt).Seconds() < float64(blockTime)
}

func (s *Service) shouldResetWindow(rateLimit *RateLimit) bool {
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
