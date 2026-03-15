package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestChain(t *testing.T) {
	var order []string

	m1 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m1")
			next.ServeHTTP(w, r)
		})
	}
	m2 := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			order = append(order, "m2")
			next.ServeHTTP(w, r)
		})
	}

	handler := Chain(m1, m2)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		order = append(order, "handler")
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if len(order) != 3 {
		t.Fatalf("order = %v", order)
	}
	if order[0] != "m1" || order[1] != "m2" || order[2] != "handler" {
		t.Errorf("order = %v, want [m1 m2 handler]", order)
	}
}

func TestDefaultMiddleware(t *testing.T) {
	handler := DefaultMiddleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Should have security, request-id, and version headers
	if w.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Error("missing security headers")
	}
	if w.Header().Get("X-Request-ID") == "" {
		t.Error("missing request ID")
	}
	if w.Header().Get("X-API-Version") != APIVersion {
		t.Error("missing version header")
	}
}
