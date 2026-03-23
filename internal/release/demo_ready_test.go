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
			_ = json.NewEncoder(w).Encode(map[string]any{"pool": map[string]any{"providerOrgId": "org_demo_provider", "availableToAllocateCents": 8000}})
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
