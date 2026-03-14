package iam

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestNewClientFromEnv_Noop(t *testing.T) {
	os.Unsetenv("IAM_UPSTREAM")

	c := NewClientFromEnv()
	if !IsNoop(c) {
		t.Fatal("expected NoopClient when IAM_UPSTREAM is not set")
	}
}

func TestNewClientFromEnv_Configured(t *testing.T) {
	t.Setenv("IAM_UPSTREAM", "http://iam:8081")

	c := NewClientFromEnv()
	if IsNoop(c) {
		t.Fatal("expected HTTPClient when IAM_UPSTREAM is set")
	}
}

func TestNoopClient_ReturnsErrNotConfigured(t *testing.T) {
	c := NoopClient{}
	_, err := c.GetActor(context.Background(), "token")
	if !errors.Is(err, ErrNotConfigured) {
		t.Errorf("expected ErrNotConfigured, got %v", err)
	}
}

func TestIsNoop(t *testing.T) {
	if !IsNoop(NoopClient{}) {
		t.Error("IsNoop(NoopClient{}) = false")
	}
	if IsNoop(NewClient("http://localhost")) {
		t.Error("IsNoop(HTTPClient) = true")
	}
}

func TestHTTPClient_GetActor_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/me" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected auth header %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"user": map[string]string{"id": "u_1", "email": "a@b.c", "name": "Test"},
			"memberships": []map[string]any{
				{"role": "buyer", "organization": map[string]string{"id": "org_1", "name": "Acme", "kind": "buyer"}},
			},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	actor, err := c.GetActor(context.Background(), "test-token")
	if err != nil {
		t.Fatal(err)
	}
	if actor.UserID != "u_1" {
		t.Errorf("user id = %s, want u_1", actor.UserID)
	}
	if len(actor.Memberships) != 1 {
		t.Fatalf("memberships len = %d, want 1", len(actor.Memberships))
	}
	if actor.Memberships[0].OrganizationKind != "buyer" {
		t.Errorf("org kind = %s, want buyer", actor.Memberships[0].OrganizationKind)
	}
}

func TestHTTPClient_GetActor_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetActor(context.Background(), "bad-token")
	if !errors.Is(err, ErrUnauthorized) {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestHTTPClient_GetActor_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetActor(context.Background(), "token")
	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestHTTPClient_GetActor_EmptyBaseURL(t *testing.T) {
	c := &HTTPClient{}
	_, err := c.GetActor(context.Background(), "token")
	if !errors.Is(err, ErrNotConfigured) {
		t.Errorf("expected ErrNotConfigured, got %v", err)
	}
}

func TestHTTPClient_GetActor_MalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{broken json`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetActor(context.Background(), "token")
	if err == nil {
		t.Error("expected error for malformed JSON response")
	}
}
