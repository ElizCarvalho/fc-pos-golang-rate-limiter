# 🚦 Rate Limiter Go

> Sistema de rate limiting em Go com Redis e algoritmo Sliding Window

## 📌 Sobre

Rate limiter robusto que utiliza Redis como storage e o algoritmo Sliding Window para controle preciso de requisições. Suporta limitação por IP e token, com priorização de token sobre IP.

## 🚀 Execução Rápida

```bash
# Subir ambiente completo
make docker-up

# Testar endpoints
curl http://localhost:8080/health
curl http://localhost:8080/api/v1/resource

# Testar rate limiting
for i in {1..15}; do curl http://localhost:8080/api/v1/resource; done

# Testar com token
curl -H "API_KEY: std_1234567890" http://localhost:8080/api/v1/resource
```

## ⚙️ Configuração

### Variáveis de Ambiente

```env
RATE_LIMIT_IP=10
RATE_LIMIT_WINDOW_SECONDS=1
REDIS_HOST=localhost
REDIS_PORT=6379
SERVER_PORT=8080
```

### Tokens (configs/tokens.json)

```json
{
  "std_1234567890": {
    "limit": 100,
    "window_seconds": 1,
    "block_duration_seconds": 300
  }
}
```

## 📚 API

### GET /health

Health check (sem rate limiting)

### GET /api/v1/resource

Recurso com rate limiting aplicado

**Headers:**

- `X-RateLimit-Limit`: Limite de requisições
- `X-RateLimit-Remaining`: Requisições restantes
- `X-RateLimit-Reset`: Timestamp de reset

**Swagger UI:** <http://localhost:8080/swagger>

## 🧪 Testes

```bash
# Testes unitários
go test ./internal/...

# Testes de integração
go test ./tests/integration/...

# Testes de carga
go test ./tests/load/... -v

# Todos os testes
make test
```

## 🏗️ Arquitetura

```bash
/
├── cmd/server/           # Entry point
├── internal/
│   ├── config/          # Configurações + testes
│   ├── limiter/         # Rate limiter + testes
│   ├── middleware/      # Middleware HTTP + testes
│   └── handler/         # Handlers HTTP
├── tests/
│   ├── integration/     # Testes com Redis real
│   └── load/            # Testes de carga
└── docs/                # Documentação
```

### Strategy Pattern

```go
type StorageStrategy interface {
    Allow(ctx, key, limit, window) (bool, int, time.Time, error)
    Reset(ctx, key) error
    Close() error
}
```

## 📝 Documentação

- [ADRs](./docs/adr/) - Decisões arquiteturais
- [Testes de Carga](./docs/load-testing.md) - Guia completo
- [Exemplos](./api/requests.http) - Requisições HTTP

## 🔧 Comandos

```bash
make help              # Ver todos os comandos
make setup             # Configurar ambiente (.env)
make docker-up         # Subir ambiente
make docker-logs       # Ver logs
make test              # Executar testes
make test-load         # Testes de carga
make docker-down       # Derrubar ambiente
make clean             # Limpar volumes
```
