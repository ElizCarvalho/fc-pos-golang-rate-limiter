package limiter

import (
	"context"
	"testing"
	"time"

	"fc-pos-golang-rate-limiter/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestRateLimiterCheck(t *testing.T) {
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
		"premium_token": config.TokenConfig{
			Limit:                1000,
			WindowSeconds:        1,
			BlockDurationSeconds: 60,
		},
	}

	rateLimiter := NewRateLimiter(mockStorage, ipConfig, tokenConfigs)
	ctx := context.Background()

	t.Run("IP limit - allowed", func(t *testing.T) {
		mockStorage.SetAllowResult("ip:192.168.1.1", true, 5)

		result, err := rateLimiter.Check(ctx, "192.168.1.1", false)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.True(t, result.Allowed)
		assert.Equal(t, "192.168.1.1", result.Identifier)
		assert.False(t, result.IsToken)
		assert.Equal(t, 10, result.Limit)
		assert.Equal(t, 5, result.Remaining)
	})

	t.Run("IP limit - blocked", func(t *testing.T) {
		mockStorage.SetAllowResult("ip:192.168.1.2", false, 10)

		result, err := rateLimiter.Check(ctx, "192.168.1.2", false)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.False(t, result.Allowed)
		assert.Equal(t, "192.168.1.2", result.Identifier)
		assert.False(t, result.IsToken)
		assert.Equal(t, 10, result.Limit)
		assert.Equal(t, 0, result.Remaining)
	})

	t.Run("Token limit - allowed", func(t *testing.T) {
		mockStorage.SetAllowResult("token:test_token", true, 50)

		result, err := rateLimiter.Check(ctx, "test_token", true)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.True(t, result.Allowed)
		assert.Equal(t, "test_token", result.Identifier)
		assert.True(t, result.IsToken)
		assert.Equal(t, 100, result.Limit)
		assert.Equal(t, 50, result.Remaining)
	})

	t.Run("Token limit - blocked", func(t *testing.T) {
		mockStorage.SetAllowResult("token:premium_token", false, 1000)

		result, err := rateLimiter.Check(ctx, "premium_token", true)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.False(t, result.Allowed)
		assert.Equal(t, "premium_token", result.Identifier)
		assert.True(t, result.IsToken)
		assert.Equal(t, 1000, result.Limit)
		assert.Equal(t, 0, result.Remaining)
	})

	t.Run("Unknown token falls back to IP limit", func(t *testing.T) {
		mockStorage.SetAllowResult("token:unknown_token", true, 3)

		result, err := rateLimiter.Check(ctx, "unknown_token", true)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.True(t, result.Allowed)
		assert.Equal(t, "unknown_token", result.Identifier)
		assert.True(t, result.IsToken)
		assert.Equal(t, 10, result.Limit) // Falls back to IP limit
		assert.Equal(t, 7, result.Remaining)
	})

	t.Run("Storage error", func(t *testing.T) {
		mockStorage.SetAllowError("ip:192.168.1.3", assert.AnError)

		result, err := rateLimiter.Check(ctx, "192.168.1.3", false)
		require.Error(t, err)
		assert.Nil(t, result)
	})
}

func TestRateLimiterReset(t *testing.T) {
	mockStorage := NewMockStorageStrategy()
	ipConfig := &config.RateLimitConfig{
		IPLimit:              10,
		WindowSeconds:        1,
		BlockDurationSeconds: 300,
	}

	rateLimiter := NewRateLimiter(mockStorage, ipConfig, nil)
	ctx := context.Background()

	mockStorage.SetAllowResult("ip:192.168.1.1", true, 5)

	err := rateLimiter.Reset(ctx, "192.168.1.1", false)
	require.NoError(t, err)

	assert.Equal(t, 0, len(mockStorage.allowResults))
}

func TestRateLimiterCreateKey(t *testing.T) {
	mockStorage := NewMockStorageStrategy()
	ipConfig := &config.RateLimitConfig{
		IPLimit:              10,
		WindowSeconds:        1,
		BlockDurationSeconds: 300,
	}

	rateLimiter := NewRateLimiter(mockStorage, ipConfig, nil)
	ctx := context.Background()

	_, err := rateLimiter.Check(ctx, "192.168.1.1", false)
	require.NoError(t, err)
	assert.Equal(t, 1, mockStorage.GetCallCount("ip:192.168.1.1"))

	_, err = rateLimiter.Check(ctx, "test_token", true)
	require.NoError(t, err)
	assert.Equal(t, 1, mockStorage.GetCallCount("token:test_token"))
}

