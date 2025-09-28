package main

import (
	"fmt"
	"log"
	"net/http"
	"rate-limiter/middleware"
	"rate-limiter/rest"
)

func main() {
	rateLimiterService := middleware.NewService()

	config, err := middleware.LoadConfig()
	if err != nil {
		log.Printf("Warning: Failed to load configuration, using defaults: %v", err)
		config = middleware.Config{ServerPort: "8080"}
	}

	r := rest.SetupRouter(rateLimiterService)
	port := rest.GetServerPort(config)

	fmt.Printf("Server starting on port %s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
