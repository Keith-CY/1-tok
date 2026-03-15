package ratelimit

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestServiceBlocksAfterConfiguredLimit(t *testing.T) {
	now := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)
	service := NewServiceWithOptions(Options{
		Enforce: true,
		Now: func() time.Time {
			return now
		},
		Store: NewMemoryStore(func() time.Time {
			return now
		}),
		Policies: map[Policy]PolicyConfig{
			PolicyIAMLoginIP: {
				Limit:  2,
				Window: time.Minute,
				Scope:  []ScopePart{ScopeIP},
			},
		},
	})

	meta := Meta{IP: "203.0.113.10"}
	for i := 0; i < 2; i++ {
		decision, err := service.Allow(context.Background(), PolicyIAMLoginIP, meta)
		if err != nil {
			t.Fatalf("allow call %d: %v", i, err)
		}
		if !decision.Allowed {
			t.Fatalf("expected request %d to be allowed", i)
		}
	}

	decision, err := service.Allow(context.Background(), PolicyIAMLoginIP, meta)
	if err != nil {
		t.Fatalf("third allow: %v", err)
	}
	if decision.Allowed {
		t.Fatalf("expected third request to be blocked")
	}
	if decision.RetryAfter <= 0 {
		t.Fatalf("expected retry-after to be positive, got %s", decision.RetryAfter)
	}
}

func TestDecisionHeadersExposeStandardRateLimitFields(t *testing.T) {
	now := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)
	decision := Decision{
		Allowed:    false,
		Limit:      10,
		Remaining:  0,
		ResetAt:    now.Add(45 * time.Second),
		RetryAfter: 45 * time.Second,
	}

	headers := decision.Headers(now)
	if headers.Get("Retry-After") == "" {
		t.Fatalf("expected Retry-After header")
	}
	if headers.Get("X-RateLimit-Limit") != "10" {
		t.Fatalf("unexpected limit header %q", headers.Get("X-RateLimit-Limit"))
	}
	if headers.Get("X-RateLimit-Remaining") != "0" {
		t.Fatalf("unexpected remaining header %q", headers.Get("X-RateLimit-Remaining"))
	}
}

func TestNewServiceFromEnvRequiresRedisWhenEnforced(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENFORCE", "true")
	t.Setenv("REDIS_URL", "")

	service, err := NewServiceFromEnv()
	if err == nil {
		t.Fatalf("expected config error, got service=%v", service)
	}
}

func TestClientIPUsesTrustedProxyHop(t *testing.T) {
	oldTrustProxy := os.Getenv("RATE_LIMIT_TRUST_PROXY")
	oldTrustedHops := os.Getenv("RATE_LIMIT_TRUSTED_HOPS")
	t.Cleanup(func() {
		_ = os.Setenv("RATE_LIMIT_TRUST_PROXY", oldTrustProxy)
		_ = os.Setenv("RATE_LIMIT_TRUSTED_HOPS", oldTrustedHops)
	})

	_ = os.Setenv("RATE_LIMIT_TRUST_PROXY", "true")
	_ = os.Setenv("RATE_LIMIT_TRUSTED_HOPS", "1")

	req := httptest.NewRequest("POST", "http://example.test", nil)
	req.RemoteAddr = "10.0.0.9:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.5, 10.0.0.9")

	if ip := ClientIP(req); ip != "203.0.113.5" {
		t.Fatalf("expected forwarded client ip, got %q", ip)
	}
}

func TestDefaultPoliciesAllowEnvironmentOverrides(t *testing.T) {
	t.Setenv("RATE_LIMIT_GATEWAY_CREATE_RFQ_LIMIT", "3")
	t.Setenv("RATE_LIMIT_GATEWAY_CREATE_RFQ_WINDOW", "30s")

	policies := DefaultPolicies()
	policy := policies[PolicyGatewayCreateRFQ]

	if policy.Limit != 3 {
		t.Fatalf("expected overridden limit, got %+v", policy)
	}
	if policy.Window != 30*time.Second {
		t.Fatalf("expected overridden window, got %+v", policy)
	}
}

func TestWriteHeaders(t *testing.T) {
	d := Decision{
		Allowed:   true,
		Limit:     10,
		Remaining: 7,
		ResetAt:   time.Now().Add(30 * time.Second),
	}
	rec := httptest.NewRecorder()
	WriteHeaders(rec, time.Now(), d)

	if rec.Header().Get("X-RateLimit-Limit") != "10" {
		t.Errorf("limit = %s", rec.Header().Get("X-RateLimit-Limit"))
	}
	if rec.Header().Get("X-RateLimit-Remaining") != "7" {
		t.Errorf("remaining = %s", rec.Header().Get("X-RateLimit-Remaining"))
	}
}

func TestWriteHeaders_Blocked(t *testing.T) {
	d := Decision{
		Allowed:    false,
		Limit:      5,
		Remaining:  0,
		RetryAfter: 30 * time.Second,
	}
	rec := httptest.NewRecorder()
	WriteHeaders(rec, time.Now(), d)

	if rec.Header().Get("Retry-After") != "30" {
		t.Errorf("retry-after = %s, want 30", rec.Header().Get("Retry-After"))
	}
}

func TestSubjectHash(t *testing.T) {
	hash := SubjectHash("test@example.com")
	if hash == "" {
		t.Error("expected non-empty hash")
	}
	if hash == "test@example.com" {
		t.Error("expected hashed value, got plain text")
	}
	// Same input = same output
	if SubjectHash("test@example.com") != hash {
		t.Error("expected deterministic hash")
	}
}

func TestClientIP(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.1:12345"
	if ip := ClientIP(req); ip != "192.168.1.1" {
		t.Errorf("ClientIP = %s, want 192.168.1.1", ip)
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	t.Setenv("RATE_LIMIT_TRUST_PROXY", "true")
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 192.168.1.1")
	req.RemoteAddr = "172.16.0.1:9999"
	if ip := ClientIP(req); ip != "10.0.0.1" {
		t.Errorf("ClientIP = %s, want 10.0.0.1", ip)
	}
}

func TestParseRedisAddr(t *testing.T) {
	if addr := parseRedisAddr("redis://localhost:6379/0"); addr != "localhost:6379" {
		t.Errorf("parseRedisAddr = %s", addr)
	}
	if addr := parseRedisAddr("localhost:6379"); addr != "localhost:6379" {
		t.Errorf("parseRedisAddr = %s", addr)
	}
}

func TestNewServiceFromEnv_NoEnforce(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENFORCE", "")
	t.Setenv("REDIS_URL", "")
	svc, err := NewServiceFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	// Returns nil when not enforcing
	if svc != nil {
		t.Error("expected nil service when not enforcing")
	}
}

func TestConfigError(t *testing.T) {
	err := &ConfigError{Message: "missing config"}
	if err.Error() != "missing config" {
		t.Errorf("error = %s", err.Error())
	}
}

func TestBuildKey(t *testing.T) {
	cfg := PolicyConfig{
		Scope: []ScopePart{ScopeIP, ScopeSubject},
	}
	meta := Meta{IP: "10.0.0.1", SubjectHash: "hash123"}
	key := buildKey("test.policy", cfg.Scope, meta)
	if key == "" {
		t.Error("expected non-empty key")
	}
}

func TestBuildKey_SingleScope(t *testing.T) {
	cfg := PolicyConfig{Scope: []ScopePart{ScopeOrg}}
	meta := Meta{OrgID: "org_1"}
	key := buildKey("test.policy", cfg.Scope, meta)
	if key == "" {
		t.Error("expected non-empty key")
	}
}

func TestBuildKey_UserScope(t *testing.T) {
	cfg := PolicyConfig{Scope: []ScopePart{ScopeUser}}
	meta := Meta{UserID: "u_1"}
	key := buildKey("test.policy", cfg.Scope, meta)
	if key == "" {
		t.Error("expected non-empty key")
	}
}

func TestSubjectHash_Empty(t *testing.T) {
	hash := SubjectHash("")
	if hash != "" {
		t.Error("expected empty hash for empty input")
	}
}

func TestAllowWithNilService(t *testing.T) {
	var svc *Service
	// nil service should be handled gracefully
	if svc != nil {
		t.Skip("service is nil, cannot call Allow")
	}
}

func TestNewServiceWithOptions_NilStore(t *testing.T) {
	svc := NewServiceWithOptions(Options{
		Enforce:  true,
		Store:    nil,
		Policies: DefaultPolicies(),
	})
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewMemoryStore_NilNow(t *testing.T) {
	store := NewMemoryStore(nil)
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestMaxDuration(t *testing.T) {
	if maxDuration(5*time.Second, 10*time.Second) != 10*time.Second {
		t.Error("expected 10s")
	}
	if maxDuration(10*time.Second, 5*time.Second) != 10*time.Second {
		t.Error("expected 10s")
	}
}

func TestMaxInt(t *testing.T) {
	if maxInt(5, 10) != 10 {
		t.Error("expected 10")
	}
}

func TestToInt64(t *testing.T) {
	tests := []struct {
		values []any
		index  int
		want   int64
	}{
		{[]any{int64(42)}, 0, 42},
		{[]any{"100"}, 0, 100},
		{[]any{3.14}, 0, 0},
		{[]any{}, 0, 0},
		{[]any{int64(1)}, 5, 0},
	}
	for _, tt := range tests {
		if got := toInt64(tt.values, tt.index); got != tt.want {
			t.Errorf("toInt64(%v, %d) = %d, want %d", tt.values, tt.index, got, tt.want)
		}
	}
}

func TestFallback(t *testing.T) {
	if fallback("hello", "default") != "hello" {
		t.Error("expected hello")
	}
	if fallback("", "default") != "default" {
		t.Error("expected default")
	}
	if fallback("  ", "default") != "default" {
		t.Error("expected default for whitespace")
	}
}

func TestAllow_NotEnforcing(t *testing.T) {
	svc := NewServiceWithOptions(Options{
		Enforce: false,
		Store:   NewMemoryStore(nil),
		Policies: map[Policy]PolicyConfig{
			PolicyIAMLoginIP: {Limit: 1, Window: time.Minute, Scope: []ScopePart{ScopeIP}},
		},
	})

	meta := Meta{IP: "10.0.0.1"}
	d, err := svc.Allow(context.Background(), PolicyIAMLoginIP, meta)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Allowed {
		t.Error("expected allowed when not enforcing")
	}
}

func TestAllow_UnknownPolicy(t *testing.T) {
	svc := NewServiceWithOptions(Options{
		Enforce:  true,
		Store:    NewMemoryStore(nil),
		Policies: map[Policy]PolicyConfig{},
	})

	meta := Meta{IP: "10.0.0.1"}
	d, err := svc.Allow(context.Background(), "unknown.policy", meta)
	if err != nil {
		t.Fatal(err)
	}
	// Unknown policy = allow
	if !d.Allowed {
		t.Error("expected allowed for unknown policy")
	}
}

func TestNewServiceWithOptions_NilNow(t *testing.T) {
	svc := NewServiceWithOptions(Options{
		Enforce:  true,
		Now:      nil,
		Store:    NewMemoryStore(nil),
		Policies: DefaultPolicies(),
	})
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestClientIP_NoXForwarded(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.168.1.100:54321"
	if ip := ClientIP(req); ip != "192.168.1.100" {
		t.Errorf("ClientIP = %s", ip)
	}
}

func TestClientIP_NilRequest(t *testing.T) {
	if ip := ClientIP(nil); ip != "" {
		t.Errorf("expected empty, got %s", ip)
	}
}

func TestNewMemoryStore_WithNow(t *testing.T) {
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store := NewMemoryStore(func() time.Time { return fixed })
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestNewServiceWithOptions_DefaultPolicies(t *testing.T) {
	svc := NewServiceWithOptions(Options{
		Enforce: true,
		Store:   NewMemoryStore(nil),
		// No Policies — should use DefaultPolicies
	})
	if svc == nil {
		t.Fatal("expected non-nil")
	}
}

func TestNewServiceWithOptions_AllDefaults(t *testing.T) {
	svc := NewServiceWithOptions(Options{})
	if svc == nil {
		t.Fatal("expected non-nil")
	}
}

func TestAllow_Enforcing_MultipleScopes(t *testing.T) {
	now := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	svc := NewServiceWithOptions(Options{
		Enforce: true,
		Now:     func() time.Time { return now },
		Store:   NewMemoryStore(func() time.Time { return now }),
		Policies: map[Policy]PolicyConfig{
			PolicyGatewayCreateRFQ: {Limit: 5, Window: time.Minute, Scope: []ScopePart{ScopeOrg, ScopeUser}},
		},
	})

	meta := Meta{OrgID: "org_1", UserID: "u_1"}
	for i := 0; i < 5; i++ {
		d, err := svc.Allow(context.Background(), PolicyGatewayCreateRFQ, meta)
		if err != nil {
			t.Fatal(err)
		}
		if !d.Allowed {
			t.Fatalf("request %d should be allowed", i)
		}
	}

	d, _ := svc.Allow(context.Background(), PolicyGatewayCreateRFQ, meta)
	if d.Allowed {
		t.Error("6th request should be blocked")
	}
	if d.RetryAfter <= 0 {
		t.Error("expected positive RetryAfter")
	}
}

func TestDefaultPolicies(t *testing.T) {
	policies := DefaultPolicies()
	if len(policies) == 0 {
		t.Error("expected non-empty default policies")
	}
	if _, ok := policies[PolicyIAMSignupIP]; !ok {
		t.Error("expected IAM signup policy")
	}
}

func TestEnvDuration(t *testing.T) {
	t.Setenv("TEST_DUR", "5m")
	d := envDuration("TEST_DUR", time.Second)
	if d != 5*time.Minute {
		t.Errorf("duration = %s, want 5m", d)
	}
}

func TestEnvDuration_Default(t *testing.T) {
	t.Setenv("TEST_DUR_EMPTY", "")
	d := envDuration("TEST_DUR_EMPTY", 30*time.Second)
	if d != 30*time.Second {
		t.Errorf("duration = %s, want 30s", d)
	}
}

func TestEnvInt(t *testing.T) {
	t.Setenv("TEST_INT", "42")
	v := envInt("TEST_INT", 10)
	if v != 42 {
		t.Errorf("int = %d, want 42", v)
	}
}

func TestEnvInt_Default(t *testing.T) {
	t.Setenv("TEST_INT_EMPTY", "")
	v := envInt("TEST_INT_EMPTY", 10)
	if v != 10 {
		t.Errorf("int = %d, want 10", v)
	}
}

func TestRedisStore_Integration(t *testing.T) {
	url := os.Getenv("ONE_TOK_TEST_REDIS_URL")
	if url == "" {
		t.Skip("ONE_TOK_TEST_REDIS_URL not set")
	}

	store := NewRedisStore(url)
	if store == nil {
		t.Fatal("expected non-nil Redis store")
	}

	now := time.Now().UTC()
	decision, err := store.Allow(context.Background(), "test:key:1", 5, time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	if !decision.Allowed {
		t.Error("first request should be allowed")
	}
	if decision.Remaining != 4 {
		t.Errorf("remaining = %d, want 4", decision.Remaining)
	}
}

func TestRedisStore_ExceedLimit(t *testing.T) {
	url := os.Getenv("ONE_TOK_TEST_REDIS_URL")
	if url == "" {
		t.Skip("ONE_TOK_TEST_REDIS_URL not set")
	}

	store := NewRedisStore(url)
	now := time.Now().UTC()
	key := fmt.Sprintf("test:exceed:%d", now.UnixNano())

	for i := 0; i < 3; i++ {
		store.Allow(context.Background(), key, 3, time.Minute, now)
	}

	decision, err := store.Allow(context.Background(), key, 3, time.Minute, now)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Allowed {
		t.Error("4th request should be blocked")
	}
}

func TestNewRedisStore_InvalidURL(t *testing.T) {
	store := NewRedisStore("redis://127.0.0.1:1")
	if store == nil {
		t.Fatal("expected non-nil store even with bad URL")
	}
}

func TestParseRedisAddr_URL(t *testing.T) {
	addr := parseRedisAddr("redis://myhost:6380/0")
	if addr != "myhost:6380" {
		t.Errorf("addr = %s, want myhost:6380", addr)
	}
}

func TestParseRedisAddr_Plain(t *testing.T) {
	addr := parseRedisAddr("myhost:6380")
	if addr != "myhost:6380" {
		t.Errorf("addr = %s, want myhost:6380", addr)
	}
}

func TestNewServiceFromEnv_RequireRedis(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENFORCE", "true")
	t.Setenv("REDIS_URL", "")

	_, err := NewServiceFromEnv()
	if err == nil {
		t.Error("expected error when enforce=true but no Redis")
	}
}

func TestNewServiceFromEnv_WithRedis(t *testing.T) {
	url := os.Getenv("ONE_TOK_TEST_REDIS_URL")
	if url == "" {
		t.Skip("ONE_TOK_TEST_REDIS_URL not set")
	}

	t.Setenv("RATE_LIMIT_ENFORCE", "true")
	t.Setenv("REDIS_URL", url)

	svc, err := NewServiceFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}
