# 📊 Testes de Carga

## Visão Geral

Testes de carga para validar o rate limiter sob **diferentes condições de alto tráfego**.

## Suíte de Testes

### 1. TestLoadIPRateLimit

- **Objetivo**: Bloqueio básico por IP
- **Config**: 10 req/s, ataque 15 req/s por 3s
- **Valida**: Requisições bloqueadas (429) quando excede limite

### 2. TestLoadTokenRateLimit

- **Objetivo**: Bloqueio básico por Token
- **Config**: 20 req/s (token), ataque 25 req/s por 3s
- **Valida**: Token tem prioridade sobre IP

### 3. TestLoadConcurrentUsers

- **Objetivo**: Isolamento entre múltiplos IPs
- **Config**: 5 usuários simultâneos, 8 req/s cada
- **Valida**: Limites isolados por IP

### 4. TestLoadHighTrafficBurst ⚡

- **Objetivo**: Burst intenso de tráfego
- **Config**: 50 req/s, ataque **100 req/s** por 5s
- **Valida**: Sistema bloqueia excedentes, latência < 50ms

### 5. TestLoadSustainedHighTraffic ⚡

- **Objetivo**: Tráfego sustentado
- **Config**: 20 req/s, ataque **30 req/s** por 10s
- **Valida**: Estabilidade por período prolongado

### 6. TestLoadMassiveConcurrency ⚡

- **Objetivo**: Concorrência massiva
- **Config**: **50 usuários simultâneos**, 20 req cada
- **Valida**: Escalabilidade, latência < 100ms

### 7. TestLoadRecoveryAfterBlock ⚡

- **Objetivo**: Recuperação após bloqueio
- **Config**: 5 req/s, block duration 3s
- **Valida**: Recuperação automática após expiração

### 8. TestLoadSpikeTraffic ⚡

- **Objetivo**: Picos alternados de tráfego
- **Config**: 15 req/s, picos de 50-100 req/s
- **Valida**: Resiliência a variações de tráfego

## Execução

### Todos os Testes

```bash
make test-load-automated
# ou
go test -v ./tests/load/...
```

### Testes Individuais

```bash
make test-load-burst        # Burst
make test-load-sustained    # Sustentado  
make test-load-concurrency  # Concorrência
make test-load-recovery     # Recuperação
make test-load-spike        # Picos
```

### Testes Manuais (Vegeta)

```bash
make test-load

# Customizado
echo "GET http://localhost:8080/api/v1/resource" | \
  vegeta attack -rate=100 -duration=10s | \
  vegeta report
```

## Métricas

- **Latência**: Média, p50, p95, p99, max
- **Taxa de Sucesso**: % de requisições HTTP 200
- **Throughput**: Requisições/s (respeitando limite)
- **Bloqueio**: Requisições HTTP 429

## Critérios de Aceitação

### Performance

- ✅ Latência média < 50ms (burst)
- ✅ Latência média < 100ms (sustentado)
- ✅ Throughput consistente com limite

### Funcionalidade

- ✅ Bloqueio correto quando excede limite
- ✅ Recuperação automática após block duration
- ✅ Isolamento perfeito entre IPs
- ✅ Priorização Token > IP

### Escalabilidade

- ✅ Suporta 50+ usuários simultâneos
- ✅ Sem degradação com alta concorrência
- ✅ Sem race conditions

## Setup

```bash
# Subir ambiente
make docker-up

# Executar testes
make test-load-automated

# Derrubar ambiente  
make docker-down
```

## Conclusão

A suíte valida que o rate limiter:

1. ✅ Funciona corretamente sob alto tráfego
2. ✅ Mantém performance em situações extremas
3. ✅ Escala com múltiplos usuários
4. ✅ Se recupera automaticamente
5. ✅ É resiliente a picos de tráfego
