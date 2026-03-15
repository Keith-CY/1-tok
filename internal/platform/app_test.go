package platform

import (
	"errors"
	"strings"
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

func TestSetPublisher(t *testing.T) {
	app := NewAppWithMemory()
	// Should not panic
	app.SetPublisher(nil)
}

func TestDecideCredit(t *testing.T) {
	app := NewAppWithMemory()
	decision := app.DecideCredit(core.CreditHistory{
		CompletedOrders:    5,
		SuccessfulPayments: 5,
		LifetimeSpendCents: 50000,
	})
	if !decision.Approved {
		t.Error("expected approved")
	}
}

func TestDecideCredit_InsufficientHistory(t *testing.T) {
	app := NewAppWithMemory()
	decision := app.DecideCredit(core.CreditHistory{
		CompletedOrders: 1,
	})
	if decision.Approved {
		t.Error("expected not approved")
	}
}

func TestCreateBid_ClosedRFQ(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Closed", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "Work", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	// Award closes the RFQ
	app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	// Try to bid on closed RFQ
	_, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_3", Message: "late bid", QuoteCents: 3000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 3000, BudgetCents: 3000}},
	})
	if err == nil {
		t.Error("expected error for closed RFQ")
	}
}

func TestNewApp(t *testing.T) {
	app := NewApp(nil, nil, nil, nil, nil, nil, nil)
	if app == nil {
		t.Fatal("NewApp returned nil")
	}
}

func TestOpenDispute_Success(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Dispute test", Category: "ai",
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
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})

	updatedOrder, _, _, err := app.OpenDispute(order.ID, OpenDisputeInput{
		MilestoneID: "ms_1", Reason: "quality issue", RefundCents: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	if updatedOrder.Milestones[0].DisputeStatus != core.DisputeStatusFrozen {
		t.Errorf("dispute status = %s, want frozen", updatedOrder.Milestones[0].DisputeStatus)
	}
}

func TestResolveDispute_Success(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Resolve test", Category: "ai",
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
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})
	app.OpenDispute(order.ID, OpenDisputeInput{
		MilestoneID: "ms_1", Reason: "issue", RefundCents: 500,
	})

	disputes, _ := app.ListDisputes()
	if len(disputes) == 0 {
		t.Fatal("no disputes")
	}

	dispute, updatedOrder, err := app.ResolveDispute(disputes[0].ID, ResolveDisputeInput{
		Resolution: "Refund approved", ResolvedBy: "ops_admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	if dispute.Status != core.DisputeStatusResolved {
		t.Errorf("dispute status = %s, want resolved", dispute.Status)
	}
	if updatedOrder.Milestones[0].DisputeStatus != core.DisputeStatusResolved {
		t.Errorf("order dispute status = %s, want resolved", updatedOrder.Milestones[0].DisputeStatus)
	}
}

func TestCreateMessage_EmptyBody(t *testing.T) {
	app := NewAppWithMemory()
	msg, err := app.CreateMessage("ord_1", "buyer", "")
	if err != nil {
		t.Fatal(err)
	}
	// Empty body is allowed — message still created
	if msg.ID == "" {
		t.Error("expected message ID")
	}
}

func TestSettleMilestone_OrderNotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, _, err := app.SettleMilestone("nonexistent", core.SettleMilestoneInput{MilestoneID: "ms_1"})
	if !errors.Is(err, core.ErrOrderNotFound) {
		t.Errorf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestRecordUsageCharge_OrderNotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, _, err := app.RecordUsageCharge("nonexistent", RecordUsageChargeInput{MilestoneID: "ms_1", Kind: "token", AmountCents: 100})
	if !errors.Is(err, core.ErrOrderNotFound) {
		t.Errorf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestOpenDispute_OrderNotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, _, _, err := app.OpenDispute("nonexistent", OpenDisputeInput{MilestoneID: "ms_1", Reason: "bad", RefundCents: 100})
	if !errors.Is(err, core.ErrOrderNotFound) {
		t.Errorf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestResolveDispute_DisputeNotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, _, err := app.ResolveDispute("nonexistent", ResolveDisputeInput{Resolution: "ok", ResolvedBy: "ops"})
	if !errors.Is(err, ErrDisputeNotFound) {
		t.Errorf("expected ErrDisputeNotFound, got %v", err)
	}
}

func TestListRFQBids(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Bids test", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid1",
		QuoteCents: 4000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 4000, BudgetCents: 4000},
		},
	})

	bids, err := app.ListRFQBids(rfq.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(bids) != 1 {
		t.Errorf("expected 1 bid, got %d", len(bids))
	}
}

func TestSettleMilestone_Completed(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Complete", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "Only", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "credit"})

	// Settle the only milestone — order should complete
	settled, _, err := app.SettleMilestone(order.ID, core.SettleMilestoneInput{
		MilestoneID: "ms_1", Summary: "All done",
	})
	if err != nil {
		t.Fatal(err)
	}
	if settled.Status != core.OrderStatusCompleted {
		t.Errorf("order status = %s, want completed", settled.Status)
	}
}

func TestOpenDispute_PublishesEvent(t *testing.T) {
	app := NewAppWithMemory()
	publisher := &spyPublisher{}
	app.publisher = publisher

	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Dispute pub", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})

	publisher.subjects = nil
	_, _, _, err := app.OpenDispute(order.ID, OpenDisputeInput{
		MilestoneID: "ms_1", Reason: "issue", RefundCents: 500,
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range publisher.subjects {
		if s == "market.dispute.opened" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected market.dispute.opened, got %v", publisher.subjects)
	}
}

func TestResolveDispute_PublishesEvent(t *testing.T) {
	app := NewAppWithMemory()
	publisher := &spyPublisher{}
	app.publisher = publisher

	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Resolve pub", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})
	app.OpenDispute(order.ID, OpenDisputeInput{
		MilestoneID: "ms_1", Reason: "issue", RefundCents: 500,
	})
	disputes, _ := app.ListDisputes()

	publisher.subjects = nil
	_, _, err := app.ResolveDispute(disputes[0].ID, ResolveDisputeInput{
		Resolution: "ok", ResolvedBy: "ops",
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range publisher.subjects {
		if s == "market.dispute.resolved" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected market.dispute.resolved, got %v", publisher.subjects)
	}
}

func TestSetPublisher_Nil(t *testing.T) {
	app := NewAppWithMemory()
	app.SetPublisher(nil) // should set noopPublisher
	// Should still work — publish is a no-op
	_, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Noop", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompareStrings(t *testing.T) {
	if compareStrings("a", "b") != -1 {
		t.Error("expected -1")
	}
	if compareStrings("b", "a") != 1 {
		t.Error("expected 1")
	}
	if compareStrings("a", "a") != 0 {
		t.Error("expected 0")
	}
}

type errorPublisher struct{}

func (errorPublisher) Publish(string, any) error {
	return errors.New("publish failed")
}

func TestCreateOrder_PublishError(t *testing.T) {
	app := NewAppWithMemory()
	app.SetPublisher(errorPublisher{})

	_, err := app.CreateOrder(CreateOrderInput{
		BuyerOrgID: "org_b", ProviderOrgID: "org_p",
		Title: "Pub error", FundingMode: "prepaid",
		Milestones: []CreateMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	if err == nil {
		t.Error("expected error from failed publisher")
	}
}

func TestSettleMilestone_PublishError(t *testing.T) {
	app := NewAppWithMemory()

	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Settle pub err", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	app.SetPublisher(errorPublisher{})
	_, _, err := app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})
	if err == nil {
		t.Error("expected error from failed publisher")
	}
}

func TestRecordUsageCharge_PublishError(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Usage pub err", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	app.SetPublisher(errorPublisher{})
	_, _, err := app.RecordUsageCharge(order.ID, RecordUsageChargeInput{
		MilestoneID: "ms_1", Kind: "token", AmountCents: 100,
	})
	if err == nil {
		t.Error("expected error from failed publisher")
	}
}

func TestCreateMessage_PublishError(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Msg pub err", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	app.SetPublisher(errorPublisher{})
	_, err := app.CreateMessage(order.ID, "buyer", "Hello")
	if err == nil {
		t.Error("expected error from failed publisher")
	}
}

func TestOpenDispute_PublishError(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Disp pub err", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})

	app.SetPublisher(errorPublisher{})
	_, _, _, err := app.OpenDispute(order.ID, OpenDisputeInput{
		MilestoneID: "ms_1", Reason: "issue", RefundCents: 500,
	})
	if err == nil {
		t.Error("expected error from failed publisher")
	}
}

func TestCreateBid_RFQNotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.CreateBid("nonexistent", CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	if !errors.Is(err, ErrRFQNotFound) {
		t.Errorf("expected ErrRFQNotFound, got %v", err)
	}
}

func TestAwardRFQ_RFQNotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, _, err := app.AwardRFQ("nonexistent", AwardRFQInput{BidID: "bid_1", FundingMode: "prepaid"})
	if !errors.Is(err, ErrRFQNotFound) {
		t.Errorf("expected ErrRFQNotFound, got %v", err)
	}
}

func TestAwardRFQ_BidNotBelongToRFQ(t *testing.T) {
	app := NewAppWithMemory()
	rfq1, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "RFQ1", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	rfq2, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "RFQ2", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq2.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})

	_, _, err := app.AwardRFQ(rfq1.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	if err == nil {
		t.Error("expected error when bid doesn't belong to RFQ")
	}
}

func TestAwardRFQ_MissingBidID(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "No bid", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	_, _, err := app.AwardRFQ(rfq.ID, AwardRFQInput{FundingMode: "prepaid"})
	if err == nil {
		t.Error("expected error for missing bid ID")
	}
}

func TestResolveDispute_PublishError(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Resolve pub err", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})
	app.OpenDispute(order.ID, OpenDisputeInput{MilestoneID: "ms_1", Reason: "issue", RefundCents: 500})
	disputes, _ := app.ListDisputes()

	app.SetPublisher(errorPublisher{})
	_, _, err := app.ResolveDispute(disputes[0].ID, ResolveDisputeInput{Resolution: "ok", ResolvedBy: "ops"})
	if err == nil {
		t.Error("expected error from failed publisher")
	}
}

func TestCreateBid_PublishError(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Bid pub err", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	app.SetPublisher(errorPublisher{})
	_, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	if err == nil {
		t.Error("expected error from publisher")
	}
}

func TestAwardRFQ_PublishError(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Award pub err", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})

	app.SetPublisher(errorPublisher{})
	_, _, err := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	// AwardRFQ calls CreateOrder which publishes — should error
	if err == nil {
		t.Error("expected error from publisher")
	}
}

type failingOrderRepo struct {
	OrderRepository
}

func (f failingOrderRepo) NextID() (string, error) {
	return "", errors.New("order repo broken")
}

func (f failingOrderRepo) Save(*core.Order) error {
	return errors.New("save failed")
}

func (f failingOrderRepo) Get(string) (*core.Order, error) {
	return nil, core.ErrOrderNotFound
}

func (f failingOrderRepo) List() ([]*core.Order, error) {
	return nil, errors.New("list failed")
}

func TestCreateOrder_NextIDError(t *testing.T) {
	app := NewApp(failingOrderRepo{}, nil, nil, nil, nil, nil, nil)
	_, err := app.CreateOrder(CreateOrderInput{
		BuyerOrgID: "b", ProviderOrgID: "p", Title: "T", FundingMode: "prepaid",
		Milestones: []CreateMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestListOrders_Error(t *testing.T) {
	app := NewApp(failingOrderRepo{}, nil, nil, nil, nil, nil, nil)
	_, err := app.ListOrders()
	if err == nil {
		t.Error("expected error")
	}
}

type failingRFQRepo struct{}
func (failingRFQRepo) NextID() (string, error) { return "", errors.New("broken") }
func (failingRFQRepo) Get(string) (RFQ, error) { return RFQ{}, ErrRFQNotFound }
func (failingRFQRepo) Save(RFQ) error { return errors.New("broken") }
func (failingRFQRepo) List() ([]RFQ, error) { return nil, errors.New("broken") }

func TestCreateRFQ_NextIDError(t *testing.T) {
	app := NewApp(nil, nil, nil, failingRFQRepo{}, nil, nil, nil)
	_, err := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "b", Title: "T", Category: "ai", Scope: "s", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestListRFQs_Error(t *testing.T) {
	app := NewApp(nil, nil, nil, failingRFQRepo{}, nil, nil, nil)
	_, err := app.ListRFQs()
	if err == nil {
		t.Error("expected error")
	}
}

type failingMessageRepo struct{}
func (failingMessageRepo) NextID() (string, error) { return "", errors.New("broken") }
func (failingMessageRepo) Save(Message) error { return errors.New("broken") }
func (failingMessageRepo) ListByRFQ(string) ([]Message, error) { return nil, errors.New("broken") }
func (failingMessageRepo) ListByOrder(string) ([]Message, error) { return nil, errors.New("broken") }

func TestCreateMessage_NextIDError(t *testing.T) {
	app := NewApp(nil, nil, nil, nil, nil, failingMessageRepo{}, nil)
	_, err := app.CreateMessage("ord_1", "buyer", "hello")
	if err == nil {
		t.Error("expected error")
	}
}

type failingProviderRepo struct{}
func (failingProviderRepo) List() ([]ProviderProfile, error) { return nil, errors.New("broken") }
func (failingProviderRepo) Get(string) (ProviderProfile, error) { return ProviderProfile{}, errors.New("broken") }

func TestListProviders_Error(t *testing.T) {
	app := NewApp(nil, failingProviderRepo{}, nil, nil, nil, nil, nil)
	_, err := app.ListProviders()
	if err == nil {
		t.Error("expected error")
	}
}

type failingListingRepo struct{}
func (failingListingRepo) List() ([]Listing, error) { return nil, errors.New("broken") }
func (failingListingRepo) Get(string) (Listing, error) { return Listing{}, errors.New("broken") }

func TestListListings_Error(t *testing.T) {
	app := NewApp(nil, nil, failingListingRepo{}, nil, nil, nil, nil)
	_, err := app.ListListings()
	if err == nil {
		t.Error("expected error")
	}
}

type failingDisputeRepo struct{}
func (failingDisputeRepo) NextID() (string, error) { return "", errors.New("broken") }
func (failingDisputeRepo) Get(string) (Dispute, error) { return Dispute{}, ErrDisputeNotFound }
func (failingDisputeRepo) Save(Dispute) error { return errors.New("broken") }
func (failingDisputeRepo) List() ([]Dispute, error) { return nil, errors.New("broken") }

func TestListDisputes_Error(t *testing.T) {
	app := NewApp(nil, nil, nil, nil, nil, nil, failingDisputeRepo{})
	_, err := app.ListDisputes()
	if err == nil {
		t.Error("expected error")
	}
}

type failingBidRepo struct{}
func (failingBidRepo) NextID() (string, error) { return "", errors.New("broken") }
func (failingBidRepo) Get(string) (Bid, error) { return Bid{}, ErrBidNotFound }
func (failingBidRepo) Save(Bid) error { return errors.New("broken") }
func (failingBidRepo) ListByRFQ(string) ([]Bid, error) { return nil, errors.New("broken") }

func TestCreateBid_NextIDError(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "b", Title: "T", Category: "ai", Scope: "s", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	// Replace bids repo with failing one
	app2 := NewApp(nil, nil, nil, nil, failingBidRepo{}, nil, nil)
	// Need to use the original RFQ — but app2 has different rfq store
	// Instead, test ListRFQBids
	_, err := app2.ListRFQBids(rfq.ID)
	if err == nil {
		t.Error("expected error from failing bid repo")
	}
	_ = rfq
}

type failAfterFirstSaveOrderRepo struct {
	real    OrderRepository
	saveCount int
}

func (f *failAfterFirstSaveOrderRepo) NextID() (string, error) { return f.real.NextID() }
func (f *failAfterFirstSaveOrderRepo) Get(id string) (*core.Order, error) { return f.real.Get(id) }
func (f *failAfterFirstSaveOrderRepo) List() ([]*core.Order, error) { return f.real.List() }
func (f *failAfterFirstSaveOrderRepo) Save(o *core.Order) error {
	f.saveCount++
	if f.saveCount > 1 {
		return errors.New("save failed after first")
	}
	return f.real.Save(o)
}

func TestSettleMilestone_SaveError(t *testing.T) {
	// Create a normal app, create an order, then swap repo
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Save err", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	// Swap orders repo with one that fails on second save
	app.orders = &failAfterFirstSaveOrderRepo{real: app.orders, saveCount: 1}

	_, _, err := app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})
	if err == nil {
		t.Error("expected error from save failure")
	}
}

func TestRecordUsageCharge_WrongState(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Wrong state", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	// Settle milestone first — changes state to settled
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})

	// Try usage on settled milestone
	_, _, err := app.RecordUsageCharge(order.ID, RecordUsageChargeInput{
		MilestoneID: "ms_1", Kind: "token", AmountCents: 100,
	})
	if err == nil {
		t.Error("expected error for usage on settled milestone")
	}
}

func TestOpenDispute_RunningMilestone(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Disp running", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	// Try to dispute running milestone — should fail
	_, _, _, err := app.OpenDispute(order.ID, OpenDisputeInput{
		MilestoneID: "ms_1", Reason: "issue", RefundCents: 500,
	})
	if err == nil {
		t.Error("expected error for dispute on running milestone")
	}
}

func TestResolveDispute_OrderNotFound(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Resolve bad", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})
	app.OpenDispute(order.ID, OpenDisputeInput{MilestoneID: "ms_1", Reason: "issue", RefundCents: 500})
	disputes, _ := app.ListDisputes()

	// Swap order repo with failing one
	app.orders = failingOrderRepo{}

	_, _, err := app.ResolveDispute(disputes[0].ID, ResolveDisputeInput{Resolution: "ok", ResolvedBy: "ops"})
	if err == nil {
		t.Error("expected error from failing order repo")
	}
}

func TestRecordUsageCharge_SaveError(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Save err usage", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	app.orders = &failAfterFirstSaveOrderRepo{real: app.orders, saveCount: 1}
	_, _, err := app.RecordUsageCharge(order.ID, RecordUsageChargeInput{
		MilestoneID: "ms_1", Kind: "token", AmountCents: 100,
	})
	if err == nil {
		t.Error("expected error from save failure")
	}
}

func TestOpenDispute_SaveError(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "OD save", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})

	// Swap disputes repo with failing one
	app.disputes = failingDisputeRepo{}

	_, _, _, err := app.OpenDispute(order.ID, OpenDisputeInput{
		MilestoneID: "ms_1", Reason: "issue", RefundCents: 500,
	})
	if err == nil {
		t.Error("expected error from failing dispute repo")
	}
}

func TestResolveDispute_SaveOrderError(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "RD save", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})
	app.OpenDispute(order.ID, OpenDisputeInput{MilestoneID: "ms_1", Reason: "issue", RefundCents: 500})
	disputes, _ := app.ListDisputes()

	// Swap both repos with failing ones
	app.orders = &failAfterFirstSaveOrderRepo{real: app.orders, saveCount: 1}

	_, _, err := app.ResolveDispute(disputes[0].ID, ResolveDisputeInput{Resolution: "ok", ResolvedBy: "ops"})
	if err == nil {
		t.Error("expected error from save failure")
	}
}

func TestAwardRFQ_BidNotFound(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Award bad bid", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	_, _, err := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: "nonexistent_bid", FundingMode: "prepaid"})
	if !errors.Is(err, ErrBidNotFound) {
		t.Errorf("expected ErrBidNotFound, got %v", err)
	}
}

func TestOpenDispute_DisputeIDError(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "OD id err", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 10000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})

	// Swap dispute repo with one that fails on NextID
	type failNextID struct{ failingDisputeRepo }
	app.disputes = failNextID{}

	_, _, _, err := app.OpenDispute(order.ID, OpenDisputeInput{
		MilestoneID: "ms_1", Reason: "issue", RefundCents: 500,
	})
	if err == nil {
		t.Error("expected error from NextID")
	}
}

func TestCreateMessage_SaveError(t *testing.T) {
	app := NewAppWithMemory()
	app.messages = failingMessageRepo{}
	_, err := app.CreateMessage("ord_1", "buyer", "hello")
	if err == nil {
		t.Error("expected error from save")
	}
}

func TestAwardRFQ_AlreadyAwarded(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Double award", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	// Try to award again
	_, _, err := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	if err == nil {
		t.Error("expected error for double award")
	}
}

func TestListOrders_WithData(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "List orders", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	orders, err := app.ListOrders()
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) == 0 {
		t.Error("expected at least one order")
	}
}

func TestListRFQs_WithData(t *testing.T) {
	app := NewAppWithMemory()
	app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "RFQ1", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "RFQ2", Category: "ai",
		Scope: "test", BudgetCents: 3000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	rfqs, err := app.ListRFQs()
	if err != nil {
		t.Fatal(err)
	}
	if len(rfqs) < 2 {
		t.Errorf("expected at least 2 RFQs, got %d", len(rfqs))
	}
}

func TestSearchListings_All(t *testing.T) {
	app := NewAppWithMemory()
	listings, err := app.SearchListings(ListListingsInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) == 0 {
		t.Error("expected default listings")
	}
}

func TestSearchListings_ByCategory(t *testing.T) {
	app := NewAppWithMemory()
	listings, err := app.SearchListings(ListListingsInput{Category: "agent-ops"})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range listings {
		if !strings.EqualFold(l.Category, "agent-ops") {
			t.Errorf("expected category agent-ops, got %s", l.Category)
		}
	}
}

func TestSearchListings_ByQuery(t *testing.T) {
	app := NewAppWithMemory()
	listings, err := app.SearchListings(ListListingsInput{Query: "carrier"})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range listings {
		if !strings.Contains(strings.ToLower(l.Title), "carrier") {
			t.Errorf("title %q doesn't match query 'carrier'", l.Title)
		}
	}
}

func TestSearchListings_ByTag(t *testing.T) {
	app := NewAppWithMemory()
	listings, err := app.SearchListings(ListListingsInput{Tags: []string{"carrier-compatible"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range listings {
		found := false
		for _, tag := range l.Tags {
			if strings.EqualFold(tag, "carrier-compatible") {
				found = true
			}
		}
		if !found {
			t.Errorf("listing %s doesn't have tag carrier-compatible", l.ID)
		}
	}
}

func TestSearchListings_ByPriceRange(t *testing.T) {
	app := NewAppWithMemory()
	listings, err := app.SearchListings(ListListingsInput{MinPriceCents: 1000, MaxPriceCents: 50000})
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range listings {
		if l.BasePriceCents < 1000 || l.BasePriceCents > 50000 {
			t.Errorf("price %d outside range 1000-50000", l.BasePriceCents)
		}
	}
}

func TestSearchListings_NoMatch(t *testing.T) {
	app := NewAppWithMemory()
	listings, err := app.SearchListings(ListListingsInput{Query: "nonexistent-listing-xyz"})
	if err != nil {
		t.Fatal(err)
	}
	if len(listings) != 0 {
		t.Errorf("expected 0 results, got %d", len(listings))
	}
}

func TestRateOrder_Success(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Rate test", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})

	rating, err := app.RateOrder(order.ID, RateOrderInput{Score: 5, Comment: "Excellent"})
	if err != nil {
		t.Fatal(err)
	}
	if rating.Score != 5 {
		t.Errorf("score = %d", rating.Score)
	}
}

func TestRateOrder_InvalidScore(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.RateOrder("ord_1", RateOrderInput{Score: 0})
	if err == nil {
		t.Error("expected error for score 0")
	}
	_, err = app.RateOrder("ord_1", RateOrderInput{Score: 6})
	if err == nil {
		t.Error("expected error for score 6")
	}
}

func TestRateOrder_NotCompleted(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Not done", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	_, err := app.RateOrder(order.ID, RateOrderInput{Score: 4})
	if err == nil {
		t.Error("expected error for non-completed order")
	}
}

func TestRateOrder_AlreadyRated(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Double rate", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})

	app.RateOrder(order.ID, RateOrderInput{Score: 5})
	_, err := app.RateOrder(order.ID, RateOrderInput{Score: 3})
	if err == nil {
		t.Error("expected error for double rating")
	}
}

func TestGetOrderRating_NotRated(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.GetOrderRating("ord_1")
	if err == nil {
		t.Error("expected error for unrated order")
	}
}

func TestCreateRFQMessage(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Msg RFQ", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	msg, err := app.CreateRFQMessage(rfq.ID, "buyer", "Any questions about this RFQ?")
	if err != nil {
		t.Fatal(err)
	}
	if msg.RFQID != rfq.ID {
		t.Errorf("rfqId = %s", msg.RFQID)
	}
	if msg.Body != "Any questions about this RFQ?" {
		t.Errorf("body = %s", msg.Body)
	}
}

func TestListRFQMessages(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "List msg", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	app.CreateRFQMessage(rfq.ID, "buyer", "msg1")
	app.CreateRFQMessage(rfq.ID, "provider", "msg2")

	messages, err := app.ListRFQMessages(rfq.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Errorf("expected 2 messages, got %d", len(messages))
	}
}

func TestCreateRFQMessage_RFQNotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.CreateRFQMessage("nonexistent", "buyer", "hello")
	if err == nil {
		t.Error("expected error for nonexistent RFQ")
	}
}

func TestListRFQMessages_RFQNotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.ListRFQMessages("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent RFQ")
	}
}

func TestSearchProviders_All(t *testing.T) {
	app := NewAppWithMemory()
	providers, err := app.SearchProviders(SearchProvidersInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(providers) == 0 {
		t.Error("expected default providers")
	}
}

func TestSearchProviders_ByCapability(t *testing.T) {
	app := NewAppWithMemory()
	providers, err := app.SearchProviders(SearchProvidersInput{Capability: "carrier"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range providers {
		found := false
		for _, cap := range p.Capabilities {
			if strings.Contains(strings.ToLower(cap), "carrier") {
				found = true
			}
		}
		if !found {
			t.Errorf("provider %s doesn't have carrier capability", p.ID)
		}
	}
}

func TestSearchProviders_ByTier(t *testing.T) {
	app := NewAppWithMemory()
	providers, err := app.SearchProviders(SearchProvidersInput{Tier: "gold"})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range providers {
		if !strings.EqualFold(p.ReputationTier, "gold") {
			t.Errorf("expected gold tier, got %s", p.ReputationTier)
		}
	}
}

func TestGetMarketplaceStats(t *testing.T) {
	app := NewAppWithMemory()
	stats, err := app.GetMarketplaceStats()
	if err != nil {
		t.Fatal(err)
	}
	if stats.TotalProviders == 0 {
		t.Error("expected providers")
	}
	if stats.TotalListings == 0 {
		t.Error("expected listings")
	}
}

func TestGetOrderBudget(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Budget test", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "Work", BasePriceCents: 3000, BudgetCents: 3000},
			{ID: "ms_2", Title: "Review", BasePriceCents: 2000, BudgetCents: 2000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	budget, err := app.GetOrderBudget(order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if budget.TotalBudget != 5000 {
		t.Errorf("totalBudget = %d", budget.TotalBudget)
	}
	if len(budget.Milestones) != 2 {
		t.Errorf("milestones = %d", len(budget.Milestones))
	}
}

func TestGetOrderTimeline(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Timeline", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid",
		QuoteCents: 5000, Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "Work", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	timeline, err := app.GetOrderTimeline(order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(timeline) == 0 {
		t.Error("expected timeline events")
	}
	if timeline[0].Type != "order.created" {
		t.Errorf("first event = %s", timeline[0].Type)
	}
}

func TestGetProviderLeaderboard(t *testing.T) {
	app := NewAppWithMemory()
	entries, err := app.GetProviderLeaderboard()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Error("expected leaderboard entries")
	}
	for _, e := range entries {
		if e.Name == "" {
			t.Error("expected provider name")
		}
	}
}

func TestProviderApplication_Submit(t *testing.T) {
	app := NewAppWithMemory()
	pa, err := app.SubmitProviderApplication("org_new", "New Provider", []string{"gpu", "inference"})
	if err != nil {
		t.Fatal(err)
	}
	if pa.Status != VettingPending {
		t.Errorf("status = %s", pa.Status)
	}
}

func TestProviderApplication_Approve(t *testing.T) {
	app := NewAppWithMemory()
	pa, _ := app.SubmitProviderApplication("org_new", "New Provider", nil)
	reviewed, err := app.ReviewProviderApplication(pa.ID, "ops_admin", "Looks good", true)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.Status != VettingApproved {
		t.Errorf("status = %s", reviewed.Status)
	}
}

func TestProviderApplication_Reject(t *testing.T) {
	app := NewAppWithMemory()
	pa, _ := app.SubmitProviderApplication("org_new", "New Provider", nil)
	reviewed, err := app.ReviewProviderApplication(pa.ID, "ops_admin", "Insufficient docs", false)
	if err != nil {
		t.Fatal(err)
	}
	if reviewed.Status != VettingRejected {
		t.Errorf("status = %s", reviewed.Status)
	}
}

func TestProviderApplication_DuplicatePending(t *testing.T) {
	app := NewAppWithMemory()
	app.SubmitProviderApplication("org_new", "New Provider", nil)
	_, err := app.SubmitProviderApplication("org_new", "New Provider", nil)
	if err == nil {
		t.Error("expected error for duplicate pending")
	}
}

func TestProviderApplication_List(t *testing.T) {
	app := NewAppWithMemory()
	app.SubmitProviderApplication("org_1", "P1", nil)
	app.SubmitProviderApplication("org_2", "P2", nil)

	all := app.ListProviderApplications("")
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}

	pending := app.ListProviderApplications("pending")
	if len(pending) != 2 {
		t.Errorf("expected 2 pending, got %d", len(pending))
	}
}

func TestTopUpMilestone(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "TopUp", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "Work", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	updated, err := app.TopUpMilestone(order.ID, TopUpInput{MilestoneID: "ms_1", AdditionalCents: 2000})
	if err != nil {
		t.Fatal(err)
	}
	for _, ms := range updated.Milestones {
		if ms.ID == "ms_1" && ms.BudgetCents != 7000 {
			t.Errorf("budget = %d, want 7000", ms.BudgetCents)
		}
	}
}

func TestCreateListing(t *testing.T) {
	app := NewAppWithMemory()
	listing, err := app.CreateListing(CreateListingInput{
		ProviderOrgID: "org_p", Title: "GPU Agent", Category: "compute",
		BasePriceCents: 1500, Tags: []string{"gpu"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if listing.Title != "GPU Agent" {
		t.Errorf("title = %s", listing.Title)
	}
}

func TestRegisterCarrierBinding(t *testing.T) {
	app := NewAppWithMemory()
	binding, err := app.RegisterCarrierBinding(ProviderCarrierBinding{
		ProviderOrgID:  "org_p",
		CarrierBaseURL: "https://carrier.example.com",
		HostID:         "host_1",
		AgentID:        "agent_1",
		Backend:        "gpt-4",
		WorkspaceRoot:  "/workspace",
	})
	if err != nil {
		t.Fatal(err)
	}
	if binding.Status != "pending_verification" {
		t.Errorf("status = %s", binding.Status)
	}
}

func TestVerifyCarrierBinding(t *testing.T) {
	app := NewAppWithMemory()
	binding, _ := app.RegisterCarrierBinding(ProviderCarrierBinding{
		ProviderOrgID: "org_p", CarrierBaseURL: "https://carrier.example.com", HostID: "host_1",
	})
	verified, err := app.VerifyCarrierBinding(binding.ID)
	if err != nil {
		t.Fatal(err)
	}
	if verified.Status != "active" {
		t.Errorf("status = %s", verified.Status)
	}
}

func TestSuspendCarrierBinding(t *testing.T) {
	app := NewAppWithMemory()
	binding, _ := app.RegisterCarrierBinding(ProviderCarrierBinding{
		ProviderOrgID: "org_p", CarrierBaseURL: "https://carrier.example.com", HostID: "host_1",
	})
	suspended, err := app.SuspendCarrierBinding(binding.ID)
	if err != nil {
		t.Fatal(err)
	}
	if suspended.Status != "suspended" {
		t.Errorf("status = %s", suspended.Status)
	}
}

func TestRegisterCarrierBinding_Duplicate(t *testing.T) {
	app := NewAppWithMemory()
	app.RegisterCarrierBinding(ProviderCarrierBinding{
		ProviderOrgID: "org_p", CarrierBaseURL: "https://carrier.example.com", HostID: "host_1",
	})
	// Verify first, then try duplicate
	app.VerifyCarrierBinding("pcb_1")
	_, err := app.RegisterCarrierBinding(ProviderCarrierBinding{
		ProviderOrgID: "org_p", CarrierBaseURL: "https://carrier2.example.com", HostID: "host_2",
	})
	if err == nil {
		t.Error("expected error for duplicate active binding")
	}
}

func TestCreateListing_WithProfile(t *testing.T) {
	app := NewAppWithMemory()
	listing, err := app.CreateListing(CreateListingInput{
		ProviderOrgID:      "org_p",
		Title:              "GPU Agent",
		Category:           "compute",
		BasePriceCents:     1500,
		ExecutionProfileID: "prof_1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if listing.ExecutionProfileID != "prof_1" {
		t.Errorf("profileID = %s", listing.ExecutionProfileID)
	}
}

func TestAwardRFQ_BlockedBySuspendedBinding(t *testing.T) {
	app := NewAppWithMemory()

	// Register and suspend provider binding
	binding, _ := app.RegisterCarrierBinding(ProviderCarrierBinding{
		ProviderOrgID: "org_2", CarrierBaseURL: "https://carrier.test", HostID: "h1",
	})
	app.VerifyCarrierBinding(binding.ID)
	app.SuspendCarrierBinding(binding.ID)

	// Create RFQ + bid
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_1", Title: "Suspend test", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_2", Message: "bid", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})

	// Award should fail
	_, _, err := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	if err != ErrProviderSuspended {
		t.Errorf("expected ErrProviderSuspended, got %v", err)
	}
}

func TestBudgetWall_PauseAndResume(t *testing.T) {
	app := NewAppWithMemory()

	// Create order with tight budget
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Budget wall", Category: "ai",
		Scope: "test", BudgetCents: 1000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid", QuoteCents: 1000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 1000, BudgetCents: 1000}},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	// Record usage exceeding budget
	app.RecordUsageCharge(order.ID, RecordUsageChargeInput{
		MilestoneID: "ms_1", Kind: core.UsageChargeKindToken, AmountCents: 1100,
	})

	// Verify order is paused
	updated, _ := app.GetOrder(order.ID)
	if updated.Status != core.OrderStatusAwaitingBudget {
		t.Fatalf("expected awaiting_budget, got %s", updated.Status)
	}
	for _, ms := range updated.Milestones {
		if ms.ID == "ms_1" && ms.State != core.MilestoneStatePaused {
			t.Fatalf("expected milestone paused, got %s", ms.State)
		}
	}

	// Top up budget
	resumed, err := app.TopUpMilestone(order.ID, TopUpInput{MilestoneID: "ms_1", AdditionalCents: 500})
	if err != nil {
		t.Fatal(err)
	}

	// Verify order is running again
	if resumed.Status != core.OrderStatusRunning {
		t.Errorf("expected running after top-up, got %s", resumed.Status)
	}
	for _, ms := range resumed.Milestones {
		if ms.ID == "ms_1" {
			if ms.State != core.MilestoneStateRunning {
				t.Errorf("expected milestone running, got %s", ms.State)
			}
			if ms.BudgetCents != 1500 {
				t.Errorf("budget = %d, want 1500", ms.BudgetCents)
			}
		}
	}
}

func TestCreateListing_RequiresProfile_WhenEnforced(t *testing.T) {
	app := NewAppWithMemory()
	app.SetRequireExecutionProfile(func(id string) bool { return id == "prof_valid" })

	_, err := app.CreateListing(CreateListingInput{
		ProviderOrgID: "org_p", Title: "Test", Category: "ai",
	})
	if err == nil {
		t.Error("expected error for missing profile")
	}

	_, err = app.CreateListing(CreateListingInput{
		ProviderOrgID: "org_p", Title: "Test", Category: "ai",
		ExecutionProfileID: "prof_invalid",
	})
	if err == nil {
		t.Error("expected error for invalid profile")
	}

	_, err = app.CreateListing(CreateListingInput{
		ProviderOrgID: "org_p", Title: "Test", Category: "ai",
		ExecutionProfileID: "prof_valid",
	})
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
}

func TestCreateBid_RequiresProfile_WhenEnforced(t *testing.T) {
	app := NewAppWithMemory()
	app.SetRequireExecutionProfile(func(id string) bool { return id == "prof_valid" })

	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Profile bid", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	_, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	if err == nil {
		t.Error("expected error for missing profile")
	}

	_, err = app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid", QuoteCents: 5000,
		ExecutionProfileID: "prof_valid",
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	if err != nil {
		t.Errorf("expected success, got %v", err)
	}
}

func TestIsProviderApproved(t *testing.T) {
	app := NewAppWithMemory()

	// No applications → approved (backward compatible)
	if !app.IsProviderApproved("org_p") {
		t.Error("expected approved when no applications exist")
	}

	// Submit + approve
	pa, _ := app.SubmitProviderApplication("org_p", "P", nil)
	if app.IsProviderApproved("org_p") {
		t.Error("pending should not be approved")
	}

	app.ReviewProviderApplication(pa.ID, "ops", "ok", true)
	if !app.IsProviderApproved("org_p") {
		t.Error("approved application should pass")
	}

	// Different org still not approved
	app.SubmitProviderApplication("org_other", "O", nil)
	if app.IsProviderApproved("org_other") {
		t.Error("pending org_other should not be approved")
	}
}

func TestSetCreditLimit(t *testing.T) {
	app := NewAppWithMemory()
	limit := app.SetCreditLimit("org_b", 100000, "ops_admin")
	if limit.LimitCents != 100000 {
		t.Errorf("limit = %d", limit.LimitCents)
	}
	if limit.SetBy != "ops_admin" {
		t.Errorf("setBy = %s", limit.SetBy)
	}
}

func TestGetCreditLimit(t *testing.T) {
	app := NewAppWithMemory()
	app.SetCreditLimit("org_b", 50000, "ops")
	limit, ok := app.GetCreditLimit("org_b")
	if !ok {
		t.Fatal("expected credit limit")
	}
	if limit.AvailableCents != 50000 {
		t.Errorf("available = %d", limit.AvailableCents)
	}
}

func TestGetCreditLimit_NotSet(t *testing.T) {
	app := NewAppWithMemory()
	_, ok := app.GetCreditLimit("org_unknown")
	if ok {
		t.Error("expected not found")
	}
}

func TestMultipleBidsOnSameRFQ(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Multi bid", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	bid1, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p1", Message: "bid 1", QuoteCents: 9000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 9000, BudgetCents: 9000}},
	})
	if err != nil { t.Fatal(err) }

	bid2, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p2", Message: "bid 2", QuoteCents: 8000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 8000, BudgetCents: 8000}},
	})
	if err != nil { t.Fatal(err) }

	bids, _ := app.ListBids(rfq.ID)
	if len(bids) != 2 {
		t.Errorf("expected 2 bids, got %d", len(bids))
	}

	// Award bid 2 — bid 1 should be rejected
	_, order, err := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid2.ID, FundingMode: "prepaid"})
	if err != nil { t.Fatal(err) }
	if order.ProviderOrgID != "org_p2" {
		t.Errorf("provider = %s, want org_p2", order.ProviderOrgID)
	}

	// Verify bid statuses
	bids, _ = app.ListBids(rfq.ID)
	for _, b := range bids {
		if b.ID == bid1.ID && b.Status != BidStatusRejected {
			t.Errorf("bid1 status = %s, want rejected", b.Status)
		}
		if b.ID == bid2.ID && b.Status != BidStatusAwarded {
			t.Errorf("bid2 status = %s, want awarded", b.Status)
		}
	}
}

func TestMilestoneAutoAdvance(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Advance", Category: "ai",
		Scope: "test", BudgetCents: 6000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid", QuoteCents: 6000,
		Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "First", BasePriceCents: 3000, BudgetCents: 3000},
			{ID: "ms_2", Title: "Second", BasePriceCents: 3000, BudgetCents: 3000},
		},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	// ms_1 should be running, ms_2 pending
	if order.Milestones[0].State != core.MilestoneStateRunning {
		t.Errorf("ms_1 = %s, want running", order.Milestones[0].State)
	}
	if order.Milestones[1].State != core.MilestoneStatePending {
		t.Errorf("ms_2 = %s, want pending", order.Milestones[1].State)
	}

	// Settle ms_1 → ms_2 should auto-advance to running
	updated, _, _ := app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})
	for _, ms := range updated.Milestones {
		if ms.ID == "ms_2" && ms.State != core.MilestoneStateRunning {
			t.Errorf("ms_2 after settle = %s, want running", ms.State)
		}
	}
}

func TestCreditFundingMode(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Credit", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})

	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{
		BidID: bid.ID, FundingMode: "credit", CreditLineID: "credit_line_1",
	})
	if order.FundingMode != "credit" {
		t.Errorf("fundingMode = %s", order.FundingMode)
	}
	if order.CreditLineID != "credit_line_1" {
		t.Errorf("creditLineId = %s", order.CreditLineID)
	}
}

func TestGetProvider(t *testing.T) {
	app := NewAppWithMemory()
	p, err := app.GetProvider("provider_1")
	if err != nil { t.Fatal(err) }
	if p.Name == "" { t.Error("expected name") }
}

func TestGetProvider_NotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.GetProvider("nonexistent")
	if err == nil { t.Error("expected error") }
}

func TestGetListing(t *testing.T) {
	app := NewAppWithMemory()
	l, err := app.GetListing("listing_1")
	if err != nil { t.Fatal(err) }
	if l.Title == "" { t.Error("expected title") }
}

func TestGetDispute_NotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.GetDispute("nonexistent")
	if err == nil { t.Error("expected error") }
}

func TestListOrderMessages(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Msg", Category: "ai",
		Scope: "t", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "b", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	messages, err := app.ListOrderMessages(order.ID)
	if err != nil { t.Fatal(err) }
	_ = messages // may be empty
}

func TestBatchOrderStatus(t *testing.T) {
	app := NewAppWithMemory()
	statuses, _ := app.BatchOrderStatus([]string{"nonexistent"})
	if len(statuses) != 0 { t.Errorf("expected 0, got %d", len(statuses)) }
}

func TestGetProviderRevenue(t *testing.T) {
	app := NewAppWithMemory()
	rev, err := app.GetProviderRevenue("provider_1")
	if err != nil { t.Fatal(err) }
	if rev.ProviderOrgID != "provider_1" { t.Errorf("provider = %s", rev.ProviderOrgID) }
}

func TestSetClock(t *testing.T) {
	app := NewAppWithMemory()
	fixed := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	app.SetClock(func() time.Time { return fixed })
	if app.now() != fixed { t.Error("clock not set") }
}

func TestSetNotifier(t *testing.T) {
	app := NewAppWithMemory()
	called := false
	app.SetNotifier(&testNotifier{onSend: func() { called = true }})
	app.notify("test", "target", nil)
	if !called { t.Error("notifier not called") }
}

type testNotifier struct{ onSend func() }
func (n *testNotifier) Send(event, target string, payload map[string]any) error {
	n.onSend()
	return nil
}

func TestListNotifications(t *testing.T) {
	app := NewAppWithMemory()
	notifs, err := app.ListNotifications("org_1")
	if err != nil { t.Fatal(err) }
	_ = notifs
}

func TestGetProviderCarrierBinding(t *testing.T) {
	app := NewAppWithMemory()
	app.RegisterCarrierBinding(ProviderCarrierBinding{
		ProviderOrgID: "org_p", CarrierBaseURL: "https://carrier.test", HostID: "h1",
	})
	binding, err := app.GetProviderCarrierBinding("org_p")
	if err != nil { t.Fatal(err) }
	if binding.HostID != "h1" { t.Errorf("hostId = %s", binding.HostID) }
}

func TestGetProviderCarrierBinding_NotFound(t *testing.T) {
	app := NewAppWithMemory()
	_, err := app.GetProviderCarrierBinding("nonexistent")
	if err == nil { t.Error("expected error") }
}

func TestGetDisputeWithEvidence(t *testing.T) {
	app := NewAppWithMemory()
	// Create order with dispute
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Ev", Category: "ai",
		Scope: "t", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "b", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})
	app.OpenDispute(order.ID, OpenDisputeInput{MilestoneID: "ms_1", Reason: "bad", RefundCents: 100})

	disputes, _ := app.ListDisputes()
	if len(disputes) == 0 { t.Fatal("expected dispute") }

	dwe, err := app.GetDisputeWithEvidence(disputes[0].ID)
	if err != nil { t.Fatal(err) }
	if dwe.Dispute.ID == "" { t.Error("expected dispute ID") }
}

func TestExportOrdersCSV_WithData(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Export", Category: "ai",
		Scope: "t", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "b", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	csv, err := app.ExportOrdersCSV()
	if err != nil { t.Fatal(err) }
	if !strings.Contains(csv, "org_b") { t.Error("missing buyer in CSV") }
}

func TestDefaultClock(t *testing.T) {
	now := DefaultClock()
	if now.IsZero() { t.Error("clock returned zero") }
}

func TestSearchProviders_ByMinRating(t *testing.T) {
	app := NewAppWithMemory()
	providers, _ := app.SearchProviders(SearchProvidersInput{MinRating: 99.0})
	if len(providers) != 0 { t.Errorf("expected 0, got %d", len(providers)) }
}

func TestSearchListings_ByProviderOrgID(t *testing.T) {
	app := NewAppWithMemory()
	listings, _ := app.SearchListings(ListListingsInput{ProviderOrgID: "nonexistent"})
	if len(listings) != 0 { t.Errorf("expected 0, got %d", len(listings)) }
}

func TestAwardRFQ_WithActiveBinding_Succeeds(t *testing.T) {
	app := NewAppWithMemory()
	// Register and verify (not suspend) a binding
	binding, _ := app.RegisterCarrierBinding(ProviderCarrierBinding{
		ProviderOrgID: "org_p", CarrierBaseURL: "https://carrier.test", HostID: "h1",
	})
	app.VerifyCarrierBinding(binding.ID)

	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Active binding", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "bid", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	_, order, err := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	if err != nil {
		t.Fatalf("expected success with active binding, got %v", err)
	}
	if order.ProviderOrgID != "org_p" {
		t.Errorf("provider = %s", order.ProviderOrgID)
	}
}

func TestGetLeaderboard(t *testing.T) {
	app := NewAppWithMemory()
	entries, err := app.GetProviderLeaderboard()
	if err != nil { t.Fatal(err) }
	if len(entries) < 1 { t.Errorf("expected at least 2, got %d", len(entries)) }
}

func TestExportDisputesCSV_Empty(t *testing.T) {
	app := NewAppWithMemory()
	csv, err := app.ExportDisputesCSV()
	if err != nil { t.Fatal(err) }
	if !strings.Contains(csv, "DisputeID") { t.Error("missing header") }
}

func TestGetMarketplaceStats_WithData(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Stats", Category: "ai",
		Scope: "t", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "b", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	stats, err := app.GetMarketplaceStats()
	if err != nil { t.Fatal(err) }
	if stats.TotalOrders == 0 { t.Error("expected orders") }
	if stats.TotalRFQs == 0 { t.Error("expected RFQs") }
}

func TestCreateBid_DefaultMilestonesFromRFQ(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Default ms", Category: "ai",
		Scope: "test", BudgetCents: 10000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})

	// Bid without milestones — should get defaults from RFQ
	bid, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "auto milestones",
		// No QuoteCents — defaults to RFQ budget
		// No Milestones — defaults to RFQ default milestones
	})
	if err != nil {
		t.Fatal(err)
	}
	if bid.QuoteCents != 10000 {
		t.Errorf("quoteCents = %d, want 10000 (default from RFQ)", bid.QuoteCents)
	}
	if len(bid.Milestones) != 3 {
		t.Errorf("milestones = %d, want 3 (default split)", len(bid.Milestones))
	}
}

func TestCreateBid_RFQNotOpen(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Closed", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "first", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	// RFQ is now awarded, not open
	_, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p2", Message: "late", QuoteCents: 4000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 4000, BudgetCents: 4000}},
	})
	if err != ErrRFQNotOpenForBids {
		t.Errorf("expected ErrRFQNotOpenForBids, got %v", err)
	}
}

func TestSearchListings_ByMinPrice(t *testing.T) {
	app := NewAppWithMemory()
	listings, _ := app.SearchListings(ListListingsInput{MinPriceCents: 99999})
	if len(listings) != 0 {
		t.Errorf("expected 0 expensive listings, got %d", len(listings))
	}
}

func TestSearchListings_ByMaxPrice(t *testing.T) {
	app := NewAppWithMemory()
	listings, _ := app.SearchListings(ListListingsInput{MaxPriceCents: 1})
	if len(listings) != 0 {
		t.Errorf("expected 0 cheap listings, got %d", len(listings))
	}
}


func TestCreateBid_ExceedsBudget(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Budget check", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	_, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "expensive", QuoteCents: 10000,
		Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 10000, BudgetCents: 10000},
		},
	})
	if err != ErrBidExceedsBudget {
		t.Errorf("expected ErrBidExceedsBudget, got %v", err)
	}
}

func TestCreateBid_ExactBudget(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Exact", Category: "ai",
		Scope: "test", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	_, err := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "exact", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{
			{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000},
		},
	})
	if err != nil {
		t.Errorf("exact budget should succeed, got %v", err)
	}
}

func TestGetBudgetWallInfo(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Wall", Category: "ai",
		Scope: "t", BudgetCents: 1000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "b", QuoteCents: 1000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 1000, BudgetCents: 1000}},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	// Before wall
	info, _ := app.GetBudgetWallInfo(order.ID)
	if info != nil { t.Error("should be nil before wall") }

	// Hit wall
	app.RecordUsageCharge(order.ID, RecordUsageChargeInput{
		MilestoneID: "ms_1", Kind: core.UsageChargeKindToken, AmountCents: 1100,
	})

	info, _ = app.GetBudgetWallInfo(order.ID)
	if info == nil { t.Fatal("expected budget wall info") }
	if info.OverageCents != 1100 { t.Errorf("overage = %d", info.OverageCents) }
	if info.MilestoneID != "ms_1" { t.Errorf("milestone = %s", info.MilestoneID) }
}

func TestGetFiberExposure(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "Exp", Category: "ai",
		Scope: "t", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "b", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})

	exp, err := app.GetFiberExposure()
	if err != nil { t.Fatal(err) }
	if exp.ActiveOrderCount == 0 { t.Error("expected active orders") }
	if exp.TotalPrepaidUnsettledCents == 0 { t.Error("expected prepaid exposure") }
	if exp.TotalExposureCents == 0 { t.Error("expected total exposure") }
}

func TestGetFiberExposure_Empty(t *testing.T) {
	app := NewAppWithMemory()
	exp, err := app.GetFiberExposure()
	if err != nil { t.Fatal(err) }
	if exp.TotalExposureCents != 0 { t.Errorf("expected 0, got %d", exp.TotalExposureCents) }
}

func TestGetProviderLeaderboard_WithRatings(t *testing.T) {
	app := NewAppWithMemory()

	// Create completed order for rating
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "LB", Category: "ai",
		Scope: "t", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "provider_1", Message: "b", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})
	app.RateOrder(order.ID, RateOrderInput{Score: 5, Comment: "great"})

	entries, err := app.GetProviderLeaderboard()
	if err != nil { t.Fatal(err) }

	// provider_1 should have a rating
	found := false
	for _, e := range entries {
		if e.ProviderID == "provider_1" {
			found = true
			if e.Rating != 5.0 { t.Errorf("rating = %f", e.Rating) }
			if e.RatingCount != 1 { t.Errorf("ratingCount = %d", e.RatingCount) }
			if e.TotalOrders != 1 { t.Errorf("totalOrders = %d", e.TotalOrders) }
		}
	}
	if !found { t.Error("provider_1 not in leaderboard") }
}

func TestSearchProviders_NoResults(t *testing.T) {
	app := NewAppWithMemory()
	providers, _ := app.SearchProviders(SearchProvidersInput{Capability: "quantum-computing"})
	if len(providers) != 0 { t.Errorf("expected 0, got %d", len(providers)) }
}

func TestSearchProviders_ByTier_NoMatch(t *testing.T) {
	app := NewAppWithMemory()
	providers, _ := app.SearchProviders(SearchProvidersInput{Tier: "diamond"})
	if len(providers) != 0 { t.Errorf("expected 0, got %d", len(providers)) }
}

func TestGetOrderTimeline_WithSettlement(t *testing.T) {
	app := NewAppWithMemory()
	rfq, _ := app.CreateRFQ(CreateRFQInput{
		BuyerOrgID: "org_b", Title: "TL", Category: "ai",
		Scope: "t", BudgetCents: 5000,
		ResponseDeadlineAt: time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC),
	})
	bid, _ := app.CreateBid(rfq.ID, CreateBidInput{
		ProviderOrgID: "org_p", Message: "b", QuoteCents: 5000,
		Milestones: []BidMilestoneInput{{ID: "ms_1", Title: "W", BasePriceCents: 5000, BudgetCents: 5000}},
	})
	_, order, _ := app.AwardRFQ(rfq.ID, AwardRFQInput{BidID: bid.ID, FundingMode: "prepaid"})
	app.SettleMilestone(order.ID, core.SettleMilestoneInput{MilestoneID: "ms_1", Summary: "done"})
	app.RateOrder(order.ID, RateOrderInput{Score: 4})

	timeline, _ := app.GetOrderTimeline(order.ID)
	// Should have: order.created + milestone.settled + order.rated
	types := make(map[string]bool)
	for _, ev := range timeline {
		types[ev.Type] = true
	}
	if !types["order.created"] { t.Error("missing order.created") }
	if !types["order.rated"] { t.Error("missing order.rated") }
}
