package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeout_Succeeds(t *testing.T) {
	handler := Timeout(1 * time.Second)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status = %d", w.Code)
	}
}

func TestTimeout_Expires(t *testing.T) {
	handler := Timeout(10 * time.Millisecond)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(1 * time.Second):
			w.WriteHeader(http.StatusOK)
		}
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503", w.Code)
	}
}

func TestDefaultRequestTimeout(t *testing.T) {
	if DefaultRequestTimeout != 30*time.Second {
		t.Errorf("default = %v", DefaultRequestTimeout)
	}
}
