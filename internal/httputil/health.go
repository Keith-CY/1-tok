package httputil

import (
	"encoding/json"
	"net/http"
	"sync"
)

// HealthChecker reports whether a dependency is healthy.
type HealthChecker interface {
	HealthCheck() error
}

// HealthCheckerFunc adapts a function to HealthChecker.
type HealthCheckerFunc func() error

func (f HealthCheckerFunc) HealthCheck() error { return f() }

// HealthEndpoints provides /livez and /readyz endpoints.
type HealthEndpoints struct {
	mu     sync.RWMutex
	checks map[string]HealthChecker
}

// NewHealthEndpoints creates health endpoints with optional dependency checks.
func NewHealthEndpoints() *HealthEndpoints {
	return &HealthEndpoints{checks: make(map[string]HealthChecker)}
}

// Register adds a named dependency check.
func (h *HealthEndpoints) Register(name string, checker HealthChecker) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.checks[name] = checker
}

// RegisterFunc adds a named dependency check from a function.
func (h *HealthEndpoints) RegisterFunc(name string, f func() error) {
	h.Register(name, HealthCheckerFunc(f))
}

// HandleLivez responds with 200 if the process is alive.
func (h *HealthEndpoints) HandleLivez(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// HandleReadyz responds with 200 if all dependencies are healthy.
func (h *HealthEndpoints) HandleReadyz(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	results := make(map[string]string, len(h.checks))
	allHealthy := true

	for name, checker := range h.checks {
		if err := checker.HealthCheck(); err != nil {
			results[name] = err.Error()
			allHealthy = false
		} else {
			results[name] = "ok"
		}
	}

	status := "ok"
	code := http.StatusOK
	if !allHealthy {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]any{"status": status, "checks": results})
}
