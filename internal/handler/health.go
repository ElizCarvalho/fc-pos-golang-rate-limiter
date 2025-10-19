package handler

import (
	"net/http"
	"time"

	"fc-pos-golang-rate-limiter/pkg/response"
)

type HealthHandler struct{}

func NewHealthHandler() *HealthHandler {
	return &HealthHandler{}
}

// @Summary Verificação de saúde
// @Description Verifica se o serviço está funcionando
// @Tags health
// @Accept json
// @Produce json
// @Success 200 {object} response.SuccessResponse
// @Router /health [get]
func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	response.WriteSuccess(w, http.StatusOK, "Service is healthy", map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now(),
		"service":   "rate-limiter",
	})
}

// @Summary Recurso de exemplo
// @Description Retorna um recurso de exemplo para testar rate limiting
// @Tags resource
// @Accept json
// @Produce json
// @Success 200 {object} response.SuccessResponse
// @Header 200 {string} X-RateLimit-Limit "Limite de requisições por janela"
// @Header 200 {string} X-RateLimit-Remaining "Requisições restantes na janela atual"
// @Header 200 {string} X-RateLimit-Reset "Tempo quando o rate limit é resetado"
// @Router /api/v1/resource [get]
func (h *HealthHandler) Resource(w http.ResponseWriter, r *http.Request) {
	response.WriteSuccess(w, http.StatusOK, "Resource accessed successfully", map[string]interface{}{
		"resource":  "sample-resource",
		"timestamp": time.Now(),
		"message":   "This is a sample resource for testing rate limiting",
	})
}
