package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVersionHeader(t *testing.T) {
	handler := VersionHeader(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if v := w.Header().Get("X-API-Version"); v != APIVersion {
		t.Errorf("version = %s, want %s", v, APIVersion)
	}
}
