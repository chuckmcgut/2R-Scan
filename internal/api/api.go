package api

import (
	"encoding/json"
	"net/http"
	"time"
)

// ScanResult represents the result of scanning a card
type ScanResult struct {
	Name     string `json:"name"`
	C        int    `json:"c"`
	Co       int    `json:"co"`
	E        int    `json:"e"`
	S        int    `json:"s"`
	Grade    float64 `json:"grade"`
	ImageURL string `json:"image_url"`
	Timestamp int64 `json:"timestamp"`
}

func (r ScanResult) MarshalJSON() ([]byte, error) {
	type Alias ScanResult
	return json.Marshal(&struct {
		*Alias
		Timestamp string `json:"timestamp"`
	}{
		Alias:    (*Alias)(&r),
		Timestamp: time.Unix(r.Timestamp, 0).Format(time.RFC3339),
	})
}

// Card represents a card in the database
type Card struct {
	Name     string `json:"name"`
	C        int    `json:"c"`
	Co       int    `json:"co"`
	E        int    `json:"e"`
	S        int    `json:"s"`
	Grade    float64 `json:"grade"`
	ImageURL string `json:"image_url"`
}

// HealthResponse for /health endpoint
type HealthResponse struct {
	Status string `json:"status"`
}

// SearchResponse for /api/v1/cards/search
type SearchResponse struct {
	Cards []Card `json:"cards"`
}

// ErrorResponse for error responses
type ErrorResponse struct {
	Error string `json:"error"`
}

// RequestLogger middleware
type RequestLogger struct{}

func (rl RequestLogger) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-Id", generateID())

		next.ServeHTTP(w, r)

		// Log could go here
	})
}

func generateID() string {
	return time.Now().Format("20060102150405") + generateRandomString(8)
}

func generateRandomString(n int) string {
	chars := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = chars[int(rand())%len(chars)]
	}
	return string(b)
}

func rand() uint32 {
	// Simplified random for demo
	return uint32(time.Now().UnixNano())
}
