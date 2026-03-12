package ratelimit

import (
	"context"
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
