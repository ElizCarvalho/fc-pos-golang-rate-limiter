package limiter

import (
	"context"
	"time"
)

// Define a interface para o armazenamento do rate limiter
type StorageStrategy interface {
	// Allow verifica se uma requisição é permitida para a chave dada dentro do limite e janela
	Allow(ctx context.Context, key string, limit int, window time.Duration, blockDuration time.Duration) (allowed bool, remaining int, resetTime time.Time, err error)
	// Reset remove todas as entradas para a chave dada
	Reset(ctx context.Context, key string) error
	// Close fecha a conexão de armazenamento
	Close() error
}

type RateLimitResult struct {
	Allowed   bool
	Remaining int
	ResetTime time.Time
	Limit     int
}

func NewRateLimitResult(allowed bool, remaining int, resetTime time.Time, limit int) *RateLimitResult {
	return &RateLimitResult{
		Allowed:   allowed,
		Remaining: remaining,
		ResetTime: resetTime,
		Limit:     limit,
	}
}
