package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"fc-pos-golang-rate-limiter/internal/config"
	"fc-pos-golang-rate-limiter/internal/limiter"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
)

type MockStorageStrategy struct {
	allowResults map[string]bool
	allowCounts  map[string]int
	allowErrors  map[string]error
	callCounts   map[string]int
}

func NewMockStorageStrategy() *MockStorageStrategy {
	return &MockStorageStrategy{
		allowResults: make(map[string]bool),
		allowCounts:  make(map[string]int),
		allowErrors:  make(map[string]error),
		callCounts:   make(map[string]int),
	}
}

func (m *MockStorageStrategy) Allow(ctx context.Context, key string, limit int, window time.Duration, blockDuration time.Duration) (bool, int, time.Time, error) {
	m.callCounts[key]++

	if err, exists := m.allowErrors[key]; exists {
		return false, 0, time.Time{}, err
	}

	allowed, exists := m.allowResults[key]
	if !exists {
		allowed = true
	}

	remaining := limit - m.allowCounts[key]
	if remaining < 0 {
		remaining = 0
	}

	return allowed, remaining, time.Now().Add(window), nil
}

func (m *MockStorageStrategy) Reset(ctx context.Context, key string) error {
	delete(m.allowResults, key)
	delete(m.allowCounts, key)
	delete(m.allowErrors, key)
	delete(m.callCounts, key)
	return nil
}

func (m *MockStorageStrategy) Close() error {
	return nil
}

func (m *MockStorageStrategy) SetAllowResult(key string, allowed bool, count int) {
	m.allowResults[key] = allowed
	m.allowCounts[key] = count
}

func (m *MockStorageStrategy) SetAllowError(key string, err error) {
	m.allowErrors[key] = err
}

func (m *MockStorageStrategy) GetCallCount(key string) int {
	return m.callCounts[key]
}

func TestRateLimitMiddleware(t *testing.T) {
	mockStorage := NewMockStorageStrategy()
	ipConfig := &config.RateLimitConfig{
		IPLimit:              10,
		WindowSeconds:        1,
		BlockDurationSeconds: 300,
	}

	tokenConfigs := config.TokenConfigs{
		"test_token": config.TokenConfig{
			Limit:                100,
			WindowSeconds:        1,
			BlockDurationSeconds: 300,
		},
	}

	rateLimiter := limiter.NewRateLimiter(mockStorage, ipConfig, tokenConfigs)

	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	router := chi.NewRouter()
	router.Use(RateLimitMiddleware(rateLimiter))
	router.Get("/test", testHandler)

	t.Run("Request allowed - IP", func(t *testing.T) {
		mockStorage.SetAllowResult("ip:192.168.1.1", true, 5)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:12345"

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())
		assert.Equal(t, "10", rr.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "5", rr.Header().Get("X-RateLimit-Remaining"))
	})

	t.Run("Request blocked - IP", func(t *testing.T) {
		mockStorage.SetAllowResult("ip:192.168.1.2", false, 10)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.2:12345"

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
		assert.Contains(t, rr.Body.String(), "you have reached the maximum number of requests")
		assert.Equal(t, "10", rr.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "0", rr.Header().Get("X-RateLimit-Remaining"))
	})

	t.Run("Request allowed - Token", func(t *testing.T) {
		mockStorage.SetAllowResult("token:test_token", true, 50)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.3:12345"
		req.Header.Set("API_KEY", "test_token")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "OK", rr.Body.String())
		assert.Equal(t, "100", rr.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "50", rr.Header().Get("X-RateLimit-Remaining"))
	})

	t.Run("Request blocked - Token", func(t *testing.T) {
		mockStorage.SetAllowResult("token:test_token", false, 100)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.4:12345"
		req.Header.Set("API_KEY", "test_token")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusTooManyRequests, rr.Code)
		assert.Contains(t, rr.Body.String(), "you have reached the maximum number of requests")
		assert.Equal(t, "100", rr.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "0", rr.Header().Get("X-RateLimit-Remaining"))
	})

	t.Run("Token takes priority over IP", func(t *testing.T) {
		// Set both IP and token results
		mockStorage.SetAllowResult("ip:192.168.1.5", true, 5)
		mockStorage.SetAllowResult("token:test_token", true, 50)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.5:12345"
		req.Header.Set("API_KEY", "test_token")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "100", rr.Header().Get("X-RateLimit-Limit")) // Token limit, not IP limit
		assert.Equal(t, "50", rr.Header().Get("X-RateLimit-Remaining"))
	})

	t.Run("X-Forwarded-For header", func(t *testing.T) {
		mockStorage.SetAllowResult("ip:203.0.113.1", true, 3)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.6:12345"
		req.Header.Set("X-Forwarded-For", "203.0.113.1")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "10", rr.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "7", rr.Header().Get("X-RateLimit-Remaining"))
	})

	t.Run("X-Real-IP header", func(t *testing.T) {
		mockStorage.SetAllowResult("ip:198.51.100.1", true, 2)

		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.7:12345"
		req.Header.Set("X-Real-IP", "198.51.100.1")

		rr := httptest.NewRecorder()
		router.ServeHTTP(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "10", rr.Header().Get("X-RateLimit-Limit"))
		assert.Equal(t, "8", rr.Header().Get("X-RateLimit-Remaining"))
	})
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		expectedIP string
	}{
		{
			name:       "RemoteAddr only",
			remoteAddr: "192.168.1.1:12345",
			expectedIP: "192.168.1.1",
		},
		{
			name:       "X-Forwarded-For single IP",
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.1"},
			expectedIP: "203.0.113.1",
		},
		{
			name:       "X-Forwarded-For multiple IPs",
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{"X-Forwarded-For": "203.0.113.1, 198.51.100.1"},
			expectedIP: "203.0.113.1",
		},
		{
			name:       "X-Real-IP",
			remoteAddr: "192.168.1.1:12345",
			headers:    map[string]string{"X-Real-IP": "198.51.100.1"},
			expectedIP: "198.51.100.1",
		},
		{
			name:       "X-Forwarded-For takes priority over X-Real-IP",
			remoteAddr: "192.168.1.1:12345",
			headers: map[string]string{
				"X-Forwarded-For": "203.0.113.1",
				"X-Real-IP":       "198.51.100.1",
			},
			expectedIP: "203.0.113.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			for header, value := range tt.headers {
				req.Header.Set(header, value)
			}

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				rateLimitInfo := GetRateLimitInfo(r.Context())
				if rateLimitInfo != nil {
					assert.Equal(t, tt.expectedIP, rateLimitInfo.Identifier)
				}
			})

			router := chi.NewRouter()
			router.Use(RateLimitMiddleware(limiter.NewRateLimiter(NewMockStorageStrategy(), &config.RateLimitConfig{}, nil)))
			router.Get("/test", handler)

			rr := httptest.NewRecorder()
			router.ServeHTTP(rr, req)
		})
	}
}

