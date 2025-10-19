package limiter

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

type RedisStrategy struct {
	client *redis.Client
}

func NewRedisStrategy(client *redis.Client) *RedisStrategy {
	return &RedisStrategy{
		client: client,
	}
}

// Implementa o algoritmo Sliding Window com BlockDuration usando Redis Sorted Sets
func (r *RedisStrategy) Allow(ctx context.Context, key string, limit int, window time.Duration, blockDuration time.Duration) (bool, int, time.Time, error) {
	now := time.Now()
	windowStart := now.Add(-window)

	// Verifica se está bloqueado
	if blocked, resetTime, err := r.checkBlockStatus(ctx, key, now); err != nil {
		return false, 0, time.Time{}, err
	} else if blocked {
		return false, 0, resetTime, nil
	}

	// Cria um pipeline para operações atômicas
	pipe := r.client.Pipeline()

	// Remove entradas expiradas (mais antigas que a janela)
	pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))

	// Conta as entradas atuais na janela
	countCmd := pipe.ZCard(ctx, key)

	// Executa o pipeline para obter a contagem atual
	_, err := pipe.Exec(ctx)
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("redis pipeline execution failed: %w", err)
	}

	// Obtém o resultado da contagem
	count, err := countCmd.Result()
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("failed to get count: %w", err)
	}

	// Verifica se estamos dentro do limite (antes de adicionar a requisição atual)
	allowed := count < int64(limit)
	remaining := limit - int(count)
	if remaining < 0 {
		remaining = 0
	}

	// Se excedeu o limite, bloqueia por blockDuration
	if !allowed {
		blockKey := key + ":block"
		err = r.client.Set(ctx, blockKey, "1", blockDuration).Err()
		if err != nil {
			return false, 0, time.Time{}, fmt.Errorf("failed to set block: %w", err)
		}
		resetTime := now.Add(blockDuration)
		return false, remaining, resetTime, nil
	}

	// Se está permitido, adiciona a requisição atual
	pipe = r.client.Pipeline()
	pipe.ZAdd(ctx, key, &redis.Z{
		Score:  float64(now.UnixNano()),
		Member: fmt.Sprintf("%d", now.UnixNano()),
	})
	pipe.Expire(ctx, key, window+time.Minute)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return false, 0, time.Time{}, fmt.Errorf("failed to add request: %w", err)
	}

	// Atualiza o remaining após adicionar a requisição
	remaining = limit - int(count) - 1
	if remaining < 0 {
		remaining = 0
	}

	// Calcula o tempo de reset (quando a entrada mais antiga na janela expirar)
	resetTime := now.Add(window)

	// Se tivermos entradas, encontramos a mais antiga para calcular o tempo de reset correto
	if count > 0 {
		oldestCmd := r.client.ZRangeWithScores(ctx, key, 0, 0)
		oldest, err := oldestCmd.Result()
		if err == nil && len(oldest) > 0 {
			oldestTime := time.Unix(0, int64(oldest[0].Score))
			resetTime = oldestTime.Add(window)
		}
	}

	return allowed, remaining, resetTime, nil
}

func (r *RedisStrategy) Reset(ctx context.Context, key string) error {
	// Remove tanto a chave de contagem quanto a de bloqueio
	pipe := r.client.Pipeline()
	pipe.Del(ctx, key)
	pipe.Del(ctx, key+":block")
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisStrategy) Close() error {
	return r.client.Close()
}

func (r *RedisStrategy) GetRedisClient() *redis.Client {
	return r.client
}

// Verifica se a chave está bloqueada e retorna o tempo de reset
func (r *RedisStrategy) checkBlockStatus(ctx context.Context, key string, now time.Time) (bool, time.Time, error) {
	blockKey := key + ":block"

	blocked, err := r.client.Exists(ctx, blockKey).Result()
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to check block status: %w", err)
	}

	if blocked == 0 {
		return false, time.Time{}, nil
	}

	blockTTL, err := r.client.TTL(ctx, blockKey).Result()
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to get block TTL: %w", err)
	}

	return true, now.Add(blockTTL), nil
}
