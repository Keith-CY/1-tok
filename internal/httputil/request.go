package httputil

import "net/http"

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
