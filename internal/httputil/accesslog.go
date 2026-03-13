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

// AccessLog wraps handler with structured request logging.
// Each completed request is logged with method, path, status, duration,
// and remote address.
func AccessLog(service string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rc := &responseCapture{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rc, r)
		log.Printf("access service=%s method=%s path=%s status=%d duration=%s remote=%s",
			service, r.Method, r.URL.Path, rc.status, time.Since(start).Round(time.Microsecond), r.RemoteAddr)
	})
}
