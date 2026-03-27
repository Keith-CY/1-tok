package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
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
	versionInputs            []carrierclient.CodeAgentVersionInput
	versionCalls             int
	versionResult            carrierclient.CodeAgentVersionResult
	versionErr               error
	postInstallVersionResult carrierclient.CodeAgentVersionResult
	installInputs            []carrierclient.CodeAgentInstallInput
	installErr               error
	runInputs                []carrierclient.CodeAgentRunInput
	runHook                  func(carrierclient.CodeAgentRunInput) error
	runResult                carrierclient.CodeAgentRunResult
	runErr                   error
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

func (s *stubCodeAgentClient) GetCodeAgentVersion(_ context.Context, input carrierclient.CodeAgentVersionInput) (carrierclient.CodeAgentVersionResult, error) {
	s.versionInputs = append(s.versionInputs, input)
	s.versionCalls++
	if s.versionCalls == 1 && s.versionErr != nil {
		return carrierclient.CodeAgentVersionResult{}, s.versionErr
	}
	if s.versionCalls > 1 && strings.TrimSpace(s.postInstallVersionResult.Value) != "" {
		return s.postInstallVersionResult, nil
	}
	if strings.TrimSpace(s.versionResult.Value) != "" {
		return s.versionResult, nil
	}
	return carrierclient.CodeAgentVersionResult{
		Backend: firstNonEmptyString(input.Backend, "codex"),
		Value:   "codex-cli 0.116.0",
	}, nil
}

func (s *stubCodeAgentClient) InstallCodeAgent(_ context.Context, input carrierclient.CodeAgentInstallInput) error {
	s.installInputs = append(s.installInputs, input)
	return s.installErr
}

func (s *stubCodeAgentClient) RunCodeAgent(_ context.Context, input carrierclient.CodeAgentRunInput) (carrierclient.CodeAgentRunResult, error) {
	s.runInputs = append(s.runInputs, input)
	if s.runHook != nil {
		if err := s.runHook(input); err != nil {
			return carrierclient.CodeAgentRunResult{}, err
		}
	}
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
	report := strings.TrimSpace(`
# Summary
Shortlisted one provider.

## Findings
- Pricing fit the posted budget.

## Recommendation
Proceed with the shortlisted provider.
`)

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
				Output:         report,
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
	if updatedOrder.Milestones[0].Summary != report {
		t.Fatalf("milestone summary = %q, want execution output markdown", updatedOrder.Milestones[0].Summary)
	}

	if len(client.runInputs) != 2 {
		t.Fatalf("run inputs = %d, want 2", len(client.runInputs))
	}
	promptInput := client.runInputs[0]
	if promptInput.Capability != "write_file" {
		t.Fatalf("prompt capability = %s, want write_file", promptInput.Capability)
	}
	if promptInput.Path != "/workspace/1tok/"+order.ID+"/ms_1/prompt.md" {
		t.Fatalf("prompt path = %q, want prompt path", promptInput.Path)
	}
	if promptInput.WriteMode != "overwrite" {
		t.Fatalf("prompt write mode = %q, want overwrite", promptInput.WriteMode)
	}
	if !strings.Contains(promptInput.Content, "Do not browse the web or use external network tools.") {
		t.Fatalf("prompt content = %q, want no-browsing instruction", promptInput.Content)
	}

	runInput := client.runInputs[1]
	if runInput.HostID != "host_1" || runInput.AgentID != "agent_1" || runInput.Backend != "codex" {
		t.Fatalf("unexpected run input routing: %+v", runInput)
	}
	if runInput.Capability != "run_shell" {
		t.Fatalf("capability = %s, want run_shell", runInput.Capability)
	}
	if runInput.StdoutPath != "" {
		t.Fatalf("stdout path = %q, want inline output capture", runInput.StdoutPath)
	}
	if runInput.StderrPath != "" {
		t.Fatalf("stderr path = %q, want shell-managed stderr capture", runInput.StderrPath)
	}
	if runInput.CWD != "/workspace/1tok/"+order.ID+"/ms_1" {
		t.Fatalf("cwd = %q, want report directory", runInput.CWD)
	}
	if !strings.Contains(runInput.Command, ".bash_profile") {
		t.Fatalf("command = %q, want profile bootstrap", runInput.Command)
	}
	if !strings.HasPrefix(runInput.Command, "bash -lc ") {
		t.Fatalf("command = %q, want bash -lc wrapper", runInput.Command)
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
	if !strings.Contains(runInput.Command, "--cd") {
		t.Fatalf("command = %q, want explicit codex workdir", runInput.Command)
	}
	if strings.Contains(runInput.Command, "-a never") {
		t.Fatalf("command = %q, want legacy codex approval flag removed", runInput.Command)
	}
	if strings.Contains(runInput.Command, "--sandbox") {
		t.Fatalf("command = %q, want sandbox selection to come from remote codex config", runInput.Command)
	}
	if !strings.Contains(runInput.Command, "tee /workspace/1tok/"+order.ID+"/ms_1/result.md.stdout.log < /workspace/1tok/"+order.ID+"/ms_1/result.md") {
		t.Fatalf("command = %q, want tee-based report capture", runInput.Command)
	}
	if !strings.Contains(runInput.Command, "exec 2>>/workspace/1tok/"+order.ID+"/ms_1/result.md.stderr.log") {
		t.Fatalf("command = %q, want shell-managed stderr log", runInput.Command)
	}
	if !strings.Contains(runInput.Command, "prompt=$(cat '/workspace/1tok/"+order.ID+"/ms_1/prompt.md')") {
		t.Fatalf("command = %q, want prompt file staging", runInput.Command)
	}
	if strings.Contains(runInput.Command, "Do not browse the web or use external network tools.") {
		t.Fatalf("command = %q, unexpectedly inlined prompt text", runInput.Command)
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

func TestCarrierAwardExecutorPersistsReportViaCarrierCallback(t *testing.T) {
	app := platform.NewAppWithMemory()
	carrierSvc := carrier.NewService()

	providerBinding, err := app.RegisterCarrierBinding(platform.ProviderCarrierBinding{
		ProviderOrgID:  "provider_1",
		CarrierBaseURL: "https://carrier.example.com",
		HostID:         "host_1",
		AgentID:        "agent_1",
		Backend:        "codex",
		WorkspaceRoot:  "/workspace",
		CallbackSecret: "callback-secret",
		CallbackKeyID:  "callback-key-1",
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

	srv, err := NewServerWithOptionsE(Options{App: app, Carrier: carrierSvc})
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	callbackServer := httptest.NewServer(srv)
	defer callbackServer.Close()
	t.Setenv("CARRIER_CALLBACK_BASE_URL", callbackServer.URL)

	report := strings.TrimSpace(`
# Summary
Three vendors fit the brief.

## Findings
- Vendor A is the fastest to onboard.
- Vendor B is the cheapest at pilot scale.

## Recommendation
Start with Vendor A and benchmark Vendor B in reserve.
`)

	client := &stubCodeAgentClient{
		runHook: func(input carrierclient.CodeAgentRunInput) error {
			if input.Capability != carrierRunCapability {
				return nil
			}
			binding, err := carrierSvc.GetBinding(order.ID, "ms_1")
			if err != nil {
				return err
			}
			jobs, err := carrierSvc.ListJobs(binding.ID)
			if err != nil {
				return err
			}
			if len(jobs) != 1 {
				return errors.New("expected one carrier job")
			}
			return sendTestCarrierIntegrationCallback(callbackServer.URL, providerBinding.CallbackSecret, providerBinding.CallbackKeyID, carrier.IntegrationCallbackEnvelope{
				EventID:            jobs[0].ID + "-ready",
				Sequence:           1,
				EventType:          "milestone.ready",
				BindingID:          binding.ID,
				CarrierExecutionID: jobs[0].ID,
				CreatedAt:          time.Now().UTC().Format(time.RFC3339),
				Payload: map[string]any{
					"jobId":   jobs[0].ID,
					"output":  "/workspace/1tok/" + order.ID + "/ms_1/result.md",
					"summary": report,
				},
			})
		},
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
	if updatedOrder.Milestones[0].Summary != report {
		t.Fatalf("milestone summary = %q, want callback markdown", updatedOrder.Milestones[0].Summary)
	}
	if len(client.runInputs) != 2 {
		t.Fatalf("run inputs = %d, want 2", len(client.runInputs))
	}
	if client.runInputs[0].Capability != "write_file" {
		t.Fatalf("prompt capability = %s, want write_file", client.runInputs[0].Capability)
	}
	if client.runInputs[1].StdoutPath == "" || client.runInputs[1].StderrPath == "" {
		t.Fatalf("run stdout/stderr paths = %q/%q, want both set", client.runInputs[1].StdoutPath, client.runInputs[1].StderrPath)
	}
	if !strings.Contains(client.runInputs[1].Command, "/api/v1/carrier/callbacks/events") {
		t.Fatalf("command = %q, want carrier callback endpoint", client.runInputs[1].Command)
	}
	if !strings.Contains(client.runInputs[1].Command, "milestone.ready") {
		t.Fatalf("command = %q, want milestone.ready callback event", client.runInputs[1].Command)
	}
	if !strings.Contains(client.runInputs[1].Command, "X-Carrier-Key-Id") {
		t.Fatalf("command = %q, want callback key header", client.runInputs[1].Command)
	}
	if strings.Contains(client.runInputs[1].Command, "Return only the delivery note markdown.") {
		t.Fatalf("command = %q, unexpectedly inlined prompt text", client.runInputs[1].Command)
	}
	assertCarrierStrictPolicySafeCommand(t, client.runInputs[1].Command)
}

func TestBuildCarrierRunCommandWithCallbackAvoidsStrictPolicyAskPatterns(t *testing.T) {
	t.Parallel()

	command := buildCarrierRunCommand(
		"/workspace/1tok/ord_99/ms_1",
		"/workspace/1tok/ord_99/ms_1/prompt.md",
		"/workspace/1tok/ord_99/ms_1/result.md",
		carrierReportCallbackConfig{
			BaseURL:        "https://api.1-tok.pro",
			BindingID:      "bind_1",
			JobID:          "job_1",
			ReportPath:     "/workspace/1tok/ord_99/ms_1/result.md",
			CallbackSecret: "callback-secret",
			CallbackKeyID:  "callback-key-1",
		},
	)

	if !strings.Contains(command, "/api/v1/carrier/callbacks/events") {
		t.Fatalf("command = %q, want callback endpoint", command)
	}
	if !strings.Contains(command, "prompt=$(cat '/workspace/1tok/ord_99/ms_1/prompt.md')") {
		t.Fatalf("command = %q, want prompt file staging", command)
	}
	if !strings.Contains(command, "set -eo pipefail") {
		t.Fatalf("command = %q, want pipefail", command)
	}
	if !strings.Contains(command, "codex exec --cd '/workspace/1tok/ord_99/ms_1' --skip-git-repo-check \"$prompt\" | tee '/workspace/1tok/ord_99/ms_1/result.md'") {
		t.Fatalf("command = %q, want stdout tee capture", command)
	}
	assertCarrierStrictPolicySafeCommand(t, command)
}

func TestCarrierAwardExecutorReadsBackReportWhenRunResultOmitsOutput(t *testing.T) {
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

	report := "# Delivery\n\nThe carrier returned the report body."
	client := &stubCodeAgentClient{}
	client.runHook = func(input carrierclient.CodeAgentRunInput) error {
		switch len(client.runInputs) {
		case 1, 2:
			client.runResult = carrierclient.CodeAgentRunResult{
				Backend: "codex",
				Result: carrierclient.CodeAgentRunOutput{
					OK:             true,
					PolicyDecision: "allow",
				},
			}
		case 3:
			client.runResult = carrierclient.CodeAgentRunResult{
				Backend: "codex",
				Result: carrierclient.CodeAgentRunOutput{
					OK:             true,
					PolicyDecision: "allow",
					Output:         report,
				},
			}
		}
		return nil
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
	if updatedOrder.Milestones[0].Summary != report {
		t.Fatalf("milestone summary = %q, want readback markdown", updatedOrder.Milestones[0].Summary)
	}
	if len(client.runInputs) != 3 {
		t.Fatalf("run inputs = %d, want 3", len(client.runInputs))
	}
	if client.runInputs[2].Capability != "run_shell" {
		t.Fatalf("readback capability = %s, want run_shell", client.runInputs[2].Capability)
	}
	if client.runInputs[2].StdoutPath != "" {
		t.Fatalf("readback stdout path = %q, want empty", client.runInputs[2].StdoutPath)
	}
	if !strings.Contains(client.runInputs[2].Command, "cat ") {
		t.Fatalf("readback command = %q, want cat report", client.runInputs[2].Command)
	}
}

func TestCarrierAwardExecutorFailsJobWithOutputPathsWhenCommandExitsNonZero(t *testing.T) {
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
				OK:             false,
				PolicyDecision: "allow",
				Stderr:         "unknown option '--output-last-message'",
				Stdout:         "{\"event\":\"error\"}",
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

	err = executor.Execute(context.Background(), carrierAwardExecutionInput{
		RFQ:     awardedRFQ,
		Order:   order,
		Binding: providerBinding,
	})
	if err == nil {
		t.Fatal("expected carrier execution error")
	}
	if !strings.Contains(err.Error(), "carrier command failed") {
		t.Fatalf("error = %q, want carrier command failure", err)
	}
	if !strings.Contains(err.Error(), "result.md.stdout.log") || !strings.Contains(err.Error(), "result.md.stderr.log") {
		t.Fatalf("error = %q, want stdout/stderr paths", err)
	}
	if !strings.Contains(err.Error(), "carrier_stderr=") || !strings.Contains(err.Error(), "unknown option '--output-last-message'") {
		t.Fatalf("error = %q, want surfaced carrier stderr", err)
	}
	if !strings.Contains(err.Error(), "carrier_stdout=") || !strings.Contains(err.Error(), "{\"event\":\"error\"}") {
		t.Fatalf("error = %q, want surfaced carrier stdout", err)
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
	if jobs[0].State != carrier.JobStateFailed {
		t.Fatalf("job state = %s, want failed", jobs[0].State)
	}
}

func TestCarrierAwardExecutorInstallsCodeAgentWhenVersionProbeFails(t *testing.T) {
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
		versionErr: errors.New("not installed"),
		postInstallVersionResult: carrierclient.CodeAgentVersionResult{
			Backend: "codex",
			Value:   "codex-cli 0.116.0",
		},
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

	if len(client.installInputs) != 1 {
		t.Fatalf("install inputs = %d, want 1", len(client.installInputs))
	}
	if client.installInputs[0].HostID != "host_1" || client.installInputs[0].AgentID != "agent_1" {
		t.Fatalf("unexpected install input: %+v", client.installInputs[0])
	}
	if len(client.versionInputs) != 2 {
		t.Fatalf("version inputs = %d, want 2", len(client.versionInputs))
	}
}

func sendTestCarrierIntegrationCallback(baseURL, secret, keyID string, envelope carrier.IntegrationCallbackEnvelope) error {
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/v1/carrier/callbacks/events", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(keyID) != "" {
		req.Header.Set("X-Carrier-Key-Id", keyID)
	}
	req.Header.Set("X-Carrier-Signature", carrier.SignIntegrationCallbackBody(secret, body))
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return errors.New("carrier callback rejected")
	}
	return nil
}

func assertCarrierStrictPolicySafeCommand(t *testing.T, command string) {
	t.Helper()

	lowered := strings.ToLower(command)
	for _, pattern := range []string{"curl ", "wget ", "nc ", "ssh ", "scp "} {
		if strings.Contains(lowered, pattern) {
			t.Fatalf("command = %q, unexpectedly matched strict policy ask pattern %q", command, pattern)
		}
	}
}
