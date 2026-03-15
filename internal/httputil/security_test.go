package httputil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSecurityHeaders(t *testing.T) {
	handler := SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	checks := map[string]string{
		"X-Content-Type-Options":    "nosniff",
		"X-Frame-Options":          "DENY",
		"X-XSS-Protection":         "1; mode=block",
		"Referrer-Policy":           "strict-origin-when-cross-origin",
	}
	for header, expected := range checks {
		if got := w.Header().Get(header); got != expected {
			t.Errorf("%s = %s, want %s", header, got, expected)
		}
	}
	if w.Header().Get("Strict-Transport-Security") == "" {
		t.Error("missing HSTS")
	}
	if w.Header().Get("Permissions-Policy") == "" {
		t.Error("missing Permissions-Policy")
	}
}
