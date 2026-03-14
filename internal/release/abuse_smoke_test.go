package release

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
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

func TestAbuseConfigFromEnv(t *testing.T) {
	t.Setenv("RELEASE_ABUSE_IAM_BASE_URL", "http://iam:8081")
	t.Setenv("RELEASE_ABUSE_SENTRY_BASE_URL", "http://sentry:8092")
	cfg := AbuseConfigFromEnv()
	if cfg.IAMBaseURL != "http://iam:8081" {
		t.Errorf("IAMBaseURL = %s", cfg.IAMBaseURL)
	}
}

func TestHealthcheck_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	if err := healthcheck(context.Background(), client, srv.URL); err != nil {
		t.Fatal(err)
	}
}

func TestHealthcheck_Failure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	if err := healthcheck(context.Background(), client, srv.URL); err == nil {
		t.Error("expected error for unhealthy service")
	}
}

func TestSentryEventCount_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":5,"events":[]}`))
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	count, err := sentryEventCount(context.Background(), client, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}
}

func TestSentryEventCount_Error(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := &http.Client{Timeout: 5 * time.Second}
	_, err := sentryEventCount(context.Background(), client, srv.URL)
	if err == nil {
		t.Error("expected error")
	}
}

func TestWaitForSentryEvents_ImmediateSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":3,"events":[]}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Second}
	count, err := waitForSentryEvents(ctx, client, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestWaitForSentryEvents_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"count":0,"events":[]}`))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	client := &http.Client{Timeout: 5 * time.Second}
	_, err := waitForSentryEvents(ctx, client, srv.URL)
	if err == nil {
		t.Error("expected error on timeout")
	}
}

func TestRunExternalDependencyPreflight_AllHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := ExternalDependencyConfig{
		FiberRPCURL:           srv.URL,
		CarrierGatewayURL:    srv.URL,
		CarrierGatewayToken:  "test-token",
	}
	if err := RunExternalDependencyPreflight(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
}

func TestRunExternalDependencyPreflight_FiberDown(t *testing.T) {
	healthySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthySrv.Close()

	downSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer downSrv.Close()

	cfg := ExternalDependencyConfig{
		FiberRPCURL:          downSrv.URL,
		CarrierGatewayURL:   healthySrv.URL,
		CarrierGatewayToken: "test-token",
	}
	if err := RunExternalDependencyPreflight(context.Background(), cfg); err == nil {
		t.Error("expected error when fiber is down")
	}
}

func TestRunExternalDependencyPreflight_CarrierDown(t *testing.T) {
	healthySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthySrv.Close()

	downSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer downSrv.Close()

	cfg := ExternalDependencyConfig{
		FiberRPCURL:         healthySrv.URL,
		CarrierGatewayURL:   downSrv.URL,
		CarrierGatewayToken: "token",
	}
	if err := RunExternalDependencyPreflight(context.Background(), cfg); err == nil {
		t.Error("expected error when carrier is down")
	}
}

func TestRunExternalDependencyPreflight_BothDown(t *testing.T) {
	downSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer downSrv.Close()

	cfg := ExternalDependencyConfig{
		FiberRPCURL:         downSrv.URL,
		CarrierGatewayURL:   downSrv.URL,
		CarrierGatewayToken: "token",
	}
	if err := RunExternalDependencyPreflight(context.Background(), cfg); err == nil {
		t.Error("expected error when both are down")
	}
}
