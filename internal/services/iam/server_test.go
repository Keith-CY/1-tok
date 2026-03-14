package iam

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
