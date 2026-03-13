package mockcarrier

import (
	"encoding/json"
	"net/http"

	"github.com/chenyu/1-tok/internal/httputil"
	"os"
	"strings"

	carrierclient "github.com/chenyu/1-tok/internal/integrations/carrier"
)

type Server struct {
	apiToken string
}

type Options struct {
	APIToken string
}

func NewServer() *Server {
	return NewServerWithOptions(Options{
		APIToken: strings.TrimSpace(os.Getenv("MOCK_CARRIER_API_TOKEN")),
	})
}

func NewServerWithOptions(options Options) *Server {
	return &Server{apiToken: strings.TrimSpace(options.APIToken)}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/healthz" {
		httputil.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "service": "mock-carrier"})
		return
	}

	if !strings.HasPrefix(r.URL.Path, "/api/v1/remote/hosts/") {
		http.NotFound(w, r)
		return
	}
	if err := s.authorize(r); err != nil {
		httputil.WriteJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		return
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 9 || parts[0] != "api" || parts[1] != "v1" || parts[2] != "remote" || parts[3] != "hosts" || parts[5] != "instances" || parts[7] != "codeagent" || parts[8] == "" {
		http.NotFound(w, r)
		return
	}

	hostID := parts[4]
	agentID := parts[6]
	action := parts[8]
	switch {
	case r.Method == http.MethodGet && action == "health":
		s.handleHealth(w, r, hostID, agentID)
	case r.Method == http.MethodGet && action == "version":
		s.handleVersion(w, r, hostID, agentID)
	case r.Method == http.MethodPost && action == "run":
		s.handleRun(w, r, hostID, agentID)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) authorize(r *http.Request) error {
	if s.apiToken == "" {
		return nil
	}
	if strings.TrimSpace(r.Header.Get("Authorization")) != "Bearer "+s.apiToken {
		return errUnauthorized
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request, _, _ string) {
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"health": carrierclient.CodeAgentHealthResult{
			Backend:       defaultString(r.URL.Query().Get("backend"), "codex"),
			WorkspaceRoot: defaultString(r.URL.Query().Get("workspaceRoot"), "/workspace"),
			Healthy:       true,
		},
	})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request, _, _ string) {
	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"version": carrierclient.CodeAgentVersionResult{
			Backend: defaultString(r.URL.Query().Get("backend"), "codex"),
			Value:   "mock-carrier/codex-1.0.0",
		},
	})
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request, hostID, agentID string) {
	var input carrierclient.CodeAgentRunInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if strings.TrimSpace(input.Backend) == "" {
		input.Backend = "codex"
	}
	if strings.TrimSpace(input.HostID) == "" {
		input.HostID = hostID
	}
	if strings.TrimSpace(input.AgentID) == "" {
		input.AgentID = agentID
	}
	if strings.TrimSpace(input.Capability) == "" {
		httputil.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": "missing capability"})
		return
	}

	httputil.WriteJSON(w, http.StatusOK, map[string]any{
		"run": carrierclient.CodeAgentRunResult{
			Backend: input.Backend,
			Result: carrierclient.CodeAgentRunOutput{
				OK:              true,
				PolicyDecision:  "allow",
				CostEstimateUSD: 0.02,
			},
		},
	})
}

var errUnauthorized = unauthorizedError("unauthorized")

type unauthorizedError string

func (e unauthorizedError) Error() string {
	return string(e)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return value
	}
	return fallback
}

