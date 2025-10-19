# ==============================================================================
# Variáveis
# ==============================================================================
APP_NAME=rate-limiter
VERSION?=latest
PORT?=8080

# Cores
BLUE=\033[0;34m
GREEN=\033[0;32m
RED=\033[0;31m
YELLOW=\033[0;33m
NC=\033[0m

# ==============================================================================
# Comandos de Execução
# ==============================================================================
.PHONY: help docker-up docker-down docker-logs test test-unit test-integration test-load test-load-automated test-load-burst test-load-sustained test-load-concurrency test-load-recovery test-load-spike clean

help: ## Mostra comandos disponíveis
	@echo "$(BLUE)Comandos disponíveis:$(NC)"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  $(YELLOW)%-20s$(NC) %s\n", $$1, $$2}'

docker-up: ## Sobe ambiente completo (Redis + App na porta 8080)
	@echo "$(BLUE)🐳 Subindo ambiente completo...$(NC)"
	@docker-compose up -d
	@echo "$(GREEN)✅ Ambiente rodando em http://localhost:$(PORT)$(NC)"
	@echo "$(YELLOW)📊 Swagger UI: http://localhost:$(PORT)/swagger$(NC)"
	@echo "$(YELLOW)🔍 Health Check: http://localhost:$(PORT)/health$(NC)"

docker-down: ## Derruba ambiente
	@echo "$(BLUE)🐳 Derrubando ambiente...$(NC)"
	@docker-compose down

docker-logs: ## Visualiza logs da aplicação
	@echo "$(BLUE)📋 Visualizando logs...$(NC)"
	@docker-compose logs -f app

test: ## Executa todos os testes
	@echo "$(BLUE)🧪 Executando todos os testes...$(NC)"
	@go test -v ./...

test-unit: ## Executa apenas testes unitários
	@echo "$(BLUE)🧪 Executando testes unitários...$(NC)"
	@go test -v ./tests/unit/...

test-integration: ## Executa apenas testes de integração
	@echo "$(BLUE)🧪 Executando testes de integração...$(NC)"
	@go test -v ./tests/integration/...

test-load-automated: ## Executa todos os testes de carga automatizados
	@echo "$(BLUE)⚡ Executando testes de carga automatizados...$(NC)"
	@go test -v ./tests/load/...

test-load-burst: ## Executa teste de burst (100 req/s)
	@echo "$(BLUE)⚡ Executando teste de burst...$(NC)"
	@go test -v ./tests/load/... -run TestLoadHighTrafficBurst

test-load-sustained: ## Executa teste de tráfego sustentado (30 req/s por 10s)
	@echo "$(BLUE)⚡ Executando teste de tráfego sustentado...$(NC)"
	@go test -v ./tests/load/... -run TestLoadSustainedHighTraffic

test-load-concurrency: ## Executa teste de concorrência massiva (50 usuários)
	@echo "$(BLUE)⚡ Executando teste de concorrência massiva...$(NC)"
	@go test -v ./tests/load/... -run TestLoadMassiveConcurrency

test-load-recovery: ## Executa teste de recuperação após bloqueio
	@echo "$(BLUE)⚡ Executando teste de recuperação...$(NC)"
	@go test -v ./tests/load/... -run TestLoadRecoveryAfterBlock

test-load-spike: ## Executa teste de picos de tráfego
	@echo "$(BLUE)⚡ Executando teste de picos de tráfego...$(NC)"
	@go test -v ./tests/load/... -run TestLoadSpikeTraffic

test-load: ## Executa testes de carga manuais com Vegeta
	@echo "$(BLUE)⚡ Executando testes de carga manuais...$(NC)"
	@echo "$(YELLOW)Testando limite por IP (10 req/s)...$(NC)"
	@echo "GET http://localhost:$(PORT)/api/v1/resource" | vegeta attack -rate=15 -duration=5s | vegeta report
	@echo ""
	@echo "$(YELLOW)Testando limite por Token (100 req/s)...$(NC)"
	@echo "GET http://localhost:$(PORT)/api/v1/resource" | vegeta attack -rate=120 -duration=5s -header="API_KEY: std_1234567890" | vegeta report

clean: ## Limpa volumes e containers
	@echo "$(BLUE)🧹 Limpando ambiente...$(NC)"
	@docker-compose down -v
	@docker system prune -f

.DEFAULT_GOAL := help
