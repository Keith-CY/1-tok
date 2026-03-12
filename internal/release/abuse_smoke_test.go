package release

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunAbuseSmokeTriggersRateLimitAndObservesSentryEvent(t *testing.T) {
	var (
		loginAttempts int
		sentryEvents  int
	)

	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/signup":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"session": map[string]any{"token": "sess_1"}})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions":
			loginAttempts++
			if loginAttempts >= 11 {
				sentryEvents = 1
				w.Header().Set("Retry-After", "60")
				w.Header().Set("X-RateLimit-Limit", "10")
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": "rate limit exceeded"})
				return
			}
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"session": map[string]any{"token": "sess_1"}})
		default:
			t.Fatalf("unexpected iam request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer iam.Close()

	sentry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodGet && r.URL.Path == "/events":
			_ = json.NewEncoder(w).Encode(map[string]any{"count": sentryEvents})
		default:
			t.Fatalf("unexpected sentry request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer sentry.Close()

	summary, err := RunAbuseSmoke(context.Background(), AbuseConfig{
		IAMBaseURL:    iam.URL,
		SentryBaseURL: sentry.URL,
	})
	if err != nil {
		t.Fatalf("run abuse smoke: %v", err)
	}

	if !summary.RateLimited {
		t.Fatalf("expected rate limited summary, got %+v", summary)
	}
	if summary.Attempts != 11 {
		t.Fatalf("expected 11 attempts, got %+v", summary)
	}
	if summary.SentryEventCount != 1 {
		t.Fatalf("expected sentry event count 1, got %+v", summary)
	}
}

func TestRunAbuseSmokeRequires429OnLoginFlood(t *testing.T) {
	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/signup":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"session":{"token":"sess_1"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"session":{"token":"sess_1"}}`))
		default:
			t.Fatalf("unexpected iam request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer iam.Close()

	sentry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && (r.URL.Path == "/healthz" || r.URL.Path == "/events") {
			_ = json.NewEncoder(w).Encode(map[string]any{"count": 0, "status": "ok"})
			return
		}
		t.Fatalf("unexpected sentry request %s %s", r.Method, r.URL.Path)
	}))
	defer sentry.Close()

	_, err := RunAbuseSmoke(context.Background(), AbuseConfig{
		IAMBaseURL:    iam.URL,
		SentryBaseURL: sentry.URL,
	})
	if err == nil {
		t.Fatalf("expected rate limit smoke to fail when no 429 is returned")
	}
}

func TestAbuseSmokeLoginRequestUsesForwardedIP(t *testing.T) {
	var signupForwarded string
	var loginForwarded string
	iam := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/healthz":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "ok"})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/signup":
			signupForwarded = r.Header.Get("X-Forwarded-For")
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"session":{"token":"sess_1"}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/v1/sessions":
			loginForwarded = r.Header.Get("X-Forwarded-For")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
		default:
			t.Fatalf("unexpected iam request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer iam.Close()

	sentry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"count": 1, "status": "ok"})
	}))
	defer sentry.Close()

	_, err := RunAbuseSmoke(context.Background(), AbuseConfig{
		IAMBaseURL:    iam.URL,
		SentryBaseURL: sentry.URL,
	})
	if err != nil {
		t.Fatalf("run abuse smoke: %v", err)
	}
	if signupForwarded == "" {
		t.Fatalf("expected forwarded ip header to be set on signup")
	}
	if loginForwarded == "" {
		t.Fatalf("expected forwarded ip header to be set on login")
	}
	if signupForwarded != loginForwarded {
		t.Fatalf("expected signup and login to use same forwarded ip, got %q vs %q", signupForwarded, loginForwarded)
	}
}
