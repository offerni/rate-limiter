package storage

import (
	"os"
	ratelimiter "rate-limiter"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	IPRateLimit     int
	IPBlockTime     int
	TokenLimits     map[string]int
	TokenBlockTimes map[string]int
	ServerPort      string
}

type AppConfig struct {
	RateLimit Config
	Storage   ratelimiter.StorageConfig
}

func LoadConfig() (AppConfig, error) {
	err := godotenv.Load()
	if err != nil {
	}

	appConfig := AppConfig{
		RateLimit: Config{
			TokenLimits:     make(map[string]int),
			TokenBlockTimes: make(map[string]int),
		},
		Storage: ratelimiter.StorageConfig{},
	}

	if val := os.Getenv("IP_RATE_LIMIT"); val != "" {
		if limit, err := strconv.Atoi(val); err == nil {
			appConfig.RateLimit.IPRateLimit = limit
		} else {
			appConfig.RateLimit.IPRateLimit = 10
		}
	} else {
		appConfig.RateLimit.IPRateLimit = 10
	}

	if val := os.Getenv("IP_BLOCK_TIME"); val != "" {
		if blockTime, err := strconv.Atoi(val); err == nil {
			appConfig.RateLimit.IPBlockTime = blockTime
		} else {
			appConfig.RateLimit.IPBlockTime = 300
		}
	} else {
		appConfig.RateLimit.IPBlockTime = 300
	}

	appConfig.RateLimit.ServerPort = os.Getenv("SERVER_PORT")
	if appConfig.RateLimit.ServerPort == "" {
		appConfig.RateLimit.ServerPort = "8080"
	}

	appConfig.Storage = ratelimiter.StorageConfig{
		Host:     getEnvOrDefault("REDIS_HOST", "localhost"),
		Port:     getEnvOrDefault("REDIS_PORT", "6379"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       0,
	}

	if redisDB := os.Getenv("REDIS_DB"); redisDB != "" {
		if db, err := strconv.Atoi(redisDB); err == nil {
			appConfig.Storage.DB = db
		}
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
				appConfig.RateLimit.TokenLimits[tokenName] = limit
			}
		}

		if strings.HasPrefix(key, "TOKEN_") && strings.HasSuffix(key, "_BLOCK_TIME") {
			tokenName := strings.TrimPrefix(key, "TOKEN_")
			tokenName = strings.TrimSuffix(tokenName, "_BLOCK_TIME")

			if blockTime, err := strconv.Atoi(value); err == nil {
				appConfig.RateLimit.TokenBlockTimes[tokenName] = blockTime
			}
		}
	}

	return appConfig, nil
}

func GetDefaultConfig() AppConfig {
	return AppConfig{
		RateLimit: Config{
			IPRateLimit:     10,
			IPBlockTime:     300,
			TokenLimits:     make(map[string]int),
			TokenBlockTimes: make(map[string]int),
			ServerPort:      "8080",
		},
		Storage: ratelimiter.StorageConfig{
			Host:     "localhost",
			Port:     "6379",
			Password: "",
			DB:       0,
		},
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
