package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"fc-pos-golang-rate-limiter/internal/config"
	"fc-pos-golang-rate-limiter/internal/handler"
	"fc-pos-golang-rate-limiter/internal/limiter"
	ratelimitMiddleware "fc-pos-golang-rate-limiter/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-redis/redis/v8"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title FullCycle Rate Limiter API
// @version 1.0
// @description Desafio técnico FullCycle - Rate Limiter API
// @termsOfService http://swagger.io/terms/
// @contact.name Eliz Carvalho
// @contact.url https://www.github.com/ElizCarvalho
// @contact.email elizabethcarvalho.ti@gmail.com

// @host localhost:8080
// @BasePath /
// @schemes http https

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name API_KEY

func main() {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	tokenConfigs, err := config.LoadTokenConfigs("configs/tokens.json")
	if err != nil {
		log.Fatalf("Failed to load token configurations: %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.GetRedisAddr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	storageStrategy := limiter.NewRedisStrategy(redisClient)
	rateLimiter := limiter.NewRateLimiter(storageStrategy, &cfg.RateLimit, tokenConfigs)

	healthHandler := handler.NewHealthHandler()

	router := setupRouter(rateLimiter, healthHandler)

	server := &http.Server{
		Addr:         ":" + cfg.Server.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Server starting on port %s", cfg.Server.Port)
		log.Printf("Environment: %s", cfg.Server.AppEnv)
		log.Printf("Swagger UI: http://localhost:%s/swagger", cfg.Server.Port)
		log.Printf("Health Check: http://localhost:%s/health", cfg.Server.Port)

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	if err := storageStrategy.Close(); err != nil {
		log.Printf("Error closing Redis connection: %v", err)
	}

	log.Println("Server exited")
}

func setupRouter(rateLimiter *limiter.RateLimiter, healthHandler *handler.HealthHandler) *chi.Mux {
	router := chi.NewRouter()

	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(60 * time.Second))

	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "API_KEY"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	router.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("http://localhost:8080/swagger/doc.json"),
	))

	router.Get("/health", healthHandler.Health)

	router.Route("/api/v1", func(r chi.Router) {
		r.Use(ratelimitMiddleware.RateLimitMiddleware(rateLimiter))
		r.Get("/resource", healthHandler.Resource)
	})

	return router
}
