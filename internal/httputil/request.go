package httputil

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// DefaultMaxBodyBytes is the maximum request body size (1 MB).
// Most API endpoints expect payloads under 10 KB.
const DefaultMaxBodyBytes int64 = 1 << 20

// LimitBody wraps r.Body with http.MaxBytesReader to cap the request
// body at maxBytes. Pass 0 to use DefaultMaxBodyBytes.
func LimitBody(next http.Handler, maxBytes int64) http.Handler {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBodyBytes
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		next.ServeHTTP(w, r)
	})
}

// WriteJSON encodes payload as JSON and writes it to w with the given HTTP
// status code. The Content-Type header is set to application/json.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("httputil.WriteJSON: encode error: %v", err)
	}
}

// DefaultHTTPClient returns a shared http.Client with sensible defaults.
var DefaultHTTPClient = &http.Client{Timeout: 10 * time.Second}

// NewHTTPClient creates an http.Client with the given timeout.
func NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout}
}
