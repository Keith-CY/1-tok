package ratelimit

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type Policy string
type ScopePart string

const (
	PolicyIAMSignupIP        Policy = "iam.signup.ip"
	PolicyIAMSignupDailyIP   Policy = "iam.signup.daily_ip"
	PolicyIAMLoginIP         Policy = "iam.login.ip"
	PolicyIAMLoginSubject    Policy = "iam.login.subject"
	PolicyIAMLogoutUser      Policy = "iam.logout.user"
	PolicyGatewayCreateRFQ   Policy = "gateway.create_rfq"
	PolicyGatewayCreateBid   Policy = "gateway.create_bid"
	PolicyGatewayAwardRFQ    Policy = "gateway.award_rfq"
	PolicyGatewayCreateOrder Policy = "gateway.create_order"
	PolicyGatewayCreateMsg   Policy = "gateway.create_message"
	PolicyGatewayCreateDisp  Policy = "gateway.create_dispute"
	PolicyGatewayResolveDisp Policy = "gateway.resolve_dispute"
	PolicyGatewayCreditDec   Policy = "gateway.credit_decision"
)

const (
	ScopeIP      ScopePart = "ip"
	ScopeSubject ScopePart = "subject"
	ScopeOrg     ScopePart = "org"
	ScopeUser    ScopePart = "user"
)

type Meta struct {
	IP          string
	SubjectHash string
	OrgID       string
	UserID      string
}

type PolicyConfig struct {
	Limit  int
	Window time.Duration
	Scope  []ScopePart
}

type Decision struct {
	Allowed    bool
	Limit      int
	Remaining  int
	ResetAt    time.Time
	RetryAfter time.Duration
}

type Limiter interface {
	Allow(ctx context.Context, policy Policy, meta Meta) (Decision, error)
}

type Store interface {
	Allow(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (Decision, error)
}

type Options struct {
	Enforce  bool
	Now      func() time.Time
	Store    Store
	Policies map[Policy]PolicyConfig
}

type Service struct {
	enforce  bool
	now      func() time.Time
	store    Store
	policies map[Policy]PolicyConfig
}

type MemoryStore struct {
	mu      sync.Mutex
	entries map[string]memoryEntry
	now     func() time.Time
}

type memoryEntry struct {
	count   int
	resetAt time.Time
}

type RedisStore struct {
	client redis.UniversalClient
	script *redis.Script
}

func NewServiceWithOptions(options Options) *Service {
	if options.Now == nil {
		options.Now = func() time.Time {
			return time.Now().UTC()
		}
	}
	if options.Policies == nil {
		options.Policies = DefaultPolicies()
	}
	return &Service{
		enforce:  options.Enforce,
		now:      options.Now,
		store:    options.Store,
		policies: options.Policies,
	}
}

func NewServiceFromEnv() (*Service, error) {
	enforce := envBool("RATE_LIMIT_ENFORCE")
	if !enforce {
		return nil, nil
	}

	redisURL := strings.TrimSpace(os.Getenv("REDIS_URL"))
	if redisURL == "" {
		return nil, &ConfigError{Message: "REDIS_URL is required when RATE_LIMIT_ENFORCE=true"}
	}
	return NewServiceWithOptions(Options{
		Enforce:  true,
		Store:    NewRedisStore(redisURL),
		Policies: DefaultPolicies(),
	}), nil
}

func (s *Service) Allow(ctx context.Context, policy Policy, meta Meta) (Decision, error) {
	if s == nil || !s.enforce || s.store == nil {
		return Decision{Allowed: true}, nil
	}
	config, ok := s.policies[policy]
	if !ok || config.Limit <= 0 || config.Window <= 0 {
		return Decision{Allowed: true}, nil
	}

	now := s.now()
	return s.store.Allow(ctx, buildKey(policy, config.Scope, meta), config.Limit, config.Window, now)
}

func NewMemoryStore(now func() time.Time) *MemoryStore {
	if now == nil {
		now = func() time.Time {
			return time.Now().UTC()
		}
	}
	return &MemoryStore{
		entries: make(map[string]memoryEntry),
		now:     now,
	}
}

func (s *MemoryStore) Allow(_ context.Context, key string, limit int, window time.Duration, now time.Time) (Decision, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := s.entries[key]
	if entry.resetAt.IsZero() || !entry.resetAt.After(now) {
		entry = memoryEntry{count: 0, resetAt: now.Add(window)}
	}
	entry.count++
	s.entries[key] = entry

	remaining := limit - entry.count
	if remaining < 0 {
		remaining = 0
	}
	return Decision{
		Allowed:    entry.count <= limit,
		Limit:      limit,
		Remaining:  remaining,
		ResetAt:    entry.resetAt,
		RetryAfter: maxDuration(entry.resetAt.Sub(now), 0),
	}, nil
}

func NewRedisStore(redisURL string) *RedisStore {
	client := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs: []string{parseRedisAddr(redisURL)},
	})
	if parsed, err := redis.ParseURL(redisURL); err == nil {
		client = redis.NewClient(parsed)
	}

	return &RedisStore{
		client: client,
		script: redis.NewScript(`
local current = redis.call("INCR", KEYS[1])
if current == 1 then
  redis.call("PEXPIRE", KEYS[1], ARGV[2])
end
local ttl = redis.call("PTTL", KEYS[1])
return {current, ttl}
`),
	}
}

func (s *RedisStore) Allow(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (Decision, error) {
	result, err := s.script.Run(ctx, s.client, []string{key}, limit, window.Milliseconds()).Result()
	if err != nil {
		return Decision{}, err
	}
	values, _ := result.([]any)
	current := int(toInt64(values, 0))
	ttl := time.Duration(toInt64(values, 1)) * time.Millisecond
	resetAt := now.Add(ttl)
	remaining := limit - current
	if remaining < 0 {
		remaining = 0
	}
	return Decision{
		Allowed:    current <= limit,
		Limit:      limit,
		Remaining:  remaining,
		ResetAt:    resetAt,
		RetryAfter: maxDuration(ttl, 0),
	}, nil
}

func (d Decision) Headers(now time.Time) http.Header {
	headers := http.Header{}
	if d.Limit > 0 {
		headers.Set("X-RateLimit-Limit", strconv.Itoa(d.Limit))
	}
	headers.Set("X-RateLimit-Remaining", strconv.Itoa(maxInt(d.Remaining, 0)))
	if !d.ResetAt.IsZero() {
		headers.Set("X-RateLimit-Reset", strconv.FormatInt(d.ResetAt.Unix(), 10))
	}
	if d.RetryAfter > 0 {
		headers.Set("Retry-After", strconv.Itoa(int(d.RetryAfter.Round(time.Second).Seconds())))
	} else if !d.ResetAt.IsZero() {
		headers.Set("Retry-After", strconv.Itoa(int(maxDuration(d.ResetAt.Sub(now), 0).Round(time.Second).Seconds())))
	}
	return headers
}

func WriteHeaders(w http.ResponseWriter, now time.Time, decision Decision) {
	for key, values := range decision.Headers(now) {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
}

func DefaultPolicies() map[Policy]PolicyConfig {
	return map[Policy]PolicyConfig{
		PolicyIAMSignupIP: overridePolicy(PolicyIAMSignupIP, PolicyConfig{
			Limit:  5,
			Window: time.Minute,
			Scope:  []ScopePart{ScopeIP},
		}),
		PolicyIAMSignupDailyIP: overridePolicy(PolicyIAMSignupDailyIP, PolicyConfig{
			Limit:  20,
			Window: 24 * time.Hour,
			Scope:  []ScopePart{ScopeIP},
		}),
		PolicyIAMLoginIP: overridePolicy(PolicyIAMLoginIP, PolicyConfig{
			Limit:  10,
			Window: 5 * time.Minute,
			Scope:  []ScopePart{ScopeIP},
		}),
		PolicyIAMLoginSubject: overridePolicy(PolicyIAMLoginSubject, PolicyConfig{
			Limit:  5,
			Window: 5 * time.Minute,
			Scope:  []ScopePart{ScopeSubject},
		}),
		PolicyIAMLogoutUser: overridePolicy(PolicyIAMLogoutUser, PolicyConfig{
			Limit:  30,
			Window: time.Minute,
			Scope:  []ScopePart{ScopeUser},
		}),
		PolicyGatewayCreateRFQ: overridePolicy(PolicyGatewayCreateRFQ, PolicyConfig{
			Limit:  10,
			Window: 10 * time.Minute,
			Scope:  []ScopePart{ScopeOrg, ScopeUser, ScopeIP},
		}),
		PolicyGatewayCreateBid: overridePolicy(PolicyGatewayCreateBid, PolicyConfig{
			Limit:  20,
			Window: 10 * time.Minute,
			Scope:  []ScopePart{ScopeOrg, ScopeUser, ScopeIP},
		}),
		PolicyGatewayAwardRFQ: overridePolicy(PolicyGatewayAwardRFQ, PolicyConfig{
			Limit:  10,
			Window: time.Hour,
			Scope:  []ScopePart{ScopeOrg, ScopeUser, ScopeIP},
		}),
		PolicyGatewayCreateOrder: overridePolicy(PolicyGatewayCreateOrder, PolicyConfig{
			Limit:  20,
			Window: time.Hour,
			Scope:  []ScopePart{ScopeOrg, ScopeUser, ScopeIP},
		}),
		PolicyGatewayCreateMsg: overridePolicy(PolicyGatewayCreateMsg, PolicyConfig{
			Limit:  60,
			Window: time.Minute,
			Scope:  []ScopePart{ScopeOrg, ScopeUser},
		}),
		PolicyGatewayCreateDisp: overridePolicy(PolicyGatewayCreateDisp, PolicyConfig{
			Limit:  10,
			Window: time.Hour,
			Scope:  []ScopePart{ScopeOrg, ScopeUser},
		}),
		PolicyGatewayResolveDisp: overridePolicy(PolicyGatewayResolveDisp, PolicyConfig{
			Limit:  30,
			Window: time.Hour,
			Scope:  []ScopePart{ScopeOrg, ScopeUser},
		}),
		PolicyGatewayCreditDec: overridePolicy(PolicyGatewayCreditDec, PolicyConfig{
			Limit:  30,
			Window: time.Hour,
			Scope:  []ScopePart{ScopeOrg, ScopeUser},
		}),
	}
}

func SubjectHash(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:8])
}

func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if envBool("RATE_LIMIT_TRUST_PROXY") {
		parts := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
		clean := make([]string, 0, len(parts))
		for _, part := range parts {
			value := strings.TrimSpace(part)
			if value != "" {
				clean = append(clean, value)
			}
		}
		if len(clean) > 0 {
			hops := envInt("RATE_LIMIT_TRUSTED_HOPS", 1)
			index := len(clean) - 1 - hops
			if index < 0 {
				index = 0
			}
			return clean[index]
		}
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}

func buildKey(policy Policy, scope []ScopePart, meta Meta) string {
	parts := []string{"rate", string(policy)}
	for _, part := range scope {
		switch part {
		case ScopeIP:
			parts = append(parts, "ip", fallback(meta.IP, "unknown"))
		case ScopeSubject:
			parts = append(parts, "subject", fallback(meta.SubjectHash, "unknown"))
		case ScopeOrg:
			parts = append(parts, "org", fallback(meta.OrgID, "unknown"))
		case ScopeUser:
			parts = append(parts, "user", fallback(meta.UserID, "unknown"))
		}
	}
	return strings.Join(parts, ":")
}

func parseRedisAddr(redisURL string) string {
	trimmed := strings.TrimSpace(redisURL)
	trimmed = strings.TrimPrefix(trimmed, "redis://")
	trimmed = strings.TrimPrefix(trimmed, "rediss://")
	if at := strings.LastIndex(trimmed, "@"); at >= 0 {
		trimmed = trimmed[at+1:]
	}
	if slash := strings.Index(trimmed, "/"); slash >= 0 {
		trimmed = trimmed[:slash]
	}
	return trimmed
}

func toInt64(values []any, index int) int64 {
	if index >= len(values) {
		return 0
	}
	switch value := values[index].(type) {
	case int64:
		return value
	case string:
		parsed, _ := strconv.ParseInt(value, 10, 64)
		return parsed
	default:
		return 0
	}
}

func envBool(key string) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return false
	}
	parsed, err := strconv.ParseBool(value)
	return err == nil && parsed
}

func envInt(key string, fallbackValue int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallbackValue
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallbackValue
	}
	return parsed
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return strings.TrimSpace(value)
}

func maxInt(value, min int) int {
	if value < min {
		return min
	}
	return value
}

func maxDuration(value, min time.Duration) time.Duration {
	if value < min {
		return min
	}
	return value
}

func overridePolicy(policy Policy, config PolicyConfig) PolicyConfig {
	prefix := "RATE_LIMIT_" + strings.ToUpper(strings.NewReplacer(".", "_").Replace(string(policy)))
	config.Limit = envInt(prefix+"_LIMIT", config.Limit)
	config.Window = envDuration(prefix+"_WINDOW", config.Window)
	return config
}

func envDuration(key string, fallbackValue time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallbackValue
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallbackValue
	}
	return parsed
}
