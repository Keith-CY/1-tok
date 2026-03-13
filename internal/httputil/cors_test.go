package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS_SetsHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := CORS("https://portal.example.com", inner)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/orders", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "https://portal.example.com" {
		t.Errorf("unexpected origin: %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestCORS_PreflightReturns204(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("inner handler should not be called for preflight")
	})

	handler := CORS("*", inner)
	req := httptest.NewRequest(http.MethodOptions, "/api/v1/orders", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

func TestCORS_DefaultsToWildcard(t *testing.T) {
	handler := CORS("", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Errorf("expected wildcard, got %s", rec.Header().Get("Access-Control-Allow-Origin"))
	}
}
