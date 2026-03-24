package gateway

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/carrier"
	carrierclient "github.com/chenyu/1-tok/internal/integrations/carrier"
	"github.com/chenyu/1-tok/internal/platform"
)

type stubCarrierAwardExecutor struct {
	calls chan carrierAwardExecutionInput
}

func (s *stubCarrierAwardExecutor) Execute(ctx context.Context, input carrierAwardExecutionInput) error {
	select {
	case s.calls <- input:
	default:
	}
	return nil
}

type stubCodeAgentClient struct {
	runInputs []carrierclient.CodeAgentRunInput
	runResult carrierclient.CodeAgentRunResult
	runErr    error
}

type stubGatewayCarrierSettlementProvisioner struct {
	result platform.EnsureProviderLiquidityResult
	err    error
}

func (s *stubGatewayCarrierSettlementProvisioner) EnsureProviderLiquidity(input platform.EnsureProviderLiquidityInput) (platform.EnsureProviderLiquidityResult, error) {
	if s.err != nil {
		return platform.EnsureProviderLiquidityResult{}, s.err
	}
	if s.result.TotalSpendableCents > 0 {
		return s.result, nil
	}
	return platform.EnsureProviderLiquidityResult{
		ChannelID:           "ch_gateway_1",
		ReuseSource:         platform.ProviderLiquidityReuseNewChannel,
		ReadyChannelCount:   1,
		TotalSpendableCents: input.NeededReserveCents + 1_000,
	}, nil
}

func (s *stubCodeAgentClient) GetCodeAgentHealth(context.Context, carrierclient.CodeAgentHealthInput) (carrierclient.CodeAgentHealthResult, error) {
	return carrierclient.CodeAgentHealthResult{}, nil
}

func (s *stubCodeAgentClient) GetCodeAgentVersion(context.Context, carrierclient.CodeAgentVersionInput) (carrierclient.CodeAgentVersionResult, error) {
	return carrierclient.CodeAgentVersionResult{}, nil
}

func (s *stubCodeAgentClient) RunCodeAgent(_ context.Context, input carrierclient.CodeAgentRunInput) (carrierclient.CodeAgentRunResult, error) {
	s.runInputs = append(s.runInputs, input)
	if s.runErr != nil {
		return carrierclient.CodeAgentRunResult{}, s.runErr
	}
	return s.runResult, nil
}

func TestHandleAwardRFQDispatchesCarrierExecutionForActiveProviderBinding(t *testing.T) {
	app := platform.NewAppWithMemory()
	binding, err := app.RegisterCarrierBinding(platform.ProviderCarrierBinding{
		ProviderOrgID:  "provider_1",
		CarrierBaseURL: "https://carrier.example.com",
		HostID:         "host_1",
		AgentID:        "agent_1",
		Backend:        "codex",
		WorkspaceRoot:  "/workspace",
	})
	if err != nil {
		t.Fatalf("register binding: %v", err)
	}
	if _, err := app.VerifyCarrierBinding(binding.ID); err != nil {
		t.Fatalf("verify binding: %v", err)
	}
	settlementBinding, err := app.RegisterProviderSettlementBinding(platform.ProviderSettlementBinding{
		ProviderOrgID: "provider_1",
		Asset:         "USDI",
		PeerID:        "peer_provider",
		P2PAddress:    "/dns4/provider/tcp/8228/p2p/peer_provider",
		NodeRPCURL:    "http://provider:8227",
		UDTTypeScript: platform.UDTTypeScript{
			CodeHash: "0xudt",
			HashType: "type",
			Args:     "0x01",
		},
	})
	if err != nil {
		t.Fatalf("register settlement binding: %v", err)
	}
	if _, err := app.VerifyProviderSettlementBinding(settlementBinding.ID); err != nil {
		t.Fatalf("verify settlement binding: %v", err)
	}

	rfq, err := app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID:         "buyer_1",
		Title:              "Research 3 vendors",
		Category:           "research",
		Scope:              "Compare 3 Japanese AI call-center vendors.",
		BudgetCents:        4_000,
		ResponseDeadlineAt: time.Date(2099, 3, 26, 14, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}
	bid, err := app.CreateBid(rfq.ID, platform.CreateBidInput{
		ProviderOrgID: "provider_1",
		Message:       "bid",
		Milestones: []platform.BidMilestoneInput{{
			ID:             "ms_1",
			Title:          "Service delivery",
			BasePriceCents: 3_200,
			BudgetCents:    3_200,
		}},
	})
	if err != nil {
		t.Fatalf("create bid: %v", err)
	}

	executor := &stubCarrierAwardExecutor{calls: make(chan carrierAwardExecutionInput, 1)}
	srv, err := NewServerWithOptionsE(Options{
		App: app,
		ProviderSettlementProvisioner: &stubGatewayCarrierSettlementProvisioner{
			result: platform.EnsureProviderLiquidityResult{
				ChannelID:           "ch_gateway_1",
				ReuseSource:         platform.ProviderLiquidityReuseNewChannel,
				ReadyChannelCount:   1,
				TotalSpendableCents: 5_000,
			},
		},
		CarrierAwardExecutor: executor,
	})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	req := httptest.NewRequest("POST", "/api/v1/rfqs/"+rfq.ID+"/award", strings.NewReader(`{"bidId":"`+bid.ID+`","fundingMode":"prepaid"}`))
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("award rfq: status=%d body=%s", w.Code, w.Body.String())
	}

	select {
	case call := <-executor.calls:
		if call.Binding.ID != binding.ID {
			t.Fatalf("binding id = %s, want %s", call.Binding.ID, binding.ID)
		}
		if call.Order == nil || call.Order.ProviderOrgID != "provider_1" {
			t.Fatalf("unexpected order dispatch payload: %+v", call.Order)
		}
		if call.RFQ.ID != rfq.ID {
			t.Fatalf("rfq id = %s, want %s", call.RFQ.ID, rfq.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected carrier execution dispatch")
	}
}

func TestCarrierAwardExecutorSettlesOrderAfterSuccessfulRun(t *testing.T) {
	app := platform.NewAppWithMemory()
	carrierSvc := carrier.NewService()

	providerBinding, err := app.RegisterCarrierBinding(platform.ProviderCarrierBinding{
		ProviderOrgID:  "provider_1",
		CarrierBaseURL: "https://carrier.example.com",
		HostID:         "host_1",
		AgentID:        "agent_1",
		Backend:        "codex",
		WorkspaceRoot:  "/workspace",
	})
	if err != nil {
		t.Fatalf("register binding: %v", err)
	}
	if _, err := app.VerifyCarrierBinding(providerBinding.ID); err != nil {
		t.Fatalf("verify binding: %v", err)
	}
	settlementBinding, err := app.RegisterProviderSettlementBinding(platform.ProviderSettlementBinding{
		ProviderOrgID: "provider_1",
		Asset:         "USDI",
		PeerID:        "peer_provider",
		P2PAddress:    "/dns4/provider/tcp/8228/p2p/peer_provider",
		NodeRPCURL:    "http://provider:8227",
		UDTTypeScript: platform.UDTTypeScript{
			CodeHash: "0xudt",
			HashType: "type",
			Args:     "0x01",
		},
	})
	if err != nil {
		t.Fatalf("register settlement binding: %v", err)
	}
	if _, err := app.VerifyProviderSettlementBinding(settlementBinding.ID); err != nil {
		t.Fatalf("verify settlement binding: %v", err)
	}
	app.SetProviderSettlementProvisioner(&stubGatewayCarrierSettlementProvisioner{
		result: platform.EnsureProviderLiquidityResult{
			ChannelID:           "ch_gateway_1",
			ReuseSource:         platform.ProviderLiquidityReuseNewChannel,
			ReadyChannelCount:   1,
			TotalSpendableCents: 5_000,
		},
	})

	rfq, err := app.CreateRFQ(platform.CreateRFQInput{
		BuyerOrgID:         "buyer_1",
		Title:              "Research 3 vendors",
		Category:           "research",
		Scope:              "Compare 3 Japanese AI call-center vendors.",
		BudgetCents:        4_000,
		ResponseDeadlineAt: time.Date(2099, 3, 26, 14, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}
	bid, err := app.CreateBid(rfq.ID, platform.CreateBidInput{
		ProviderOrgID: "provider_1",
		Message:       "bid",
		Milestones: []platform.BidMilestoneInput{{
			ID:             "ms_1",
			Title:          "Service delivery",
			BasePriceCents: 3_200,
			BudgetCents:    3_200,
		}},
	})
	if err != nil {
		t.Fatalf("create bid: %v", err)
	}

	awardedRFQ, order, err := app.AwardRFQ(rfq.ID, platform.AwardRFQInput{
		BidID:       bid.ID,
		FundingMode: "prepaid",
	})
	if err != nil {
		t.Fatalf("award rfq: %v", err)
	}

	client := &stubCodeAgentClient{
		runResult: carrierclient.CodeAgentRunResult{
			Backend: "codex",
			Result: carrierclient.CodeAgentRunOutput{
				OK:             true,
				PolicyDecision: "allow",
			},
		},
	}
	executor := &carrierOrderAutoExecutor{
		app:     app,
		carrier: carrierSvc,
		now: func() time.Time {
			return time.Date(2099, 3, 24, 15, 4, 5, 0, time.UTC)
		},
		clientForBinding: func(platform.ProviderCarrierBinding) carrierclient.CodeAgentClient {
			return client
		},
	}

	if err := executor.Execute(context.Background(), carrierAwardExecutionInput{
		RFQ:     awardedRFQ,
		Order:   order,
		Binding: providerBinding,
	}); err != nil {
		t.Fatalf("execute carrier award: %v", err)
	}

	updatedOrder, err := app.GetOrder(order.ID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if updatedOrder.Status != "completed" {
		t.Fatalf("order status = %s, want completed", updatedOrder.Status)
	}
	if updatedOrder.Milestones[0].State != "settled" {
		t.Fatalf("milestone state = %s, want settled", updatedOrder.Milestones[0].State)
	}
	if !strings.Contains(updatedOrder.Milestones[0].Summary, "Carrier execution completed.") {
		t.Fatalf("milestone summary = %q, want carrier completion summary", updatedOrder.Milestones[0].Summary)
	}
	if !strings.Contains(updatedOrder.Milestones[0].Summary, "/workspace/1tok/"+order.ID+"/ms_1/result.md") {
		t.Fatalf("milestone summary = %q, want result path", updatedOrder.Milestones[0].Summary)
	}

	if len(client.runInputs) != 1 {
		t.Fatalf("run inputs = %d, want 1", len(client.runInputs))
	}
	runInput := client.runInputs[0]
	if runInput.HostID != "host_1" || runInput.AgentID != "agent_1" || runInput.Backend != "codex" {
		t.Fatalf("unexpected run input routing: %+v", runInput)
	}
	if runInput.Capability != "run_shell" {
		t.Fatalf("capability = %s, want run_shell", runInput.Capability)
	}
	if !strings.Contains(runInput.Command, ".bash_profile") {
		t.Fatalf("command = %q, want profile bootstrap", runInput.Command)
	}
	if !strings.Contains(runInput.Command, "HOME=/home/carrier") {
		t.Fatalf("command = %q, want fixed carrier home", runInput.Command)
	}
	if !strings.Contains(runInput.Command, "CODEX_HOME=/home/carrier/.codex") {
		t.Fatalf("command = %q, want fixed codex home", runInput.Command)
	}
	if !strings.Contains(runInput.Command, "codex exec") {
		t.Fatalf("command = %q, want codex exec", runInput.Command)
	}

	carrierBinding, err := carrierSvc.GetBinding(order.ID, "ms_1")
	if err != nil {
		t.Fatalf("get carrier binding: %v", err)
	}
	jobs, err := carrierSvc.ListJobs(carrierBinding.ID)
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(jobs))
	}
	if jobs[0].State != carrier.JobStateCompleted {
		t.Fatalf("job state = %s, want completed", jobs[0].State)
	}
}
