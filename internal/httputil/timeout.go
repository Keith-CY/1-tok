package httputil

import (
	"net/http"
	"time"
)

// DefaultRequestTimeout is the default request timeout.
const DefaultRequestTimeout = 30 * time.Second

// Timeout returns middleware that applies a request timeout.
func Timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.TimeoutHandler(next, d, `{"error":"request timeout"}`).ServeHTTP(w, r)
		})
	}
}
