# ADR-0002: Redis como Storage Principal para Rate Limiting

## Status

Aceito

## Contexto

Precisávamos escolher uma solução de storage para o rate limiter que fosse:

- Rápida para leitura/escrita
- Suportasse operações atômicas
- Tivesse TTL automático
- Fosse confiável e escalável
- Permitisse implementação eficiente do Sliding Window

As opções consideradas foram:

1. **Memória local**: Rápida, mas não compartilhada entre instâncias
2. **Banco de dados relacional**: Confiável, mas lento para operações de rate limiting
3. **Redis**: Rápido, atômico, TTL nativo, ideal para rate limiting
4. **Memcached**: Rápido, mas sem estruturas de dados avançadas

## Decisão

Escolhemos **Redis** como storage principal para o rate limiter.

## Justificativa

### Vantagens do Redis

- **Performance**: Sub-milissegundo para operações de leitura/escrita
- **Atomicidade**: Operações atômicas garantem consistência
- **Estruturas de Dados**: Sorted Sets ideais para Sliding Window
- **TTL Nativo**: Limpeza automática de dados expirados
- **Escalabilidade**: Suporta alta concorrência
- **Persistência**: Opção de persistir dados em disco
- **Clustering**: Suporte a cluster para alta disponibilidade

### Estruturas Redis Utilizadas

- **Sorted Sets**: Para implementar Sliding Window
- **TTL**: Para limpeza automática de dados antigos
- **Pipeline**: Para operações atômicas em lote

## Alternativas Consideradas

### Memória Local

```go
// Problema: Não compartilhada entre instâncias
type LocalStorage struct {
    data map[string][]time.Time
    mutex sync.RWMutex
}
```

**Problemas:**

- ❌ Não funciona com múltiplas instâncias
- ❌ Perda de dados em restart
- ❌ Não escalável horizontalmente

### Banco Relacional

```sql
-- Problema: Muito lento para rate limiting
SELECT COUNT(*) FROM rate_limits 
WHERE key = ? AND timestamp > ?
```

**Problemas:**

- ❌ Latência alta (>10ms)
- ❌ Lock de tabela em alta concorrência
- ❌ Complexidade de limpeza de dados antigos

### Memcached

```go
// Problema: Sem estruturas de dados avançadas
memcached.Set(key, timestamps, ttl)
```

**Problemas:**

- ❌ Sem Sorted Sets
- ❌ Operações não atômicas
- ❌ Menos funcionalidades que Redis

## Implementação

### Estrutura de Dados

```go
// Chave: "ip:192.168.1.1" ou "token:abc123"
// Valor: Sorted Set com timestamps como scores
// TTL: window + buffer (ex: 1 minuto)

type RedisStrategy struct {
    client *redis.Client
}

func (r *RedisStrategy) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Time, error) {
    now := time.Now()
    windowStart := now.Add(-window)
    
    // Pipeline para operações atômicas
    pipe := r.client.Pipeline()
    
    // 1. Remove timestamps antigos
    pipe.ZRemRangeByScore(ctx, key, "0", fmt.Sprintf("%d", windowStart.UnixNano()))
    
    // 2. Conta requisições atuais
    countCmd := pipe.ZCard(ctx, key)
    
    // 3. Adiciona timestamp atual
    pipe.ZAdd(ctx, key, &redis.Z{
        Score:  float64(now.UnixNano()),
        Member: fmt.Sprintf("%d", now.UnixNano()),
    })
    
    // 4. Define TTL
    pipe.Expire(ctx, key, window+time.Minute)
    
    // 5. Executa tudo atomicamente
    _, err := pipe.Exec(ctx)
    if err != nil {
        return false, 0, time.Time{}, err
    }
    
    count, _ := countCmd.Result()
    allowed := count <= int64(limit)
    remaining := limit - int(count)
    if remaining < 0 {
        remaining = 0
    }
    
    return allowed, remaining, now.Add(window), nil
}
```

### Configuração

```go
// Configuração otimizada para rate limiting
redisClient := redis.NewClient(&redis.Options{
    Addr:     "localhost:6379",
    Password: "",
    DB:       0,
    // Configurações de performance
    PoolSize:     100,
    MinIdleConns: 10,
    MaxRetries:   3,
    DialTimeout:  5 * time.Second,
    ReadTimeout:  3 * time.Second,
    WriteTimeout: 3 * time.Second,
})
```

## Consequências

### Positivas

- ✅ Performance excelente (<1ms)
- ✅ Operações atômicas garantem consistência
- ✅ TTL automático limpa dados antigos
- ✅ Escalável horizontalmente com cluster
- ✅ Suporte a alta concorrência
- ✅ Estruturas de dados ideais para rate limiting

### Negativas

- ❌ Dependência externa (Redis)
- ❌ Ponto único de falha (sem cluster)
- ❌ Uso de memória para armazenar dados
- ❌ Complexidade de configuração

### Riscos Mitigados

- **Redis indisponível**: Middleware falha graciosamente
- **Alta memória**: TTL automático + configuração de maxmemory
- **Performance**: Pool de conexões + pipeline
- **Disponibilidade**: Cluster Redis + health checks

## Estratégia de Fallback

```go
// Em caso de falha do Redis, permitir requisições
func (rl *RateLimiter) Check(ctx context.Context, identifier string, isToken bool) (*CheckResult, error) {
    result, err := rl.storage.Allow(ctx, key, limit, window)
    if err != nil {
        // Log error mas permite requisição
        log.Printf("Redis error, allowing request: %v", err)
        return &CheckResult{
            Allowed:    true,
            Remaining:  limit,
            ResetTime:  time.Now().Add(window),
            Limit:      limit,
            Identifier: identifier,
            IsToken:    isToken,
        }, nil
    }
    return result, nil
}
```

## Monitoramento

### Métricas Importantes

- Latência de operações Redis
- Taxa de erro de conexão
- Uso de memória Redis
- Número de chaves ativas
- Throughput de operações

### Alertas

- Redis indisponível
- Latência > 5ms
- Uso de memória > 80%
- Taxa de erro > 1%

## Referências

- [Redis Documentation](https://redis.io/docs/)
- [Redis Sorted Sets](https://redis.io/docs/data-types/sorted-sets/)
- [Redis Performance](https://redis.io/docs/management/optimization/)
- [Rate Limiting with Redis](https://redis.io/docs/use-cases/rate-limiting/)
