package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewSingleHostE_ValidURL(t *testing.T) {
	h, err := NewSingleHostE("http://localhost:8080", func(r *http.Request) {})
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if h == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestNewSingleHostE_InvalidURL(t *testing.T) {
	_, err := NewSingleHostE("://bad", func(r *http.Request) {})
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestNewSingleHostE_MissingScheme(t *testing.T) {
	_, err := NewSingleHostE("localhost:8080", func(r *http.Request) {})
	if err == nil {
		t.Fatal("expected error for URL without scheme")
	}
}

func TestNewSingleHostE_EmptyHost(t *testing.T) {
	_, err := NewSingleHostE("http://", func(r *http.Request) {})
	if err == nil {
		t.Fatal("expected error for URL without host")
	}
}

func TestNewSingleHost_PanicsOnBadURL(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for invalid URL")
		}
	}()
	NewSingleHost("://bad", func(r *http.Request) {})
}


func TestNewSingleHostE_RewritesPath(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Path", r.URL.Path)
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler, err := NewSingleHostE(backend.URL, func(req *http.Request) {
		req.URL.Path = "/rewritten" + req.URL.Path
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/original", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Path") != "/rewritten/original" {
		t.Errorf("path = %s", rec.Header().Get("X-Path"))
	}
}

func TestNewSingleHost_Success(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	handler := NewSingleHost(backend.URL, func(r *http.Request) {})
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

