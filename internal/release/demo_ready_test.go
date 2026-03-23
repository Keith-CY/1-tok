package release

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenyu/1-tok/internal/demoenv"
)

func TestEnsureDemoActorSignsUpWhenAllowed(t *testing.T) {
	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sessions":
			http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		case "/v1/signup":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"organization": map[string]any{"id": "org_demo_buyer"},
				"session":      map[string]any{"token": "buyer-token"},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer iamServer.Close()

	client := &smokeClient{httpClient: iamServer.Client()}
	actor, created, err := ensureDemoActor(context.Background(), client, iamServer.URL, demoenv.ActorConfig{
		Email:            "buyer@example.com",
		Password:         "correct horse battery staple 123",
		Name:             "Buyer",
		OrganizationName: "Buyer Org",
		OrganizationKind: "buyer",
		OrganizationID:   "org_demo_buyer",
	}, true)
	if err != nil {
		t.Fatalf("ensure demo actor: %v", err)
	}
	if !created {
		t.Fatal("expected actor to be created")
	}
	if actor.OrgID != "org_demo_buyer" {
		t.Fatalf("actor org id = %s", actor.OrgID)
	}
}

func TestErrorContainsFold(t *testing.T) {
	err := fmt.Errorf("register provider carrier binding: %w", errors.New("Active Binding Already Exists For Provider org_demo_provider"))
	if !errorContainsFold(err, activeCarrierBindingAlreadyExistsMessage) {
		t.Fatal("expected case-insensitive wrapped match")
	}
	if errorContainsFold(err, activeSettlementBindingAlreadyExistsMessage) {
		t.Fatal("did not expect settlement fragment to match carrier error")
	}
	if errorContainsFold(nil, activeCarrierBindingAlreadyExistsMessage) {
		t.Fatal("nil error should never match")
	}
}

func TestRunDemoVerifyReturnsBlockedVerdict(t *testing.T) {
	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sessions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"token": "tok_" + strings.TrimPrefix(r.URL.RawQuery, "")},
			})
		case "/v1/me":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"memberships": []map[string]any{
					{"organization": map[string]any{"id": "org_demo", "kind": "buyer"}},
					{"organization": map[string]any{"id": "org_demo_provider", "kind": "provider"}},
					{"organization": map[string]any{"id": "org_demo_ops", "kind": "ops"}},
				},
			})
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer iamServer.Close()

	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/ops/demo/status" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": map[string]any{
				"checkedAt":      "2026-03-23T00:00:00Z",
				"resourcePrefix": "demo-live",
				"verdict":        "blocked",
				"blockerReasons": []string{"provider liquidity pool is below the demo threshold"},
				"actors": []map[string]any{
					{"role": "buyer", "ready": true},
					{"role": "provider", "ready": true},
					{"role": "ops", "ready": true},
				},
				"buyerBalance": map[string]any{
					"buyerOrgId":            "org_demo_buyer",
					"settledTopUpCents":     6000,
					"minimumRequiredCents":  5000,
					"meetsMinimumThreshold": true,
				},
				"providerSettlement": map[string]any{
					"providerOrgId":         "org_demo_provider",
					"minimumRequiredCents":  5500,
					"meetsMinimumThreshold": false,
				},
			},
		})
	}))
	defer gatewayServer.Close()

	summary, err := RunDemoVerify(context.Background(), DemoRunConfig{
		Demo: demoenv.Config{
			APIBaseURL:                gatewayServer.URL,
			IAMBaseURL:                iamServer.URL,
			Buyer:                     demoenv.ActorConfig{Email: "buyer@example.com", Password: "pw", OrganizationKind: "buyer"},
			Provider:                  demoenv.ActorConfig{Email: "provider@example.com", Password: "pw", OrganizationKind: "provider"},
			Ops:                       demoenv.ActorConfig{Email: "ops@example.com", Password: "pw", OrganizationKind: "ops"},
			MinBuyerBalanceCents:      5000,
			MinProviderLiquidityCents: 5500,
			ResourcePrefix:            "demo-live",
		},
	})
	if !errors.Is(err, ErrDemoNotReady) {
		t.Fatalf("err = %v, want ErrDemoNotReady", err)
	}
	if summary.Status.Verdict != demoenv.VerdictBlocked {
		t.Fatalf("verdict = %s, want blocked", summary.Status.Verdict)
	}
}

func TestRunDemoPrepareEnsuresBindingsAndWarmup(t *testing.T) {
	originalEnsureDemoFNNBootstrap := ensureDemoFNNBootstrapFunc
	defer func() {
		ensureDemoFNNBootstrapFunc = originalEnsureDemoFNNBootstrap
	}()
	ensureDemoFNNBootstrapFunc = func(_ context.Context, _ DemoRunConfig) error { return nil }

	var carrierRegistered bool
	var warmupCalled bool
	demoStatusCalls := 0

	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sessions":
			http.Error(w, `{"error":"invalid credentials"}`, http.StatusUnauthorized)
		case "/v1/signup":
			var payload struct {
				OrganizationKind string `json:"organizationKind"`
				Email            string `json:"email"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			orgID := map[string]string{
				"buyer":    "org_demo_buyer",
				"provider": "org_demo_provider",
				"ops":      "org_demo_ops",
			}[payload.OrganizationKind]
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"organization": map[string]any{"id": orgID},
				"session":      map[string]any{"token": payload.OrganizationKind + "-token"},
			})
		default:
			t.Fatalf("unexpected iam path %s", r.URL.Path)
		}
	}))
	defer iamServer.Close()

	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/carrier-bindings/org_demo_provider":
			http.NotFound(w, r)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/carrier-bindings":
			carrierRegistered = true
			_ = json.NewEncoder(w).Encode(map[string]any{"binding": map[string]any{"id": "pcb_1"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/carrier-bindings/pcb_1/verify":
			_ = json.NewEncoder(w).Encode(map[string]any{"binding": map[string]any{"id": "pcb_1", "status": "active"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/provider-settlement-bindings/org_demo_provider":
			_ = json.NewEncoder(w).Encode(map[string]any{"binding": map[string]any{"id": "psb_existing", "status": "active"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ops/demo/warmup":
			warmupCalled = true
			_ = json.NewEncoder(w).Encode(map[string]any{"pool": map[string]any{
				"providerOrgId":               "org_demo_provider",
				"providerSettlementBindingId": "psb_existing",
				"status":                      "healthy",
				"readyChannelCount":           1,
				"availableToAllocateCents":    8000,
				"reservedOutstandingCents":    0,
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/ops/demo/status":
			demoStatusCalls++
			ready := demoStatusCalls >= 2
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": map[string]any{
					"checkedAt":      "2026-03-23T00:00:00Z",
					"resourcePrefix": "demo-live",
					"verdict":        map[bool]string{true: "ready", false: "blocked"}[ready],
					"blockerReasons": map[bool][]string{true: {}, false: {"provider liquidity pool is below the demo threshold"}}[ready],
					"actors": []map[string]any{
						{"role": "buyer", "ready": true},
						{"role": "provider", "ready": true},
						{"role": "ops", "ready": true},
					},
					"buyerBalance": map[string]any{
						"buyerOrgId":            "org_demo_buyer",
						"settledTopUpCents":     6000,
						"minimumRequiredCents":  5000,
						"meetsMinimumThreshold": true,
					},
					"providerSettlement": map[string]any{
						"providerOrgId":         "org_demo_provider",
						"minimumRequiredCents":  5500,
						"meetsMinimumThreshold": ready,
					},
				},
			})
		default:
			t.Fatalf("unexpected gateway path %s %s", r.Method, r.URL.Path)
		}
	}))
	defer gatewayServer.Close()

	summary, err := RunDemoPrepare(context.Background(), DemoRunConfig{
		Demo: demoenv.Config{
			APIBaseURL:                gatewayServer.URL,
			IAMBaseURL:                iamServer.URL,
			Buyer:                     demoenv.ActorConfig{Email: "buyer@example.com", Password: "correct horse battery staple 123", Name: "Buyer", OrganizationName: "Buyer Org", OrganizationKind: "buyer", OrganizationID: "org_demo_buyer"},
			Provider:                  demoenv.ActorConfig{Email: "provider@example.com", Password: "correct horse battery staple 123", Name: "Provider", OrganizationName: "Provider Org", OrganizationKind: "provider", OrganizationID: "org_demo_provider"},
			Ops:                       demoenv.ActorConfig{Email: "ops@example.com", Password: "correct horse battery staple 123", Name: "Ops", OrganizationName: "Ops Org", OrganizationKind: "ops", OrganizationID: "org_demo_ops"},
			MinBuyerBalanceCents:      5000,
			MinProviderLiquidityCents: 5500,
			ResourcePrefix:            "demo-live",
		},
		USDI: USDIMarketplaceE2EConfig{
			CarrierBaseURL:                      "http://carrier.local",
			CarrierIntegrationToken:             "token",
			CarrierHostID:                       "host_1",
			CarrierAgentID:                      "main",
			CarrierBackend:                      "codex",
			CarrierWorkspaceRoot:                "/workspace",
			CarrierCallbackSecret:               "secret",
			CarrierCallbackKeyID:                "key_1",
			ProviderSettlementRPCURL:            "http://provider-fnn:8227",
			ProviderSettlementP2PHost:           "provider-fnn",
			ProviderSettlementP2PPort:           8228,
			ProviderSettlementUDTTypeScriptJSON: `{"codeHash":"0x1","hashType":"type","args":"0x2"}`,
		},
	})
	if err != nil {
		t.Fatalf("run demo prepare: %v", err)
	}
	if summary.Status.Verdict != demoenv.VerdictReady {
		t.Fatalf("verdict = %s, blockers=%v", summary.Status.Verdict, summary.Status.BlockerReasons)
	}
	if !carrierRegistered || !warmupCalled {
		t.Fatalf("carrierRegistered=%t warmupCalled=%t", carrierRegistered, warmupCalled)
	}
}

func TestRunDemoPrepareEnsuresBuyerTopUpRailBeforePayment(t *testing.T) {
	originalEnsureDemoFNNBootstrap := ensureDemoFNNBootstrapFunc
	originalEnsureBuyerTopUpRail := ensureDemoBuyerTopUpRailFunc
	defer func() {
		ensureDemoFNNBootstrapFunc = originalEnsureDemoFNNBootstrap
		ensureDemoBuyerTopUpRailFunc = originalEnsureBuyerTopUpRail
	}()

	callSequence := make([]string, 0, 2)
	ensureDemoFNNBootstrapFunc = func(_ context.Context, cfg DemoRunConfig) error {
		callSequence = append(callSequence, "bootstrap")
		if cfg.USDI.PayerRPCURL != "http://fnn2:8227" {
			t.Fatalf("payer rpc url = %q, want http://fnn2:8227", cfg.USDI.PayerRPCURL)
		}
		return nil
	}

	var buyerTopUpRailEnsured bool
	ensureDemoBuyerTopUpRailFunc = func(_ context.Context, cfg USDIMarketplaceE2EConfig, minimumAvailableCents int64) error {
		callSequence = append(callSequence, "rail")
		buyerTopUpRailEnsured = true
		if minimumAvailableCents != 5000 {
			t.Fatalf("minimumAvailableCents = %d, want 5000", minimumAvailableCents)
		}
		if cfg.BuyerTopUpInvoiceRPCURL != "http://fnn:8227" {
			t.Fatalf("buyer topup invoice rpc url = %q, want http://fnn:8227", cfg.BuyerTopUpInvoiceRPCURL)
		}
		return nil
	}

	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sessions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"token": "tok"},
			})
		case "/v1/me":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"memberships": []map[string]any{
					{"organization": map[string]any{"id": "org_demo_buyer", "kind": "buyer"}},
					{"organization": map[string]any{"id": "org_demo_provider", "kind": "provider"}},
					{"organization": map[string]any{"id": "org_demo_ops", "kind": "ops"}},
				},
			})
		default:
			t.Fatalf("unexpected iam path %s", r.URL.Path)
		}
	}))
	defer iamServer.Close()

	settlementCalls := 0
	settlementServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/topups":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"invoice":  "fibt_demo_topup",
				"recordId": "fund_demo_topup",
				"asset":    "USDI",
			})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/funding-records":
			settlementCalls++
			state := "UNPAID"
			if settlementCalls >= 2 {
				state = "SETTLED"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"records": []map[string]any{
					{"id": "fund_demo_topup", "kind": "buyer_topup", "buyerOrgId": "org_demo_buyer", "amount": "50.00", "state": state},
				},
			})
		default:
			t.Fatalf("unexpected settlement path %s %s", r.Method, r.URL.Path)
		}
	}))
	defer settlementServer.Close()

	fiberServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var rpc struct {
			Method string `json:"method"`
		}
		if err := json.NewDecoder(r.Body).Decode(&rpc); err != nil {
			t.Fatalf("decode fiber rpc: %v", err)
		}
		switch rpc.Method {
		case "withdrawal.request":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"jsonrpc": "2.0",
				"id":      1,
				"result":  map[string]any{"id": "wdreq_demo_topup", "state": "COMPLETED"},
			})
		default:
			t.Fatalf("unexpected fiber rpc method %s", rpc.Method)
		}
	}))
	defer fiberServer.Close()

	statusCalls := 0
	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/carrier-bindings/org_demo_provider":
			_ = json.NewEncoder(w).Encode(map[string]any{"binding": map[string]any{"id": "pcb_1", "status": "active"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/provider-settlement-bindings/org_demo_provider":
			_ = json.NewEncoder(w).Encode(map[string]any{"binding": map[string]any{"id": "psb_1", "status": "active"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/ops/demo/status":
			statusCalls++
			ready := statusCalls >= 2
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": map[string]any{
					"checkedAt":      "2026-03-23T00:00:00Z",
					"resourcePrefix": "demo-live",
					"verdict":        map[bool]string{true: "ready", false: "blocked"}[ready],
					"blockerReasons": map[bool][]string{true: {}, false: {"buyer prefund balance is below the demo threshold"}}[ready],
					"actors": []map[string]any{
						{"role": "buyer", "ready": true},
						{"role": "provider", "ready": true},
						{"role": "ops", "ready": true},
					},
					"buyerBalance": map[string]any{
						"buyerOrgId":            "org_demo_buyer",
						"settledTopUpCents":     map[bool]int64{true: 5000, false: 0}[ready],
						"pendingTopUpCount":     map[bool]int64{true: 0, false: 1}[ready],
						"minimumRequiredCents":  5000,
						"meetsMinimumThreshold": ready,
					},
					"providerSettlement": map[string]any{
						"providerOrgId":            "org_demo_provider",
						"minimumRequiredCents":     5500,
						"meetsMinimumThreshold":    true,
						"carrierBindingStatus":     "active",
						"settlementBindingStatus":  "active",
						"poolStatus":               "healthy",
						"readyChannelCount":        1,
						"availableToAllocateCents": 8000,
					},
				},
			})
		default:
			t.Fatalf("unexpected gateway path %s %s", r.Method, r.URL.Path)
		}
	}))
	defer gatewayServer.Close()

	summary, err := RunDemoPrepare(context.Background(), DemoRunConfig{
		Demo: demoenv.Config{
			APIBaseURL:                gatewayServer.URL,
			IAMBaseURL:                iamServer.URL,
			SettlementBaseURL:         settlementServer.URL,
			SettlementServiceToken:    "settlement-token",
			Buyer:                     demoenv.ActorConfig{Email: "buyer@example.com", Password: "correct horse battery staple 123", OrganizationKind: "buyer", OrganizationID: "org_demo_buyer"},
			Provider:                  demoenv.ActorConfig{Email: "provider@example.com", Password: "correct horse battery staple 123", OrganizationKind: "provider", OrganizationID: "org_demo_provider"},
			Ops:                       demoenv.ActorConfig{Email: "ops@example.com", Password: "correct horse battery staple 123", OrganizationKind: "ops", OrganizationID: "org_demo_ops"},
			MinBuyerBalanceCents:      5000,
			MinProviderLiquidityCents: 5500,
			BuyerTopUpAmount:          "50.00",
			ResourcePrefix:            "demo-live",
		},
		USDI: USDIMarketplaceE2EConfig{
			FiberAdapterBaseURL:         fiberServer.URL,
			FiberAdapterAppID:           "app_local",
			FiberAdapterHMACSecret:      "secret_local",
			PayerRPCURL:                 "http://fnn2:8227",
			BuyerTopUpInvoiceRPCURL:     "http://fnn:8227",
			BuyerTopUpInvoiceP2PHost:    "fnn",
			BuyerTopUpInvoiceP2PPort:    8228,
			BuyerTopUpUDTTypeScriptJSON: `{"codeHash":"0x1","hashType":"type","args":"0x2"}`,
		},
	})
	if err != nil {
		t.Fatalf("run demo prepare: %v", err)
	}
	if summary.Status.Verdict != demoenv.VerdictReady {
		t.Fatalf("verdict = %s, blockers=%v", summary.Status.Verdict, summary.Status.BlockerReasons)
	}
	if !buyerTopUpRailEnsured {
		t.Fatal("expected buyer topup rail to be ensured before payment")
	}
	if len(callSequence) < 2 || callSequence[0] != "bootstrap" || callSequence[1] != "rail" {
		t.Fatalf("call sequence = %v, want [bootstrap rail ...]", callSequence)
	}
}

func TestRunDemoPrepareBuildsFinalStatusWithoutSecondStatusFetch(t *testing.T) {
	originalEnsureDemoFNNBootstrap := ensureDemoFNNBootstrapFunc
	defer func() {
		ensureDemoFNNBootstrapFunc = originalEnsureDemoFNNBootstrap
	}()
	ensureDemoFNNBootstrapFunc = func(_ context.Context, _ DemoRunConfig) error { return nil }

	statusCalls := 0
	var warmupCalled bool

	iamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/sessions":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"session": map[string]any{"token": "tok"},
			})
		case "/v1/me":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"memberships": []map[string]any{
					{"organization": map[string]any{"id": "org_demo_buyer", "kind": "buyer"}},
					{"organization": map[string]any{"id": "org_demo_provider", "kind": "provider"}},
					{"organization": map[string]any{"id": "org_demo_ops", "kind": "ops"}},
				},
			})
		default:
			t.Fatalf("unexpected iam path %s", r.URL.Path)
		}
	}))
	defer iamServer.Close()

	gatewayServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/carrier-bindings/org_demo_provider":
			_ = json.NewEncoder(w).Encode(map[string]any{"binding": map[string]any{"id": "pcb_1", "status": "active"}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/provider-settlement-bindings/org_demo_provider":
			_ = json.NewEncoder(w).Encode(map[string]any{"binding": map[string]any{"id": "psb_1", "status": "active"}})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/ops/demo/warmup":
			warmupCalled = true
			_ = json.NewEncoder(w).Encode(map[string]any{"pool": map[string]any{
				"providerOrgId":               "org_demo_provider",
				"providerSettlementBindingId": "psb_1",
				"status":                      "healthy",
				"readyChannelCount":           2,
				"availableToAllocateCents":    8000,
				"reservedOutstandingCents":    0,
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/ops/demo/status":
			statusCalls++
			if statusCalls > 1 {
				http.Error(w, `{"error":"iam session status 429"}`, http.StatusTooManyRequests)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": map[string]any{
					"checkedAt":      "2026-03-23T00:00:00Z",
					"resourcePrefix": "demo-live",
					"verdict":        "blocked",
					"blockerReasons": []string{"provider liquidity pool is below the demo threshold"},
					"services": []map[string]any{
						{"id": "api-gateway", "label": "API Gateway", "healthy": true},
						{"id": "settlement", "label": "Settlement", "healthy": true},
					},
					"actors": []map[string]any{
						{"role": "buyer", "orgId": "org_demo_buyer", "ready": true, "detail": "resolved via IAM login"},
						{"role": "provider", "orgId": "org_demo_provider", "ready": true, "detail": "resolved via IAM login"},
						{"role": "ops", "orgId": "org_demo_ops", "ready": true, "detail": "resolved via IAM login"},
					},
					"buyerBalance": map[string]any{
						"buyerOrgId":            "org_demo_buyer",
						"settledTopUpCents":     6000,
						"minimumRequiredCents":  5000,
						"meetsMinimumThreshold": true,
					},
					"providerSettlement": map[string]any{
						"providerOrgId":            "org_demo_provider",
						"minimumRequiredCents":     5500,
						"meetsMinimumThreshold":    false,
						"carrierBindingStatus":     "active",
						"settlementBindingStatus":  "active",
						"poolStatus":               "degraded",
						"readyChannelCount":        0,
						"availableToAllocateCents": 0,
					},
				},
			})
		default:
			t.Fatalf("unexpected gateway path %s %s", r.Method, r.URL.Path)
		}
	}))
	defer gatewayServer.Close()

	summary, err := RunDemoPrepare(context.Background(), DemoRunConfig{
		Demo: demoenv.Config{
			APIBaseURL:                gatewayServer.URL,
			IAMBaseURL:                iamServer.URL,
			Buyer:                     demoenv.ActorConfig{Email: "buyer@example.com", Password: "correct horse battery staple 123", OrganizationKind: "buyer", OrganizationID: "org_demo_buyer"},
			Provider:                  demoenv.ActorConfig{Email: "provider@example.com", Password: "correct horse battery staple 123", OrganizationKind: "provider", OrganizationID: "org_demo_provider"},
			Ops:                       demoenv.ActorConfig{Email: "ops@example.com", Password: "correct horse battery staple 123", OrganizationKind: "ops", OrganizationID: "org_demo_ops"},
			MinBuyerBalanceCents:      5000,
			MinProviderLiquidityCents: 5500,
			ResourcePrefix:            "demo-live",
		},
	})
	if err != nil {
		t.Fatalf("run demo prepare: %v", err)
	}
	if !warmupCalled {
		t.Fatal("expected provider warmup to be called")
	}
	if statusCalls != 1 {
		t.Fatalf("statusCalls = %d, want 1", statusCalls)
	}
	if summary.Status.Verdict != demoenv.VerdictReady {
		t.Fatalf("verdict = %s, blockers=%v", summary.Status.Verdict, summary.Status.BlockerReasons)
	}
}
