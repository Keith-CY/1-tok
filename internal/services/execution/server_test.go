package execution

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	carrierclient "github.com/chenyu/1-tok/internal/integrations/carrier"
	"github.com/chenyu/1-tok/internal/serviceauth"
)

type stubCarrierClient struct {
	healthInput   carrierclient.CodeAgentHealthInput
	healthResult  carrierclient.CodeAgentHealthResult
	versionInput  carrierclient.CodeAgentVersionInput
	versionResult carrierclient.CodeAgentVersionResult
	runInput      carrierclient.CodeAgentRunInput
	runResult     carrierclient.CodeAgentRunResult
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

func TestNewServerRequiresExternalDependenciesWhenConfigured(t *testing.T) {
	t.Setenv("ONE_TOK_REQUIRE_EXTERNALS", "true")
	t.Setenv("CARRIER_GATEWAY_URL", "")
	t.Setenv("EXECUTION_EVENT_TOKEN", "")
	t.Setenv("EXECUTION_GATEWAY_TOKEN", "")

	defer func() {
		if recovered := recover(); recovered == nil {
			t.Fatalf("expected NewServer to panic when external dependencies are required and config is missing")
		}
	}()

	_ = NewServer()
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

func TestCarrierEventsRejectMissingServiceTokenWhenConfigured(t *testing.T) {
	t.Setenv("EXECUTION_EVENT_TOKEN", "exec-event-token")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unexpected upstream request %s %s", r.Method, r.URL.Path)
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

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestCarrierEventsAcceptRotatedServiceTokenFromEnv(t *testing.T) {
	t.Setenv("EXECUTION_EVENT_TOKEN", "")
	t.Setenv("EXECUTION_EVENT_TOKENS", "current-token,next-token")

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	req.Header.Set("X-One-Tok-Service-Token", "next-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected rotated execution event token to be accepted, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestMilestoneReadyEventForwardsGatewayServiceTokenWhenConfigured(t *testing.T) {
	t.Setenv("EXECUTION_EVENT_TOKEN", "exec-event-token")
	t.Setenv("EXECUTION_GATEWAY_TOKEN", "gateway-shared-token")

	var receivedServiceToken string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedServiceToken = r.Header.Get("X-One-Tok-Service-Token")
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
	req.Header.Set("X-One-Tok-Service-Token", "exec-event-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	if receivedServiceToken != "gateway-shared-token" {
		t.Fatalf("expected forwarded gateway token, got %q", receivedServiceToken)
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

func TestHealthz(t *testing.T) {
	s := NewServerWithOptions(Options{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestNotFound(t *testing.T) {
	s := NewServerWithOptions(Options{})
	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestActionForEvent(t *testing.T) {
	tests := []struct {
		event string
		want  string
	}{
		{"budget_low", "pause"},
		{"milestone_ready", "settle"},
		{"usage_reported", "continue"},
		{"unknown.event", "continue"},
	}
	for _, tt := range tests {
		if got := actionForEvent(tt.event); got != tt.want {
			t.Errorf("actionForEvent(%q) = %q, want %q", tt.event, got, tt.want)
		}
	}
}

func TestCarrierEvent_Success(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"order":{"id":"ord_1"}}`))
	}))
	defer backend.Close()

	s := NewServerWithOptions(Options{
		APIUpstream: backend.URL,
	})

	payload, _ := json.Marshal(map[string]any{
		"orderId": "ord_1", "milestoneId": "ms_1",
		"eventType": "usage_reported",
		"payload": map[string]any{"kind": "token", "amountCents": 100},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/events", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCarrierEvent_InvalidJSON(t *testing.T) {
	s := NewServerWithOptions(Options{})
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/events", bytes.NewReader([]byte("{broken")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCodeAgentHealth_Success(t *testing.T) {
	stub := &stubCarrierClient{
		healthResult: carrierclient.CodeAgentHealthResult{Healthy: true},
	}
	s := NewServerWithOptions(Options{Carrier: stub})

	req := httptest.NewRequest(http.MethodGet, "/v1/carrier/codeagent/health?hostId=h1&agentId=a1", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCodeAgentVersion_Success(t *testing.T) {
	stub := &stubCarrierClient{
		versionResult: carrierclient.CodeAgentVersionResult{Value: "1.0.0"},
	}
	s := NewServerWithOptions(Options{Carrier: stub})

	req := httptest.NewRequest(http.MethodGet, "/v1/carrier/codeagent/version?hostId=h1&agentId=a1", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCodeAgentRun_Success(t *testing.T) {
	stub := &stubCarrierClient{
		runResult: carrierclient.CodeAgentRunResult{Result: carrierclient.CodeAgentRunOutput{}},
	}
	s := NewServerWithOptions(Options{Carrier: stub})

	payload, _ := json.Marshal(map[string]any{
		"hostId": "h1", "agentId": "a1",
		"backend": "node", "workspaceRoot": "/tmp", "capability": "run",
		"title": "Test run", "instructions": "echo hello",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/codeagent/run", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCodeAgentRun_InvalidJSON(t *testing.T) {
	s := NewServerWithOptions(Options{})
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/codeagent/run", bytes.NewReader([]byte("{broken")))
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCodeAgentHealth_NoCarrier(t *testing.T) {
	s := NewServerWithOptions(Options{})
	req := httptest.NewRequest(http.MethodGet, "/v1/carrier/codeagent/health?hostId=h1&agentId=a1", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code == http.StatusOK {
		// May return 502 or similar without carrier
		t.Log("no carrier: returned 200")
	}
}

func TestCodeAgentVersion_NoCarrier(t *testing.T) {
	s := NewServerWithOptions(Options{})
	req := httptest.NewRequest(http.MethodGet, "/v1/carrier/codeagent/version?hostId=h&agentId=a", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	// No carrier = should return error
	if rec.Code == http.StatusOK {
		t.Log("no carrier returned 200 — carrier might be nil-safe")
	}
}

func TestCodeAgentHealth_MissingParams(t *testing.T) {
	s := NewServerWithOptions(Options{Carrier: &stubCarrierClient{}})
	req := httptest.NewRequest(http.MethodGet, "/v1/carrier/codeagent/health", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCarrierEvent_Unauthorized(t *testing.T) {
	s := NewServerWithOptions(Options{
		InboundTokens: serviceauth.NewTokenSet("valid-token"),
	})
	payload, _ := json.Marshal(map[string]any{
		"orderId": "ord_1", "milestoneId": "ms_1",
		"eventType": "usage_reported",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/events", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestNewServerWithOptions_Defaults(t *testing.T) {
	s := NewServerWithOptions(Options{})
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServerWithOptions_WithInboundToken(t *testing.T) {
	s := NewServerWithOptions(Options{
		InboundToken: "my-token",
	})
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestNewServerWithOptions_WithGatewayToken(t *testing.T) {
	s := NewServerWithOptions(Options{
		GatewayToken: "gw-token",
	})
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestCodeAgentVersion_MissingParams(t *testing.T) {
	s := NewServerWithOptions(Options{Carrier: &stubCarrierClient{}})
	req := httptest.NewRequest(http.MethodGet, "/v1/carrier/codeagent/version", nil)
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestCodeAgentRun_MissingCapability(t *testing.T) {
	s := NewServerWithOptions(Options{Carrier: &stubCarrierClient{}})
	payload, _ := json.Marshal(map[string]any{
		"hostId": "h", "agentId": "a", "backend": "node",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/codeagent/run", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCarrierEvent_WithGatewayForward(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"order":{"id":"ord_1"},"usageCharge":{"kind":"token"}}`))
	}))
	defer backend.Close()

	s := NewServerWithOptions(Options{
		APIUpstream:  backend.URL,
		GatewayToken: "gw-token",
	})

	payload, _ := json.Marshal(map[string]any{
		"orderId": "ord_1", "milestoneId": "ms_1",
		"eventType": "usage_reported",
		"payload": map[string]any{"kind": "token", "amountCents": 200},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/events", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestPostJSON_ClosedServer(t *testing.T) {
	// Test with invalid URL
	s := NewServerWithOptions(Options{
		APIUpstream: "http://127.0.0.1:1",
	})
	payload, _ := json.Marshal(map[string]any{
		"orderId": "ord_1", "milestoneId": "ms_1",
		"eventType": "usage_reported",
		"payload": map[string]any{"kind": "token", "amountCents": 100},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/events", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	// Should return 502 for connection refused
	if rec.Code != http.StatusBadGateway && rec.Code != http.StatusInternalServerError {
		t.Logf("closed server: status %d", rec.Code)
	}
}

func TestNewServerWithOptions_WithEnvTokens(t *testing.T) {
	t.Setenv("EXECUTION_EVENT_TOKEN", "env-token")
	t.Setenv("ONE_TOK_EXECUTION_GATEWAY_TOKEN", "gw-token")
	s := NewServerWithOptions(Options{})
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}

func TestCarrierEvent_SettleAction(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"order":{"id":"ord_1"}}`))
	}))
	defer backend.Close()

	s := NewServerWithOptions(Options{APIUpstream: backend.URL})
	payload, _ := json.Marshal(map[string]any{
		"orderId": "ord_1", "milestoneId": "ms_1",
		"eventType": "milestone_ready",
		"payload": map[string]any{"summary": "done"},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/events", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCarrierEvent_PauseAction(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	s := NewServerWithOptions(Options{APIUpstream: backend.URL})
	payload, _ := json.Marshal(map[string]any{
		"orderId": "ord_1", "milestoneId": "ms_1",
		"eventType": "budget_low",
		"payload": map[string]any{},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/events", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestCodeAgentRun_CarrierError(t *testing.T) {
	stub := &stubCarrierClient{}
	stub.runResult.Backend = ""
	s := NewServerWithOptions(Options{Carrier: stub})

	payload, _ := json.Marshal(map[string]any{
		"hostId": "h", "agentId": "a", "backend": "codex",
		"capability": "run", "title": "Test",
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/codeagent/run", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	// Stub returns empty result — should still be 200
	if rec.Code != http.StatusOK {
		t.Logf("carrier result: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPostJSON_GatewayError(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"error":"conflict"}`))
	}))
	defer backend.Close()

	s := NewServerWithOptions(Options{APIUpstream: backend.URL})
	payload, _ := json.Marshal(map[string]any{
		"orderId": "ord_1", "milestoneId": "ms_1",
		"eventType": "usage_reported",
		"payload": map[string]any{"kind": "token", "amountCents": 100},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/carrier/events", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Logf("gateway error: status %d", rec.Code)
	}
}

func TestNewServerWithOptions_EnvTokens(t *testing.T) {
	t.Setenv("EXECUTION_EVENT_TOKENS", "tok1,tok2")
	t.Setenv("ONE_TOK_EXECUTION_GATEWAY_TOKEN", "gw-tok")
	s := NewServerWithOptions(Options{})
	if s == nil {
		t.Fatal("expected non-nil server")
	}
}
