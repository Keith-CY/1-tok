package release

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/core"
	"github.com/chenyu/1-tok/internal/usageproof"
)

func TestBootstrapCarrierTargetCreatesRemoteHostAndInstallsCodeAgent(t *testing.T) {
	var createCalled bool
	var installCalled bool

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/remote/hosts":
			createCalled = true
			if got := r.Header.Get("Authorization"); got != "Bearer gateway-token" {
				t.Fatalf("unexpected auth header %q", got)
			}
			var payload carrierRemoteHostRequest
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode create host payload: %v", err)
			}
			if payload.Name != "carrier-remote" || payload.Host != "remote-vps" || payload.Port != 22 {
				t.Fatalf("unexpected create host payload: %+v", payload)
			}
			if payload.User != "carrier" || payload.AuthMode != "private_key" || payload.KeyPath != "/keys/id_ed25519" || payload.RuntimeMode != "on_demand" {
				t.Fatalf("unexpected create host payload: %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"host": map[string]any{
					"id": "host_carrier",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/remote/hosts/host_carrier/instances/main/codeagent/install":
			installCalled = true
			if got := r.Header.Get("Authorization"); got != "Bearer gateway-token" {
				t.Fatalf("unexpected auth header %q", got)
			}
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode install payload: %v", err)
			}
			if payload["backend"] != "codex" || payload["workspaceRoot"] != "/workspace" {
				t.Fatalf("unexpected install payload: %+v", payload)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "ok"})
		default:
			t.Fatalf("unexpected carrier bootstrap request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := &smokeClient{httpClient: server.Client()}
	cfg := USDIMarketplaceE2EConfig{
		CarrierBaseURL:        server.URL,
		CarrierGatewayToken:   "gateway-token",
		CarrierAgentID:        "main",
		CarrierBackend:        "codex",
		CarrierWorkspaceRoot:  "/workspace",
		CarrierRemoteHostName: "carrier-remote",
		CarrierRemoteHostHost: "remote-vps",
		CarrierRemoteHostPort: 22,
		CarrierRemoteHostUser: "carrier",
		CarrierRemoteKeyPath:  "/keys/id_ed25519",
	}

	if err := client.bootstrapCarrierTarget(context.Background(), &cfg); err != nil {
		t.Fatalf("bootstrap carrier target: %v", err)
	}
	if !createCalled || !installCalled {
		t.Fatalf("expected create and install to be called, create=%t install=%t", createCalled, installCalled)
	}
	if cfg.CarrierHostID != "host_carrier" {
		t.Fatalf("expected host id to be updated, got %q", cfg.CarrierHostID)
	}
	if cfg.CarrierAgentID != "main" {
		t.Fatalf("expected main agent id, got %q", cfg.CarrierAgentID)
	}
}

func TestBootstrapCarrierTargetSkipsWithoutRemoteHostConfig(t *testing.T) {
	client := &smokeClient{httpClient: http.DefaultClient}
	cfg := USDIMarketplaceE2EConfig{
		CarrierBaseURL:      "http://carrier.invalid",
		CarrierGatewayToken: "gateway-token",
	}
	if err := client.bootstrapCarrierTarget(context.Background(), &cfg); err != nil {
		t.Fatalf("bootstrap carrier target should skip missing remote host config: %v", err)
	}
}

func TestBuildUsageReportedEnvelopeIncludesVerifiableProof(t *testing.T) {
	eventAt := time.Date(2026, time.March, 22, 10, 11, 12, 0, time.UTC)
	step := usageReportedStep{
		EventID:     "evt-usage-1",
		Sequence:    2,
		Kind:        core.UsageChargeKindStep,
		AmountCents: 100,
		ProofRef:    "fiber:proof:usage-1",
	}

	envelope := buildUsageReportedEnvelope("bind_1", "job_1", "ms_1", "proof-secret", step, eventAt)
	payload, ok := envelope.Payload.(map[string]any)
	if !ok {
		t.Fatalf("expected payload map, got %T", envelope.Payload)
	}

	if got := payload["proofRef"]; got != "fiber:proof:usage-1" {
		t.Fatalf("expected proofRef to be preserved, got %v", got)
	}
	if got := payload["proofTimestamp"]; got != eventAt.Format(time.RFC3339) {
		t.Fatalf("expected proofTimestamp %q, got %v", eventAt.Format(time.RFC3339), got)
	}

	signature, _ := payload["proofSignature"].(string)
	if signature == "" {
		t.Fatal("expected proofSignature to be populated")
	}

	if err := usageproof.Verify("proof-secret", usageproof.Proof{
		ExecutionID: "job_1",
		MilestoneID: "ms_1",
		Kind:        string(core.UsageChargeKindStep),
		AmountCents: 100,
		Timestamp:   eventAt.Format(time.RFC3339),
		Signature:   signature,
	}); err != nil {
		t.Fatalf("expected usage proof to verify, got %v", err)
	}
}

func TestUSDIMarketplaceE2EConfigFromEnvDefaultsProviderSettlementToDedicatedNode(t *testing.T) {
	t.Setenv("RELEASE_USDI_E2E_PROVIDER_SETTLEMENT_RPC_URL", "")
	t.Setenv("RELEASE_USDI_E2E_PROVIDER_SETTLEMENT_P2P_HOST", "")
	t.Setenv("RELEASE_USDI_E2E_PROVIDER_SETTLEMENT_P2P_PORT", "")
	t.Setenv("FIBER_USDI_UDT_TYPE_SCRIPT_JSON", `{"codeHash":"0xudt","hashType":"type","args":"0x01"}`)

	cfg := USDIMarketplaceE2EConfigFromEnv()
	if cfg.ProviderSettlementRPCURL != "http://provider-fnn:8227" {
		t.Fatalf("provider settlement rpc url = %q, want http://provider-fnn:8227", cfg.ProviderSettlementRPCURL)
	}
	if cfg.ProviderSettlementP2PHost != "provider-fnn" {
		t.Fatalf("provider settlement p2p host = %q, want provider-fnn", cfg.ProviderSettlementP2PHost)
	}
	if cfg.ProviderSettlementP2PPort != 8228 {
		t.Fatalf("provider settlement p2p port = %d, want 8228", cfg.ProviderSettlementP2PPort)
	}
}

func TestRegisterProviderSettlementBindingUsesNodeInfoAndConfiguredUDTScript(t *testing.T) {
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read node request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode node request: %v", err)
		}
		if rpc.Method != "node_info" {
			t.Fatalf("unexpected node rpc method %q", rpc.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      1,
			"result": map[string]any{
				"node_id": "0x021111111111111111111111111111111111111111111111111111111111111111",
			},
		})
	}))
	defer node.Close()

	var createdPayload map[string]any
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/provider-settlement-bindings":
			if err := json.NewDecoder(r.Body).Decode(&createdPayload); err != nil {
				t.Fatalf("decode create payload: %v", err)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"binding": map[string]any{"id": "psb_1"},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/provider-settlement-bindings/psb_1/verify":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"binding": map[string]any{"id": "psb_1", "status": "active"},
			})
		default:
			t.Fatalf("unexpected api request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer api.Close()

	client := &smokeClient{httpClient: api.Client()}
	cfg := USDIMarketplaceE2EConfig{
		ProviderSettlementRPCURL:            node.URL,
		ProviderSettlementP2PHost:           "fnn",
		ProviderSettlementP2PPort:           8228,
		ProviderSettlementUDTTypeScriptJSON: `{"codeHash":"0xudt","hashType":"type","args":"0x01"}`,
	}

	bindingID, err := client.registerProviderSettlementBinding(context.Background(), api.URL, "provider_1", cfg)
	if err != nil {
		t.Fatalf("register provider settlement binding: %v", err)
	}
	if bindingID != "psb_1" {
		t.Fatalf("binding id = %q, want psb_1", bindingID)
	}
	if createdPayload["peerId"] == "" {
		t.Fatalf("expected peerId in payload, got %+v", createdPayload)
	}
	if createdPayload["nodeRpcUrl"] != node.URL {
		t.Fatalf("nodeRpcUrl = %v, want %s", createdPayload["nodeRpcUrl"], node.URL)
	}
}

func TestCreateProviderInvoiceViaProviderSettlementNodeUsesNewInvoice(t *testing.T) {
	var methods []string
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read node request: %v", err)
		}
		var rpc struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		if err := json.Unmarshal(payload, &rpc); err != nil {
			t.Fatalf("decode node request: %v", err)
		}
		methods = append(methods, rpc.Method)
		switch rpc.Method {
		case "new_invoice":
			if len(rpc.Params) != 1 {
				t.Fatalf("expected one new_invoice param, got %d", len(rpc.Params))
			}
			param, ok := rpc.Params[0].(map[string]any)
			if !ok {
				t.Fatalf("unexpected new_invoice param type: %T", rpc.Params[0])
			}
			if param["amount"] != "0x9" {
				t.Fatalf("amount = %v, want 0x9", param["amount"])
			}
			udt, ok := param["udt_type_script"].(map[string]any)
			if !ok {
				t.Fatalf("missing udt_type_script in payload: %+v", param)
			}
			if udt["code_hash"] != "0xudt" || udt["hash_type"] != "type" || udt["args"] != "0x01" {
				t.Fatalf("unexpected udt_type_script: %+v", udt)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result": map[string]any{
					"invoice_address": "fiber:invoice:provider-fnn",
				},
			})
		default:
			t.Fatalf("unexpected node rpc method %q", rpc.Method)
		}
	}))
	defer node.Close()

	client := &smokeClient{httpClient: http.DefaultClient}
	cfg := USDIMarketplaceE2EConfig{
		ProviderSettlementRPCURL:            node.URL,
		ProviderSettlementUDTTypeScriptJSON: `{"codeHash":"0xudt","hashType":"type","args":"0x01"}`,
	}

	invoice, err := client.createProviderInvoiceViaProviderSettlementNode(context.Background(), cfg, "9.00")
	if err != nil {
		t.Fatalf("create provider invoice: %v", err)
	}
	if invoice != "fiber:invoice:provider-fnn" {
		t.Fatalf("invoice = %q, want fiber:invoice:provider-fnn", invoice)
	}
	if len(methods) != 1 || methods[0] != "new_invoice" {
		t.Fatalf("methods = %v, want [new_invoice]", methods)
	}
}
