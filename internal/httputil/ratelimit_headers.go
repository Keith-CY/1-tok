package httputil

import (
	"fmt"
	"net/http"
	"time"
)

// RateLimitHeaders writes standard rate limit headers.
func RateLimitHeaders(w http.ResponseWriter, limit, remaining int, resetAt time.Time) {
	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", resetAt.Unix()))
}

// RetryAfterHeader writes a Retry-After header.
func RetryAfterHeader(w http.ResponseWriter, d time.Duration) {
	w.Header().Set("Retry-After", fmt.Sprintf("%d", int(d.Seconds())))
}
