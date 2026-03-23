package release

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/core"
	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/serviceauth"
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
	eventAt := time.Now().UTC().Truncate(time.Second)
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

func TestBuyerDepositCreditWaitTimeout(t *testing.T) {
	if got := buyerDepositCreditWaitTimeout(buyerDepositSummaryResponse{}); got != usdiMarketplaceBuyerDepositCreditWaitFloor {
		t.Fatalf("timeout without confirmation blocks = %s, want %s", got, usdiMarketplaceBuyerDepositCreditWaitFloor)
	}

	summary := buyerDepositSummaryResponse{ConfirmationBlocks: 24}
	want := 24*usdiMarketplaceBuyerDepositBlockIntervalEstimate + usdiMarketplaceBuyerDepositConfirmationGrace
	if got := buyerDepositCreditWaitTimeout(summary); got != want {
		t.Fatalf("timeout with 24 confirmation blocks = %s, want %s", got, want)
	}
}

func TestExtendBuyerDepositDeadlineExtendsForAdditionalFaucetRound(t *testing.T) {
	summary := buyerDepositSummaryResponse{ConfirmationBlocks: 24}
	firstNow := time.Date(2026, 3, 23, 18, 52, 0, 0, time.UTC)
	firstDeadline := extendBuyerDepositDeadline(time.Time{}, firstNow, summary)
	secondNow := firstNow.Add(2 * time.Minute)
	secondDeadline := extendBuyerDepositDeadline(firstDeadline, secondNow, summary)

	if !secondDeadline.After(firstDeadline) {
		t.Fatalf("second deadline %s should extend first deadline %s", secondDeadline, firstDeadline)
	}
}

func TestRequestUSDIFaucetRetriesTransientFailures(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/faucet" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		current := atomic.AddInt32(&attempts, 1)
		if current < 3 {
			http.Error(w, "temporary upstream failure", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	err := requestUSDIFaucetWithRetry(context.Background(), server.Client(), USDIMarketplaceE2EConfig{
		USDIFaucetAPIBase: server.URL,
	}, "ckt1qyqtestaddress", "buyer-topup", 3, 0, 0)
	if err != nil {
		t.Fatalf("request usdi faucet with retry: %v", err)
	}
	if got := atomic.LoadInt32(&attempts); got != 3 {
		t.Fatalf("attempts = %d, want 3", got)
	}
}

func TestWaitBuyerDepositCreditRetriesFaucetFailures(t *testing.T) {
	originalRequestUSDIFaucet := requestUSDIFaucetFunc
	originalPollInterval := waitBuyerDepositCreditPollInterval
	originalFaucetRetryDelay := waitBuyerDepositCreditFaucetRetryDelay
	defer func() {
		requestUSDIFaucetFunc = originalRequestUSDIFaucet
		waitBuyerDepositCreditPollInterval = originalPollInterval
		waitBuyerDepositCreditFaucetRetryDelay = originalFaucetRetryDelay
	}()

	waitBuyerDepositCreditPollInterval = 0
	waitBuyerDepositCreditFaucetRetryDelay = 0

	var faucetAttempts int32
	requestUSDIFaucetFunc = func(_ context.Context, _ *http.Client, _ USDIMarketplaceE2EConfig, address, label string) error {
		if address != "ckt1qyqbuyerdeposit" {
			t.Fatalf("unexpected address %q", address)
		}
		if label != "buyer-topup" {
			t.Fatalf("unexpected label %q", label)
		}
		if atomic.AddInt32(&faucetAttempts, 1) == 1 {
			return io.EOF
		}
		return nil
	}

	var summaryCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/v1/buyer/deposit-address":
			call := atomic.AddInt32(&summaryCalls, 1)
			summary := map[string]any{
				"address":             "ckt1qyqbuyerdeposit",
				"asset":               "USDI",
				"confirmationBlocks":  24,
				"rawMinimumSweepUnits": 10,
			}
			if call >= 3 {
				summary["creditedBalanceCents"] = 5000
			} else {
				summary["creditedBalanceCents"] = 0
				summary["rawOnChainUnits"] = 0
				summary["rawConfirmedUnits"] = 0
			}
			_ = json.NewEncoder(w).Encode(summary)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := &smokeClient{httpClient: server.Client()}
	summary, err := client.waitBuyerDepositCredit(context.Background(), USDIMarketplaceE2EConfig{
		SettlementBaseURL:      server.URL,
		SettlementServiceToken: "settlement-token",
	}, actorIdentity{OrgID: "buyer_1"}, "50.00")
	if err != nil {
		t.Fatalf("wait buyer deposit credit: %v", err)
	}
	if summary.CreditedBalanceCents != 5000 {
		t.Fatalf("credited balance cents = %d, want 5000", summary.CreditedBalanceCents)
	}
	if got := atomic.LoadInt32(&faucetAttempts); got != 2 {
		t.Fatalf("faucet attempts = %d, want 2", got)
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
		ProviderSettlementP2PHost:           "localhost",
		ProviderSettlementP2PPort:           8228,
		ProviderSettlementUDTTypeScriptJSON: `{"code_hash":"0xudt","hash_type":"type","args":"0x01"}`,
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
	expectedP2PAddress := "/ip4/127.0.0.1/tcp/8228/p2p/" + createdPayload["peerId"].(string)
	if createdPayload["p2pAddress"] != expectedP2PAddress {
		t.Fatalf("p2pAddress = %v, want %s", createdPayload["p2pAddress"], expectedP2PAddress)
	}
	if createdPayload["nodeRpcUrl"] != node.URL {
		t.Fatalf("nodeRpcUrl = %v, want %s", createdPayload["nodeRpcUrl"], node.URL)
	}
	udtTypeScript, ok := createdPayload["udtTypeScript"].(map[string]any)
	if !ok {
		t.Fatalf("missing udtTypeScript in payload: %+v", createdPayload)
	}
	if udtTypeScript["codeHash"] != "0xudt" || udtTypeScript["hashType"] != "type" || udtTypeScript["args"] != "0x01" {
		t.Fatalf("unexpected udtTypeScript payload: %+v", udtTypeScript)
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
			if param["currency"] != "Fibt" {
				t.Fatalf("currency = %v, want Fibt", param["currency"])
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
		ProviderSettlementUDTTypeScriptJSON: `{"code_hash":"0xudt","hash_type":"type","args":"0x01"}`,
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

func TestRequestProviderPayoutWithRetryRetriesOnServerError(t *testing.T) {
	invoiceAttempts := 0
	node := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
			t.Fatalf("decode node rpc request: %v", err)
		}
		if rpc.Method != "new_invoice" {
			t.Fatalf("unexpected node rpc method %q", rpc.Method)
		}
		invoiceAttempts++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"jsonrpc": "2.0",
			"id":      "1",
			"result": map[string]any{
				"invoice_address": "invoice_retry_" + strconv.Itoa(invoiceAttempts),
			},
		})
	}))
	defer node.Close()

	payoutAttempts := 0
	settlement := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/provider-payouts" {
			t.Fatalf("unexpected settlement request %s %s", r.Method, r.URL.Path)
		}
		payoutAttempts++
		if payoutAttempts == 1 {
			http.Error(w, "upstream fiber unavailable", http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"recordId": "fund_retry_ok",
		})
	}))
	defer settlement.Close()

	client := &smokeClient{httpClient: settlement.Client()}
	recordID, err := client.requestProviderPayoutWithRetry(context.Background(), USDIMarketplaceE2EConfig{
		SettlementBaseURL:                   settlement.URL,
		SettlementServiceToken:              "settlement-token",
		ProviderSettlementRPCURL:            node.URL,
		ProviderSettlementUDTTypeScriptJSON: `{"code_hash":"0xudt","hash_type":"type","args":"0x01"}`,
	}, "ord_1", "buyer_1", "provider_1", "1.00", 3, 0)
	if err != nil {
		t.Fatalf("request provider payout with retry: %v", err)
	}
	if recordID != "fund_retry_ok" {
		t.Fatalf("record id = %q, want fund_retry_ok", recordID)
	}
	if payoutAttempts != 2 {
		t.Fatalf("payout attempts = %d, want 2", payoutAttempts)
	}
	if invoiceAttempts != 2 {
		t.Fatalf("invoice attempts = %d, want 2", invoiceAttempts)
	}
}

func TestGetOrderProviderSettlementReservationUsesOrderSubresource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/api/v1/orders/ord_1/provider-settlement-reservation" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer buyer-token" {
			t.Fatalf("unexpected auth header %q", got)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"reservation": map[string]any{
				"id":            "plr_1",
				"orderId":       "ord_1",
				"channelId":     "ch_1",
				"reuseSource":   "reused",
				"reservedCents": 1980,
			},
		})
	}))
	defer server.Close()

	client := &smokeClient{httpClient: server.Client()}
	reservation, err := client.getOrderProviderSettlementReservation(context.Background(), server.URL, "buyer-token", "ord_1")
	if err != nil {
		t.Fatalf("get order provider settlement reservation: %v", err)
	}
	if reservation.OrderID != "ord_1" || reservation.ChannelID != "ch_1" {
		t.Fatalf("unexpected reservation: %+v", reservation)
	}
	if reservation.ReuseSource != "reused" {
		t.Fatalf("reuse source = %s, want reused", reservation.ReuseSource)
	}
}

func TestProviderSettlementDisconnectAndRecoverUseGatewayServiceToken(t *testing.T) {
	var seen []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get(serviceauth.HeaderName); got != "gateway-token" {
			t.Fatalf("unexpected service token %q", got)
		}
		seen = append(seen, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/provider-settlement-bindings/provider_1/disconnect":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				t.Fatalf("decode disconnect payload: %v", err)
			}
			if payload["reason"] != "provider closed channel" {
				t.Fatalf("disconnect reason = %v", payload["reason"])
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"pool": map[string]any{
					"providerOrgId": "provider_1",
					"status":        "disconnected",
				},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/provider-settlement-bindings/provider_1/recover":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"pool": map[string]any{
					"providerOrgId": "provider_1",
					"status":        "healthy",
				},
			})
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client := &smokeClient{httpClient: server.Client()}
	disconnected, err := client.reportProviderSettlementDisconnect(context.Background(), server.URL, "gateway-token", "provider_1", "provider closed channel")
	if err != nil {
		t.Fatalf("report provider settlement disconnect: %v", err)
	}
	if disconnected.Status != "disconnected" {
		t.Fatalf("disconnect status = %s, want disconnected", disconnected.Status)
	}

	recovered, err := client.recoverProviderSettlement(context.Background(), server.URL, "gateway-token", "provider_1")
	if err != nil {
		t.Fatalf("recover provider settlement: %v", err)
	}
	if recovered.Status != "healthy" {
		t.Fatalf("recover status = %s, want healthy", recovered.Status)
	}
	if len(seen) != 2 {
		t.Fatalf("seen requests = %v, want 2", seen)
	}
}

func TestValidateUSDIMarketplaceOrderScenarios_AllowsDisconnectRecoveryToRotateChannel(t *testing.T) {
	err := validateUSDIMarketplaceOrderScenarios(
		usdiMarketplaceOrderFlowResult{
			OrderID:              "ord_bootstrap",
			ReservationChannelID: "ch_bootstrap",
			ReservationStatus:    "released",
		},
		usdiMarketplaceOrderFlowResult{
			OrderID:                     "ord_reuse",
			InitialReservationChannelID: "ch_bootstrap",
			InitialReservationReuse:     string(platform.ProviderLiquidityReuseReused),
			ReservationChannelID:        "ch_bootstrap",
		},
		usdiMarketplaceOrderFlowResult{
			OrderID:                     "ord_disconnect",
			InitialReservationChannelID: "ch_bootstrap",
			InitialReservationReuse:     string(platform.ProviderLiquidityReuseReused),
			ReservationChannelID:        "ch_recovered",
			DisconnectStatus:            string(core.OrderStatusAwaitingPaymentRail),
			RecoveredStatus:             string(core.OrderStatusRunning),
		},
	)
	if err != nil {
		t.Fatalf("validate scenarios: %v", err)
	}
}

func TestValidateUSDIMarketplaceOrderScenarios_AllowsReusePoolToRotateChannel(t *testing.T) {
	err := validateUSDIMarketplaceOrderScenarios(
		usdiMarketplaceOrderFlowResult{
			OrderID:              "ord_bootstrap",
			ReservationChannelID: "ch_bootstrap",
			ReservationStatus:    "released",
		},
		usdiMarketplaceOrderFlowResult{
			OrderID:                     "ord_reuse",
			InitialReservationChannelID: "ch_other",
			InitialReservationReuse:     string(platform.ProviderLiquidityReuseReused),
			ReservationChannelID:        "ch_other",
		},
		usdiMarketplaceOrderFlowResult{
			OrderID:                     "ord_disconnect",
			InitialReservationChannelID: "ch_bootstrap",
			InitialReservationReuse:     string(platform.ProviderLiquidityReuseReused),
			ReservationChannelID:        "ch_recovered",
			DisconnectStatus:            string(core.OrderStatusAwaitingPaymentRail),
			RecoveredStatus:             string(core.OrderStatusRunning),
		},
	)
	if err != nil {
		t.Fatalf("validate scenarios: %v", err)
	}
}

func TestValidateUSDIMarketplaceOrderScenarios_RejectsReuseWhenPoolWasNotReused(t *testing.T) {
	err := validateUSDIMarketplaceOrderScenarios(
		usdiMarketplaceOrderFlowResult{
			OrderID:              "ord_bootstrap",
			ReservationChannelID: "ch_bootstrap",
			ReservationStatus:    "released",
		},
		usdiMarketplaceOrderFlowResult{
			OrderID:                     "ord_reuse",
			InitialReservationChannelID: "ch_other",
			InitialReservationReuse:     string(platform.ProviderLiquidityReuseNewChannel),
			ReservationChannelID:        "ch_other",
		},
		usdiMarketplaceOrderFlowResult{
			OrderID:                     "ord_disconnect",
			InitialReservationChannelID: "ch_other",
			InitialReservationReuse:     string(platform.ProviderLiquidityReuseReused),
			ReservationChannelID:        "ch_recovered",
			DisconnectStatus:            string(core.OrderStatusAwaitingPaymentRail),
			RecoveredStatus:             string(core.OrderStatusRunning),
		},
	)
	if err == nil {
		t.Fatal("expected reuse source validation error")
	}
}

func TestParseProviderSettlementUDTTypeScriptJSONSupportsCamelAndSnakeCase(t *testing.T) {
	snakeCase, err := parseProviderSettlementUDTTypeScriptJSON(`{"code_hash":"0xudt","hash_type":"type","args":"0x01"}`)
	if err != nil {
		t.Fatalf("parse snake_case: %v", err)
	}
	if snakeCase.CodeHash != "0xudt" || snakeCase.HashType != "type" || snakeCase.Args != "0x01" {
		t.Fatalf("unexpected snake_case script: %+v", snakeCase)
	}

	camelCase, err := parseProviderSettlementUDTTypeScriptJSON(`{"codeHash":"0xabc","hashType":"data1","args":"0x02"}`)
	if err != nil {
		t.Fatalf("parse camelCase: %v", err)
	}
	if camelCase.CodeHash != "0xabc" || camelCase.HashType != "data1" || camelCase.Args != "0x02" {
		t.Fatalf("unexpected camelCase script: %+v", camelCase)
	}
}

func TestParseProviderSettlementUDTTypeScriptJSONSupportsEscapedEnvJSON(t *testing.T) {
	script, err := parseProviderSettlementUDTTypeScriptJSON(`{\"code_hash\":\"0xudt\",\"hash_type\":\"type\",\"args\":\"0x01\"}`)
	if err != nil {
		t.Fatalf("parse escaped env json: %v", err)
	}
	if script.CodeHash != "0xudt" || script.HashType != "type" || script.Args != "0x01" {
		t.Fatalf("unexpected escaped env script: %+v", script)
	}
}
