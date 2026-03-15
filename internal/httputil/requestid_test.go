package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequestID_Generated(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := GetRequestID(r.Context())
		if id == "" {
			t.Error("expected request ID in context")
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if id := w.Header().Get("X-Request-ID"); id == "" {
		t.Error("expected X-Request-ID header")
	}
}

func TestRequestID_Preserved(t *testing.T) {
	handler := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("X-Request-ID", "custom-123")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if id := w.Header().Get("X-Request-ID"); id != "custom-123" {
		t.Errorf("expected custom-123, got %s", id)
	}
}

func TestGetRequestID_Missing(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	if id := GetRequestID(req.Context()); id != "" {
		t.Errorf("expected empty, got %s", id)
	}
}
