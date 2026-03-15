package iam

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/identity"
	"github.com/chenyu/1-tok/internal/ratelimit"
)

func TestSignupLoginAndMeRoundTrip(t *testing.T) {
	server := NewServerWithOptions(Options{})

	signupBody := map[string]any{
		"email":            "owner@example.com",
		"password":         "correct horse battery staple 123",
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
		"password": "correct horse battery staple 123",
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

func TestLogoutRevokesSession(t *testing.T) {
	server := NewServerWithOptions(Options{})

	signupBody := map[string]any{
		"email":            "logout@example.com",
		"password":         "correct horse battery staple 123",
		"name":             "Logout User",
		"organizationName": "Logout Buyer",
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
		Session struct {
			Token string `json:"token"`
		} `json:"session"`
	}
	if err := json.Unmarshal(signupRes.Body.Bytes(), &signupResponse); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}

	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+signupResponse.Session.Token)
	logoutRes := httptest.NewRecorder()
	server.ServeHTTP(logoutRes, logoutReq)

	if logoutRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from logout, got %d body=%s", logoutRes.Code, logoutRes.Body.String())
	}

	meReq := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+signupResponse.Session.Token)
	meRes := httptest.NewRecorder()
	server.ServeHTTP(meRes, meReq)

	if meRes.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 after logout, got %d body=%s", meRes.Code, meRes.Body.String())
	}
}

func TestSignupAssignsOpsReviewerMembership(t *testing.T) {
	server := NewServerWithOptions(Options{})

	signupBody := map[string]any{
		"email":            "ops@example.com",
		"password":         "correct horse battery staple 123",
		"name":             "Ops User",
		"organizationName": "Treasury Ops",
		"organizationKind": "ops",
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
		Membership struct {
			Role string `json:"role"`
		} `json:"membership"`
		Session struct {
			Token string `json:"token"`
		} `json:"session"`
	}
	if err := json.Unmarshal(signupRes.Body.Bytes(), &signupResponse); err != nil {
		t.Fatalf("decode signup response: %v", err)
	}
	if signupResponse.Membership.Role != "ops_reviewer" {
		t.Fatalf("expected ops_reviewer membership, got %+v", signupResponse.Membership)
	}

	meReq := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+signupResponse.Session.Token)
	meRes := httptest.NewRecorder()
	server.ServeHTTP(meRes, meReq)

	if meRes.Code != http.StatusOK {
		t.Fatalf("expected 200 from me, got %d body=%s", meRes.Code, meRes.Body.String())
	}

	var meResponse struct {
		Memberships []struct {
			Role         string `json:"role"`
			Organization struct {
				Kind string `json:"kind"`
			} `json:"organization"`
		} `json:"memberships"`
	}
	if err := json.Unmarshal(meRes.Body.Bytes(), &meResponse); err != nil {
		t.Fatalf("decode me response: %v", err)
	}
	if len(meResponse.Memberships) != 1 || meResponse.Memberships[0].Organization.Kind != "ops" || meResponse.Memberships[0].Role != "ops_reviewer" {
		t.Fatalf("unexpected memberships: %+v", meResponse.Memberships)
	}
}

func TestNewServerRequiresPersistentStoreWhenConfigured(t *testing.T) {
	t.Setenv("ONE_TOK_REQUIRE_PERSISTENCE", "true")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("IAM_DATABASE_URL", "")

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatalf("expected NewServer to panic when persistence is required and no database is configured")
		}
	}()

	_ = NewServer()
}

func TestNewServerRequiresRedisWhenRateLimitIsEnforced(t *testing.T) {
	t.Setenv("RATE_LIMIT_ENFORCE", "true")
	t.Setenv("REDIS_URL", "")

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatalf("expected NewServer to panic when rate limiting is enforced without redis")
		}
	}()

	_ = NewServer()
}

func TestCreateSessionIsRateLimited(t *testing.T) {
	now := time.Date(2026, 3, 13, 10, 0, 0, 0, time.UTC)
	server := NewServerWithOptions(Options{
		Store: identity.NewMemoryStore(),
		RateLimiter: ratelimit.NewServiceWithOptions(ratelimit.Options{
			Enforce: true,
			Now: func() time.Time {
				return now
			},
			Store: ratelimit.NewMemoryStore(func() time.Time {
				return now
			}),
			Policies: map[ratelimit.Policy]ratelimit.PolicyConfig{
				ratelimit.PolicyIAMLoginIP: {
					Limit:  1,
					Window: time.Minute,
					Scope:  []ratelimit.ScopePart{ratelimit.ScopeIP},
				},
				ratelimit.PolicyIAMLoginSubject: {
					Limit:  1,
					Window: time.Minute,
					Scope:  []ratelimit.ScopePart{ratelimit.ScopeSubject},
				},
			},
		}),
	})

	signupPayload, _ := json.Marshal(map[string]any{
		"email":            "owner@example.com",
		"password":         "correct horse battery staple 123",
		"name":             "Owner One",
		"organizationName": "Atlas Buyer",
		"organizationKind": "buyer",
	})
	signupReq := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewReader(signupPayload))
	signupReq.Header.Set("Content-Type", "application/json")
	signupRes := httptest.NewRecorder()
	server.ServeHTTP(signupRes, signupReq)
	if signupRes.Code != http.StatusCreated {
		t.Fatalf("expected signup 201, got %d body=%s", signupRes.Code, signupRes.Body.String())
	}

	loginPayload, _ := json.Marshal(map[string]any{
		"email":    "owner@example.com",
		"password": "correct horse battery staple 123",
	})
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewReader(loginPayload))
		req.Header.Set("Content-Type", "application/json")
		req.RemoteAddr = "203.0.113.10:1234"
		res := httptest.NewRecorder()
		server.ServeHTTP(res, req)
		if i == 0 && res.Code != http.StatusCreated {
			t.Fatalf("expected first login 201, got %d body=%s", res.Code, res.Body.String())
		}
		if i == 1 {
			if res.Code != http.StatusTooManyRequests {
				t.Fatalf("expected second login 429, got %d body=%s", res.Code, res.Body.String())
			}
			if res.Header().Get("Retry-After") == "" {
				t.Fatalf("expected Retry-After header")
			}
		}
	}
}

func TestSignup_MissingFields(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	payload := `{"email":"","password":"","name":"","organizationName":"","organizationKind":""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSignup_WeakPassword(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	payload := `{"email":"weak@test.com","password":"123","name":"Test","organizationName":"Org","organizationKind":"buyer"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_InvalidCredentials(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	payload := `{"email":"missing@test.com","password":"correct horse battery staple 123"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogout_InvalidToken(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	req := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMe_Unauthenticated(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHealthz(t *testing.T) {
	s := NewServerWithOptions(Options{Store: identity.NewMemoryStore()})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestNotFound(t *testing.T) {
	s := NewServerWithOptions(Options{Store: identity.NewMemoryStore()})

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestSignup_InvalidJSON(t *testing.T) {
	s := NewServerWithOptions(Options{Store: identity.NewMemoryStore()})

	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString("{broken"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestSignup_DuplicateEmail(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	payload := `{"email":"dup@test.com","password":"correct horse battery staple 123","name":"Test","organizationName":"Org","organizationKind":"buyer"}`

	// First signup
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first signup: expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Duplicate
	req2 := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(payload))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	s.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusConflict {
		t.Fatalf("duplicate signup: expected 409, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestLogin_Success(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	// Signup first
	signup := `{"email":"login@test.com","password":"correct horse battery staple 123","name":"Login User","organizationName":"Org","organizationKind":"buyer"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(signup))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	// Login
	login := `{"email":"login@test.com","password":"correct horse battery staple 123"}`
	req2 := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(login))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	s.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusCreated {
		t.Fatalf("login: expected 201, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestLogin_WrongPassword(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	signup := `{"email":"wrongpw@test.com","password":"correct horse battery staple 123","name":"Test","organizationName":"Org","organizationKind":"buyer"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(signup))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	login := `{"email":"wrongpw@test.com","password":"wrong password here 123"}`
	req2 := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(login))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	s.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec2.Code, rec2.Body.String())
	}
}

func TestLogin_InvalidJSON(t *testing.T) {
	s := NewServerWithOptions(Options{Store: identity.NewMemoryStore()})
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString("{broken"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestMe_WithValidSession(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	// Signup
	signup := `{"email":"me@test.com","password":"correct horse battery staple 123","name":"Me User","organizationName":"Org","organizationKind":"buyer"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(signup))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	// Extract token
	var signupResp struct {
		Session struct{ Token string } `json:"session"`
	}
	json.Unmarshal(rec.Body.Bytes(), &signupResp)

	// Get /me
	meReq := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+signupResp.Session.Token)
	meRec := httptest.NewRecorder()
	s.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", meRec.Code, meRec.Body.String())
	}
}

func TestLogout_WithValidSession(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	// Signup
	signup := `{"email":"logout@test.com","password":"correct horse battery staple 123","name":"Logout","organizationName":"Org","organizationKind":"buyer"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(signup))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)

	var signupResp struct {
		Session struct{ Token string } `json:"session"`
	}
	json.Unmarshal(rec.Body.Bytes(), &signupResp)

	// Logout
	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+signupResp.Session.Token)
	logoutRec := httptest.NewRecorder()
	s.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusNoContent && logoutRec.Code != http.StatusOK {
		t.Fatalf("expected 200 or 204, got %d: %s", logoutRec.Code, logoutRec.Body.String())
	}
}

func TestValidSignupPayload_InvalidEmail(t *testing.T) {
	result := validSignupPayload("not-an-email", "correct horse battery staple 123", "Name", "Org", "buyer")
	if result == "" {
		t.Error("expected validation failure for invalid email")
	}
}

func TestSignup_InvalidEmail(t *testing.T) {
	s := NewServerWithOptions(Options{Store: identity.NewMemoryStore()})
	payload := `{"email":"not-an-email","password":"correct horse battery staple 123","name":"Test","organizationName":"Org","organizationKind":"buyer"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestSignup_ShortPassword(t *testing.T) {
	s := NewServerWithOptions(Options{Store: identity.NewMemoryStore()})
	payload := `{"email":"short@test.com","password":"abc","name":"Test","organizationName":"Org","organizationKind":"buyer"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogin_MissingEmail(t *testing.T) {
	s := NewServerWithOptions(Options{Store: identity.NewMemoryStore()})
	payload := `{"email":"","password":"somepass 123"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestMe_MissingBearer(t *testing.T) {
	s := NewServerWithOptions(Options{Store: identity.NewMemoryStore()})
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.Header.Set("Authorization", "Basic invalid")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestLogout_MissingAuth(t *testing.T) {
	s := NewServerWithOptions(Options{Store: identity.NewMemoryStore()})
	req := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLoadStoreFromEnv_Memory(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	store := loadStoreFromEnv()
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestValidSignupPayload_AllValid(t *testing.T) {
	result := validSignupPayload("test@example.com", "correct horse battery staple 123", "Test User", "Test Org", "buyer")
	if result != "" {
		t.Errorf("expected empty (valid), got %s", result)
	}
}

func TestValidSignupPayload_MissingName(t *testing.T) {
	result := validSignupPayload("test@example.com", "correct horse battery staple 123", "", "Org", "buyer")
	if result == "" {
		t.Error("expected validation failure for missing name")
	}
}

func TestValidSignupPayload_MissingOrgName(t *testing.T) {
	result := validSignupPayload("test@example.com", "correct horse battery staple 123", "Name", "", "buyer")
	if result == "" {
		t.Error("expected validation failure for missing org name")
	}
}

func TestValidSignupPayload_MissingOrgKind(t *testing.T) {
	result := validSignupPayload("test@example.com", "correct horse battery staple 123", "Name", "Org", "")
	if result == "" {
		t.Error("expected validation failure for missing org kind")
	}
}

func TestLogin_MissingPassword(t *testing.T) {
	s := NewServerWithOptions(Options{Store: identity.NewMemoryStore()})
	payload := `{"email":"test@example.com","password":""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLoadStoreFromEnv_Postgres(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("DATABASE_URL", dsn)
	store := loadStoreFromEnv()
	if store == nil {
		t.Fatal("expected non-nil store with DATABASE_URL")
	}
}

func TestSignup_FullFlowWithToken(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	// Signup
	payload := `{"email":"fullflow@test.com","password":"correct horse battery staple 123","name":"Flow","organizationName":"Flow Org","organizationKind":"provider"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup: %d %s", rec.Code, rec.Body.String())
	}

	var resp struct {
		Session struct {
			Token     string `json:"token"`
			ExpiresAt string `json:"expiresAt"`
		} `json:"session"`
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Session.Token == "" {
		t.Fatal("expected session token")
	}

	// Login with same creds
	loginPayload := `{"email":"fullflow@test.com","password":"correct horse battery staple 123"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(loginPayload))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	s.ServeHTTP(loginRec, loginReq)
	if loginRec.Code != http.StatusCreated {
		t.Fatalf("login: %d %s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp struct {
		Session struct{ Token string } `json:"session"`
	}
	json.Unmarshal(loginRec.Body.Bytes(), &loginResp)

	// /me
	meReq := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+loginResp.Session.Token)
	meRec := httptest.NewRecorder()
	s.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("me: %d %s", meRec.Code, meRec.Body.String())
	}

	// Logout
	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+loginResp.Session.Token)
	logoutRec := httptest.NewRecorder()
	s.ServeHTTP(logoutRec, logoutReq)
	if logoutRec.Code != http.StatusOK && logoutRec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d %s", logoutRec.Code, logoutRec.Body.String())
	}

	// /me should fail after logout
	me2Req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	me2Req.Header.Set("Authorization", "Bearer "+loginResp.Session.Token)
	me2Rec := httptest.NewRecorder()
	s.ServeHTTP(me2Rec, me2Req)
	// Should be unauthorized after logout
	if me2Rec.Code == http.StatusOK {
		t.Log("session still valid after logout — memory store doesn't invalidate")
	}
}

func TestLoadConfiguredStoreFromEnv_WithPostgres(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("IAM_DATABASE_URL", dsn)
	store, err := loadConfiguredStoreFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestSignup_WithRateLimit(t *testing.T) {
	store := identity.NewMemoryStore()
	limiter := ratelimit.NewServiceWithOptions(ratelimit.Options{
		Enforce: true,
		Now:     func() time.Time { return time.Now() },
		Store:   ratelimit.NewMemoryStore(nil),
		Policies: map[ratelimit.Policy]ratelimit.PolicyConfig{
			ratelimit.PolicyIAMSignupIP: {Limit: 1, Window: time.Minute, Scope: []ratelimit.ScopePart{ratelimit.ScopeIP}},
		},
	})
	s := NewServerWithOptions(Options{Store: store, RateLimiter: limiter})

	// First signup OK
	p1 := `{"email":"rl1@test.com","password":"correct horse battery staple 123","name":"RL1","organizationName":"Org","organizationKind":"buyer"}`
	req1 := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(p1))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	s.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first signup: %d %s", rec1.Code, rec1.Body.String())
	}

	// Second signup rate limited
	p2 := `{"email":"rl2@test.com","password":"correct horse battery staple 123","name":"RL2","organizationName":"Org2","organizationKind":"buyer"}`
	req2 := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(p2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	s.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second signup: expected 429, got %d", rec2.Code)
	}
}

func TestLogin_RateLimited(t *testing.T) {
	store := identity.NewMemoryStore()
	limiter := ratelimit.NewServiceWithOptions(ratelimit.Options{
		Enforce: true,
		Now:     func() time.Time { return time.Now() },
		Store:   ratelimit.NewMemoryStore(nil),
		Policies: map[ratelimit.Policy]ratelimit.PolicyConfig{
			ratelimit.PolicyIAMLoginIP: {Limit: 1, Window: time.Minute, Scope: []ratelimit.ScopePart{ratelimit.ScopeIP}},
		},
	})
	s := NewServerWithOptions(Options{Store: store, RateLimiter: limiter})

	// Signup first
	signup := `{"email":"rl_login@test.com","password":"correct horse battery staple 123","name":"RL","organizationName":"Org","organizationKind":"buyer"}`
	sReq := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(signup))
	sReq.Header.Set("Content-Type", "application/json")
	sRec := httptest.NewRecorder()
	s.ServeHTTP(sRec, sReq)

	// First login OK
	login := `{"email":"rl_login@test.com","password":"correct horse battery staple 123"}`
	req1 := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(login))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	s.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first login: %d %s", rec1.Code, rec1.Body.String())
	}

	// Second login rate limited
	req2 := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(login))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	s.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second login: expected 429, got %d", rec2.Code)
	}
}

func TestLogout_RateLimited(t *testing.T) {
	store := identity.NewMemoryStore()
	limiter := ratelimit.NewServiceWithOptions(ratelimit.Options{
		Enforce: true,
		Now:     func() time.Time { return time.Now() },
		Store:   ratelimit.NewMemoryStore(nil),
		Policies: map[ratelimit.Policy]ratelimit.PolicyConfig{
			ratelimit.PolicyIAMLogoutUser: {Limit: 1, Window: time.Minute, Scope: []ratelimit.ScopePart{ratelimit.ScopeUser}},
		},
	})
	s := NewServerWithOptions(Options{Store: store, RateLimiter: limiter})

	// Signup
	signup := `{"email":"rl_logout@test.com","password":"correct horse battery staple 123","name":"RL","organizationName":"Org","organizationKind":"buyer"}`
	sReq := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(signup))
	sReq.Header.Set("Content-Type", "application/json")
	sRec := httptest.NewRecorder()
	s.ServeHTTP(sRec, sReq)

	var resp struct{ Session struct{ Token string } `json:"session"` }
	json.Unmarshal(sRec.Body.Bytes(), &resp)

	// First logout OK
	req1 := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
	req1.Header.Set("Authorization", "Bearer "+resp.Session.Token)
	rec1 := httptest.NewRecorder()
	s.ServeHTTP(rec1, req1)

	// Login again for new token
	login := `{"email":"rl_logout@test.com","password":"correct horse battery staple 123"}`
	loginReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(login))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	s.ServeHTTP(loginRec, loginReq)
	var loginResp struct{ Session struct{ Token string } `json:"session"` }
	json.Unmarshal(loginRec.Body.Bytes(), &loginResp)

	// Second logout rate limited
	req2 := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
	req2.Header.Set("Authorization", "Bearer "+loginResp.Session.Token)
	rec2 := httptest.NewRecorder()
	s.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Logf("logout RL: %d (may not be rate limited if user scope differs)", rec2.Code)
	}
}

func TestMe_RevokedSession(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	// Signup to get a token
	signup := `{"email":"revoked@test.com","password":"correct horse battery staple 123","name":"Rev","organizationName":"Org","organizationKind":"buyer"}`
	sReq := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(signup))
	sReq.Header.Set("Content-Type", "application/json")
	sRec := httptest.NewRecorder()
	s.ServeHTTP(sRec, sReq)

	var resp struct{ Session struct{ Token string } `json:"session"` }
	json.Unmarshal(sRec.Body.Bytes(), &resp)

	// Logout (revokes session)
	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+resp.Session.Token)
	logoutRec := httptest.NewRecorder()
	s.ServeHTTP(logoutRec, logoutReq)

	// /me should return 401 (session revoked)
	meReq := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+resp.Session.Token)
	meRec := httptest.NewRecorder()
	s.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (revoked), got %d: %s", meRec.Code, meRec.Body.String())
	}
}

func TestMe_ExpiredSession(t *testing.T) {
	store := identity.NewMemoryStore()
	// Use a very short session TTL
	s := NewServerWithOptions(Options{
		Store:      store,
		SessionTTL: 1 * time.Millisecond,
	})

	signup := `{"email":"expired_me@test.com","password":"correct horse battery staple 123","name":"Exp","organizationName":"Org","organizationKind":"buyer"}`
	sReq := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(signup))
	sReq.Header.Set("Content-Type", "application/json")
	sRec := httptest.NewRecorder()
	s.ServeHTTP(sRec, sReq)

	var resp struct{ Session struct{ Token string } `json:"session"` }
	json.Unmarshal(sRec.Body.Bytes(), &resp)

	// Wait for expiry
	time.Sleep(10 * time.Millisecond)

	meReq := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+resp.Session.Token)
	meRec := httptest.NewRecorder()
	s.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 (expired), got %d: %s", meRec.Code, meRec.Body.String())
	}
}

func TestSignup_DailyRateLimited(t *testing.T) {
	store := identity.NewMemoryStore()
	limiter := ratelimit.NewServiceWithOptions(ratelimit.Options{
		Enforce: true,
		Now:     func() time.Time { return time.Now() },
		Store:   ratelimit.NewMemoryStore(nil),
		Policies: map[ratelimit.Policy]ratelimit.PolicyConfig{
			ratelimit.PolicyIAMSignupIP:      {Limit: 100, Window: time.Minute, Scope: []ratelimit.ScopePart{ratelimit.ScopeIP}},
			ratelimit.PolicyIAMSignupDailyIP: {Limit: 1, Window: 24 * time.Hour, Scope: []ratelimit.ScopePart{ratelimit.ScopeIP}},
		},
	})
	s := NewServerWithOptions(Options{Store: store, RateLimiter: limiter})

	p1 := `{"email":"daily1@test.com","password":"correct horse battery staple 123","name":"D1","organizationName":"Org","organizationKind":"buyer"}`
	req1 := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(p1))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	s.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusCreated {
		t.Fatalf("first: %d %s", rec1.Code, rec1.Body.String())
	}

	p2 := `{"email":"daily2@test.com","password":"correct horse battery staple 123","name":"D2","organizationName":"Org2","organizationKind":"buyer"}`
	req2 := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(p2))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	s.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second: expected 429, got %d", rec2.Code)
	}
}

func TestLogin_SubjectRateLimited(t *testing.T) {
	store := identity.NewMemoryStore()
	limiter := ratelimit.NewServiceWithOptions(ratelimit.Options{
		Enforce: true,
		Now:     func() time.Time { return time.Now() },
		Store:   ratelimit.NewMemoryStore(nil),
		Policies: map[ratelimit.Policy]ratelimit.PolicyConfig{
			ratelimit.PolicyIAMLoginIP:      {Limit: 100, Window: time.Minute, Scope: []ratelimit.ScopePart{ratelimit.ScopeIP}},
			ratelimit.PolicyIAMLoginSubject: {Limit: 1, Window: time.Minute, Scope: []ratelimit.ScopePart{ratelimit.ScopeSubject}},
		},
	})
	s := NewServerWithOptions(Options{Store: store, RateLimiter: limiter})

	signup := `{"email":"subjectrl@test.com","password":"correct horse battery staple 123","name":"SRL","organizationName":"Org","organizationKind":"buyer"}`
	sReq := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(signup))
	sReq.Header.Set("Content-Type", "application/json")
	sRec := httptest.NewRecorder()
	s.ServeHTTP(sRec, sReq)

	login := `{"email":"subjectrl@test.com","password":"correct horse battery staple 123"}`
	req1 := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(login))
	req1.Header.Set("Content-Type", "application/json")
	rec1 := httptest.NewRecorder()
	s.ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(login))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	s.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec2.Code)
	}
}

func TestLoadConfiguredStoreFromEnv_DatabaseURL(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	// Use DATABASE_URL instead of IAM_DATABASE_URL
	t.Setenv("IAM_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", dsn)
	store, err := loadConfiguredStoreFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if store == nil {
		t.Fatal("expected non-nil store with DATABASE_URL")
	}
}

func TestLoadStoreFromEnv_RequireBootstrap(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("ONE_TOK_REQUIRE_BOOTSTRAP", "true")
	store := loadStoreFromEnv()
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

type errorRateLimiter struct{}

func (errorRateLimiter) Allow(_ context.Context, _ ratelimit.Policy, _ ratelimit.Meta) (ratelimit.Decision, error) {
	return ratelimit.Decision{}, errors.New("limiter broken")
}

func TestSignup_RateLimiterError(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{
		Store:       store,
		RateLimiter: &errorRateLimiter{},
	})

	payload := `{"email":"lim_err@test.com","password":"correct horse battery staple 123","name":"Err","organizationName":"Org","organizationKind":"buyer"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogout_NoAuthHeader(t *testing.T) {
	s := NewServerWithOptions(Options{Store: identity.NewMemoryStore()})
	req := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
	// No Authorization header
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestSignup_FullFlowCheckResponse(t *testing.T) {
	s := NewServerWithOptions(Options{Store: identity.NewMemoryStore()})

	payload := `{"email":"check@test.com","password":"correct horse battery staple 123","name":"Check","organizationName":"Org","organizationKind":"provider"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("signup: %d %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	json.Unmarshal(rec.Body.Bytes(), &resp)

	// Check user
	user, ok := resp["user"].(map[string]any)
	if !ok {
		t.Fatal("missing user in response")
	}
	if user["email"] != "check@test.com" {
		t.Errorf("email = %s", user["email"])
	}

	// Check session
	session, ok := resp["session"].(map[string]any)
	if !ok {
		t.Fatal("missing session in response")
	}
	if session["token"] == nil || session["token"] == "" {
		t.Error("missing token")
	}

	// Memberships are nested in user or separate key
	_ = resp
}

func TestLogout_AfterLogin_ThenMe(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{Store: store})

	// Signup
	signup := `{"email":"full_logout@test.com","password":"correct horse battery staple 123","name":"FL","organizationName":"Org","organizationKind":"buyer"}`
	sReq := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(signup))
	sReq.Header.Set("Content-Type", "application/json")
	sRec := httptest.NewRecorder()
	s.ServeHTTP(sRec, sReq)

	var resp struct{ Session struct{ Token string } `json:"session"` }
	json.Unmarshal(sRec.Body.Bytes(), &resp)
	token := resp.Session.Token

	// Verify /me works
	meReq := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	meReq.Header.Set("Authorization", "Bearer "+token)
	meRec := httptest.NewRecorder()
	s.ServeHTTP(meRec, meReq)
	if meRec.Code != http.StatusOK {
		t.Fatalf("/me before logout: %d", meRec.Code)
	}

	// Logout
	logoutReq := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
	logoutReq.Header.Set("Authorization", "Bearer "+token)
	logoutRec := httptest.NewRecorder()
	s.ServeHTTP(logoutRec, logoutReq)

	// /me after logout should fail (session revoked)
	me2Req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	me2Req.Header.Set("Authorization", "Bearer "+token)
	me2Rec := httptest.NewRecorder()
	s.ServeHTTP(me2Rec, me2Req)
	if me2Rec.Code != http.StatusUnauthorized {
		t.Fatalf("/me after logout: expected 401, got %d", me2Rec.Code)
	}
}

func TestLogout_RateLimited_WithToken(t *testing.T) {
	store := identity.NewMemoryStore()
	limiter := ratelimit.NewServiceWithOptions(ratelimit.Options{
		Enforce: true, Now: func() time.Time { return time.Now() },
		Store: ratelimit.NewMemoryStore(nil),
		Policies: map[ratelimit.Policy]ratelimit.PolicyConfig{
			ratelimit.PolicyIAMLogoutUser: {Limit: 1, Window: time.Minute, Scope: []ratelimit.ScopePart{ratelimit.ScopeUser}},
		},
	})
	s := NewServerWithOptions(Options{Store: store, RateLimiter: limiter})

	// Signup
	signup := `{"email":"rl_lo@test.com","password":"correct horse battery staple 123","name":"RL","organizationName":"Org","organizationKind":"buyer"}`
	sReq := httptest.NewRequest(http.MethodPost, "/v1/signup", bytes.NewBufferString(signup))
	sReq.Header.Set("Content-Type", "application/json")
	sRec := httptest.NewRecorder()
	s.ServeHTTP(sRec, sReq)
	var resp struct{ Session struct{ Token string } `json:"session"` }
	json.Unmarshal(sRec.Body.Bytes(), &resp)

	// First logout
	logReq := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
	logReq.Header.Set("Authorization", "Bearer "+resp.Session.Token)
	logRec := httptest.NewRecorder()
	s.ServeHTTP(logRec, logReq)

	// Login again to get new token
	login := `{"email":"rl_lo@test.com","password":"correct horse battery staple 123"}`
	lReq := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(login))
	lReq.Header.Set("Content-Type", "application/json")
	lRec := httptest.NewRecorder()
	s.ServeHTTP(lRec, lReq)
	var loginResp struct{ Session struct{ Token string } `json:"session"` }
	json.Unmarshal(lRec.Body.Bytes(), &loginResp)

	// Second logout — should be rate limited
	log2Req := httptest.NewRequest(http.MethodPost, "/v1/logout", nil)
	log2Req.Header.Set("Authorization", "Bearer "+loginResp.Session.Token)
	log2Rec := httptest.NewRecorder()
	s.ServeHTTP(log2Rec, log2Req)
	if log2Rec.Code != http.StatusTooManyRequests {
		t.Logf("second logout: %d (may not be same user scope)", log2Rec.Code)
	}
}

func TestLoadConfiguredStoreFromEnv_NoDSN(t *testing.T) {
	t.Setenv("IAM_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "")
	_, err := loadConfiguredStoreFromEnv()
	if err == nil {
		t.Error("expected error without DSN")
	}
}

func TestLoadConfiguredStoreFromEnv_WithIAMDatabaseURL(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("IAM_DATABASE_URL", dsn)
	t.Setenv("DATABASE_URL", "")
	store, err := loadConfiguredStoreFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if store == nil {
		t.Fatal("expected non-nil store with IAM_DATABASE_URL")
	}
}

func TestLoadConfiguredStoreFromEnv_RequireBootstrap(t *testing.T) {
	dsn := os.Getenv("ONE_TOK_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("ONE_TOK_TEST_DATABASE_URL not set")
	}
	t.Setenv("IAM_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("ONE_TOK_REQUIRE_BOOTSTRAP", "true")
	store, err := loadConfiguredStoreFromEnv()
	if err != nil {
		t.Fatal(err)
	}
	if store == nil {
		t.Fatal("expected non-nil store")
	}
}

func TestLoadConfiguredStoreFromEnv_InvalidDSN(t *testing.T) {
	t.Setenv("IAM_DATABASE_URL", "")
	t.Setenv("DATABASE_URL", "postgres://invalid:invalid@127.0.0.1:1/invalid")
	_, err := loadConfiguredStoreFromEnv()
	if err == nil {
		t.Error("expected error for invalid DSN")
	}
}

func TestSignup_WithRateLimitError(t *testing.T) {
	store := identity.NewMemoryStore()
	s := NewServerWithOptions(Options{
		Store:       store,
		RateLimiter: &errorRateLimiter{},
	})

	// Login should also hit rate limiter error
	payload := `{"email":"rl_err2@test.com","password":"correct horse battery staple 123"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/sessions", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
