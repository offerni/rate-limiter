package rest

import (
	"fmt"
	"net/http"
	"rate-limiter/middleware"
	"rate-limiter/storage"
	"time"

	"github.com/go-chi/chi/v5"
)

func SetupRouter(rateLimiterService *middleware.Service) *chi.Mux {
	r := chi.NewRouter()
	r.Use(middleware.RateLimiter(rateLimiterService))
	r.Use(logRequest)
	SetupRoutes(r)
	return r
}

func SetupRoutes(r *chi.Mux) {
	r.Get("/", homeHandler)
	r.Get("/health", healthHandler)
	r.Get("/api/test", apiTestHandler)
	r.Get("/api/load-test", loadTestHandler)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"message": "Rate limiter is working", "path": "/", "timestamp": "`+time.Now().Format(time.RFC3339)+`"}`)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, `{"status": "healthy", "service": "rate-limiter", "timestamp": "`+time.Now().Format(time.RFC3339)+`"}`)
}

func apiTestHandler(w http.ResponseWriter, r *http.Request) {
	apiKey := r.Header.Get("API_KEY")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if apiKey != "" {
		fmt.Fprintf(w, `{"message": "API endpoint accessed", "api_key": "%s", "timestamp": "%s"}`, apiKey, time.Now().Format(time.RFC3339))
	} else {
		fmt.Fprintf(w, `{"message": "API endpoint accessed", "api_key": null, "timestamp": "%s"}`, time.Now().Format(time.RFC3339))
	}
}

func loadTestHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"message": "Load test endpoint", "timestamp": "%s", "client_ip": "%s"}`, time.Now().Format(time.RFC3339), r.RemoteAddr)
}

func GetServerPort(config storage.Config) string {
	if config.ServerPort != "" {
		return config.ServerPort
	}
	return "8080"
}

func logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		fmt.Printf("[%s] %s %s from %s", start.Format("15:04:05"), r.Method, r.URL.Path, r.RemoteAddr)

		if apiKey := r.Header.Get("API_KEY"); apiKey != "" {
			fmt.Printf(" (API_KEY: %s)", apiKey)
		}

		next.ServeHTTP(w, r)
		fmt.Printf(" - %v\n", time.Since(start))
	})
}
