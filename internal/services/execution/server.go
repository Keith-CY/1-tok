package execution

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/chenyu/1-tok/internal/core"
	carrierclient "github.com/chenyu/1-tok/internal/integrations/carrier"
	"github.com/chenyu/1-tok/internal/runtimeconfig"
	"github.com/chenyu/1-tok/internal/serviceauth"
)

type Server struct {
	client        *http.Client
	upstream      string
	carrier       carrierclient.CodeAgentClient
	inboundTokens serviceauth.TokenSet
	gatewayToken  string
}

type Options struct {
	APIUpstream   string
	Carrier       carrierclient.CodeAgentClient
	InboundToken  string
	InboundTokens serviceauth.TokenSet
	GatewayToken  string
}

type carrierEventPayload struct {
	OrderID     string               `json:"orderId"`
	MilestoneID string               `json:"milestoneId"`
	EventType   string               `json:"eventType"`
	UsageKind   core.UsageChargeKind `json:"usageKind"`
	AmountCents int64                `json:"amountCents"`
	ProofRef    string               `json:"proofRef"`
	Summary     string               `json:"summary"`
}

func NewServer() *Server {
	return NewServerWithOptions(Options{
		APIUpstream: runtimeconfig.APIGatewayUpstream(),
		Carrier:     carrierclient.NewClientFromEnv(),
	})
}

func NewServerWithUpstream(upstream string) *Server {
	return NewServerWithOptions(Options{
		APIUpstream: upstream,
		Carrier:     carrierclient.NewClientFromEnv(),
	})
}

func NewServerWithOptions(options Options) *Server {
	if options.APIUpstream == "" {
		if runtimeconfig.RequireExternalDependencies() && strings.TrimSpace(os.Getenv("API_GATEWAY_UPSTREAM")) == "" {
			panic("API_GATEWAY_UPSTREAM is required when ONE_TOK_REQUIRE_EXTERNALS=true")
		}
		options.APIUpstream = runtimeconfig.APIGatewayUpstream()
	}
	if options.Carrier == nil {
		if runtimeconfig.RequireExternalDependencies() {
			if strings.TrimSpace(os.Getenv("CARRIER_GATEWAY_URL")) == "" {
				panic("CARRIER_GATEWAY_URL is required when ONE_TOK_REQUIRE_EXTERNALS=true")
			}
			if strings.TrimSpace(os.Getenv("CARRIER_GATEWAY_API_TOKEN")) == "" {
				panic("CARRIER_GATEWAY_API_TOKEN is required when ONE_TOK_REQUIRE_EXTERNALS=true")
			}
		}
		options.Carrier = carrierclient.NewClientFromEnv()
	}
	if options.InboundTokens.Empty() {
		if options.InboundToken != "" {
			options.InboundTokens = serviceauth.NewTokenSet(options.InboundToken)
		} else {
			options.InboundTokens = serviceauth.FromEnv("EXECUTION_EVENT_TOKENS", "EXECUTION_EVENT_TOKEN")
		}
	}
	if options.GatewayToken == "" {
		options.GatewayToken = serviceauth.FromEnv("EXECUTION_GATEWAY_TOKENS", "EXECUTION_GATEWAY_TOKEN").Primary()
	}
	if runtimeconfig.RequireExternalDependencies() {
		if options.InboundTokens.Empty() {
			panic("EXECUTION_EVENT_TOKEN or EXECUTION_EVENT_TOKENS is required when ONE_TOK_REQUIRE_EXTERNALS=true")
		}
		if strings.TrimSpace(options.GatewayToken) == "" {
			panic("EXECUTION_GATEWAY_TOKEN or EXECUTION_GATEWAY_TOKENS is required when ONE_TOK_REQUIRE_EXTERNALS=true")
		}
	}

	return &Server{
		client:        &http.Client{Timeout: 5 * time.Second},
		upstream:      options.APIUpstream,
		carrier:       options.Carrier,
		inboundTokens: options.InboundTokens,
		gatewayToken:  options.GatewayToken,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/healthz" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok","service":"execution"}`))
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/v1/carrier/codeagent/health" {
		s.handleCodeAgentHealth(w, r)
		return
	}

	if r.Method == http.MethodGet && r.URL.Path == "/v1/carrier/codeagent/version" {
		s.handleCodeAgentVersion(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/carrier/codeagent/run" {
		s.handleCodeAgentRun(w, r)
		return
	}

	if r.Method == http.MethodPost && r.URL.Path == "/v1/carrier/events" {
		if !s.inboundTokens.MatchesRequest(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}

		var payload carrierEventPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}

		response, err := s.handleCarrierEvent(payload)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleCodeAgentHealth(w http.ResponseWriter, r *http.Request) {
	input := carrierclient.CodeAgentHealthInput{
		HostID:        r.URL.Query().Get("hostId"),
		AgentID:       r.URL.Query().Get("agentId"),
		Backend:       r.URL.Query().Get("backend"),
		WorkspaceRoot: r.URL.Query().Get("workspaceRoot"),
	}
	if input.HostID == "" || input.AgentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}

	result, err := s.carrier.GetCodeAgentHealth(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"health": result})
}

func (s *Server) handleCodeAgentVersion(w http.ResponseWriter, r *http.Request) {
	input := carrierclient.CodeAgentVersionInput{
		HostID:  r.URL.Query().Get("hostId"),
		AgentID: r.URL.Query().Get("agentId"),
		Backend: r.URL.Query().Get("backend"),
	}
	if input.HostID == "" || input.AgentID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}

	result, err := s.carrier.GetCodeAgentVersion(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"version": result})
}

func (s *Server) handleCodeAgentRun(w http.ResponseWriter, r *http.Request) {
	var input carrierclient.CodeAgentRunInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if input.HostID == "" || input.AgentID == "" || input.Capability == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing required fields"})
		return
	}

	result, err := s.carrier.RunCodeAgent(r.Context(), input)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": result})
}

func actionForEvent(eventType string) string {
	switch eventType {
	case "budget_low":
		return "pause"
	case "milestone_ready":
		return "settle"
	case "usage_reported":
		return "continue"
	default:
		return "continue"
	}
}

func (s *Server) handleCarrierEvent(payload carrierEventPayload) (map[string]any, error) {
	result := map[string]any{
		"accepted": true,
		"recommendedAction": map[string]any{
			"type":      actionForEvent(payload.EventType),
			"timestamp": time.Now().UTC(),
		},
	}

	switch payload.EventType {
	case "milestone_ready":
		var gatewayResponse map[string]any
		if err := s.postJSON(
			fmt.Sprintf("/api/v1/orders/%s/milestones/%s/settle", payload.OrderID, payload.MilestoneID),
			map[string]any{
				"milestoneId": payload.MilestoneID,
				"summary":     payload.Summary,
				"source":      "carrier",
			},
			&gatewayResponse,
		); err != nil {
			return nil, err
		}

		result["continueAllowed"] = true
		result["result"] = gatewayResponse
		return result, nil
	case "usage_reported":
		var gatewayResponse struct {
			Order struct {
				Status string `json:"status"`
			} `json:"order"`
		}
		if err := s.postJSON(
			fmt.Sprintf("/api/v1/orders/%s/milestones/%s/usage", payload.OrderID, payload.MilestoneID),
			map[string]any{
				"kind":        payload.UsageKind,
				"amountCents": payload.AmountCents,
				"proofRef":    payload.ProofRef,
			},
			&gatewayResponse,
		); err != nil {
			return nil, err
		}

		continueAllowed := gatewayResponse.Order.Status != string(core.OrderStatusAwaitingBudget)
		result["continueAllowed"] = continueAllowed
		if !continueAllowed {
			result["recommendedAction"] = map[string]any{
				"type":      "pause",
				"timestamp": time.Now().UTC(),
			}
		}
		result["result"] = gatewayResponse
		return result, nil
	case "budget_low":
		result["continueAllowed"] = false
		return result, nil
	default:
		result["continueAllowed"] = true
		return result, nil
	}
}

func (s *Server) postJSON(path string, payload any, target any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPost, s.upstream+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if strings.TrimSpace(s.gatewayToken) != "" {
		req.Header.Set(serviceauth.HeaderName, strings.TrimSpace(s.gatewayToken))
	}

	res, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= http.StatusBadRequest {
		responseBody, _ := io.ReadAll(res.Body)
		return fmt.Errorf("gateway returned %d: %s", res.StatusCode, string(responseBody))
	}

	if target == nil {
		return nil
	}

	return json.NewDecoder(res.Body).Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

