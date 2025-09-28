package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

type ErrorResponse struct {
	Error string `json:"error"`
}

func RateLimiter(service *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := getClientIP(r)
			apiKey := getAPIKey(r)
			key, isToken := determineRateLimitKey(clientIP, apiKey)

			allowed, err := service.CheckRateLimit(key, isToken)
			if err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if !allowed {
				sendRateLimitError(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			if isValidIP(clientIP) {
				return clientIP
			}
		}
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		if isValidIP(xri) {
			return xri
		}
	}

	if cfIP := r.Header.Get("CF-Connecting-IP"); cfIP != "" {
		if isValidIP(cfIP) {
			return cfIP
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func getAPIKey(r *http.Request) string {
	return r.Header.Get("API_KEY")
}

func determineRateLimitKey(clientIP, apiKey string) (string, bool) {
	if apiKey != "" {
		return "token:" + apiKey, true
	}
	return clientIP, false
}

func sendRateLimitError(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)

	response := ErrorResponse{
		Error: "you have reached the maximum number of requests or actions allowed within a certain time frame",
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		return
	}
}

func isValidIP(ip string) bool {
	return net.ParseIP(ip) != nil
}
