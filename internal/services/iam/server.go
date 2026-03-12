package iam

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/chenyu/1-tok/internal/identity"
	"github.com/chenyu/1-tok/internal/observability"
	"github.com/chenyu/1-tok/internal/ratelimit"
	"github.com/chenyu/1-tok/internal/runtimeconfig"
	"github.com/chenyu/1-tok/internal/store/postgres"
)

const defaultSessionTTL = 30 * 24 * time.Hour

type Server struct {
	store      identity.Store
	now        func() time.Time
	sessionTTL time.Duration
	rateLimiter ratelimit.Limiter
}

type Options struct {
	Store      identity.Store
	SessionTTL time.Duration
	Now        func() time.Time
	RateLimiter ratelimit.Limiter
}

func NewServer() *Server {
	return NewServerWithOptions(Options{
		Store: loadStoreFromEnv(),
	})
}

func NewServerWithOptions(options Options) *Server {
	if options.Store == nil {
		options.Store = loadStoreFromEnv()
	}
	if options.RateLimiter == nil {
		limiter, err := ratelimit.NewServiceFromEnv()
		if err != nil {
			panic(err)
		}
		options.RateLimiter = limiter
	}
	if options.SessionTTL <= 0 {
		options.SessionTTL = defaultSessionTTL
	}
	if options.Now == nil {
		options.Now = func() time.Time {
			return time.Now().UTC()
		}
	}

	return &Server{
		store:      options.Store,
		now:        options.Now,
		sessionTTL: options.SessionTTL,
		rateLimiter: options.RateLimiter,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "iam"})
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/v1/roles" {
		writeJSON(w, http.StatusOK, map[string]any{
			"roles": map[string][]string{
				"buyer":    {"org_owner", "procurement", "operator", "finance_viewer"},
				"provider": {"org_owner", "sales", "delivery_operator", "finance_viewer"},
				"ops":      {"ops_reviewer", "risk_admin", "finance_admin", "super_admin"},
			},
		})
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/signup" {
		s.handleSignup(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/sessions" {
		s.handleCreateSession(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/logout" {
		s.handleLogout(w, r)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/v1/me" {
		s.handleMe(w, r)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email            string `json:"email"`
		Password         string `json:"password"`
		Name             string `json:"name"`
		OrganizationName string `json:"organizationName"`
		OrganizationKind string `json:"organizationKind"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if !validSignupPayload(payload.Email, payload.Password, payload.Name, payload.OrganizationName, payload.OrganizationKind) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:   "/v1/signup",
		Subject: ratelimit.SubjectHash(payload.Email),
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyIAMSignupIP, ratelimit.Meta{
		IP: ratelimit.ClientIP(r),
	}); blocked {
		return
	}
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyIAMSignupDailyIP, ratelimit.Meta{
		IP: ratelimit.ClientIP(r),
	}); blocked {
		return
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(payload.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	actor, err := s.store.CreateSignup(identity.Signup{
		Email:            payload.Email,
		Name:             payload.Name,
		PasswordHash:     string(passwordHash),
		OrganizationName: payload.OrganizationName,
		OrganizationKind: payload.OrganizationKind,
	})
	if errors.Is(err, identity.ErrConflict) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "email already exists"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	session, token, err := s.issueSession(actor.User.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	membership := map[string]any{}
	if len(actor.Memberships) > 0 {
		membership = map[string]any{
			"role": actor.Memberships[0].Role,
		}
	}
	organization := map[string]any{}
	if len(actor.Memberships) > 0 {
		organization = map[string]any{
			"id":   actor.Memberships[0].Organization.ID,
			"name": actor.Memberships[0].Organization.Name,
			"kind": actor.Memberships[0].Organization.Kind,
		}
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"user": map[string]any{
			"id":    actor.User.ID,
			"email": actor.User.Email,
			"name":  actor.User.Name,
		},
		"organization": organization,
		"membership":   membership,
		"session": map[string]any{
			"id":        session.ID,
			"token":     token,
			"expiresAt": session.ExpiresAt.Format(time.RFC3339),
		},
	})
}

func (s *Server) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(payload.Email) == "" || strings.TrimSpace(payload.Password) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}
	subjectHash := ratelimit.SubjectHash(payload.Email)
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:   "/v1/sessions",
		Subject: subjectHash,
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyIAMLoginIP, ratelimit.Meta{
		IP: ratelimit.ClientIP(r),
	}); blocked {
		return
	}
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyIAMLoginSubject, ratelimit.Meta{
		SubjectHash: subjectHash,
	}); blocked {
		return
	}

	user, err := s.store.FindUserByEmail(payload.Email)
	if errors.Is(err, identity.ErrNotFound) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(payload.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	session, token, err := s.issueSession(user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"user": map[string]any{
			"id":    user.ID,
			"email": user.Email,
			"name":  user.Name,
		},
		"session": map[string]any{
			"id":        session.ID,
			"token":     token,
			"expiresAt": session.ExpiresAt.Format(time.RFC3339),
		},
	})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
		return
	}

	actor, err := s.store.GetAuthenticatedActorBySessionDigest(tokenDigest(token))
	if errors.Is(err, identity.ErrNotFound) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid session"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if actor.Session.RevokedAt != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session revoked"})
		return
	}
	if actor.Session.ExpiresAt.Before(s.now()) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "session expired"})
		return
	}

	memberships := make([]map[string]any, 0, len(actor.Memberships))
	for _, membership := range actor.Memberships {
		memberships = append(memberships, map[string]any{
			"role": membership.Role,
			"organization": map[string]any{
				"id":   membership.Organization.ID,
				"name": membership.Organization.Name,
				"kind": membership.Organization.Kind,
			},
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user": map[string]any{
			"id":    actor.User.ID,
			"email": actor.User.Email,
			"name":  actor.User.Name,
		},
		"memberships": memberships,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
		return
	}
	actor, err := s.store.GetAuthenticatedActorBySessionDigest(tokenDigest(token))
	if errors.Is(err, identity.ErrNotFound) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid session"})
		return
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	r = observability.WithRequestTags(r, observability.RequestTags{
		Route:  "/v1/logout",
		UserID: actor.User.ID,
	})
	if blocked := s.applyRateLimit(w, r, ratelimit.PolicyIAMLogoutUser, ratelimit.Meta{
		UserID: actor.User.ID,
	}); blocked {
		return
	}

	if err := s.store.RevokeSession(tokenDigest(token)); errors.Is(err, identity.ErrNotFound) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid session"})
		return
	} else if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"revoked": true})
}

func (s *Server) issueSession(userID string) (identity.Session, string, error) {
	token, err := randomToken()
	if err != nil {
		return identity.Session{}, "", err
	}

	expiresAt := s.now().Add(s.sessionTTL)
	session, err := s.store.CreateSession(identity.NewSession{
		UserID:      userID,
		TokenDigest: tokenDigest(token),
		ExpiresAt:   expiresAt,
	})
	if err != nil {
		return identity.Session{}, "", err
	}
	session.ExpiresAt = expiresAt
	return session, token, nil
}

func loadStoreFromEnv() identity.Store {
	store, err := loadConfiguredStoreFromEnv()
	if err != nil {
		if runtimeconfig.RequirePersistence() {
			panic(err)
		}
		return identity.NewMemoryStore()
	}
	return store
}

func loadConfiguredStoreFromEnv() (identity.Store, error) {
	dsn := strings.TrimSpace(os.Getenv("IAM_DATABASE_URL"))
	if dsn == "" {
		dsn = strings.TrimSpace(os.Getenv("DATABASE_URL"))
	}
	if dsn == "" {
		return nil, errors.New("IAM_DATABASE_URL or DATABASE_URL is required")
	}

	db, err := postgres.Open(dsn)
	if err != nil {
		return nil, fmt.Errorf("open iam store: %w", err)
	}
	if runtimeconfig.RequireBootstrappedDatabase() {
		if err := postgres.VerifyCoreSchema(db); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("verify iam store: %w", err)
		}
	} else {
		if err := postgres.Migrate(db); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("migrate iam store: %w", err)
		}
	}

	return postgres.NewIdentityStore(db), nil
}

func validSignupPayload(email, password, name, organizationName, organizationKind string) bool {
	if strings.TrimSpace(email) == "" || strings.TrimSpace(password) == "" || strings.TrimSpace(name) == "" {
		return false
	}
	if strings.TrimSpace(organizationName) == "" {
		return false
	}
	switch strings.TrimSpace(organizationKind) {
	case "buyer", "provider", "ops":
		return true
	default:
		return false
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func bearerToken(header string) (string, bool) {
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	return token, token != ""
}

func tokenDigest(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (s *Server) applyRateLimit(w http.ResponseWriter, r *http.Request, policy ratelimit.Policy, meta ratelimit.Meta) bool {
	if s.rateLimiter == nil {
		return false
	}
	decision, err := s.rateLimiter.Allow(r.Context(), policy, meta)
	if err != nil {
		observability.CaptureError(r.Context(), err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "rate limiter unavailable"})
		return true
	}
	if decision.Allowed {
		return false
	}
	ratelimit.WriteHeaders(w, s.now(), decision)
	observability.CaptureMessage(r.Context(), "rate limit exceeded")
	writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
	return true
}
