package httputil

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLivez(t *testing.T) {
	h := NewHealthEndpoints()
	req := httptest.NewRequest("GET", "/livez", nil)
	w := httptest.NewRecorder()
	h.HandleLivez(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "ok" {
		t.Errorf("status = %s", resp["status"])
	}
}

func TestReadyz_AllHealthy(t *testing.T) {
	h := NewHealthEndpoints()
	h.RegisterFunc("db", func() error { return nil })
	h.RegisterFunc("nats", func() error { return nil })

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	h.HandleReadyz(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestReadyz_Degraded(t *testing.T) {
	h := NewHealthEndpoints()
	h.RegisterFunc("db", func() error { return errors.New("connection refused") })
	h.RegisterFunc("nats", func() error { return nil })

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	h.HandleReadyz(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "degraded" {
		t.Errorf("status = %s", resp["status"])
	}
	checks := resp["checks"].(map[string]any)
	if checks["db"] != "connection refused" {
		t.Errorf("db = %s", checks["db"])
	}
}

func TestReadyz_NoDeps(t *testing.T) {
	h := NewHealthEndpoints()

	req := httptest.NewRequest("GET", "/readyz", nil)
	w := httptest.NewRecorder()
	h.HandleReadyz(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestHealthCheckerFunc(t *testing.T) {
	f := HealthCheckerFunc(func() error { return nil })
	if err := f.HealthCheck(); err != nil {
		t.Fatal(err)
	}
}
