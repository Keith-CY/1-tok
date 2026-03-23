package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/chenyu/1-tok/internal/demoenv"
	iamclient "github.com/chenyu/1-tok/internal/integrations/iam"
	"github.com/chenyu/1-tok/internal/platform"
	"github.com/chenyu/1-tok/internal/release"
)

func TestDemoStatusEndpointRequiresOpsAuth(t *testing.T) {
	server, err := NewServerWithOptionsE(Options{
		App: platform.NewAppWithMemory(),
		IAM: &stubIAMClient{actor: iamclient.Actor{
			UserID: "user_1",
			Memberships: []iamclient.ActorMembership{
				{OrganizationID: "buyer_1", OrganizationKind: "buyer", Role: "org_owner"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ops/demo/status", nil)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusUnauthorized {
		t.Fatalf("status code = %d, want 401", res.Code)
	}
}

func TestDemoStatusEndpointReturnsAggregatedReadiness(t *testing.T) {
	healthServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			t.Fatalf("unexpected health path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer healthServer.Close()

	settlementServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/healthz":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		case "/v1/funding-records":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"records": []map[string]any{
					{"id": "fund_1", "kind": "buyer_topup", "buyerOrgId": "org_demo_buyer", "amount": "60.00", "state": "SETTLED"},
				},
			})
		default:
			t.Fatalf("unexpected settlement path %s", r.URL.Path)
		}
	}))
	defer settlementServer.Close()

	t.Setenv("DEMO_API_BASE_URL", "http://api.demo.local")
	t.Setenv("DEMO_SETTLEMENT_BASE_URL", settlementServer.URL)
	t.Setenv("DEMO_EXECUTION_BASE_URL", healthServer.URL)
	t.Setenv("DEMO_CARRIER_BASE_URL", healthServer.URL)
	t.Setenv("DEMO_FIBER_ADAPTER_BASE_URL", healthServer.URL)
	t.Setenv("DEMO_IAM_BASE_URL", healthServer.URL)
	t.Setenv("DEMO_BUYER_ORG_ID", "org_demo_buyer")
	t.Setenv("DEMO_PROVIDER_ORG_ID", "org_demo_provider")
	t.Setenv("DEMO_OPS_ORG_ID", "org_demo_ops")
	t.Setenv("DEMO_MIN_BUYER_BALANCE_CENTS", "5000")
	t.Setenv("DEMO_MIN_PROVIDER_LIQUIDITY_CENTS", "5500")

	app := platform.NewAppWithMemory()
	registerActiveCarrierBindingForGatewayTest(t, app, "org_demo_provider")
	registerActiveSettlementBindingForGatewayTest(t, app, "org_demo_provider")
	app.SetProviderSettlementProvisioner(&gatewayStubSettlementProvisioner{
		result: platform.EnsureProviderLiquidityResult{
			ChannelID:           "ch_demo_1",
			ReuseSource:         platform.ProviderLiquidityReuseNewChannel,
			ReadyChannelCount:   1,
			TotalSpendableCents: 8_000,
		},
	})
	if _, err := app.WarmProviderSettlementPool("org_demo_provider", 5_500); err != nil {
		t.Fatalf("warm pool: %v", err)
	}

	server, err := NewServerWithOptionsE(Options{
		App: app,
		IAM: &stubIAMClient{actor: iamclient.Actor{
			UserID: "ops_user",
			Memberships: []iamclient.ActorMembership{
				{OrganizationID: "org_demo_ops", OrganizationKind: "ops", Role: "ops_reviewer"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ops/demo/status", nil)
	req.Header.Set("Authorization", "Bearer ops-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status code = %d, body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Status demoenv.Status `json:"status"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Status.Verdict != demoenv.VerdictReady {
		t.Fatalf("verdict = %s, blockers=%v", response.Status.Verdict, response.Status.BlockerReasons)
	}
	if response.Status.BuyerBalance.SettledTopUpCents != 6000 {
		t.Fatalf("buyer settled cents = %d, want 6000", response.Status.BuyerBalance.SettledTopUpCents)
	}
	if !response.Status.ProviderSettlement.MeetsMinimumThreshold {
		t.Fatalf("provider settlement should meet threshold: %+v", response.Status.ProviderSettlement)
	}
}

func TestDemoWarmupEndpointWarmsProviderLiquidity(t *testing.T) {
	t.Setenv("DEMO_PROVIDER_ORG_ID", "org_demo_provider")
	t.Setenv("DEMO_MIN_PROVIDER_LIQUIDITY_CENTS", "5500")

	app := platform.NewAppWithMemory()
	registerActiveCarrierBindingForGatewayTest(t, app, "org_demo_provider")
	registerActiveSettlementBindingForGatewayTest(t, app, "org_demo_provider")
	app.SetProviderSettlementProvisioner(&gatewayStubSettlementProvisioner{
		result: platform.EnsureProviderLiquidityResult{
			ChannelID:           "ch_demo_1",
			ReuseSource:         platform.ProviderLiquidityReuseNewChannel,
			ReadyChannelCount:   1,
			TotalSpendableCents: 8_000,
		},
	})

	server, err := NewServerWithOptionsE(Options{
		App: app,
		IAM: &stubIAMClient{actor: iamclient.Actor{
			UserID: "ops_user",
			Memberships: []iamclient.ActorMembership{
				{OrganizationID: "org_demo_ops", OrganizationKind: "ops", Role: "ops_reviewer"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ops/demo/warmup", nil)
	req.Header.Set("Authorization", "Bearer ops-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status code = %d, body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Pool platform.ProviderLiquidityPool `json:"pool"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Pool.AvailableToAllocateCents != 8_000 {
		t.Fatalf("available cents = %d, want 8000", response.Pool.AvailableToAllocateCents)
	}
}

func TestDemoPrepareEndpointReturnsSummary(t *testing.T) {
	server, err := NewServerWithOptionsE(Options{
		App: platform.NewAppWithMemory(),
		IAM: &stubIAMClient{actor: iamclient.Actor{
			UserID: "ops_user",
			Memberships: []iamclient.ActorMembership{
				{OrganizationID: "org_demo_ops", OrganizationKind: "ops", Role: "ops_reviewer"},
			},
		}},
		DemoPrepare: func(_ context.Context) (release.DemoRunSummary, error) {
			return release.DemoRunSummary{
				Status: demoenv.Status{
					Verdict: demoenv.VerdictReady,
				},
				Actions: []string{"buyer top-up settled", "provider liquidity warmed"},
			}, nil
		},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ops/demo/prepare", nil)
	req.Header.Set("Authorization", "Bearer ops-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("status code = %d, body=%s", res.Code, res.Body.String())
	}

	var response struct {
		Summary release.DemoRunSummary `json:"summary"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if response.Summary.Status.Verdict != demoenv.VerdictReady {
		t.Fatalf("verdict = %s, want ready", response.Summary.Status.Verdict)
	}
	if len(response.Summary.Actions) != 2 {
		t.Fatalf("actions = %v, want 2 entries", response.Summary.Actions)
	}
}

func TestDemoPrepareEndpointReturnsConflictWhenBlocked(t *testing.T) {
	server, err := NewServerWithOptionsE(Options{
		App: platform.NewAppWithMemory(),
		IAM: &stubIAMClient{actor: iamclient.Actor{
			UserID: "ops_user",
			Memberships: []iamclient.ActorMembership{
				{OrganizationID: "org_demo_ops", OrganizationKind: "ops", Role: "ops_reviewer"},
			},
		}},
		DemoPrepare: func(_ context.Context) (release.DemoRunSummary, error) {
			return release.DemoRunSummary{
				Status: demoenv.Status{
					Verdict:        demoenv.VerdictBlocked,
					BlockerReasons: []string{"buyer prefund balance is below the demo threshold"},
				},
			}, release.ErrDemoNotReady
		},
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/ops/demo/prepare", nil)
	req.Header.Set("Authorization", "Bearer ops-token")
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusConflict {
		t.Fatalf("status code = %d, want 409, body=%s", res.Code, res.Body.String())
	}
}
