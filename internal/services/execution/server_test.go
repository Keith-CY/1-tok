package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	carrierclient "github.com/chenyu/1-tok/internal/integrations/carrier"
)

type stubCarrierClient struct {
	healthInput  carrierclient.CodeAgentHealthInput
	healthResult carrierclient.CodeAgentHealthResult
	versionInput  carrierclient.CodeAgentVersionInput
	versionResult carrierclient.CodeAgentVersionResult
	runInput     carrierclient.CodeAgentRunInput
	runResult    carrierclient.CodeAgentRunResult
}

func (s *stubCarrierClient) GetCodeAgentHealth(_ context.Context, input carrierclient.CodeAgentHealthInput) (carrierclient.CodeAgentHealthResult, error) {
	s.healthInput = input
	return s.healthResult, nil
}

func (s *stubCarrierClient) GetCodeAgentVersion(_ context.Context, input carrierclient.CodeAgentVersionInput) (carrierclient.CodeAgentVersionResult, error) {
	s.versionInput = input
	return s.versionResult, nil
}

func (s *stubCarrierClient) RunCodeAgent(_ context.Context, input carrierclient.CodeAgentRunInput) (carrierclient.CodeAgentRunResult, error) {
	s.runInput = input
	return s.runResult, nil
}

func TestMilestoneReadyEventSettlesThroughGateway(t *testing.T) {
	var receivedPath string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order": map[string]any{
				"id":     "ord_1",
				"status": "running",
			},
			"ledgerEntry": map[string]any{
				"kind":        "platform_exposure",
				"amountCents": 1200,
			},
		})
	}))
	defer upstream.Close()

	server := NewServerWithUpstream(upstream.URL)
	body := map[string]any{
		"orderId":     "ord_1",
		"milestoneId": "ms_1",
		"eventType":   "milestone_ready",
		"summary":     "carrier completed milestone",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/events", bytes.NewReader(payload))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	if receivedPath != "/api/v1/orders/ord_1/milestones/ms_1/settle" {
		t.Fatalf("unexpected upstream path %s", receivedPath)
	}

	var response struct {
		ContinueAllowed bool `json:"continueAllowed"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if !response.ContinueAllowed {
		t.Fatalf("expected continueAllowed true after settlement")
	}
}

func TestUsageReportedCanPauseOrderViaGateway(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"order": map[string]any{
				"id":     "ord_1",
				"status": "awaiting_budget",
			},
		})
	}))
	defer upstream.Close()

	server := NewServerWithUpstream(upstream.URL)
	body := map[string]any{
		"orderId":     "ord_1",
		"milestoneId": "ms_1",
		"eventType":   "usage_reported",
		"usageKind":   "external_api",
		"amountCents": 150,
		"proofRef":    "evt_1",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/events", bytes.NewReader(payload))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}

	var response struct {
		ContinueAllowed   bool `json:"continueAllowed"`
		RecommendedAction struct {
			Type string `json:"type"`
		} `json:"recommendedAction"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if response.ContinueAllowed {
		t.Fatalf("expected continueAllowed false")
	}

	if response.RecommendedAction.Type != "pause" {
		t.Fatalf("expected pause action, got %s", response.RecommendedAction.Type)
	}
}

func TestCodeAgentHealthRouteUsesCarrierClient(t *testing.T) {
	stub := &stubCarrierClient{
		healthResult: carrierclient.CodeAgentHealthResult{
			Backend:       "codex",
			WorkspaceRoot: "/workspace",
			Healthy:       true,
		},
	}
	server := NewServerWithOptions(Options{
		APIUpstream: "http://127.0.0.1:8080",
		Carrier:     stub,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/carrier/codeagent/health?hostId=host_1&agentId=agent_1&backend=codex&workspaceRoot=/workspace", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	if stub.healthInput.HostID != "host_1" || stub.healthInput.AgentID != "agent_1" {
		t.Fatalf("unexpected health input: %+v", stub.healthInput)
	}

	var response struct {
		Health carrierclient.CodeAgentHealthResult `json:"health"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Health.Healthy {
		t.Fatalf("expected healthy response, got %+v", response.Health)
	}
}

func TestCodeAgentVersionRouteUsesCarrierClient(t *testing.T) {
	stub := &stubCarrierClient{
		versionResult: carrierclient.CodeAgentVersionResult{
			Backend: "opencode",
			Value:   "1.2.3",
		},
	}
	server := NewServerWithOptions(Options{
		APIUpstream: "http://127.0.0.1:8080",
		Carrier:     stub,
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/carrier/codeagent/version?hostId=host_1&agentId=agent_1&backend=opencode", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	if stub.versionInput.Backend != "opencode" {
		t.Fatalf("expected opencode backend, got %+v", stub.versionInput)
	}
}

func TestCodeAgentRunRouteUsesCarrierClient(t *testing.T) {
	stub := &stubCarrierClient{
		runResult: carrierclient.CodeAgentRunResult{
			Backend: "codex",
			Result: carrierclient.CodeAgentRunOutput{
				OK:             true,
				PolicyDecision: "allow",
			},
		},
	}
	server := NewServerWithOptions(Options{
		APIUpstream: "http://127.0.0.1:8080",
		Carrier:     stub,
	})

	body := map[string]any{
		"hostId":        "host_1",
		"agentId":       "agent_1",
		"backend":       "codex",
		"workspaceRoot": "/workspace",
		"capability":    "run_shell",
		"command":       "ls -la",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/codeagent/run", bytes.NewReader(payload))
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	if stub.runInput.Command != "ls -la" || stub.runInput.Capability != "run_shell" {
		t.Fatalf("unexpected run input: %+v", stub.runInput)
	}

	var response struct {
		Run carrierclient.CodeAgentRunResult `json:"run"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !response.Run.Result.OK || response.Run.Result.PolicyDecision != "allow" {
		t.Fatalf("unexpected run response: %+v", response.Run)
	}
}
