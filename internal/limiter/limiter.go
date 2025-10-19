package limiter

import (
	"context"
	"fmt"
	"time"

	"fc-pos-golang-rate-limiter/internal/config"
)

type RateLimiter struct {
	storage      StorageStrategy
	ipConfig     *config.RateLimitConfig
	tokenConfigs config.TokenConfigs
}

func NewRateLimiter(storage StorageStrategy, ipConfig *config.RateLimitConfig, tokenConfigs config.TokenConfigs) *RateLimiter {
	return &RateLimiter{
		storage:      storage,
		ipConfig:     ipConfig,
		tokenConfigs: tokenConfigs,
	}
}

type CheckResult struct {
	Allowed    bool
	Remaining  int
	ResetTime  time.Time
	Limit      int
	Identifier string
	IsToken    bool
}

// Verifica se uma requisição é permitida baseada no IP ou Token
func (rl *RateLimiter) Check(ctx context.Context, identifier string, isToken bool) (*CheckResult, error) {
	var limit int
	var window time.Duration
	var blockDuration time.Duration

	if isToken {
		// Verifica se o token existe na configuração
		tokenConfig, exists := rl.tokenConfigs.GetTokenConfig(identifier)
		if !exists {
			// Token não encontrado, volta para o limite de IP
			limit = rl.ipConfig.IPLimit
			window = rl.ipConfig.GetWindowDuration()
			blockDuration = rl.ipConfig.GetBlockDuration()
		} else {
			// Usa a configuração específica do token
			limit = tokenConfig.Limit
			window = tokenConfig.GetWindowDuration()
			blockDuration = tokenConfig.GetBlockDuration()
		}
	} else {
		// Usa a configuração de IP
		limit = rl.ipConfig.IPLimit
		window = rl.ipConfig.GetWindowDuration()
		blockDuration = rl.ipConfig.GetBlockDuration()
	}

	// Cria a chave de armazenamento
	key := rl.createKey(identifier, isToken)

	// Verifica com o armazenamento
	allowed, remaining, resetTime, err := rl.storage.Allow(ctx, key, limit, window, blockDuration)
	if err != nil {
		return nil, fmt.Errorf("storage check failed: %w", err)
	}

	return &CheckResult{
		Allowed:    allowed,
		Remaining:  remaining,
		ResetTime:  resetTime,
		Limit:      limit,
		Identifier: identifier,
		IsToken:    isToken,
	}, nil
}

func (rl *RateLimiter) Reset(ctx context.Context, identifier string, isToken bool) error {
	key := rl.createKey(identifier, isToken)
	return rl.storage.Reset(ctx, key)
}

func (rl *RateLimiter) createKey(identifier string, isToken bool) string {
	if isToken {
		return fmt.Sprintf("token:%s", identifier)
	}
	return fmt.Sprintf("ip:%s", identifier)
}

func (rl *RateLimiter) GetConfig() (*config.RateLimitConfig, config.TokenConfigs) {
	return rl.ipConfig, rl.tokenConfigs
}
