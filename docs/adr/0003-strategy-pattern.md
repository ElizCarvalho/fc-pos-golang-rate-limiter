# ADR-0003: Strategy Pattern para Storage do Rate Limiter

## Status

Aceito

## Contexto

Precisávamos projetar o sistema de storage do rate limiter de forma que fosse:

- Flexível para trocar implementações
- Testável com mocks
- Extensível para novos storages
- Desacoplado da lógica de negócio

As opções consideradas foram:

1. **Implementação direta**: Código acoplado, difícil de testar
2. **Interface simples**: Boa, mas sem flexibilidade
3. **Strategy Pattern**: Flexível, testável, extensível
4. **Dependency Injection**: Complexo demais para o caso

## Decisão

Escolhemos o **Strategy Pattern** para abstrair o storage do rate limiter.

## Justificativa

### Vantagens do Strategy Pattern

- **Flexibilidade**: Fácil troca de implementações
- **Testabilidade**: Mock simples para testes
- **Extensibilidade**: Novos storages sem modificar código existente
- **Desacoplamento**: Lógica de negócio independente do storage
- **Manutenibilidade**: Mudanças isoladas por implementação

### Interface Strategy (Exemplo)

```go
type StorageStrategy interface {
    // Allow checks if a request is allowed for the given key within the limit and window
    Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, resetTime time.Time, err error)
    
    // Reset removes all entries for the given key
    Reset(ctx context.Context, key string) error
    
    // Close closes the storage connection
    Close() error
}
```

## Alternativas Consideradas

### Implementação Acoplada

```go
// Problema: Código acoplado
type RateLimiter struct {
    redisClient *redis.Client
}

func (rl *RateLimiter) Check(ctx context.Context, identifier string, isToken bool) (*CheckResult, error) {
    // Lógica Redis diretamente aqui
    pipe := rl.redisClient.Pipeline()
    // ...
}
```

**Problemas:**

- ❌ Difícil de testar
- ❌ Acoplado ao Redis
- ❌ Impossível trocar storage
- ❌ Violação do princípio de responsabilidade única

### Interface Simples

```go
// Problema: Pouca flexibilidade
type Storage interface {
    Allow(key string, limit int) bool
}
```

**Problemas:**

- ❌ Interface muito simples
- ❌ Sem contexto para cancelamento
- ❌ Sem informações de rate limit
- ❌ Sem operação de reset

### Dependency Injection

```go
// Problema: Complexidade desnecessária
type RateLimiter struct {
    storage StorageStrategy
    config  Config
    logger  Logger
    metrics Metrics
}
```

**Problemas:**

- ❌ Muito complexo para o caso
- ❌ Muitas dependências
- ❌ Over-engineering

## Implementação

### Interface Strategy

```go
// StorageStrategy defines the interface for rate limiter storage
type StorageStrategy interface {
    // Allow checks if a request is allowed for the given key within the limit and window
    Allow(ctx context.Context, key string, limit int, window time.Duration) (allowed bool, remaining int, resetTime time.Time, err error)
    
    // Reset removes all entries for the given key
    Reset(ctx context.Context, key string) error
    
    // Close closes the storage connection
    Close() error
}
```

### Implementação Redis

```go
// RedisStrategy implements StorageStrategy using Redis with Sliding Window algorithm
type RedisStrategy struct {
    client *redis.Client
}

func NewRedisStrategy(client *redis.Client) *RedisStrategy {
    return &RedisStrategy{client: client}
}

func (r *RedisStrategy) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Time, error) {
    // Implementação do Sliding Window com Redis
    // ...
}

func (r *RedisStrategy) Reset(ctx context.Context, key string) error {
    return r.client.Del(ctx, key).Err()
}

func (r *RedisStrategy) Close() error {
    return r.client.Close()
}
```

### Rate Limiter com Strategy

```go
// RateLimiter handles rate limiting logic
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

func (rl *RateLimiter) Check(ctx context.Context, identifier string, isToken bool) (*CheckResult, error) {
    // Lógica de negócio independente do storage
    // ...
    return rl.storage.Allow(ctx, key, limit, window)
}
```

### Mock para Testes

```go
// MockStorageStrategy is a mock implementation for testing
type MockStorageStrategy struct {
    allowResults map[string]bool
    allowCounts  map[string]int
    allowErrors  map[string]error
    callCounts   map[string]int
}

func (m *MockStorageStrategy) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Time, error) {
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
```

## Consequências

### Positivas

- ✅ Fácil troca de implementações
- ✅ Testes simples com mocks
- ✅ Código desacoplado e manutenível
- ✅ Extensível para novos storages
- ✅ Segue princípios SOLID
- ✅ Interface clara e bem definida

### Negativas

- ❌ Maior complexidade inicial
- ❌ Mais interfaces para manter
- ❌ Possível over-engineering se não usado

### Riscos Mitigados

- **Interface muito complexa**: Interface simples e focada
- **Muitas implementações**: Apenas Redis implementado
- **Over-engineering**: Pattern justificado pelo requisito de flexibilidade

## Implementações Futuras

### Memória Local

```go
type MemoryStrategy struct {
    data  map[string][]time.Time
    mutex sync.RWMutex
}

func (m *MemoryStrategy) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Time, error) {
    // Implementação em memória
}
```

### Database

```go
type DatabaseStrategy struct {
    db *sql.DB
}

func (d *DatabaseStrategy) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Time, error) {
    // Implementação com banco de dados
}
```

### Cache Distribuído

```go
type CacheStrategy struct {
    cache cache.Cache
}

func (c *CacheStrategy) Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, int, time.Time, error) {
    // Implementação com cache distribuído
}
```

## Testes

### Testes Unitários

```go
func TestRateLimiter_Check(t *testing.T) {
    mockStorage := NewMockStorageStrategy()
    rateLimiter := limiter.NewRateLimiter(mockStorage, ipConfig, tokenConfigs)
    
    // Teste com mock
    mockStorage.SetAllowResult("ip:192.168.1.1", true, 5)
    result, err := rateLimiter.Check(ctx, "192.168.1.1", false)
    assert.NoError(t, err)
    assert.True(t, result.Allowed)
}
```

### Testes de Integração

```go
func TestRedisStrategy_Integration(t *testing.T) {
    redisClient := setupRedis()
    strategy := limiter.NewRedisStrategy(redisClient)
    
    // Teste com Redis real
    allowed, remaining, resetTime, err := strategy.Allow(ctx, "test:key", 5, time.Second)
    assert.NoError(t, err)
    assert.True(t, allowed)
}
```

## Referências

- [Strategy Pattern](https://en.wikipedia.org/wiki/Strategy_pattern)
- [Go Interface Best Practices](https://github.com/golang/go/wiki/CodeReviewComments#interfaces)
- [SOLID Principles](https://en.wikipedia.org/wiki/SOLID)
- [Dependency Injection in Go](https://blog.drewolson.org/dependency-injection-in-go)
