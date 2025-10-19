package response

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

type ErrorResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

type SuccessResponse struct {
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
}

type RateLimitResponse struct {
	Error     string    `json:"error"`
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

func WriteError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := ErrorResponse{
		Error:     http.StatusText(statusCode),
		Message:   message,
		Timestamp: time.Now(),
	}

	_ = json.NewEncoder(w).Encode(response)
}

func WriteSuccess(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := SuccessResponse{
		Message:   message,
		Data:      data,
		Timestamp: time.Now(),
	}

	_ = json.NewEncoder(w).Encode(response)
}

func WriteRateLimitError(w http.ResponseWriter, remaining int, resetTime time.Time) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
	w.Header().Set("X-RateLimit-Reset", resetTime.Format(time.RFC3339))
	w.WriteHeader(http.StatusTooManyRequests)

	response := RateLimitResponse{
		Error:     "Too Many Requests",
		Message:   "you have reached the maximum number of requests or actions allowed within a certain time frame",
		Timestamp: time.Now(),
	}

	_ = json.NewEncoder(w).Encode(response)
}
