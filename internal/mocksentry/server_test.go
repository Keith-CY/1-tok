package mocksentry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthz(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestPostEvent(t *testing.T) {
	s := NewServer()

	req := httptest.NewRequest(http.MethodPost, "/api/1/envelope/", strings.NewReader(`{"event":"test"}`))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}

	// Verify event stored
	req2 := httptest.NewRequest(http.MethodGet, "/events", nil)
	rec2 := httptest.NewRecorder()
	s.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec2.Code)
	}
	if !strings.Contains(rec2.Body.String(), `"count":1`) {
		t.Errorf("expected count 1, got %s", rec2.Body.String())
	}
}

func TestNotFound(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestEventsEmpty(t *testing.T) {
	s := NewServer()
	req := httptest.NewRequest(http.MethodGet, "/events", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), `"count":0`) {
		t.Errorf("expected count 0, got %s", rec.Body.String())
	}
}
