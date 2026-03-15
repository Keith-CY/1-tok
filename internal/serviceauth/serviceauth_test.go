package serviceauth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTokenSetMatchesCommaSeparatedRotatedTokens(t *testing.T) {
	tokens := NewTokenSet(" current-token , next-token ")

	if tokens.Primary() != "current-token" {
		t.Fatalf("expected primary token to be current-token, got %q", tokens.Primary())
	}

	req := httptest.NewRequest("POST", "/internal", nil)
	req.Header.Set(HeaderName, "next-token")

	if !tokens.MatchesRequest(req) {
		t.Fatalf("expected rotated token to match request")
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("TEST_MULTI_TOKENS", "token1,token2")
	t.Setenv("TEST_SINGLE_TOKEN", "token3")

	ts := FromEnv("TEST_MULTI_TOKENS", "TEST_SINGLE_TOKEN")
	if ts.Empty() {
		t.Error("expected non-empty token set")
	}
	if ts.Primary() != "token1" {
		t.Errorf("primary = %s, want token1", ts.Primary())
	}
}

func TestFromEnv_Empty(t *testing.T) {
	t.Setenv("TEST_MULTI_TOKENS", "")
	t.Setenv("TEST_SINGLE_TOKEN", "")

	ts := FromEnv("TEST_MULTI_TOKENS", "TEST_SINGLE_TOKEN")
	if !ts.Empty() {
		t.Error("expected empty token set")
	}
}

func TestMatchesRequest_Package(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(HeaderName, "valid-token")

	if !MatchesRequest(req, "valid-token", "other-token") {
		t.Error("expected match")
	}
	if MatchesRequest(req, "wrong-token") {
		t.Error("expected no match")
	}
}

func TestMatchesRequest_EmptyExpected(t *testing.T) {
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	// Empty expected = no auth required = always matches
	if !MatchesRequest(req) {
		t.Error("empty expected should match any request")
	}
}

func TestTokenSet_MatchesRequest_EmptySet(t *testing.T) {
	ts := NewTokenSet()
	req, _ := http.NewRequest(http.MethodGet, "/", nil)
	if !ts.MatchesRequest(req) {
		t.Error("empty set should match any request")
	}
}
