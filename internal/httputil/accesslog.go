package httputil

import (
	"log"
	"net/http"
	"time"
)

// responseCapture wraps http.ResponseWriter to capture the status code.
type responseCapture struct {
	http.ResponseWriter
	status int
}

func (rc *responseCapture) WriteHeader(code int) {
	rc.status = code
	rc.ResponseWriter.WriteHeader(code)
}

// Flush implements http.Flusher, delegating to the wrapped writer if supported.
func (rc *responseCapture) Flush() {
	if f, ok := rc.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap returns the original ResponseWriter for interface assertion chains.
func (rc *responseCapture) Unwrap() http.ResponseWriter {
	return rc.ResponseWriter
}

// AccessLog wraps handler with structured request logging.
// Each completed request is logged with method, path, status, duration,
// and remote address.
func AccessLog(service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rc := &responseCapture{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rc, r)
		log.Printf("access service=%q method=%q path=%q status=%d duration=%s remote=%q",
			service, r.Method, r.URL.Path, rc.status, time.Since(start).Round(time.Microsecond), r.RemoteAddr)
	})
}
