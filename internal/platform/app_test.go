package platform

import (
	"errors"
	"testing"
	"time"

	"github.com/chenyu/1-tok/internal/core"
)

func TestAppCreateOrderPersistsAndListsOrders(t *testing.T) {
	app := NewAppWithMemory()

	order, err := app.CreateOrder(CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Agent operations",
		FundingMode:   core.FundingModeCredit,
		CreditLineID:  "credit_1",
		Milestones: []CreateMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Plan",
				BasePriceCents: 1000,
				BudgetCents:    1400,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	orders, err := app.ListOrders()
	if err != nil {
		t.Fatalf("list orders: %v", err)
	}

	if len(orders) != 1 {
		t.Fatalf("expected one order, got %d", len(orders))
	}

	if orders[0].ID != order.ID {
		t.Fatalf("expected order %s, got %s", order.ID, orders[0].ID)
	}
}

func TestAppCreateRFQPersistsAndListsRFQs(t *testing.T) {
	app := NewAppWithMemory()

	rfq, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID:         "buyer_1",
		Title:              "Need carrier-backed triage",
		Category:           "agent-ops",
		Scope:              "Investigate failures, stabilize runtime, and report next steps.",
		BudgetCents:        9_500,
		ResponseDeadlineAt: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}

	rfqs, err := app.ListRFQs()
	if err != nil {
		t.Fatalf("list rfqs: %v", err)
	}

	if len(rfqs) != 1 {
		t.Fatalf("expected one rfq, got %d", len(rfqs))
	}

	if rfqs[0].ID != rfq.ID {
		t.Fatalf("expected rfq %s, got %s", rfq.ID, rfqs[0].ID)
	}

	if rfqs[0].Status != RFQStatusOpen {
		t.Fatalf("expected open rfq, got %s", rfqs[0].Status)
	}
}

func TestAppCreateBidPersistsAndListsRFQBids(t *testing.T) {
	app := NewAppWithMemory()

	rfq, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID:         "buyer_1",
		Title:              "Need carrier-backed triage",
		Category:           "agent-ops",
		Scope:              "Investigate failures, stabilize runtime, and report next steps.",
		BudgetCents:        9_500,
		ResponseDeadlineAt: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}

	bid, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "provider_1",
		Message:       "We can take the incident and deliver a stabilization report.",
		QuoteCents:    7_200,
		Milestones: []BidMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Triage",
				BasePriceCents: 3000,
				BudgetCents:    3600,
			},
			{
				ID:             "ms_2",
				Title:          "Stabilize",
				BasePriceCents: 4200,
				BudgetCents:    5000,
			},
		},
	})
	if err != nil {
		t.Fatalf("create bid: %v", err)
	}

	bids, err := app.ListRFQBids(rfq.ID)
	if err != nil {
		t.Fatalf("list bids: %v", err)
	}

	if len(bids) != 1 {
		t.Fatalf("expected one bid, got %d", len(bids))
	}

	if bids[0].ID != bid.ID {
		t.Fatalf("expected bid %s, got %s", bid.ID, bids[0].ID)
	}

	if bids[0].Status != BidStatusOpen {
		t.Fatalf("expected open bid, got %s", bids[0].Status)
	}
}

func TestAppAwardRFQCreatesOrderAndMarksWinningBid(t *testing.T) {
	app := NewAppWithMemory()

	rfq, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID:         "buyer_1",
		Title:              "Need carrier-backed triage",
		Category:           "agent-ops",
		Scope:              "Investigate failures, stabilize runtime, and report next steps.",
		BudgetCents:        9_500,
		ResponseDeadlineAt: time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("create rfq: %v", err)
	}

	winningBid, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "provider_1",
		Message:       "We can take the incident and deliver a stabilization report.",
		QuoteCents:    7_200,
		Milestones: []BidMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Triage",
				BasePriceCents: 3000,
				BudgetCents:    3600,
			},
		},
	})
	if err != nil {
		t.Fatalf("create winning bid: %v", err)
	}

	if _, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "provider_2",
		Message:       "Alternate bid",
		QuoteCents:    7_500,
		Milestones: []BidMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Triage",
				BasePriceCents: 3200,
				BudgetCents:    3600,
			},
		},
	}); err != nil {
		t.Fatalf("create losing bid: %v", err)
	}

	awardedRFQ, order, err := app.AwardRFQ(rfq.ID, AwardRFQInput{
		BidID:        winningBid.ID,
		FundingMode:  core.FundingModeCredit,
		CreditLineID: "credit_1",
	})
	if err != nil {
		t.Fatalf("award rfq: %v", err)
	}

	if awardedRFQ.Status != RFQStatusAwarded {
		t.Fatalf("expected awarded rfq, got %s", awardedRFQ.Status)
	}

	if awardedRFQ.AwardedBidID != winningBid.ID {
		t.Fatalf("expected awarded bid %s, got %s", winningBid.ID, awardedRFQ.AwardedBidID)
	}

	if order.ProviderOrgID != "provider_1" || order.BuyerOrgID != "buyer_1" {
		t.Fatalf("unexpected order parties: %+v", order)
	}

	bids, err := app.ListRFQBids(rfq.ID)
	if err != nil {
		t.Fatalf("list bids: %v", err)
	}

	if bids[0].Status != BidStatusAwarded && bids[1].Status != BidStatusAwarded {
		t.Fatalf("expected one awarded bid, got %+v", bids)
	}
}

func TestAppOpenDisputePersistsAndListsDisputes(t *testing.T) {
	app := NewAppWithMemory()
	order, err := app.CreateOrder(CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Agent operations",
		FundingMode:   core.FundingModeCredit,
		CreditLineID:  "credit_1",
		Milestones: []CreateMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Plan",
				BasePriceCents: 1000,
				BudgetCents:    1400,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	if _, _, err := app.SettleMilestone(order.ID, SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "done",
		Source:      "carrier",
		OccurredAt:  time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("settle milestone: %v", err)
	}

	_, _, _, err = app.OpenDispute(order.ID, OpenDisputeInput{
		MilestoneID: "ms_1",
		Reason:      "carrier output was incomplete",
		RefundCents: 800,
	})
	if err != nil {
		t.Fatalf("open dispute: %v", err)
	}

	disputes, err := app.ListDisputes()
	if err != nil {
		t.Fatalf("list disputes: %v", err)
	}
	if len(disputes) != 1 {
		t.Fatalf("expected one dispute, got %d", len(disputes))
	}
	if disputes[0].OrderID != order.ID || disputes[0].Reason != "carrier output was incomplete" {
		t.Fatalf("unexpected dispute: %+v", disputes[0])
	}
}

func TestAppSettleMilestoneAdvancesNextMilestone(t *testing.T) {
	app := NewAppWithMemory()
	order, err := app.CreateOrder(CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Agent operations",
		FundingMode:   core.FundingModeCredit,
		CreditLineID:  "credit_1",
		Milestones: []CreateMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Plan",
				BasePriceCents: 1000,
				BudgetCents:    1400,
			},
			{
				ID:             "ms_2",
				Title:          "Deliver",
				BasePriceCents: 500,
				BudgetCents:    900,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	updated, entry, err := app.SettleMilestone(order.ID, SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "done",
		Source:      "carrier",
		OccurredAt:  time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("settle milestone: %v", err)
	}

	if entry.Kind != core.LedgerEntryKindPlatformExposure {
		t.Fatalf("expected platform exposure, got %s", entry.Kind)
	}

	if updated.Milestones[1].State != core.MilestoneStateRunning {
		t.Fatalf("expected second milestone running, got %s", updated.Milestones[1].State)
	}
}

func TestAppResolveDisputeMarksMilestoneResolved(t *testing.T) {
	app := NewAppWithMemory()
	order, err := app.CreateOrder(CreateOrderInput{
		BuyerOrgID:    "buyer_1",
		ProviderOrgID: "provider_1",
		Title:         "Agent operations",
		FundingMode:   core.FundingModeCredit,
		CreditLineID:  "credit_1",
		Milestones: []CreateMilestoneInput{
			{
				ID:             "ms_1",
				Title:          "Plan",
				BasePriceCents: 1000,
				BudgetCents:    1400,
			},
		},
	})
	if err != nil {
		t.Fatalf("create order: %v", err)
	}

	if _, _, err := app.SettleMilestone(order.ID, SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "done",
		Source:      "carrier",
		OccurredAt:  time.Date(2026, 3, 12, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("settle milestone: %v", err)
	}

	if _, _, _, err := app.OpenDispute(order.ID, OpenDisputeInput{
		MilestoneID: "ms_1",
		Reason:      "carrier output was incomplete",
		RefundCents: 800,
	}); err != nil {
		t.Fatalf("open dispute: %v", err)
	}

	disputes, err := app.ListDisputes()
	if err != nil {
		t.Fatalf("list disputes: %v", err)
	}

	resolvedDispute, updatedOrder, err := app.ResolveDispute(disputes[0].ID, ResolveDisputeInput{
		Resolution: "Provider supplied corrected remediation evidence.",
		ResolvedBy: "ops_reviewer_1",
	})
	if err != nil {
		t.Fatalf("resolve dispute: %v", err)
	}

	if resolvedDispute.Status != core.DisputeStatusResolved {
		t.Fatalf("expected resolved dispute status, got %s", resolvedDispute.Status)
	}
	if resolvedDispute.Resolution != "Provider supplied corrected remediation evidence." {
		t.Fatalf("unexpected dispute resolution: %+v", resolvedDispute)
	}
	if resolvedDispute.ResolvedBy != "ops_reviewer_1" || resolvedDispute.ResolvedAt == nil {
		t.Fatalf("expected resolver metadata, got %+v", resolvedDispute)
	}
	if updatedOrder.Milestones[0].DisputeStatus != core.DisputeStatusResolved {
		t.Fatalf("expected milestone dispute status resolved, got %s", updatedOrder.Milestones[0].DisputeStatus)
	}
}

func TestListProviders(t *testing.T) {
	app := NewAppWithMemory()
	providers, err := app.ListProviders()
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 {
		t.Error("expected default providers")
	}
}

func TestListListings(t *testing.T) {
	app := NewAppWithMemory()
	listings, err := app.ListListings()
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) == 0 {
		t.Error("expected default listings")
	}
}

func TestGetRFQ(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Test", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	got, err := app.GetRFQ(rfq.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Test" {
		t.Errorf("title = %s, want Test", got.Title)
	}
}

func TestGetRFQ_NotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.GetRFQ("nonexistent")
	if !errors.Is(err, ErrRFQNotFound) {
		t.Errorf("expected ErrRFQNotFound, got %v", err)
	}
}

func TestListOrders(t *testing.T) {
	app := NewAppWithMemory()
	orders, err := app.ListOrders()
	if err != nil {
		t.Fatal(err)
	}
	if orders == nil {
		t.Error("expected non-nil slice")
	}
}

func TestListDisputes(t *testing.T) {
	app := NewAppWithMemory()
	disputes, err := app.ListDisputes()
	if err != nil {
		t.Fatal(err)
	}
	// Empty list may be nil or empty slice
	if len(disputes) != 0 {
		t.Errorf("expected 0 disputes, got %d", len(disputes))
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.GetOrder("nonexistent")
	if !errors.Is(err, core.ErrOrderNotFound) {
		t.Errorf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestCreateRFQ_MissingFields(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.CreateRFQ(CreateRFQInput{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestCreateRFQ_MissingDeadline(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Test", Category: "ai",
		Scope: "test", BudgetCents: 5000,
	})
	if err == nil {
		t.Error("expected error for missing deadline")
	}
}

func TestCreateOrder_MissingFields(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.CreateOrder(CreateOrderInput{})
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestSettleMilestone(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Settle test", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "Phase 1", BasePriceCents: 5000, BudgetCents: 5000},
			{ID: "ms_2", Title: "Phase 2", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	settled, entry, err := app.SettleMilestone(order.ID, core.SettleMilestoneInput{
		MilestoneID: "ms_1",
		Summary:     "Done",
	})
	if err != nil {
		t.Fatal(err)
	}
	if settled.Milestones[0].State != core.MilestoneStateSettled {
		t.Errorf("milestone state = %s, want settled", settled.Milestones[0].State)
	}
	if entry.Kind == "" {
		t.Error("expected ledger entry")
	}
	// ms_2 should advance to running
	if settled.Milestones[1].State != core.MilestoneStateRunning {
		t.Errorf("next milestone state = %s, want running", settled.Milestones[1].State)
	}
}

func TestRecordUsageCharge(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Usage test", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "Work", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	updated, charge, err := app.RecordUsageCharge(order.ID, RecordUsageChargeInput{
		MilestoneID: "ms_1",
		Kind:        "token",
		AmountCents: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if charge.AmountCents != 500 {
		t.Errorf("charge = %d, want 500", charge.AmountCents)
	}
	if len(updated.Milestones[0].UsageCharges) != 1 {
		t.Errorf("usage charges = %d, want 1", len(updated.Milestones[0].UsageCharges))
	}
}

func TestCreateMessage(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Msg test", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "Work", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	msg, err := app.CreateMessage(order.ID, "buyer", "Hello provider")
	if err != nil {
		t.Fatal(err)
	}
	if msg.Body != "Hello provider" {
		t.Errorf("body = %s", msg.Body)
	}
}
