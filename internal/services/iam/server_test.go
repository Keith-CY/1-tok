package iam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSignupLoginAndMeRoundTrip(t *testing.T) {
	server := NewServerWithOptions(Options{})

	signupBody := map[string]any{
		"email":            "owner@example.com",
		"password":         "correct horse battery staple",
		"name":             "Owner One",
		"organizationName": "Atlas Buyer",
		"organizationKind": "buyer",
	}
	signupPayload, _ := json.Marshal(signupBody)

	signupReq := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewReader(signupPayload))
	signupReq.Header.Set("Content-Type", "application/json")
	signupRes := httptest.NewRecorder()
	server.ServeHTTP(signupRes, signupReq)

	if signupRes.Code != http.StatusCreated {
		t.Fatalf("expected 201 from signup, got %d body=%s", signupRes.Code, signupRes.Body.String())
	}

	var signupResponse struct {
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"user"`
		Organization struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Kind string `json:"kind"`
		} `json:"organization"`
		Membership struct {
			Role string `json:"role"`
		} `json:"membership"`
		Session struct {
			ID        string `json:"id"`
			Token     string `json:"token"`
			ExpiresAt string `json:"expiresAt"`
		} `json:"session"`
	}
	if err := json.Unmarshal(signupRes.Body.Bytes(), &signupResponse); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}
	if signupResponse.User.Email != "owner@example.com" || signupResponse.Organization.Kind != "buyer" {
		t.Fatalf("unexpected signup response: %+v", signupResponse)
	}
	if signupResponse.Membership.Role != "org_owner" {
		t.Fatalf("expected org_owner membership, got %+v", signupResponse.Membership)
	}
	if signupResponse.Session.Token == "" || signupResponse.Session.ExpiresAt == "" {
		t.Fatalf("expected issued session token, got %+v", signupResponse.Session)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+signupResponse.Session.Token)
	meRes := httptest.NewRecorder()
	server.ServeHTTP(meRes, meReq)

	if meRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from me, got %d body=%s", meRes.Code, meRes.Body.String())
	}

	var meResponse struct {
		User struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
		Memberships []struct {
			Role         string `json:"role"`
			Organization struct {
				ID   string `json:"id"`
				Name string `json:"name"`
				Kind string `json:"kind"`
			} `json:"organization"`
		} `json:"memberships"`
	}
	if err := json.Unmarshal(meRes.Body.Bytes(), &meResponse); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if meResponse.User.ID != signupResponse.User.ID || len(meResponse.Memberships) != 1 {
		t.Fatalf("unexpected me response: %+v", meResponse)
	}
	if meResponse.Memberships[0].Organization.ID != signupResponse.Organization.ID || meResponse.Memberships[0].Role != "org_owner" {
		t.Fatalf("unexpected memberships: %+v", meResponse.Memberships)
	}

	loginBody := map[string]any{
		"email":    "owner@example.com",
		"password": "correct horse battery staple",
	}
	loginPayload, _ := json.Marshal(loginBody)
	loginReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader(loginPayload))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRes := httptest.NewRecorder()
	server.ServeHTTP(loginRes, loginReq)

	if loginRes.Code != http.StatusCreated {
		t.Fatalf("expected 201 from login, got %d body=%s", loginRes.Code, loginRes.Body.String())
	}

	var loginResponse struct {
		Session struct {
			ID    string `json:"id"`
			Token string `json:"token"`
		} `json:"session"`
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := json.Unmarshal(loginRes.Body.Bytes(), &loginResponse); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	if loginResponse.User.ID != signupResponse.User.ID {
		t.Fatalf("expected same user id, got %+v", loginResponse)
	}
	if loginResponse.Session.Token == "" || loginResponse.Session.Token == signupResponse.Session.Token {
		t.Fatalf("expected a fresh session token, got %+v", loginResponse.Session)
	}
}
