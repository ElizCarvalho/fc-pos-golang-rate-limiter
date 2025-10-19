package load

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"fc-pos-golang-rate-limiter/internal/config"
	"fc-pos-golang-rate-limiter/internal/handler"
	"fc-pos-golang-rate-limiter/internal/limiter"
	"fc-pos-golang-rate-limiter/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

func TestLoadIPRateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
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
	storageStrategy := limiter.NewRedisStrategy(redisClient)
	ipConfig := &config.RateLimitConfig{
		IPLimit:              10,
		WindowSeconds:        1,
		BlockDurationSeconds: 300,
	}

	rateLimiter := limiter.NewRateLimiter(storageStrategy, ipConfig, nil)
	healthHandler := handler.NewHealthHandler()

	router := chi.NewRouter()
	router.Use(middleware.RateLimitMiddleware(rateLimiter))
	router.Get("/api/v1/resource", healthHandler.Resource)

	server := &http.Server{
		Addr:    ":8081",
		Handler: router,
	}

	go func() {
		_ = server.ListenAndServe()
	}()
	defer func() { _ = server.Shutdown(ctx) }()

	time.Sleep(100 * time.Millisecond)

	rate := vegeta.Rate{Freq: 15, Per: time.Second} // 15 requisições por segundo
	duration := 3 * time.Second
	targeter := vegeta.NewStaticTargeter(vegeta.Target{
		Method: "GET",
		URL:    "http://localhost:8081/api/v1/resource",
		Header: http.Header{
			"X-Forwarded-For": []string{"192.168.1.100"},
		},
	})

	attacker := vegeta.NewAttacker()
	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Load Test") {
		metrics.Add(res)
	}
	metrics.Close()

	t.Logf("Load test results:")
	t.Logf("  Total requests: %d", metrics.Requests)
	t.Logf("  Successful requests: %d", int(metrics.Requests)-metrics.StatusCodes["429"])
	t.Logf("  Rate limited requests: %d", metrics.StatusCodes["429"])
	t.Logf("  Success rate: %.2f%%", (1-float64(metrics.StatusCodes["429"])/float64(metrics.Requests))*100)
	t.Logf("  Average latency: %v", metrics.Latencies.Mean)

	assert.True(t, metrics.StatusCodes["429"] > 0, "Expected some requests to be rate limited")
	assert.True(t, metrics.StatusCodes["200"] > 0, "Expected some requests to succeed")

	_ = storageStrategy.Close()
}

func TestLoadTokenRateLimit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
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
	storageStrategy := limiter.NewRedisStrategy(redisClient)
	ipConfig := &config.RateLimitConfig{
		IPLimit:              5,
		WindowSeconds:        1,
		BlockDurationSeconds: 300,
	}

	tokenConfigs := config.TokenConfigs{
		"load_test_token": config.TokenConfig{
			Limit:                20,
			WindowSeconds:        1,
			BlockDurationSeconds: 300,
		},
	}

	rateLimiter := limiter.NewRateLimiter(storageStrategy, ipConfig, tokenConfigs)
	healthHandler := handler.NewHealthHandler()

	router := chi.NewRouter()
	router.Use(middleware.RateLimitMiddleware(rateLimiter))
	router.Get("/api/v1/resource", healthHandler.Resource)

	server := &http.Server{
		Addr:    ":8082",
		Handler: router,
	}

	go func() {
		_ = server.ListenAndServe()
	}()
	defer func() { _ = server.Shutdown(ctx) }()

	time.Sleep(100 * time.Millisecond)

	rate := vegeta.Rate{Freq: 25, Per: time.Second} // 25 requisições por segundo
	duration := 3 * time.Second
	targeter := vegeta.NewStaticTargeter(vegeta.Target{
		Method: "GET",
		URL:    "http://localhost:8082/api/v1/resource",
		Header: http.Header{
			"API_KEY": []string{"load_test_token"},
		},
	})

	attacker := vegeta.NewAttacker()
	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Token Load Test") {
		metrics.Add(res)
	}
	metrics.Close()

	t.Logf("Token load test results:")
	t.Logf("  Total requests: %d", metrics.Requests)
	t.Logf("  Successful requests: %d", int(metrics.Requests)-metrics.StatusCodes["429"])
	t.Logf("  Rate limited requests: %d", metrics.StatusCodes["429"])
	t.Logf("  Success rate: %.2f%%", (1-float64(metrics.StatusCodes["429"])/float64(metrics.Requests))*100)
	t.Logf("  Average latency: %v", metrics.Latencies.Mean)

	assert.True(t, metrics.StatusCodes["429"] > 0, "Expected some requests to be rate limited")
	assert.True(t, metrics.StatusCodes["200"] > 0, "Expected some requests to succeed")

	_ = storageStrategy.Close()
}

func TestLoadConcurrentUsers(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
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
	storageStrategy := limiter.NewRedisStrategy(redisClient)
	ipConfig := &config.RateLimitConfig{
		IPLimit:              5,
		WindowSeconds:        1,
		BlockDurationSeconds: 300,
	}

	rateLimiter := limiter.NewRateLimiter(storageStrategy, ipConfig, nil)
	healthHandler := handler.NewHealthHandler()

	router := chi.NewRouter()
	router.Use(middleware.RateLimitMiddleware(rateLimiter))
	router.Get("/api/v1/resource", healthHandler.Resource)

	server := &http.Server{
		Addr:    ":8083",
		Handler: router,
	}

	go func() {
		_ = server.ListenAndServe()
	}()
	defer func() { _ = server.Shutdown(ctx) }()

	time.Sleep(100 * time.Millisecond)

	users := []string{"192.168.1.100", "192.168.1.101", "192.168.1.102", "192.168.1.103", "192.168.1.104"}

	for i, userIP := range users {
		rate := vegeta.Rate{Freq: 8, Per: time.Second} // 8 requisições por segundo por usuário
		duration := 2 * time.Second
		targeter := vegeta.NewStaticTargeter(vegeta.Target{
			Method: "GET",
			URL:    fmt.Sprintf("http://localhost:8083/api/v1/resource?user=%d", i),
			Header: http.Header{
				"X-Forwarded-For": []string{userIP},
			},
		})

		attacker := vegeta.NewAttacker()
		var metrics vegeta.Metrics
		for res := range attacker.Attack(targeter, rate, duration, fmt.Sprintf("User %s Load Test", userIP)) {
			metrics.Add(res)
		}
		metrics.Close()

		t.Logf("Load test results for user %s:", userIP)
		t.Logf("  Total requests: %d", metrics.Requests)
		t.Logf("  Successful requests: %d", int(metrics.Requests)-metrics.StatusCodes["429"])
		t.Logf("  Rate limited requests: %d", metrics.StatusCodes["429"])
	}

	_ = storageStrategy.Close()
}

// Testa ocomportamento sob tráfego intenso em burst
func TestLoadHighTrafficBurst(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
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
	storageStrategy := limiter.NewRedisStrategy(redisClient)
	ipConfig := &config.RateLimitConfig{
		IPLimit:              50,
		WindowSeconds:        1,
		BlockDurationSeconds: 5, // Bloqueio curto para teste
	}

	rateLimiter := limiter.NewRateLimiter(storageStrategy, ipConfig, nil)
	healthHandler := handler.NewHealthHandler()

	router := chi.NewRouter()
	router.Use(middleware.RateLimitMiddleware(rateLimiter))
	router.Get("/api/v1/resource", healthHandler.Resource)

	server := &http.Server{
		Addr:    ":8084",
		Handler: router,
	}

	go func() {
		_ = server.ListenAndServe()
	}()
	defer func() { _ = server.Shutdown(ctx) }()

	time.Sleep(100 * time.Millisecond)

	// Teste de burst: 100 req/s por 5 segundos
	rate := vegeta.Rate{Freq: 100, Per: time.Second}
	duration := 5 * time.Second
	targeter := vegeta.NewStaticTargeter(vegeta.Target{
		Method: "GET",
		URL:    "http://localhost:8084/api/v1/resource",
		Header: http.Header{
			"X-Forwarded-For": []string{"192.168.1.200"},
		},
	})

	attacker := vegeta.NewAttacker()
	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Burst Load Test") {
		metrics.Add(res)
	}
	metrics.Close()

	t.Logf("Load test results for burst (100 req/s for 5s):")
	t.Logf("  Total requests: %d", metrics.Requests)
	t.Logf("  Successful requests: %d", int(metrics.Requests)-metrics.StatusCodes["429"])
	t.Logf("  Rate limited requests: %d", metrics.StatusCodes["429"])
	t.Logf("  Success rate: %.2f%%", (1-float64(metrics.StatusCodes["429"])/float64(metrics.Requests))*100)
	t.Logf("  Average latency: %v", metrics.Latencies.Mean)
	t.Logf("  P50 latency: %v", metrics.Latencies.P50)
	t.Logf("  P95 latency: %v", metrics.Latencies.P95)
	t.Logf("  P99 latency: %v", metrics.Latencies.P99)
	t.Logf("  Maximum latency: %v", metrics.Latencies.Max)

	// Validações
	assert.True(t, metrics.StatusCodes["429"] > 0, "Expected some requests to be rate limited")
	assert.True(t, metrics.StatusCodes["200"] > 0, "Expected some requests to succeed")
	assert.True(t, metrics.Latencies.Mean < 50*time.Millisecond, "Average latency should be < 50ms")

	_ = storageStrategy.Close()
}

// Testa o comportamento sob tráfego sustentado
func TestLoadSustainedHighTraffic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
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
	storageStrategy := limiter.NewRedisStrategy(redisClient)
	ipConfig := &config.RateLimitConfig{
		IPLimit:              20,
		WindowSeconds:        1,
		BlockDurationSeconds: 2,
	}

	rateLimiter := limiter.NewRateLimiter(storageStrategy, ipConfig, nil)
	healthHandler := handler.NewHealthHandler()

	router := chi.NewRouter()
	router.Use(middleware.RateLimitMiddleware(rateLimiter))
	router.Get("/api/v1/resource", healthHandler.Resource)

	server := &http.Server{
		Addr:    ":8085",
		Handler: router,
	}

	go func() {
		_ = server.ListenAndServe()
	}()
	defer func() { _ = server.Shutdown(ctx) }()

	time.Sleep(100 * time.Millisecond)

	// Teste sustentado: 30 req/s por 10 segundos
	rate := vegeta.Rate{Freq: 30, Per: time.Second}
	duration := 10 * time.Second
	targeter := vegeta.NewStaticTargeter(vegeta.Target{
		Method: "GET",
		URL:    "http://localhost:8085/api/v1/resource",
		Header: http.Header{
			"X-Forwarded-For": []string{"192.168.1.201"},
		},
	})

	attacker := vegeta.NewAttacker()
	var metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Sustained Load Test") {
		metrics.Add(res)
	}
	metrics.Close()

	t.Logf("Load test results for sustained traffic (30 req/s for 10s):")
	t.Logf("  Total requests: %d", metrics.Requests)
	t.Logf("  Successful requests: %d", int(metrics.Requests)-metrics.StatusCodes["429"])
	t.Logf("  Rate limited requests: %d", metrics.StatusCodes["429"])
	t.Logf("  Success rate: %.2f%%", (1-float64(metrics.StatusCodes["429"])/float64(metrics.Requests))*100)
	t.Logf("  Average latency: %v", metrics.Latencies.Mean)
	t.Logf("  Throughput: %.2f req/s", metrics.Rate)

	assert.True(t, metrics.StatusCodes["429"] > 0, "Expected some requests to be rate limited")
	assert.True(t, metrics.Latencies.Mean < 100*time.Millisecond, "Average latency should be < 100ms")

	_ = storageStrategy.Close()
}

// Testa a concorrência massiva de múltiplos IPs
func TestLoadMassiveConcurrency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
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
	storageStrategy := limiter.NewRedisStrategy(redisClient)
	ipConfig := &config.RateLimitConfig{
		IPLimit:              10,
		WindowSeconds:        1,
		BlockDurationSeconds: 5,
	}

	rateLimiter := limiter.NewRateLimiter(storageStrategy, ipConfig, nil)
	healthHandler := handler.NewHealthHandler()

	router := chi.NewRouter()
	router.Use(middleware.RateLimitMiddleware(rateLimiter))
	router.Get("/api/v1/resource", healthHandler.Resource)

	server := &http.Server{
		Addr:    ":8086",
		Handler: router,
	}

	go func() {
		_ = server.ListenAndServe()
	}()
	defer func() { _ = server.Shutdown(ctx) }()

	time.Sleep(100 * time.Millisecond)

	// Simula 50 usuários concorrentes, cada um fazendo 20 requisições
	numUsers := 50
	requestsPerUser := 20
	var wg sync.WaitGroup
	results := make(chan vegeta.Metrics, numUsers)

	t.Logf("=== Massive Concurrency Test (%d concurrent users) ===", numUsers)

	for i := 0; i < numUsers; i++ {
		wg.Add(1)
		go func(userID int) {
			defer wg.Done()

			rate := vegeta.Rate{Freq: requestsPerUser, Per: time.Second}
			duration := 1 * time.Second
			targeter := vegeta.NewStaticTargeter(vegeta.Target{
				Method: "GET",
				URL:    fmt.Sprintf("http://localhost:8086/api/v1/resource?user=%d", userID),
				Header: http.Header{
					"X-Forwarded-For": []string{fmt.Sprintf("192.168.%d.%d", userID/254+1, userID%254+1)},
				},
			})

			attacker := vegeta.NewAttacker()
			var metrics vegeta.Metrics
			for res := range attacker.Attack(targeter, rate, duration, fmt.Sprintf("User %d", userID)) {
				metrics.Add(res)
			}
			metrics.Close()
			results <- metrics
		}(i)
	}

	wg.Wait()
	close(results)

	// Agrega resultados
	var totalRequests uint64
	var totalSuccess int
	var totalBlocked int
	var totalLatency time.Duration

	for metrics := range results {
		totalRequests += metrics.Requests
		totalSuccess += int(metrics.Requests) - metrics.StatusCodes["429"]
		totalBlocked += metrics.StatusCodes["429"]
		totalLatency += metrics.Latencies.Mean
	}

	avgLatency := totalLatency / time.Duration(numUsers)

	t.Logf("  Total users: %d", numUsers)
	t.Logf("  Total requests: %d", totalRequests)
	t.Logf("  Successful requests: %d", totalSuccess)
	t.Logf("  Rate limited requests: %d", totalBlocked)
	t.Logf("  Success rate: %.2f%%", float64(totalSuccess)/float64(totalRequests)*100)
	t.Logf("  Average latency: %v", avgLatency)

	assert.True(t, totalSuccess > 0, "Expected some requests to succeed")
	assert.True(t, totalBlocked > 0, "Expected some requests to be rate limited")
	assert.True(t, avgLatency < 100*time.Millisecond, "Average latency should be < 100ms")

	_ = storageStrategy.Close()
}

// Testa a recuperação após bloqueio
func TestLoadRecoveryAfterBlock(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
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
	storageStrategy := limiter.NewRedisStrategy(redisClient)
	ipConfig := &config.RateLimitConfig{
		IPLimit:              5,
		WindowSeconds:        1,
		BlockDurationSeconds: 3, // Bloqueio de 3 segundos
	}

	rateLimiter := limiter.NewRateLimiter(storageStrategy, ipConfig, nil)
	healthHandler := handler.NewHealthHandler()

	router := chi.NewRouter()
	router.Use(middleware.RateLimitMiddleware(rateLimiter))
	router.Get("/api/v1/resource", healthHandler.Resource)

	server := &http.Server{
		Addr:    ":8087",
		Handler: router,
	}

	go func() {
		_ = server.ListenAndServe()
	}()
	defer func() { _ = server.Shutdown(ctx) }()

	time.Sleep(100 * time.Millisecond)

	t.Logf("Load test results for recovery after block:")

	// Fase 1: Excede o limite para ser bloqueado
	t.Logf("Phase 1: Exceeding limit to cause block...")
	rate := vegeta.Rate{Freq: 10, Per: time.Second}
	duration := 1 * time.Second
	targeter := vegeta.NewStaticTargeter(vegeta.Target{
		Method: "GET",
		URL:    "http://localhost:8087/api/v1/resource",
		Header: http.Header{
			"X-Forwarded-For": []string{"192.168.1.202"},
		},
	})

	attacker := vegeta.NewAttacker()
	var phase1Metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Phase 1") {
		phase1Metrics.Add(res)
	}
	phase1Metrics.Close()

	t.Logf("  Rate limited requests: %d", phase1Metrics.StatusCodes["429"])
	assert.True(t, phase1Metrics.StatusCodes["429"] > 0, "Expected some requests to be rate limited in phase 1")

	// Fase 2: Tenta requisições durante o bloqueio
	t.Logf("Phase 2: Trying requests during block...")
	time.Sleep(500 * time.Millisecond)

	rate = vegeta.Rate{Freq: 5, Per: time.Second}
	duration = 1 * time.Second

	var phase2Metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Phase 2") {
		phase2Metrics.Add(res)
	}
	phase2Metrics.Close()

	t.Logf("  Rate limited requests: %d", phase2Metrics.StatusCodes["429"])
	assert.Equal(t, int(phase2Metrics.Requests), phase2Metrics.StatusCodes["429"], "All requests should be rate limited")

	// Fase 3: Aguarda expiração do bloqueio e tenta novamente
	t.Logf("Phase 3: Waiting for block expiration (3s)...")
	time.Sleep(3500 * time.Millisecond) // Aguarda bloqueio expirar

	t.Logf("Phase 4: Testing recovery after block...")
	rate = vegeta.Rate{Freq: 5, Per: time.Second}
	duration = 1 * time.Second

	var phase3Metrics vegeta.Metrics
	for res := range attacker.Attack(targeter, rate, duration, "Phase 3") {
		phase3Metrics.Add(res)
	}
	phase3Metrics.Close()

	t.Logf("  Successful requests: %d", int(phase3Metrics.Requests)-phase3Metrics.StatusCodes["429"])
	t.Logf("  Rate limited requests: %d", phase3Metrics.StatusCodes["429"])
	assert.True(t, phase3Metrics.StatusCodes["200"] > 0, "Expected some requests to succeed after block expiration")

	_ = storageStrategy.Close()
}

// Testa o comportamento sob picos de tráfego
func TestLoadSpikeTraffic(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping load test in short mode")
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
	storageStrategy := limiter.NewRedisStrategy(redisClient)
	ipConfig := &config.RateLimitConfig{
		IPLimit:              15,
		WindowSeconds:        1,
		BlockDurationSeconds: 2,
	}

	rateLimiter := limiter.NewRateLimiter(storageStrategy, ipConfig, nil)
	healthHandler := handler.NewHealthHandler()

	router := chi.NewRouter()
	router.Use(middleware.RateLimitMiddleware(rateLimiter))
	router.Get("/api/v1/resource", healthHandler.Resource)

	server := &http.Server{
		Addr:    ":8088",
		Handler: router,
	}

	go func() {
		_ = server.ListenAndServe()
	}()
	defer func() { _ = server.Shutdown(ctx) }()

	time.Sleep(100 * time.Millisecond)

	t.Logf("Load test results for spike traffic:")

	// Simula tráfego normal seguido de picos
	scenarios := []struct {
		name     string
		rate     int
		duration time.Duration
	}{
		{"Normal traffic", 10, 2 * time.Second},
		{"Spike 1", 50, 1 * time.Second},
		{"Normal traffic", 10, 2 * time.Second},
		{"Spike 2", 100, 1 * time.Second},
		{"Normal traffic", 10, 2 * time.Second},
	}

	for _, scenario := range scenarios {
		t.Logf("  Scenario: %s (%d req/s for %v)", scenario.name, scenario.rate, scenario.duration)

		rate := vegeta.Rate{Freq: scenario.rate, Per: time.Second}
		targeter := vegeta.NewStaticTargeter(vegeta.Target{
			Method: "GET",
			URL:    "http://localhost:8088/api/v1/resource",
			Header: http.Header{
				"X-Forwarded-For": []string{"192.168.1.203"},
			},
		})

		attacker := vegeta.NewAttacker()
		var metrics vegeta.Metrics
		for res := range attacker.Attack(targeter, rate, scenario.duration, scenario.name) {
			metrics.Add(res)
		}
		metrics.Close()

		t.Logf("    Total: %d | Successful requests: %d | Rate limited requests: %d | Average latency: %v",
			metrics.Requests,
			int(metrics.Requests)-metrics.StatusCodes["429"],
			metrics.StatusCodes["429"],
			metrics.Latencies.Mean)

		// Aguarda um pouco entre cenários
		time.Sleep(500 * time.Millisecond)
	}

	_ = storageStrategy.Close()
}
