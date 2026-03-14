package marketplace

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthz(t *testing.T) {
	s, err := NewServerE()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestNotFound(t *testing.T) {
	s, err := NewServerE()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestProvidersProxied(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	t.Setenv("API_GATEWAY_UPSTREAM", backend.URL)
	s, err := NewServerE()
	if err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/v1/providers", "/v1/listings", "/v1/rfqs", "/v1/rfqs/rfq_1", "/v1/orders", "/v1/orders/ord_1"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("%s: expected 200, got %d", path, rec.Code)
		}
	}
}

func TestNewServer_PanicsOnBadUpstream(t *testing.T) {
	t.Setenv("API_GATEWAY_UPSTREAM", "://invalid")
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for bad upstream")
		}
	}()
	_ = NewServer()
}
