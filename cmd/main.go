package main

import (
	"fmt"
	"log"
	"net/http"

	"rate-limiter/middleware"
	"rate-limiter/rest"
	"rate-limiter/storage"
)

func main() {
	appConfig, err := storage.LoadConfig()
	if err != nil {
		log.Printf("Warning: Failed to load configuration, using defaults: %v", err)
		appConfig = storage.GetDefaultConfig()
	}

	redisStorage, err := storage.NewRedisStorage(appConfig.Storage)
	if err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	rateLimiterService := middleware.NewService(appConfig.RateLimit, redisStorage)

	r := rest.SetupRouter(rateLimiterService)
	port := rest.GetServerPort(appConfig.RateLimit)

	fmt.Printf("Server starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
