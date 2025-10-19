# 📊 Testes de Carga - Rate Limiter

## Visão Geral

Este documento descreve os testes de carga implementados para validar o comportamento do rate limiter sob **diferentes condições de alto tráfego**, conforme requisito do desafio:

> "Teste seu rate limiter sob diferentes condições de carga para garantir que ele funcione conforme esperado em situações de alto tráfego."

## Suíte de Testes

### 1. TestLoadIPRateLimit
**Objetivo**: Validar bloqueio básico por IP

**Configuração**:
- Limite: 10 req/s
- Taxa de ataque: 15 req/s
- Duração: 3 segundos

**Validações**:
- ✅ Requisições dentro do limite são permitidas
- ✅ Requisições acima do limite são bloqueadas (429)
- ✅ Headers de rate limit corretos

---

### 2. TestLoadTokenRateLimit
**Objetivo**: Validar bloqueio básico por Token

**Configuração**:
- Limite: 20 req/s (token)
- Taxa de ataque: 25 req/s
- Duração: 3 segundos

**Validações**:
- ✅ Token tem prioridade sobre IP
- ✅ Limites específicos do token são respeitados
- ✅ Bloqueio correto quando excede limite

---

### 3. TestLoadConcurrentUsers
**Objetivo**: Validar isolamento entre múltiplos IPs

**Configuração**:
- 5 usuários simultâneos
- Cada um com IP diferente
- 8 req/s por usuário

**Validações**:
- ✅ Limites são isolados por IP
- ✅ Um IP bloqueado não afeta outros
- ✅ Sistema mantém performance com múltiplos IPs

---

### 4. TestLoadHighTrafficBurst ⚡
**Objetivo**: Validar comportamento sob burst intenso

**Configuração**:
- Limite: 50 req/s
- Taxa de ataque: **100 req/s**
- Duração: 5 segundos
- Total: ~500 requisições

**Métricas Coletadas**:
- Total de requisições
- Taxa de sucesso
- Latência média, p50, p95, p99, max
- Requisições bloqueadas

**Validações**:
- ✅ Sistema bloqueia requisições excedentes
- ✅ Latência média < 50ms
- ✅ Sistema se recupera após burst
- ✅ Não há degradação de performance

**Exemplo de Saída**:
```
=== Teste de Burst (100 req/s por 5s) ===
  Total de requisições: 500
  Requisições bem-sucedidas: 250
  Requisições bloqueadas (429): 250
  Taxa de sucesso: 50.00%
  Latência média: 2.5ms
  Latência p50: 2ms
  Latência p95: 5ms
  Latência p99: 8ms
  Latência máxima: 15ms
```

---

### 5. TestLoadSustainedHighTraffic ⚡
**Objetivo**: Validar estabilidade sob tráfego sustentado

**Configuração**:
- Limite: 20 req/s
- Taxa de ataque: **30 req/s**
- Duração: 10 segundos
- Total: ~300 requisições

**Métricas Coletadas**:
- Total de requisições
- Taxa de sucesso
- Latência média
- Throughput real

**Validações**:
- ✅ Sistema mantém estabilidade por período prolongado
- ✅ Latência média < 100ms
- ✅ Bloqueio consistente ao longo do tempo
- ✅ Sem degradação progressiva

**Exemplo de Saída**:
```
=== Teste de Tráfego Sustentado (30 req/s por 10s) ===
  Total de requisições: 300
  Requisições bem-sucedidas: 200
  Requisições bloqueadas (429): 100
  Taxa de sucesso: 66.67%
  Latência média: 3.2ms
  Throughput: 30.00 req/s
```

---

### 6. TestLoadMassiveConcurrency ⚡
**Objetivo**: Validar escalabilidade com concorrência massiva

**Configuração**:
- **50 usuários simultâneos**
- Cada usuário: IP único
- 20 requisições por usuário
- Total: ~1000 requisições

**Métricas Agregadas**:
- Total de usuários
- Total de requisições
- Taxa de sucesso global
- Latência média agregada

**Validações**:
- ✅ Sistema suporta 50+ usuários simultâneos
- ✅ Isolamento perfeito entre IPs
- ✅ Latência média < 100ms
- ✅ Sem race conditions

**Exemplo de Saída**:
```
=== Teste de Concorrência Massiva (50 usuários simultâneos) ===
  Total de usuários: 50
  Total de requisições: 1000
  Requisições bem-sucedidas: 500
  Requisições bloqueadas (429): 500
  Taxa de sucesso: 50.00%
  Latência média: 5.8ms
```

---

### 7. TestLoadRecoveryAfterBlock ⚡
**Objetivo**: Validar recuperação após bloqueio

**Configuração**:
- Limite: 5 req/s
- Block duration: 3 segundos
- 4 fases de teste

**Fases**:
1. **Fase 1**: Excede limite para causar bloqueio
2. **Fase 2**: Tenta requisições durante bloqueio (todas devem falhar)
3. **Fase 3**: Aguarda expiração do bloqueio (3s)
4. **Fase 4**: Valida recuperação (requisições devem passar)

**Validações**:
- ✅ Bloqueio é ativado quando limite é excedido
- ✅ Todas requisições são bloqueadas durante block duration
- ✅ Sistema se recupera automaticamente após expiração
- ✅ Requisições voltam a passar normalmente

**Exemplo de Saída**:
```
=== Teste de Recuperação Após Bloqueio ===
Fase 1: Excedendo limite para causar bloqueio...
  Requisições bloqueadas: 5
Fase 2: Tentando requisições durante bloqueio...
  Todas requisições bloqueadas: 5
Fase 3: Aguardando expiração do bloqueio (3s)...
Fase 4: Testando recuperação após bloqueio...
  Requisições bem-sucedidas: 5
  Requisições bloqueadas: 0
```

---

### 8. TestLoadSpikeTraffic ⚡
**Objetivo**: Validar resiliência sob picos alternados

**Configuração**:
- Limite: 15 req/s
- Block duration: 2 segundos
- 5 cenários alternados

**Cenários**:
1. Tráfego normal: 10 req/s por 2s
2. **Pico 1**: 50 req/s por 1s
3. Tráfego normal: 10 req/s por 2s
4. **Pico 2**: 100 req/s por 1s
5. Tráfego normal: 10 req/s por 2s

**Validações**:
- ✅ Sistema bloqueia durante picos
- ✅ Sistema se recupera entre picos
- ✅ Tráfego normal é permitido após picos
- ✅ Resiliência mantida ao longo do teste

**Exemplo de Saída**:
```
=== Teste de Picos de Tráfego ===
  Cenário: Tráfego normal (10 req/s por 2s)
    Total: 20 | Sucesso: 20 | Bloqueado: 0 | Latência: 2ms
  Cenário: Pico 1 (50 req/s por 1s)
    Total: 50 | Sucesso: 15 | Bloqueado: 35 | Latência: 3ms
  Cenário: Tráfego normal (10 req/s por 2s)
    Total: 20 | Sucesso: 20 | Bloqueado: 0 | Latência: 2ms
  Cenário: Pico 2 (100 req/s por 1s)
    Total: 100 | Sucesso: 15 | Bloqueado: 85 | Latência: 4ms
  Cenário: Tráfego normal (10 req/s por 2s)
    Total: 20 | Sucesso: 20 | Bloqueado: 0 | Latência: 2ms
```

---

## Execução dos Testes

### Todos os Testes de Carga
```bash
make test-load-automated
# ou
go test -v ./tests/load/...
```

### Testes Individuais

```bash
# Teste de burst
make test-load-burst

# Teste de tráfego sustentado
make test-load-sustained

# Teste de concorrência massiva
make test-load-concurrency

# Teste de recuperação
make test-load-recovery

# Teste de picos
make test-load-spike
```

### Testes Manuais com Vegeta

```bash
# Teste básico
make test-load

# Teste customizado
echo "GET http://localhost:8080/api/v1/resource" | \
  vegeta attack -rate=100 -duration=10s | \
  vegeta report
```

---

## Métricas Importantes

### Latência
- **Média**: Tempo médio de resposta
- **p50**: 50% das requisições abaixo deste valor
- **p95**: 95% das requisições abaixo deste valor
- **p99**: 99% das requisições abaixo deste valor
- **Max**: Latência máxima observada

### Taxa de Sucesso
- Percentual de requisições HTTP 200
- Deve refletir o limite configurado

### Throughput
- Requisições processadas por segundo
- Deve respeitar o limite configurado

### Bloqueio
- Requisições HTTP 429
- Deve bloquear excedentes corretamente

---

## Critérios de Aceitação

### Performance
- ✅ Latência média < 50ms em burst
- ✅ Latência média < 100ms em tráfego sustentado
- ✅ Throughput consistente com limite configurado

### Funcionalidade
- ✅ Bloqueio correto quando limite é excedido
- ✅ Recuperação automática após block duration
- ✅ Isolamento perfeito entre IPs
- ✅ Priorização Token > IP

### Escalabilidade
- ✅ Suporta 50+ usuários simultâneos
- ✅ Sem degradação com alta concorrência
- ✅ Sem race conditions

### Resiliência
- ✅ Recuperação após picos de tráfego
- ✅ Estabilidade em tráfego sustentado
- ✅ Comportamento previsível

---

## Ambiente de Teste

### Requisitos
- Docker e Docker Compose
- Go 1.23.5+
- Redis 7+
- Vegeta (para testes manuais)

### Setup
```bash
# Subir ambiente
make docker-up

# Executar testes
make test-load-automated

# Derrubar ambiente
make docker-down
```

---

## Conclusão

A suíte de testes de carga valida que o rate limiter:

1. ✅ **Funciona corretamente** sob diferentes condições de carga
2. ✅ **Mantém performance** mesmo em situações de alto tráfego
3. ✅ **Escala adequadamente** com múltiplos usuários
4. ✅ **Se recupera automaticamente** após bloqueios
5. ✅ **É resiliente** a picos e variações de tráfego

Todos os testes foram projetados para simular **cenários reais de produção** e garantir que o sistema atende aos requisitos do desafio.

