package middleware

import (
	"context"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fc-pos-golang-rate-limiter/internal/limiter"
	"fc-pos-golang-rate-limiter/pkg/response"
)

// Define um tipo personalizado para chaves de contexto
type contextKey string

const (
	rateLimitInfoKey contextKey = "rate_limit_info"
)

// Cria um middleware de rate limiting
func RateLimitMiddleware(rateLimiter *limiter.RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// Extrai o endereço IP da requisição
			ip := extractIP(r)

			// Extrai a chave API do header da requisição
			apiKey := r.Header.Get("API_KEY")

			var identifier string
			var isToken bool

			// Prioridade: Token > IP (Token tem prioridade sobre IP)
			if apiKey != "" {
				identifier = apiKey
				isToken = true
			} else {
				identifier = ip
				isToken = false
			}

			// Verifica o limite de requisições
			result, err := rateLimiter.Check(ctx, identifier, isToken)
			if err != nil {
				// Loga o erro mas permite que a requisição continue
				log.Printf("Rate limiter error: %v | IP: %s | Identifier: %s | IsToken: %v",
					err, ip, identifier, isToken)
				next.ServeHTTP(w, r)
				return
			}

			// Adiciona headers de rate limit
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(result.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(result.Remaining))
			w.Header().Set("X-RateLimit-Reset", result.ResetTime.Format(time.RFC3339))

			// Verifica se a requisição é permitida
			if !result.Allowed {
				response.WriteRateLimitError(w, result.Remaining, result.ResetTime)
				return
			}

			// Adiciona informações de rate limit ao contexto para potencial uso por handlers
			ctx = context.WithValue(ctx, rateLimitInfoKey, result)
			r = r.WithContext(ctx)

			next.ServeHTTP(w, r)
		})
	}
}

// Extrai o endereço IP real da requisição, priorizando headers de proxy
func extractIP(r *http.Request) string {
	// Verifica o header X-Forwarded-For primeiro (para balanceadores de carga/proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For pode conter múltiplos IPs, pega o primeiro
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Verifica o header X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Volta para RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

func GetRateLimitInfo(ctx context.Context) *limiter.CheckResult {
	if info, ok := ctx.Value(rateLimitInfoKey).(*limiter.CheckResult); ok {
		return info
	}
	return nil
}
