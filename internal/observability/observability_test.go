package observability

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestWrapHTTPRecoversPanicAndReportsToSentry(t *testing.T) {
	var (
		mu      sync.Mutex
		paths   []string
		payload [][]byte
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(body)
		mu.Lock()
		paths = append(paths, r.URL.Path)
		payload = append(payload, body)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	shutdown, err := Init(Config{
		Service:     "gateway",
		DSN:         sentryDSN(server.URL),
		Environment: "test",
		Release:     "sha-test",
		SampleRate:  1,
	})
	if err != nil {
		t.Fatalf("init observability: %v", err)
	}
	defer shutdown(2 * time.Second)

	handler := WrapHTTP("gateway", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = WithRequestTags(r, RequestTags{
			OrgID:   "buyer_1",
			UserID:  "user_1",
			OrderID: "ord_1",
			Route:   "/panic",
		})
		panic("boom")
	}))

	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	_ = Flush(2 * time.Second)

	if res.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", res.Code)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(paths) == 0 {
		t.Fatalf("expected sentry request")
	}
	if !strings.Contains(paths[0], "/api/1/envelope/") {
		t.Fatalf("expected envelope path, got %q", paths[0])
	}
	if len(payload) == 0 || !strings.Contains(string(payload[0]), "ord_1") {
		t.Fatalf("expected envelope payload to include request tags, got %q", string(payload[0]))
	}
}

func TestCaptureMessageUsesRequestTags(t *testing.T) {
	var last []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		last = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(last)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	shutdown, err := Init(Config{
		Service:     "iam",
		DSN:         sentryDSN(server.URL),
		Environment: "test",
		Release:     "sha-test",
		SampleRate:  1,
	})
	if err != nil {
		t.Fatalf("init observability: %v", err)
	}
	defer shutdown(2 * time.Second)

	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", strings.NewReader(`{"email":"user@example.com"}`))
	req = WithRequestTags(req, RequestTags{
		Route:   "/v1/sessions",
		UserID:  "usr_1",
		Subject: "email_hash",
	})

	CaptureMessage(req.Context(), "rate limit exceeded")
	_ = Flush(2 * time.Second)

	if !strings.Contains(string(last), "rate limit exceeded") {
		t.Fatalf("expected captured message payload, got %q", string(last))
	}
}

func sentryDSN(baseURL string) string {
	trimmed := strings.TrimPrefix(baseURL, "http://")
	return "http://public@" + trimmed + "/1"
}

func TestConfigFromEnvReadsSentryVariables(t *testing.T) {
	t.Setenv("SENTRY_DSN", "http://public@example.com/1")
	t.Setenv("SENTRY_ENVIRONMENT", "production")
	t.Setenv("SENTRY_RELEASE", "sha-123")
	t.Setenv("SENTRY_SAMPLE_RATE", "0.5")
	t.Setenv("SENTRY_TRACES_SAMPLE_RATE", "0.25")

	cfg := ConfigFromEnv("gateway")
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if !strings.Contains(string(raw), "sha-123") {
		t.Fatalf("expected release in config, got %s", string(raw))
	}
}

func TestCaptureError_NilHub(t *testing.T) {
	// Should not panic with nil context
	CaptureError(context.Background(), errors.New("test error"))
}

func TestInitFromEnv(t *testing.T) {
	t.Setenv("SENTRY_DSN", "")
	flush, err := InitFromEnv("test-svc")
	if err != nil {
		t.Fatal(err)
	}
	flush(0)
}

func TestEnvFloat(t *testing.T) {
	t.Setenv("TEST_FLOAT", "0.5")
	if got := envFloat("TEST_FLOAT", 1.0); got != 0.5 {
		t.Errorf("envFloat = %f, want 0.5", got)
	}
}

func TestEnvFloat_Default(t *testing.T) {
	t.Setenv("TEST_FLOAT_MISSING", "")
	if got := envFloat("TEST_FLOAT_MISSING", 0.75); got != 0.75 {
		t.Errorf("envFloat = %f, want 0.75", got)
	}
}

func TestEnvFloat_Invalid(t *testing.T) {
	t.Setenv("TEST_FLOAT_BAD", "abc")
	if got := envFloat("TEST_FLOAT_BAD", 0.5); got != 0.5 {
		t.Errorf("envFloat = %f, want 0.5 (fallback)", got)
	}
}

func TestFallback(t *testing.T) {
	if got := fallback("hello", "default"); got != "hello" {
		t.Errorf("fallback = %s, want hello", got)
	}
	if got := fallback("", "default"); got != "default" {
		t.Errorf("fallback = %s, want default", got)
	}
}
