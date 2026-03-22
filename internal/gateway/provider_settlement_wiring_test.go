package gateway

import (
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/core"
	"github.com/chenyu/1-tok/internal/platform"
)

type stubGatewaySettlementProvisioner struct {
	calls int
}

func (s *stubGatewaySettlementProvisioner) EnsureProviderLiquidity(input platform.EnsureProviderLiquidityInput) (platform.EnsureProviderLiquidityResult, error) {
	s.calls++
	return platform.EnsureProviderLiquidityResult{
		ChannelID:           "ch_gateway_1",
		ReuseSource:         platform.ProviderLiquidityReuseNewChannel,
		ReadyChannelCount:   1,
		TotalSpendableCents: input.NeededReserveCents + 1_000,
	}, nil
}

func TestNewServerWithOptionsE_WiresProviderSettlementProvisioner(t *testing.T) {
	app := platform.NewAppWithMemory()

	carrierBinding, err := app.RegisterCarrierBinding(platform.ProviderCarrierBinding{
		ProviderOrgID:  "provider_1",
		CarrierBaseURL: "https://carrier.example.com",
		HostID:         "host_1",
	})
	if err != nil {
		t.Fatalf("register carrier binding: %v", err)
	}
	if _, err := app.VerifyCarrierBinding(carrierBinding.ID); err != nil {
		t.Fatalf("verify carrier binding: %v", err)
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
		Title:              "wired provisioner rfq",
		Category:           "ops",
		Scope:              "ensure gateway wires provisioner",
		BudgetCents:        5_000,
		ResponseDeadlineAt: time.Date(2099, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}
	bid, err := app.CreateBid(rfq.ID, platform.CreateBidInput{
		ProviderOrgID: "provider_1",
		Message:       "bid",
		Milestones: []platform.BidMilestoneInput{{
			ID:             "ms_1",
			Title:          "Execution",
			BasePriceCents: 4_000,
			BudgetCents:    5_000,
		}},
	})
	if err != nil {
		t.Fatalf("create bid: %v", err)
	}

	provisioner := &stubGatewaySettlementProvisioner{}
	if _, err := NewServerWithOptionsE(Options{
		App:                           app,
		ProviderSettlementProvisioner: provisioner,
	}); err != nil {
		t.Fatalf("new server: %v", err)
	}

	_, order, err := app.AwardRFQ(rfq.ID, platform.AwardRFQInput{
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
}
