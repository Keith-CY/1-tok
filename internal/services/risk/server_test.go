package risk

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
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %s", ct)
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

func TestCreditDecisionProxied(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"proxied":true,"path":"` + r.URL.Path + `"}`))
	}))
	defer backend.Close()

	t.Setenv("API_GATEWAY_UPSTREAM", backend.URL)
	s, err := NewServerE()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/credits/decision", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestDisputeProxied(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"proxied":true,"path":"` + r.URL.Path + `"}`))
	}))
	defer backend.Close()

	t.Setenv("API_GATEWAY_UPSTREAM", backend.URL)
	s, err := NewServerE()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/disputes", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
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

func TestCreditDecisionPathRewrite(t *testing.T) {
	var receivedPath string
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	t.Setenv("API_GATEWAY_UPSTREAM", backend.URL)
	s, err := NewServerE()
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/credits/decision", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if receivedPath != "/api/v1/credits/decision" {
		t.Errorf("expected path /api/v1/credits/decision, got %s", receivedPath)
	}
}
