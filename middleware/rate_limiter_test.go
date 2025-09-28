package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		expectedIP string
	}{
		{
			name: "xff_single",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name: "xff_multiple",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100, 10.0.0.1, 172.16.0.1",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name: "real_ip",
			headers: map[string]string{
				"X-Real-IP": "203.0.113.1",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "203.0.113.1",
		},
		{
			name: "cloudflare",
			headers: map[string]string{
				"CF-Connecting-IP": "198.51.100.1",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "198.51.100.1",
		},
		{
			name: "priority_xff",
			headers: map[string]string{
				"X-Forwarded-For": "192.168.1.100",
				"X-Real-IP":       "203.0.113.1",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "192.168.1.100",
		},
		{
			name:       "fallback_remote",
			headers:    map[string]string{},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "10.0.0.1",
		},
		{
			name: "invalid_fallback",
			headers: map[string]string{
				"X-Forwarded-For": "invalid-ip",
				"X-Real-IP":       "203.0.113.1",
			},
			remoteAddr: "10.0.0.1:12345",
			expectedIP: "203.0.113.1",
		},
		{
			name:       "no_port",
			headers:    map[string]string{},
			remoteAddr: "10.0.0.1",
			expectedIP: "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = tt.remoteAddr

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			result := getClientIP(req)
			assert.Equal(t, tt.expectedIP, result)
		})
	}
}

func TestIsValidIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"ipv4", "192.168.1.1", true},
		{"ipv4_loopback", "127.0.0.1", true},
		{"ipv4_public", "8.8.8.8", true},
		{"ipv6", "2001:db8::1", true},
		{"ipv6_loopback", "::1", true},
		{"invalid_letters", "192.168.1.abc", false},
		{"invalid_octets", "192.168.1.1.1", false},
		{"empty", "", false},
		{"text", "not-an-ip", false},
		{"incomplete", "192.168", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidIP(tt.ip)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAPIKey(t *testing.T) {
	tests := []struct {
		name        string
		headerValue string
		expected    string
	}{
		{"Valid API key", "ABC123", "ABC123"},
		{"Empty API key", "", ""},
		{"Complex API key", "sk-1234567890abcdef", "sk-1234567890abcdef"},
		{"API key with special chars", "key_with-special.chars", "key_with-special.chars"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.headerValue != "" {
				req.Header.Set("API_KEY", tt.headerValue)
			}

			result := getAPIKey(req)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetermineRateLimitKey(t *testing.T) {
	tests := []struct {
		name          string
		clientIP      string
		apiKey        string
		expectedKey   string
		expectedToken bool
	}{
		{
			name:          "with_token",
			clientIP:      "192.168.1.1",
			apiKey:        "ABC123",
			expectedKey:   "token:ABC123",
			expectedToken: true,
		},
		{
			name:          "no_token",
			clientIP:      "192.168.1.1",
			apiKey:        "",
			expectedKey:   "192.168.1.1",
			expectedToken: false,
		},
		{
			name:          "empty_token",
			clientIP:      "10.0.0.1",
			apiKey:        "",
			expectedKey:   "10.0.0.1",
			expectedToken: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			key, isToken := determineRateLimitKey(tt.clientIP, tt.apiKey)
			assert.Equal(t, tt.expectedKey, key)
			assert.Equal(t, tt.expectedToken, isToken)
		})
	}
}

func TestSendRateLimitError(t *testing.T) {
	w := httptest.NewRecorder()
	sendRateLimitError(w)

	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	expected := "you have reached the maximum number of requests or actions allowed within a certain time frame"
	assert.Equal(t, expected, response.Error)
}

func TestRateLimiterMiddleware(t *testing.T) {
	service := &Service{
		config: Config{
			IPRateLimit:     2,
			IPBlockTime:     1,
			TokenLimits:     map[string]int{"ABC123": 5},
			TokenBlockTimes: map[string]int{"ABC123": 1},
		},
		rateLimits: make(map[string]*RateLimit),
	}

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})

	middleware := RateLimiter(service)
	handler := middleware(testHandler)

	t.Run("within_limit", func(t *testing.T) {
		service.rateLimits = make(map[string]*RateLimit)

		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.1:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "success", w.Body.String())

		req2 := httptest.NewRequest("GET", "/", nil)
		req2.RemoteAddr = "192.168.1.1:12345"
		w2 := httptest.NewRecorder()

		handler.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
		assert.Equal(t, "success", w2.Body.String())
	})

	t.Run("exceeds_limit", func(t *testing.T) {
		service.rateLimits = make(map[string]*RateLimit)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "192.168.1.2:12345"
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.2:12345"
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)
		assert.Contains(t, response.Error, "maximum number of requests")
	})

	t.Run("token_higher_limit", func(t *testing.T) {
		service.rateLimits = make(map[string]*RateLimit)

		for i := 0; i < 4; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "192.168.1.3:12345"
			req.Header.Set("API_KEY", "ABC123")
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}
	})

	t.Run("separate_ips", func(t *testing.T) {
		service.rateLimits = make(map[string]*RateLimit)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "192.168.1.4:12345"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		req1 := httptest.NewRequest("GET", "/", nil)
		req1.RemoteAddr = "192.168.1.4:12345"
		w1 := httptest.NewRecorder()
		handler.ServeHTTP(w1, req1)
		assert.Equal(t, http.StatusTooManyRequests, w1.Code)

		req2 := httptest.NewRequest("GET", "/", nil)
		req2.RemoteAddr = "192.168.1.5:12345"
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})

	t.Run("window_reset", func(t *testing.T) {
		service.rateLimits = make(map[string]*RateLimit)

		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "192.168.1.6:12345"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		}

		req := httptest.NewRequest("GET", "/", nil)
		req.RemoteAddr = "192.168.1.6:12345"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equal(t, http.StatusTooManyRequests, w.Code)

		time.Sleep(1100 * time.Millisecond)

		req2 := httptest.NewRequest("GET", "/", nil)
		req2.RemoteAddr = "192.168.1.6:12345"
		w2 := httptest.NewRecorder()
		handler.ServeHTTP(w2, req2)
		assert.Equal(t, http.StatusOK, w2.Code)
	})
}

func TestRateLimiterIntegrationWithChi(t *testing.T) {
	// Create a service
	service := &Service{
		config: Config{
			IPRateLimit: 1,
			IPBlockTime: 1,
		},
		rateLimits: make(map[string]*RateLimit),
	}

	// Create Chi router with middleware
	r := chi.NewRouter()
	r.Use(RateLimiter(service))
	r.Get("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("test response"))
	})

	// Test the integration
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "test response", w.Body.String())

	// Second request should be blocked
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "192.168.1.100:12345"
	w2 := httptest.NewRecorder()

	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusTooManyRequests, w2.Code)
}

func BenchmarkRateLimiterMiddleware(b *testing.B) {
	service := &Service{
		config: Config{
			IPRateLimit: 1000000, // Very high limit to avoid blocking
			IPBlockTime: 300,
		},
		rateLimits: make(map[string]*RateLimit),
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RateLimiter(service)
	wrappedHandler := middleware(handler)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req := httptest.NewRequest("GET", "/", nil)
			req.RemoteAddr = "192.168.1.1:12345"
			w := httptest.NewRecorder()
			wrappedHandler.ServeHTTP(w, req)
		}
	})
}

func BenchmarkGetClientIP(b *testing.B) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.1, 192.168.1.1")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getClientIP(req)
	}
}
