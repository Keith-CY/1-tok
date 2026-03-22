package platform

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/core"
)

type stubSettlementProvisioner struct {
	calls  int
	inputs []EnsureProviderLiquidityInput
	result EnsureProviderLiquidityResult
	err    error
}

func (s *stubSettlementProvisioner) EnsureProviderLiquidity(input EnsureProviderLiquidityInput) (EnsureProviderLiquidityResult, error) {
	s.calls++
	s.inputs = append(s.inputs, input)
	if s.err != nil {
		return EnsureProviderLiquidityResult{}, s.err
	}
	return s.result, nil
}

func registerActiveCarrierBinding(t *testing.T, app *App, providerOrgID string) {
	t.Helper()

	binding, err := app.RegisterCarrierBinding(ProviderCarrierBinding{
		ProviderOrgID:  providerOrgID,
		CarrierBaseURL: "https://carrier.example.com",
		HostID:         "host_1",
	})
	if err != nil {
		t.Fatalf("register carrier binding: %v", err)
	}
	if _, err := app.VerifyCarrierBinding(binding.ID); err != nil {
		t.Fatalf("verify carrier binding: %v", err)
	}
}

func createCarrierBackedRFQAndBid(t *testing.T, app *App, buyerOrgID, providerOrgID string, budgetCents int64) (RFQ, Bid) {
	t.Helper()

	rfq, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID:         buyerOrgID,
		Title:              "Carrier settlement order",
		Category:           "ops",
		Scope:              "run provider settlement flow",
		BudgetCents:        budgetCents,
		ResponseDeadlineAt: time.Date(2099, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}

	bid, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: providerOrgID,
		Message:       "bid",
		QuoteCents:    budgetCents,
		Milestones: []BidMilestoneInput{{
			ID:             "ms_1",
			Title:          "Execution",
			BasePriceCents: budgetCents,
			BudgetCents:    budgetCents,
		}},
	})
	if err != nil {
		t.Fatalf("create bid: %v", err)
	}

	return rfq, bid
}

func TestAwardRFQ_RequiresActiveSettlementBindingForCarrierProvider(t *testing.T) {
	app := NewAppWithMemory()
	registerActiveCarrierBinding(t, app, "org_provider")
	rfq, bid := createCarrierBackedRFQAndBid(t, app, "org_buyer", "org_provider", 5_000)

	_, _, err := app.AwardRFQ(rfq.ID, AwardRFQInput{
		BidID:       bid.ID,
		FundingMode: core.FundingModePrepaid,
	})
	if !errors.Is(err, ErrProviderSettlementBindingRequired) {
		t.Fatalf("expected ErrProviderSettlementBindingRequired, got %v", err)
	}
}

func TestAwardRFQ_ProvisionsProviderLiquidityPool(t *testing.T) {
	app := NewAppWithMemory()
	registerActiveCarrierBinding(t, app, "org_provider")

	settlementBinding, err := app.RegisterProviderSettlementBinding(ProviderSettlementBinding{
		ProviderOrgID:         "org_provider",
		Asset:                 "USDI",
		PeerID:                "peer_provider",
		P2PAddress:            "/dns4/provider/tcp/8228/p2p/peer_provider",
		PaymentRequestBaseURL: "https://carrier.example.com/payment-requests",
		UDTTypeScript: UDTTypeScript{
			CodeHash: "0xudt",
			HashType: "type",
			Args:     "0x01",
		},
		OwnershipProof: "proof_1",
	})
	if err != nil {
		t.Fatalf("register settlement binding: %v", err)
	}
	if _, err := app.VerifyProviderSettlementBinding(settlementBinding.ID); err != nil {
		t.Fatalf("verify settlement binding: %v", err)
	}

	provisioner := &stubSettlementProvisioner{
		result: EnsureProviderLiquidityResult{
			ChannelID:           "ch_1",
			ReuseSource:         ProviderLiquidityReuseNewChannel,
			ReadyChannelCount:   1,
			TotalSpendableCents: 7_000,
		},
	}
	app.SetProviderSettlementProvisioner(provisioner)

	rfq, bid := createCarrierBackedRFQAndBid(t, app, "org_buyer", "org_provider", 5_000)
	_, order, err := app.AwardRFQ(rfq.ID, AwardRFQInput{
		BidID:       bid.ID,
		FundingMode: core.FundingModePrepaid,
	})
	if err != nil {
		t.Fatalf("award rfq: %v", err)
	}
	if order == nil {
		t.Fatal("expected order")
	}
	if provisioner.calls != 1 {
		t.Fatalf("provisioner calls = %d, want 1", provisioner.calls)
	}

	pool, err := app.GetProviderSettlementPool("org_provider")
	if err != nil {
		t.Fatalf("get pool: %v", err)
	}
	if pool.Status != ProviderLiquidityPoolStatusHealthy {
		t.Fatalf("pool status = %s, want healthy", pool.Status)
	}
	if pool.ReadyChannelCount != 1 {
		t.Fatalf("ready channels = %d, want 1", pool.ReadyChannelCount)
	}
	if pool.ReservedOutstandingCents != 5_500 {
		t.Fatalf("reserved cents = %d, want 5500", pool.ReservedOutstandingCents)
	}
	if pool.AvailableToAllocateCents != 1_500 {
		t.Fatalf("available cents = %d, want 1500", pool.AvailableToAllocateCents)
	}

	reservation, err := app.GetProviderSettlementReservation(order.ID)
	if err != nil {
		t.Fatalf("get reservation: %v", err)
	}
	if reservation.ChannelID != "ch_1" {
		t.Fatalf("channel id = %s, want ch_1", reservation.ChannelID)
	}
	if reservation.ReuseSource != ProviderLiquidityReuseNewChannel {
		t.Fatalf("reuse source = %s", reservation.ReuseSource)
	}
}

func TestReportProviderSettlementDisconnectPausesRunningOrders(t *testing.T) {
	app := NewAppWithMemory()
	registerActiveCarrierBinding(t, app, "org_provider")

	settlementBinding, _ := app.RegisterProviderSettlementBinding(ProviderSettlementBinding{
		ProviderOrgID:         "org_provider",
		Asset:                 "USDI",
		PeerID:                "peer_provider",
		P2PAddress:            "/dns4/provider/tcp/8228/p2p/peer_provider",
		PaymentRequestBaseURL: "https://carrier.example.com/payment-requests",
		UDTTypeScript: UDTTypeScript{
			CodeHash: "0xudt",
			HashType: "type",
			Args:     "0x01",
		},
		OwnershipProof: "proof_1",
	})
	app.VerifyProviderSettlementBinding(settlementBinding.ID)
	app.SetProviderSettlementProvisioner(&stubSettlementProvisioner{
		result: EnsureProviderLiquidityResult{
			ChannelID:           "ch_1",
			ReuseSource:         ProviderLiquidityReuseNewChannel,
			ReadyChannelCount:   1,
			TotalSpendableCents: 7_000,
		},
	})

	rfq, bid := createCarrierBackedRFQAndBid(t, app, "org_buyer", "org_provider", 5_000)
	_, order, err := app.AwardRFQ(rfq.ID, AwardRFQInput{
		BidID:       bid.ID,
		FundingMode: core.FundingModePrepaid,
	})
	if err != nil {
		t.Fatalf("award rfq: %v", err)
	}

	if err := app.ReportProviderSettlementDisconnect("org_provider", "peer offline"); err != nil {
		t.Fatalf("report disconnect: %v", err)
	}

	pool, err := app.GetProviderSettlementPool("org_provider")
	if err != nil {
		t.Fatalf("get pool: %v", err)
	}
	if pool.Status != ProviderLiquidityPoolStatusDisconnected {
		t.Fatalf("pool status = %s, want disconnected", pool.Status)
	}
	if pool.DisconnectReason != "peer offline" {
		t.Fatalf("disconnect reason = %q", pool.DisconnectReason)
	}

	updated, err := app.GetOrder(order.ID)
	if err != nil {
		t.Fatalf("get order: %v", err)
	}
	if updated.Status != core.OrderStatusAwaitingPaymentRail {
		t.Fatalf("order status = %s, want awaiting_payment_rail", updated.Status)
	}
	if updated.Milestones[0].State != core.MilestoneStatePaused {
		t.Fatalf("milestone state = %s, want paused", updated.Milestones[0].State)
	}
	if !slices.Contains(updated.Milestones[0].AnomalyFlags, "provider_settlement_disconnected") {
		t.Fatalf("expected anomaly flag, got %v", updated.Milestones[0].AnomalyFlags)
	}
}

func TestAwardRFQ_BlockedWhenProviderLiquidityPoolIsDisconnected(t *testing.T) {
	app := NewAppWithMemory()
	registerActiveCarrierBinding(t, app, "org_provider")

	settlementBinding, _ := app.RegisterProviderSettlementBinding(ProviderSettlementBinding{
		ProviderOrgID:         "org_provider",
		Asset:                 "USDI",
		PeerID:                "peer_provider",
		P2PAddress:            "/dns4/provider/tcp/8228/p2p/peer_provider",
		PaymentRequestBaseURL: "https://carrier.example.com/payment-requests",
		UDTTypeScript: UDTTypeScript{
			CodeHash: "0xudt",
			HashType: "type",
			Args:     "0x01",
		},
		OwnershipProof: "proof_1",
	})
	app.VerifyProviderSettlementBinding(settlementBinding.ID)
	app.SetProviderSettlementProvisioner(&stubSettlementProvisioner{
		result: EnsureProviderLiquidityResult{
			ChannelID:           "ch_1",
			ReuseSource:         ProviderLiquidityReuseNewChannel,
			ReadyChannelCount:   1,
			TotalSpendableCents: 7_000,
		},
	})

	rfqOne, bidOne := createCarrierBackedRFQAndBid(t, app, "org_buyer", "org_provider", 5_000)
	if _, _, err := app.AwardRFQ(rfqOne.ID, AwardRFQInput{
		BidID:       bidOne.ID,
		FundingMode: core.FundingModePrepaid,
	}); err != nil {
		t.Fatalf("first award rfq: %v", err)
	}
	if err := app.ReportProviderSettlementDisconnect("org_provider", "provider closed channel"); err != nil {
		t.Fatalf("report disconnect: %v", err)
	}

	rfqTwo, bidTwo := createCarrierBackedRFQAndBid(t, app, "org_buyer_2", "org_provider", 4_000)
	_, _, err := app.AwardRFQ(rfqTwo.ID, AwardRFQInput{
		BidID:       bidTwo.ID,
		FundingMode: core.FundingModePrepaid,
	})
	if !errors.Is(err, ErrProviderSettlementPoolUnavailable) {
		t.Fatalf("expected ErrProviderSettlementPoolUnavailable, got %v", err)
	}
}
