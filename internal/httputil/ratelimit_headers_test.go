package httputil

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimitHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	reset := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	RateLimitHeaders(w, 100, 95, reset)

	if w.Header().Get("X-RateLimit-Limit") != "100" {
		t.Errorf("limit = %s", w.Header().Get("X-RateLimit-Limit"))
	}
	if w.Header().Get("X-RateLimit-Remaining") != "95" {
		t.Errorf("remaining = %s", w.Header().Get("X-RateLimit-Remaining"))
	}
	if w.Header().Get("X-RateLimit-Reset") == "" {
		t.Error("expected reset header")
	}
}

func TestRetryAfterHeader(t *testing.T) {
	w := httptest.NewRecorder()
	RetryAfterHeader(w, 30*time.Second)

	if w.Header().Get("Retry-After") != "30" {
		t.Errorf("retry = %s", w.Header().Get("Retry-After"))
	}
}
