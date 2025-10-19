package integration

import (
	"context"
	"testing"
	"time"

	"fc-pos-golang-rate-limiter/internal/config"
	"fc-pos-golang-rate-limiter/internal/limiter"

	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRedisStrategyIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}

	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	defer func() { _ = redisContainer.Terminate(ctx) }()

	host, err := redisContainer.Host(ctx)
	require.NoError(t, err)

	port, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	redisClient := redis.NewClient(&redis.Options{
		Addr: host + ":" + port.Port(),
		DB:   0,
	})

	err = redisClient.Ping(ctx).Err()
	require.NoError(t, err)

	strategy := limiter.NewRedisStrategy(redisClient)

	t.Run("Allow requests within limit", func(t *testing.T) {
		key := "test:ip:192.168.1.1"
		limit := 5
		window := 1 * time.Second

		// Faz requisições dentro do limite
		blockDuration := 5 * time.Minute
		for i := 0; i < limit; i++ {
			allowed, remaining, resetTime, err := strategy.Allow(ctx, key, limit, window, blockDuration)
			require.NoError(t, err)
			assert.True(t, allowed)
			assert.Equal(t, limit-i-1, remaining)
			assert.True(t, resetTime.After(time.Now()))
		}
	})

	t.Run("Block requests exceeding limit", func(t *testing.T) {
		key := "test:ip:192.168.1.2"
		limit := 3
		window := 1 * time.Second

		// Faz requisições dentro do limite
		for i := 0; i < limit; i++ {
			allowed, remaining, _, err := strategy.Allow(ctx, key, limit, window, 5*time.Minute)
			require.NoError(t, err)
			assert.True(t, allowed)
			assert.Equal(t, limit-i-1, remaining)
		}

		// Esta requisição deve ser bloqueada
		allowed, remaining, resetTime, err := strategy.Allow(ctx, key, limit, window, 5*time.Minute)
		require.NoError(t, err)
		assert.False(t, allowed)
		assert.Equal(t, 0, remaining)
		assert.True(t, resetTime.After(time.Now()))
	})

	t.Run("Reset removes all entries", func(t *testing.T) {
		key := "test:ip:192.168.1.3"
		limit := 2
		window := 1 * time.Second

		// Faz algumas requisições
		for i := 0; i < limit; i++ {
			allowed, _, _, err := strategy.Allow(ctx, key, limit, window, 5*time.Minute)
			require.NoError(t, err)
			assert.True(t, allowed)
		}

		// Reseta
		err := strategy.Reset(ctx, key)
		require.NoError(t, err)

		// Deve ser capaz de fazer requisições novamente
		allowed, remaining, _, err := strategy.Allow(ctx, key, limit, window, 5*time.Minute)
		require.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, limit-1, remaining)
	})

	t.Run("Sliding window behavior", func(t *testing.T) {
		key := "test:ip:192.168.1.4"
		limit := 3
		window := 2 * time.Second
		blockDuration := 1 * time.Second // BlockDuration menor para o teste

		// Faz requisições para preencher a janela
		for i := 0; i < limit; i++ {
			allowed, _, _, err := strategy.Allow(ctx, key, limit, window, blockDuration)
			require.NoError(t, err)
			assert.True(t, allowed)
		}

		// Deve ser bloqueada
		allowed, _, _, err := strategy.Allow(ctx, key, limit, window, blockDuration)
		require.NoError(t, err)
		assert.False(t, allowed)

		// Aguarda o BlockDuration expirar
		time.Sleep(blockDuration + 100*time.Millisecond)

		// Aguarda a janela deslizar (as entradas antigas devem expirar)
		time.Sleep(window + 100*time.Millisecond)

		// Deve ser permitida novamente
		allowed, remaining, _, err := strategy.Allow(ctx, key, limit, window, blockDuration)
		require.NoError(t, err)
		assert.True(t, allowed)
		assert.Equal(t, limit-1, remaining)
	})

	// Limpa
	err = strategy.Close()
	require.NoError(t, err)
}

func TestRateLimiterIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx := context.Background()
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}

	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	defer func() { _ = redisContainer.Terminate(ctx) }()

	host, err := redisContainer.Host(ctx)
	require.NoError(t, err)

	port, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	redisClient := redis.NewClient(&redis.Options{
		Addr: host + ":" + port.Port(),
		DB:   0,
	})

	err = redisClient.Ping(ctx).Err()
	require.NoError(t, err)

	ipConfig := &config.RateLimitConfig{
		IPLimit:              5,
		WindowSeconds:        1,
		BlockDurationSeconds: 300,
	}

	tokenConfigs := config.TokenConfigs{
		"test_token": config.TokenConfig{
			Limit:                10,
			WindowSeconds:        1,
			BlockDurationSeconds: 300,
		},
	}

	storageStrategy := limiter.NewRedisStrategy(redisClient)
	rateLimiter := limiter.NewRateLimiter(storageStrategy, ipConfig, tokenConfigs)

	t.Run("IP rate limiting", func(t *testing.T) {
		ip := "192.168.1.100"

		// Faz requisições dentro do limite de IP
		for i := 0; i < ipConfig.IPLimit; i++ {
			result, err := rateLimiter.Check(ctx, ip, false)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, ip, result.Identifier)
			assert.False(t, result.IsToken)
			assert.Equal(t, ipConfig.IPLimit, result.Limit)
		}

		// Esta requisição deve ser bloqueada
		result, err := rateLimiter.Check(ctx, ip, false)
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, 0, result.Remaining)
	})

	t.Run("Token rate limiting", func(t *testing.T) {
		token := "test_token"

		// Faz requisições dentro do limite de token
		for i := 0; i < tokenConfigs[token].Limit; i++ {
			result, err := rateLimiter.Check(ctx, token, true)
			require.NoError(t, err)
			assert.True(t, result.Allowed)
			assert.Equal(t, token, result.Identifier)
			assert.True(t, result.IsToken)
			assert.Equal(t, tokenConfigs[token].Limit, result.Limit)
		}

		// Esta requisição deve ser bloqueada
		result, err := rateLimiter.Check(ctx, token, true)
		require.NoError(t, err)
		assert.False(t, result.Allowed)
		assert.Equal(t, 0, result.Remaining)
	})

	t.Run("Token takes priority over IP", func(t *testing.T) {
		ip := "192.168.1.200"
		token := "test_token"

		// Reseta o token para garantir que não está bloqueado
		err := rateLimiter.Reset(ctx, token, true)
		require.NoError(t, err)

		// Primeiro verifica com IP apenas
		result, err := rateLimiter.Check(ctx, ip, false)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, ipConfig.IPLimit, result.Limit)

		// Depois verifica com token (deve usar o limite de token)
		result, err = rateLimiter.Check(ctx, token, true)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, tokenConfigs[token].Limit, result.Limit)
	})

	t.Run("Unknown token falls back to IP limit", func(t *testing.T) {
		unknownToken := "unknown_token"

		result, err := rateLimiter.Check(ctx, unknownToken, true)
		require.NoError(t, err)
		assert.True(t, result.Allowed)
		assert.Equal(t, unknownToken, result.Identifier)
		assert.True(t, result.IsToken)
		assert.Equal(t, ipConfig.IPLimit, result.Limit) // Volta para o limite de IP
	})

	// Limpa
	err = storageStrategy.Close()
	require.NoError(t, err)
}
