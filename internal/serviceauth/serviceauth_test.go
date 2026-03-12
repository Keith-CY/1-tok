package serviceauth

import (
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
